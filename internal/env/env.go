package env

import (
	"log"
	"os"
)

type EnvironmentVariables struct {
	Port                    string
	RedisAddr               string
	InstanceName            string
	PaymentDefaultEndpoint  string
	PaymentFallbackEndpoint string
}

var (
	Env *EnvironmentVariables
)

func Load() {
	Env = &EnvironmentVariables{
		Port:                    getRequiredEnv("PORT"),
		RedisAddr:               getRequiredEnv("REDIS_ADDR"),
		InstanceName:            getRequiredEnv("INSTANCE_NAME"),
		PaymentDefaultEndpoint:  getRequiredEnv("PAYMENT_DEFAULT_ENDPOINT"),
		PaymentFallbackEndpoint: getRequiredEnv("PAYMENT_FALLBACK_ENDPOINT"),
	}

	log.Printf("[ENV] Environment variables loaded successfully:")
	log.Printf("  - Instance: %s", Env.InstanceName)
	log.Printf("  - Redis: %s", Env.RedisAddr)
	log.Printf("  - Backend Port: %s", Env.Port)
	log.Printf("  - Default Processor: %s", Env.PaymentDefaultEndpoint)
	log.Printf("  - Fallback Processor: %s", Env.PaymentFallbackEndpoint)
}

func getRequiredEnv(key string) string {
	value := os.Getenv(key)
	if value == "" {
		log.Fatalf("[ENV] Required environment variable %s is not set", key)
	}
	return value
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
