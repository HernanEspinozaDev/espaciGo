package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/platform/health"
	"github.com/jackc/pgx/v5/pgxpool"
)

const readyURL = "http://127.0.0.1:8080/health/ready"

func main() {
	if len(os.Args) == 2 && os.Args[1] == "check" {
		if err := checkEndpoint(readyURL); err != nil {
			os.Exit(1)
		}
		return
	}
	if err := run(); err != nil {
		log.Print("API startup or shutdown failed; check local configuration and service health")
		os.Exit(1)
	}
}

func run() error {
	cfg, err := loadConfig(os.Getenv)
	if err != nil {
		return errors.New("invalid API configuration")
	}

	pool, err := pgxpool.New(context.Background(), cfg.databaseURL())
	if err != nil {
		return errors.New("could not initialize PostgreSQL connection pool")
	}
	defer pool.Close()

	server := &http.Server{
		Addr:              cfg.httpAddr,
		Handler:           health.NewHandler(pool, cfg.allowedOrigins),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- server.ListenAndServe()
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	select {
	case err := <-serverErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return errors.New("HTTP server failed")
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return errors.New("HTTP server shutdown failed")
		}
		return nil
	}
}

func checkEndpoint(target string) error {
	client := &http.Client{Timeout: 2 * time.Second}
	response, err := client.Get(target)
	if err != nil {
		return errors.New("health endpoint is unavailable")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return errors.New("health endpoint is not ready")
	}
	return nil
}
