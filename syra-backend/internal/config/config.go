package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	GatewayAddr      string
	DatabaseURL      string
	RedisAddr        string
	LogLevel         string
	ReadTimeout      time.Duration
	WriteTimeout     time.Duration
	IdleTimeout      time.Duration
	ShutdownTimeout  time.Duration
	RequestBodyLimit int64
}

func Load() Config {
	return Config{
		GatewayAddr:      getenv("GATEWAY_ADDR", ":8080"),
		DatabaseURL:      getenv("DATABASE_URL", ""),
		RedisAddr:        getenv("REDIS_ADDR", "localhost:6379"),
		LogLevel:         getenv("LOG_LEVEL", "info"),
		ReadTimeout:      getenvDuration("HTTP_READ_TIMEOUT", 5*time.Second),
		WriteTimeout:     getenvDuration("HTTP_WRITE_TIMEOUT", 30*time.Second),
		IdleTimeout:      getenvDuration("HTTP_IDLE_TIMEOUT", 60*time.Second),
		ShutdownTimeout:  getenvDuration("SHUTDOWN_TIMEOUT", 10*time.Second),
		RequestBodyLimit: getenvInt64("REQUEST_BODY_LIMIT_BYTES", 1<<20),
	}
}

func getenv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func getenvDuration(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	duration, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}

	return duration
}

func getenvInt64(key string, fallback int64) int64 {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		return fallback
	}

	return parsed
}
