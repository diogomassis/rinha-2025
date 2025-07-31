package env

import (
	"os"
	"strconv"

	"github.com/rs/zerolog/log"
)

type EnvironmentVariables struct {
	Port  string
	DbUrl string
}

var (
	Env *EnvironmentVariables
)

func Load() {
	Env = &EnvironmentVariables{
		Port:  getRequiredEnv("APP_PORT"),
		DbUrl: getRequiredEnv("DB_URL"),
	}
}

func getRequiredEnv(key string) string {
	value := os.Getenv(key)
	if value == "" {
		log.Fatal().Str("key", key).Msg("Required environment variable is not set")
	}
	return value
}

func getOptionalEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value != "" {
		return value
	}
	if fallback == "" {
		log.Warn().Str("key", key).Msg("Optional environment variable not set and no fallback provided")
	}
	return fallback
}

func getRequiredEnvInt(key string) int {
	value := getRequiredEnv(key)
	intValue, err := strconv.Atoi(value)
	if err != nil {
		log.Fatal().Str("key", key).Str("value", value).Msg("Environment variable must be an integer")
	}
	return intValue
}
