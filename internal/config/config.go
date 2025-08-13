package config

import (
	"fmt"
	"os"
)

type AppConfig struct {
	Port string
}

func Load() (*AppConfig, error) {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	return &AppConfig{Port: port}, nil
}

func (c *AppConfig) Addr() string { return fmt.Sprintf(":%s", c.Port) }
