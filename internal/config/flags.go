// Package config used for flags and env variables.
package config

import (
	"flag"
	"os"
)

var (
	// FlagRunAddr server run address.
	FlagRunAddr string
	// FlagBaseURLResult base url.
	FlagBaseURLResult string
	// FlagLogLevel log level severity.
	FlagLogLevel string
	// FlagDatabaseDSN dsn of database.
	FlagDatabaseDSN string
)

// ParseFlags global flags.
func ParseFlags() {
	flag.StringVar(&FlagRunAddr, "a", ":8080", "address and port to run server")
	flag.StringVar(&FlagBaseURLResult, "b", "http://localhost:8080", "base address")
	flag.StringVar(&FlagLogLevel, "l", "info", "log level")
	flag.StringVar(&FlagDatabaseDSN, "d", "", "database DSN")

	flag.Parse()

	if envRunAddr, ok := os.LookupEnv("SERVER_ADDRESS"); ok {
		FlagRunAddr = envRunAddr
	}

	if envBaseURLResult, ok := os.LookupEnv("BASE_URL"); ok {
		FlagBaseURLResult = envBaseURLResult
	}

	if envLogLevel, ok := os.LookupEnv("LOG_LEVEL"); ok {
		FlagLogLevel = envLogLevel
	}

	if envDatabaseDSN, ok := os.LookupEnv("DATABASE_DSN"); ok {
		FlagDatabaseDSN = envDatabaseDSN
	}
}
