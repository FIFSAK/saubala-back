package app

import (
	"context"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/FIFSAK/saubala-back/internal/config"
	"github.com/FIFSAK/saubala-back/internal/handler"
	"github.com/FIFSAK/saubala-back/internal/repository"
	"github.com/FIFSAK/saubala-back/internal/service"
	"github.com/FIFSAK/saubala-back/pkg/auth"
	"github.com/FIFSAK/saubala-back/pkg/server"
	"github.com/FIFSAK/saubala-back/pkg/store"
)

const startupTimeout = 30 * time.Second

func initApp(logger *zap.Logger) (*App, error) {
	app := &App{logger: logger}

	ctx, cancel := context.WithTimeout(context.Background(), startupTimeout)
	defer cancel()

	if err := app.loadConfiguration(); err != nil {
		return nil, err
	}

	app.tokenManager = auth.NewTokenManager(app.configs.JWT.Secret, app.configs.JWT.AccessTTL)

	if err := app.initializeRepositories(ctx); err != nil {
		return nil, err
	}

	if err := app.initializeServices(ctx); err != nil {
		app.cleanup(ctx)
		return nil, err
	}

	if err := app.initializeServers(); err != nil {
		app.cleanup(ctx)
		return nil, err
	}

	if err := app.initializeHandlers(); err != nil {
		app.cleanup(ctx)
		return nil, err
	}

	return app, nil
}

func (app *App) loadConfiguration() error {
	configs, err := config.New()
	if err != nil {
		app.logger.Error("config load error", zap.Error(err))
		return err
	}

	app.configs = configs
	app.logger.Info("configuration loaded",
		zap.String("mode", configs.APP.Mode),
		zap.String("http_port", configs.HTTP.Port),
		zap.String("mongo_db", configs.Mongo.DB),
	)
	return nil
}

func (app *App) initializeRepositories(ctx context.Context) error {
	app.logger.Info("connecting to mongodb", zap.String("uri", app.configs.Mongo.URI))

	mongo, err := store.NewMongo(ctx, app.configs.Mongo.URI, app.configs.Mongo.DB)
	if err != nil {
		app.logger.Error("mongo connect error", zap.Error(err))
		return err
	}
	app.mongo = mongo

	repositories, err := repository.New(
		repository.WithMongoStore(ctx, mongo),
	)
	if err != nil {
		app.logger.Error("repository init error", zap.Error(err))
		return err
	}

	app.repositories = repositories
	app.logger.Info("repositories initialized")
	return nil
}

func (app *App) initializeServices(ctx context.Context) error {
	services, err := service.New(
		service.Dependencies{
			Repositories: app.repositories,
			TokenManager: app.tokenManager,
		},
		service.WithAuthService(),
		service.WithUserService(),
		service.WithBrandService(),
		service.WithPositionService(),
		service.WithReceiptService(),
		service.WithContractService(),
		service.WithReleaseService(),
		service.WithSettingsService(),
		service.WithOrgService(),
		service.WithSupplierService(),
		service.WithInvoiceService(),
	)
	if err != nil {
		app.logger.Error("service init error", zap.Error(err))
		return err
	}
	app.services = services

	// Seed the super administrator account if it does not exist yet.
	if err := services.User.EnsureSuperAdmin(ctx, app.configs.SuperAdmin.Email, app.configs.SuperAdmin.Password); err != nil {
		app.logger.Error("super admin seed error", zap.Error(err))
		return err
	}

	// Seed the settings singleton (invoice defaults) with the customer's values.
	if err := services.Settings.EnsureDefault(ctx); err != nil {
		app.logger.Error("settings seed error", zap.Error(err))
		return err
	}

	// Seed the customer's firm as the first sender organization.
	if err := services.Org.EnsureDefault(ctx); err != nil {
		app.logger.Error("organization seed error", zap.Error(err))
		return err
	}

	app.logger.Info("services initialized")
	return nil
}

func (app *App) initializeServers() error {
	var origins []string
	for _, o := range strings.Split(app.configs.CORS.AllowedOrigins, ",") {
		if t := strings.TrimSpace(o); t != "" {
			origins = append(origins, t)
		}
	}

	servers, err := server.NewServer(
		server.WithCORS(origins),
		server.WithHTTP(app.configs.HTTP.Port),
	)
	if err != nil {
		app.logger.Error("server init error", zap.Error(err))
		return err
	}

	app.servers = servers
	app.logger.Info("servers initialized")
	return nil
}

func (app *App) initializeHandlers() error {
	handlers, err := handler.New(
		handler.Dependencies{
			Services:     app.services,
			Repositories: app.repositories,
			TokenManager: app.tokenManager,
		},
		handler.WithAuthHandler(),
		handler.WithUserHandler(),
		handler.WithBrandHandler(),
		handler.WithPositionHandler(),
		handler.WithReceiptHandler(),
		handler.WithContractHandler(),
		handler.WithReleaseHandler(),
		handler.WithSettingsHandler(),
		handler.WithOrgHandler(),
		handler.WithSupplierHandler(),
		handler.WithInvoiceHandler(),
	)
	if err != nil {
		app.logger.Error("handler init error", zap.Error(err))
		return err
	}

	handlers.RegisterHTTP(app.servers.Router())

	app.handlers = handlers
	app.logger.Info("handlers initialized")
	return nil
}

func (app *App) cleanup(ctx context.Context) {
	if app.repositories != nil {
		if err := app.repositories.Close(ctx); err != nil {
			app.logger.Error("repositories cleanup error", zap.Error(err))
		} else {
			app.logger.Info("repositories cleanup complete")
		}
	}
}
