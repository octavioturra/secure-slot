package main

import (
	"log"

	"github.com/octavioturra/secure-slot/backend/internal/config"
	"github.com/octavioturra/secure-slot/backend/internal/keymgr"
	"github.com/octavioturra/secure-slot/backend/internal/server"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	km, err := keymgr.NewVaultKeyManager(cfg.VaultAddr, cfg.VaultToken, cfg.VaultKeyName)
	if err != nil {
		log.Fatalf("keymgr: %v", err)
	}
	_ = km // will be wired into handlers in future steps

	srv := server.New(cfg)
	log.Printf("starting server on :%s (env=%s)", cfg.Port, cfg.Env)
	if err := srv.Run(); err != nil {
		log.Fatalf("server: %v", err)
	}
}
