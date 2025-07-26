package env

import (
	"log"
	"os"
	"strconv"
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

	log.Printf("[ENV] Environment variables loaded successfully:")
	log.Printf("  - Instance: %s", Env.InstanceName)
	log.Printf("  - Redis: %s", Env.RedisAddr)
	log.Printf("  - Backend Port: %s", Env.Port)
	log.Printf("  - Default Processor: %s", Env.PaymentDefaultEndpoint)
	log.Printf("  - Fallback Processor: %s", Env.PaymentFallbackEndpoint)
	log.Printf("  - Worker Concurrency: %d", Env.WorkerConcurrency)
	log.Printf("  - Redis Queue Name: %s", Env.RedisQueueName)
	log.Printf("  - Redis Delayed Queue Name: %s", Env.RedisDelayedQueueName)
	log.Printf("  - Redis Dead Letter Queue Name: %s", Env.RedisDeadLetterQueueName)
}

func getRequiredEnv(key string) string {
	value := os.Getenv(key)
	if value == "" {
		log.Fatalf("[ENV] Required environment variable %s is not set", key)
	}
	return value
}

func getRequiredEnvInt(key string) int {
	value := getRequiredEnv(key)
	intValue, err := strconv.Atoi(value)
	if err != nil {
		log.Fatalf("[ENV] Environment variable %s must be an integer, got '%s'", key, value)
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
