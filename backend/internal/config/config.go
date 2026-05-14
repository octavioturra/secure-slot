package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	DatabaseURL   string
	RedisURL      string
	VaultAddr     string
	VaultToken    string
	VaultKeyName  string

	KeycloakURL          string
	KeycloakRealm        string
	KeycloakClientID     string
	KeycloakClientSecret string

	GoogleAllowedDomain string

	JWTSecret         string
	JWTExpiryMinutes  int

	OTPEmailFrom string
	SMTPHost     string
	SMTPPort     int
	SMTPUser     string
	SMTPPass     string

	SlotDefaultTTLHours int
	MaxPayloadBytes     int64

	Port string
	Env  string
}

func Load() (*Config, error) {
	var errs []error

	require := func(key string) string {
		v := os.Getenv(key)
		if v == "" {
			errs = append(errs, fmt.Errorf("missing required env var: %s", key))
		}
		return v
	}

	optional := func(key, defaultVal string) string {
		if v := os.Getenv(key); v != "" {
			return v
		}
		return defaultVal
	}

	optionalInt := func(key string, defaultVal int) int {
		v := os.Getenv(key)
		if v == "" {
			return defaultVal
		}
		n, err := strconv.Atoi(v)
		if err != nil {
			errs = append(errs, fmt.Errorf("env var %s must be an integer: %w", key, err))
			return defaultVal
		}
		return n
	}

	cfg := &Config{
		DatabaseURL:          require("DATABASE_URL"),
		RedisURL:             require("REDIS_URL"),
		VaultAddr:            require("VAULT_ADDR"),
		VaultToken:           require("VAULT_TOKEN"),
		VaultKeyName:         optional("VAULT_KEY_NAME", "secureslot"),
		KeycloakURL:          require("KEYCLOAK_URL"),
		KeycloakRealm:        require("KEYCLOAK_REALM"),
		KeycloakClientID:     require("KEYCLOAK_CLIENT_ID"),
		KeycloakClientSecret: optional("KEYCLOAK_CLIENT_SECRET", ""),
		GoogleAllowedDomain:  optional("GOOGLE_ALLOWED_DOMAIN", ""),
		JWTSecret:            require("JWT_SECRET"),
		JWTExpiryMinutes:     optionalInt("JWT_EXPIRY_MINUTES", 15),
		OTPEmailFrom:         require("OTP_EMAIL_FROM"),
		SMTPHost:             require("SMTP_HOST"),
		SMTPPort:             optionalInt("SMTP_PORT", 1025),
		SMTPUser:             optional("SMTP_USER", ""),
		SMTPPass:             optional("SMTP_PASS", ""),
		SlotDefaultTTLHours:  optionalInt("SLOT_DEFAULT_TTL_HOURS", 72),
		MaxPayloadBytes:      int64(optionalInt("MAX_PAYLOAD_BYTES", 10485760)),
		Port:                 require("PORT"),
		Env:                  optional("ENV", "development"),
	}

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return cfg, nil
}
