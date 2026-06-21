package app

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/FIFSAK/saubala-back/internal/config"

	"go.uber.org/zap"

	"github.com/FIFSAK/saubala-back/internal/handler"
	"github.com/FIFSAK/saubala-back/internal/repository"
	"github.com/FIFSAK/saubala-back/internal/service"
	"github.com/FIFSAK/saubala-back/pkg/log"
	"github.com/FIFSAK/saubala-back/pkg/server"
)

type App struct {
	logger         *zap.Logger
	configs        *config.Configs
	repositories   *repository.Repositories
	services       *service.Services
	servers        *server.Servers
	handlers       *handler.Handlers
	tracerShutdown func(context.Context) error
}

func Run() {
	logger := log.GetLogger()
	app, err := initApp(logger)
	if err != nil {
		logger.Error("app init error", zap.Error(err))
		return
	}

	if err := app.startServers(); err != nil {
		app.logger.Error("server start error", zap.Error(err))
		app.shutdown()
		return
	}

	app.logStartupInfo()

	wait := parseGracefulTimeout()
	app.waitForShutdown(wait)
}

func (app *App) startServers() error {
	if err := app.servers.Run(app.logger); err != nil {
		app.logger.Error("server startup failed", zap.Error(err))
		return err
	}

	app.logger.Info("http server started",
		zap.String("address", fmt.Sprintf("http://localhost%s", app.configs.HTTP.Port)),
		zap.String("port", app.configs.HTTP.Port),
		zap.String("mode", app.configs.APP.Mode),
	)
	return nil
}

func (app *App) logStartupInfo() {
	app.logger.Info("application started",
		zap.String("time", time.Now().Format("02.01.2006 15:04:05")),
		zap.String("mode", app.configs.APP.Mode),
	)
}

func (app *App) waitForShutdown(timeout time.Duration) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	sig := <-quit
	app.logger.Info("shutdown signal received",
		zap.String("signal", sig.String()),
		zap.Duration("timeout", timeout),
	)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := app.servers.Stop(ctx); err != nil {
		app.logger.Error("server shutdown error", zap.Error(err))
	} else {
		app.logger.Info("server stopped gracefully")
	}

	app.shutdown()
}

func (app *App) shutdown() {
	app.logger.Info("running cleanup tasks")

	if app.tracerShutdown != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := app.tracerShutdown(ctx); err != nil {
			app.logger.Error("tracer shutdown error", zap.Error(err))
		} else {
			app.logger.Info("tracer stopped")
		}
	}

	if app.repositories != nil {
		app.repositories.Close()
		app.logger.Info("repositories closed")
	}

	if app.logger != nil {
		_ = app.logger.Sync()
	}

	app.logger.Info("application shutdown complete")
}

func parseGracefulTimeout() time.Duration {
	var wait time.Duration
	flag.DurationVar(&wait, "graceful-timeout", 15*time.Second,
		"duration for which the server waits for existing connections to finish")
	flag.Parse()
	return wait
}
