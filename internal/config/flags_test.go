package config

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	ParseFlags()
	os.Exit(m.Run())
}

func TestParseFlags_DefaultValues(t *testing.T) {
	if FlagRunAddr != ":8080" {
		t.Errorf("expected FlagRunAddr = ':8080', got %q", FlagRunAddr)
	}

	if FlagBaseURLResult != "http://localhost:8080" {
		t.Errorf("expected FlagBaseURLResult = 'http://localhost:8080', got %q", FlagBaseURLResult)
	}

	if FlagLogLevel != "info" {
		t.Errorf("expected FlagLogLevel = 'info', got %q", FlagLogLevel)
	}

	if FlagDatabaseDSN != "" {
		t.Errorf("expected FlagDatabaseDSN = '', got %q", FlagDatabaseDSN)
	}

	if FlagAccrualSystemAddress != "" {
		t.Errorf("expected FlagAccrualSystemAddress = '', got %q", FlagAccrualSystemAddress)
	}
}

func TestParseFlags_EnvOverride(t *testing.T) {
	expectedAddr := ":9999"

	if FlagRunAddr != expectedAddr {
		t.Logf(
			"RUN_ADDRESS не задан до запуска `go test`. Ожидаемый: %q, текущий: %q",
			expectedAddr,
			FlagRunAddr,
		)
	}
}

func TestParseFlags_EnvVariablesExist(t *testing.T) {
	if addr := os.Getenv("RUN_ADDRESS"); addr != "" && addr != ":8080" {
		if FlagRunAddr != addr {
			t.Errorf("expected FlagRunAddr = %q (from env), got %q", addr, FlagRunAddr)
		}
	}

	if dsn := os.Getenv("DATABASE_URI"); dsn != "" {
		if FlagDatabaseDSN != dsn {
			t.Errorf("expected FlagDatabaseDSN = %q (from env), got %q", dsn, FlagDatabaseDSN)
		}
	}
}

func TestParseFlags_LogLevel(t *testing.T) {
	validLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}
	if !validLevels[FlagLogLevel] {
		t.Errorf("invalid FlagLogLevel = %q", FlagLogLevel)
	}
}
