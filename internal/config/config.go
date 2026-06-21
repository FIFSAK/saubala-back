package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

// Configs is the root configuration aggregate for the application.
type Configs struct {
	APP        AppConfig
	HTTP       HTTPConfig
	Mongo      MongoConfig
	JWT        JWTConfig
	SuperAdmin SuperAdminConfig
}

type AppConfig struct {
	Mode    string        `default:"dev"`
	Port    string        `default:":80"`
	Path    string        `default:"/api/v1"`
	Timeout time.Duration `default:"60s"`
}

type HTTPConfig struct {
	Port string `default:":8080"`
}

type MongoConfig struct {
	URI string `envconfig:"URI" default:"mongodb://localhost:27017"`
	DB  string `envconfig:"DB" default:"saubala"`
}

type JWTConfig struct {
	Secret    string        `envconfig:"SECRET" default:"change-me"`
	AccessTTL time.Duration `envconfig:"ACCESS_TTL" default:"24h"`
}

type SuperAdminConfig struct {
	Email    string `envconfig:"EMAIL" default:"admin@saubala.kz"`
	Password string `envconfig:"PASSWORD" default:"change-me"`
}

func New() (*Configs, error) {
	cfg := &Configs{}

	root, err := os.Getwd()
	if err != nil {
		logStructured("error", "get_workdir", map[string]interface{}{"error": err.Error()})
		return cfg, fmt.Errorf("unable to get working directory: %w", err)
	}

	envPath := filepath.Join(root, ".env")
	if _, statErr := os.Stat(envPath); statErr == nil {
		if loadErr := godotenv.Load(envPath); loadErr != nil {
			logStructured("error", "load_env", map[string]interface{}{"file": envPath, "error": loadErr.Error()})
			return cfg, fmt.Errorf("failed to load env file %s: %w", envPath, loadErr)
		}
		logStructured("info", "load_env", map[string]interface{}{"file": envPath})
	} else if !os.IsNotExist(statErr) {
		logStructured("error", "stat_env_file", map[string]interface{}{"file": envPath, "error": statErr.Error()})
		return cfg, fmt.Errorf("failed to stat env file %s: %w", envPath, statErr)
	} else {
		logStructured("info", "env_file_missing", map[string]interface{}{"file": envPath})
	}

	targets := map[string]interface{}{
		"APP":        &cfg.APP,
		"HTTP":       &cfg.HTTP,
		"MONGO":      &cfg.Mongo,
		"JWT":        &cfg.JWT,
		"SUPERADMIN": &cfg.SuperAdmin,
	}

	for p, target := range targets {
		if procErr := envconfig.Process(p, target); procErr != nil {
			logStructured("error", "env_process", map[string]interface{}{"prefix": p, "error": procErr.Error()})
			return cfg, fmt.Errorf("failed to process env for %s: %w", p, procErr)
		}
	}

	return cfg, nil
}

func logStructured(level string, action string, params map[string]interface{}) {
	msg := fmt.Sprintf("level=%s component=config action=%s", level, action)
	for k, v := range params {
		msg = fmt.Sprintf("%s %s=%v", msg, k, v)
	}
	log.Println(msg)
}
