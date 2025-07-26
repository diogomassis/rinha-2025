package env

import (
	"os"
	"strconv"

	"github.com/rs/zerolog/log"
)

type EnvironmentVariables struct {
	Port                    string
	RedisAddr               string
	InstanceName            string
	PaymentDefaultEndpoint  string
	PaymentFallbackEndpoint string
	WorkerConcurrency       int
}

var (
	Env *EnvironmentVariables
)

func Load() {
	Env = &EnvironmentVariables{
		Port:                    getRequiredEnv("APP_PORT"),
		RedisAddr:               getRequiredEnv("REDIS_URL"),
		InstanceName:            getRequiredEnv("INSTANCE_ID"),
		PaymentDefaultEndpoint:  getRequiredEnv("PROCESSOR_DEFAULT_URL"),
		PaymentFallbackEndpoint: getRequiredEnv("PROCESSOR_FALLBACK_URL"),
		WorkerConcurrency:       getRequiredEnvInt("WORKER_CONCURRENCY"),
	}
}

func getRequiredEnv(key string) string {
	value := os.Getenv(key)
	if value == "" {
		log.Fatal().Str("key", key).Msg("Required environment variable is not set")
	}
	return value
}

func getRequiredEnvInt(key string) int {
	value := getRequiredEnv(key)
	intValue, err := strconv.Atoi(value)
	if err != nil {
		log.Fatal().Str("key", key).Str("value", value).Msg("Environment variable must be an integer")
	}
	return intValue
}
