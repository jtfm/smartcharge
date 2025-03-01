package utils

import (
	"fmt"
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

func FormatFloatPointer(p *float64) string {
	if p != nil {
		return fmt.Sprintf("%f", *p)
	}
	return "nil"
}
