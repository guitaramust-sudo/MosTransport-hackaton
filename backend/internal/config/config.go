package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds every runtime setting, sourced from environment variables.
type Config struct {
	ServerPort string

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
	SituationTimeout     time.Duration
	MaxTurnsPerSituation int
}

func Load() *Config {
	return &Config{
		ServerPort: env("SERVER_PORT", "8080"),

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
		SituationTimeout:     envDuration("SITUATION_TIMEOUT", 120*time.Second),
		MaxTurnsPerSituation: envInt("MAX_TURNS_PER_SITUATION", 3),
	}
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
