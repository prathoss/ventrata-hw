package internal

import (
	"github.com/prathoss/hw/pkg"
)

type Config struct {
	DatabaseDSN string
}

func NewConfigFromEnv() (Config, error) {
	databaseDSN, err := pkg.ReadRequiredEnv("HW_DATABASE")
	if err != nil {
		return Config{}, err
	}
	return Config{
		DatabaseDSN: databaseDSN,
	}, nil
}
