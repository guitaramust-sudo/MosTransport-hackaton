package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds every runtime setting, sourced from environment variables.
type Config struct {
	ServerPort string
	AppEnv     string

	DatabaseURL string

	JWTSecret     string
	JWTAccessTTL  time.Duration
	JWTRefreshTTL time.Duration

	LLMMode string // gigachat | mock

	GigaChatClientID     string
	GigaChatClientSecret string
	GigaChatAuthURL      string
	GigaChatAPIURL       string
	GigaChatModel        string
	GigaChatInsecure     bool

	SituationsPerSession int
}

func Load() *Config {
	return &Config{
		ServerPort: env("SERVER_PORT", "8080"),
		AppEnv:     env("APP_ENV", "development"),

		DatabaseURL: env("DATABASE_URL", "postgres://vsm:vsm@localhost:5432/vsm?sslmode=disable"),

		JWTSecret:     env("JWT_SECRET", "dev-secret-change-me"),
		JWTAccessTTL:  envDuration("JWT_ACCESS_TTL", 24*time.Hour),
		JWTRefreshTTL: envDuration("JWT_REFRESH_TTL", 30*24*time.Hour),

		LLMMode: env("LLM_MODE", "mock"),

		GigaChatClientID:     os.Getenv("GIGACHAT_CLIENT_ID"),
		GigaChatClientSecret: os.Getenv("GIGACHAT_CLIENT_SECRET"),
		GigaChatAuthURL:      env("GIGACHAT_AUTH_URL", "https://ngw.devices.sberbank.ru:9443/api/v2/oauth"),
		GigaChatAPIURL:       env("GIGACHAT_API_URL", "https://gigachat.devices.sberbank.ru/api/v1"),
		GigaChatModel:        env("GIGACHAT_MODEL", "GigaChat-2"),
		GigaChatInsecure:     envBool("GIGACHAT_INSECURE", false),

		SituationsPerSession: envInt("SITUATIONS_PER_SESSION", 4),
	}
}

func (c *Config) Validate() error {
	if c.AppEnv != "development" && c.AppEnv != "production" {
		return fmt.Errorf("APP_ENV must be development or production")
	}
	if c.AppEnv == "production" {
		if c.JWTSecret == "" || c.JWTSecret == "dev-secret-change-me" {
			return fmt.Errorf("JWT_SECRET must be set to a unique value in production")
		}
		if c.GigaChatInsecure {
			return fmt.Errorf("GIGACHAT_INSECURE is forbidden in production")
		}
	}
	return nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func envDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}
