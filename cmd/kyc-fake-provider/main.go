package main

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const listenAddress = "127.0.0.1:19091"

func main() {
	if len(os.Args) != 1 {
		log.Print("local fake provider accepts no arguments")
		os.Exit(2)
	}
	if err := run(); err != nil {
		log.Print("local fake provider stopped")
		os.Exit(1)
	}
}

func run() error {
	mode := os.Getenv("KYC_FAKE_SYNTHETIC_APPROVAL")
	if mode != "" && mode != "1" {
		return errors.New("invalid synthetic approval mode")
	}
	handler := NewHandler()
	if mode == "1" {
		handler = NewSyntheticApprovalHandler()
	}
	listener, err := net.Listen("tcp", listenAddress)
	if err != nil {
		return errors.New("local fake provider listener unavailable")
	}
	server := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       30 * time.Second,
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	serverErrors := make(chan error, 1)
	go func() { serverErrors <- server.Serve(listener) }()
	select {
	case err := <-serverErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return errors.New("local fake provider HTTP server failed")
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return errors.New("local fake provider shutdown failed")
		}
		return nil
	}
}
