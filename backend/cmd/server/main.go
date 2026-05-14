package main

import (
	"context"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/octavioturra/secure-slot/backend/internal/auth"
	"github.com/octavioturra/secure-slot/backend/internal/config"
	"github.com/octavioturra/secure-slot/backend/internal/keymgr"
	"github.com/octavioturra/secure-slot/backend/internal/server"
	"github.com/octavioturra/secure-slot/backend/internal/store"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db pool: %v", err)
	}
	defer pool.Close()

	redisClient, err := store.NewRedisClient(cfg.RedisURL)
	if err != nil {
		log.Fatalf("redis: %v", err)
	}

	km, err := keymgr.NewVaultKeyManager(cfg.VaultAddr, cfg.VaultToken, cfg.VaultKeyName)
	if err != nil {
		log.Fatalf("keymgr: %v", err)
	}
	_ = km // will be wired into slot handlers in a future step

	userRepo := auth.NewPostgresUserRepository(pool)
	passkeyRepo := auth.NewPostgresPasskeyRepository(pool)

	jwtSvc := auth.NewJWTService(cfg.JWTSecret, cfg.JWTExpiryMinutes)

	oidcClient, err := auth.NewOIDCClient(ctx,
		cfg.KeycloakURL, cfg.KeycloakRealm,
		cfg.KeycloakClientID, cfg.KeycloakClientSecret,
		cfg.AppURL+"/api/v1/auth/callback",
		cfg.GoogleAllowedDomain,
	)
	if err != nil {
		log.Fatalf("oidc: %v", err)
	}

	authHandler := auth.NewHandler(oidcClient, jwtSvc, userRepo, redisClient)

	passkeyHandler, err := auth.NewPasskeyHandler(cfg.AppURL, userRepo, passkeyRepo, redisClient)
	if err != nil {
		log.Fatalf("passkey: %v", err)
	}

	srv := server.New(cfg, jwtSvc, authHandler, passkeyHandler)
	log.Printf("starting server on :%s (env=%s)", cfg.Port, cfg.Env)
	if err := srv.Run(); err != nil {
		log.Fatalf("server: %v", err)
	}
}
