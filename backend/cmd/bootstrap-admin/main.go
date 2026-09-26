package main

import (
	"context"
	"log"
	"os"

	"github.com/mostransport/vsm-trainer/internal/config"
	"github.com/mostransport/vsm-trainer/internal/repo/postgres"
)

func main() {
	cfg := config.Load()
	store, err := postgres.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()
	player, err := store.BootstrapAdmin(context.Background(), os.Getenv("ADMIN_BOOTSTRAP_EMAIL"), os.Getenv("ADMIN_BOOTSTRAP_PASSWORD"))
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("admin account ready: %s", player.Email)
}
