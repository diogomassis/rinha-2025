package env

import (
	"os"

	"github.com/rs/zerolog/log"
)

type EnvironmentVariables struct {
	Port                 string
	DbUrl                string
	ProcessorDefaultUrl  string
	ProcessorFallbackUrl string
}

var (
	Env *EnvironmentVariables
)

func Load() {
	Env = &EnvironmentVariables{
		Port:                 getRequiredEnv("APP_PORT"),
		DbUrl:                getRequiredEnv("DB_URL"),
		ProcessorDefaultUrl:  getRequiredEnv("PROCESSOR_DEFAULT_URL"),
		ProcessorFallbackUrl: getRequiredEnv("PROCESSOR_FALLBACK_URL"),
	}
}

func getRequiredEnv(key string) string {
	value := os.Getenv(key)
	if value == "" {
		log.Fatal().Str("key", key).Msg("Required environment variable is not set")
	}
	return value
}
