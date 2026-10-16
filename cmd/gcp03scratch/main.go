package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/gcp03scratch"
)

const migrationDirectory = "/app/db/migrations"

func main() {
	config, err := gcp03scratch.LoadConfig(os.Getenv)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	password, err := gcp03scratch.ReadPassword(config.PasswordFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Hour)
	defer cancel()
	if err := gcp03scratch.RunPhase(ctx, config, password, migrationDirectory); err != nil {
		fmt.Fprintf(os.Stderr, "GCP-03 scratch phase %s failed: %v\n", config.Phase, err)
		os.Exit(1)
	}
	if config.Phase == gcp03scratch.PhaseBootstrap {
		versions, err := gcp03scratch.ReadScratchExtensionVersions(ctx, config, password)
		if err != nil {
			fmt.Fprintln(os.Stderr, "GCP-03 scratch extension version verification failed")
			os.Exit(1)
		}
		fmt.Printf("GCP-03 scratch extensions: btree_gist=%s postgis=%s unaccent=%s\n", versions["btree_gist"], versions["postgis"], versions["unaccent"])
	}
	fmt.Printf("GCP-03 scratch phase %s completed\n", config.Phase)
}
