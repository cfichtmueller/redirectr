package config

import (
	"os"
)

var (
	ApiHost       string
	ApiPort       string
	HandlerHost   string
	HandlerPort   string
	MongoUri      string
	MongoDatabase string
	SentryDsn     string
	Release       string
	Environment   string = "development"
)

func Load() {
	ApiHost = getEnv("API_HOST", "")
	ApiPort = getEnv("API_PORT", "8080")
	HandlerHost = getEnv("HANDLER_HOST", "")
	HandlerPort = getEnv("HANDLER_PORT", "8081")
	MongoUri = getEnv("MONGODB_URI", "mongodb://localhost:27017")
	MongoDatabase = getEnv("MONGODB_DATABASE", "redirectr")
	SentryDsn = os.Getenv("SENTRY_DSN")
	Environment = os.Getenv("ENVIRONMENT")
}

func getEnv(key, fallback string) string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v
}
