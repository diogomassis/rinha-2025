package env

import (
	"os"
	"strconv"

	"github.com/rs/zerolog/log"
)

type EnvironmentVariables struct {
	Port                     string
	RedisAddr                string
	InstanceName             string
	PaymentDefaultEndpoint   string
	PaymentFallbackEndpoint  string
	WorkerConcurrency        int
	RedisQueueName           string
	RedisDelayedQueueName    string
	RedisDeadLetterQueueName string
}

var (
	Env *EnvironmentVariables
)

func Load() {
	Env = &EnvironmentVariables{
		Port:                     getRequiredEnv("APP_PORT"),
		RedisAddr:                getRequiredEnv("REDIS_URL"),
		InstanceName:             getRequiredEnv("INSTANCE_ID"),
		PaymentDefaultEndpoint:   getRequiredEnv("PROCESSOR_DEFAULT_URL"),
		PaymentFallbackEndpoint:  getRequiredEnv("PROCESSOR_FALLBACK_URL"),
		WorkerConcurrency:        getRequiredEnvInt("WORKER_CONCURRENCY"),
		RedisQueueName:           getRequiredEnv("REDIS_QUEUE_NAME"),
		RedisDelayedQueueName:    getOptionalEnv("REDIS_DELAYED_QUEUE_NAME", "payments_queue_delayed"),
		RedisDeadLetterQueueName: getOptionalEnv("REDIS_DEAD_LETTER_QUEUE_NAME", "payments_queue_dead-letter"),
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

func getOptionalEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func IsProduction() bool {
	return getOptionalEnv("ENVIRONMENT", "development") == "production"
}

func IsDevelopment() bool {
	return !IsProduction()
}
