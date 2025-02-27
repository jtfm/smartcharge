package utils

import (
	"os"

	"github.com/rs/zerolog/log"
)

func GetEnvStrict(key string) string {
	value := os.Getenv(key)
	if value == "" {
		log.Fatal().Msgf("Environment variable %s is required", key)
	}
	return value
}

