package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/devauth"
	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/evidencefs"
	password "github.com/HernanEspinozaDev/espaciGo/internal/adapters/password/bcrypt"
	identitypg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/identity"
	occupancypg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/occupancy"
	spacespg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/spaces"
	verificationpg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/verification"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
	identityhttp "github.com/HernanEspinozaDev/espaciGo/internal/identity/transport/http"
	"github.com/HernanEspinozaDev/espaciGo/internal/occupancy"
	"github.com/HernanEspinozaDev/espaciGo/internal/platform/health"
	"github.com/HernanEspinozaDev/espaciGo/internal/privacy"
	spacesdomain "github.com/HernanEspinozaDev/espaciGo/internal/spaces"
	spaceshttp "github.com/HernanEspinozaDev/espaciGo/internal/spaces/transport/http"
	"github.com/HernanEspinozaDev/espaciGo/internal/verification"
	verificationhttp "github.com/HernanEspinozaDev/espaciGo/internal/verification/transport/http"
	"github.com/jackc/pgx/v5/pgxpool"
)

const readyURL = "http://127.0.0.1:8080/health/ready"

func main() {
	if len(os.Args) == 2 && os.Args[1] == "migrate" {
		if migrateLocal() != nil {
			log.Print("local migration failed; no credentials or SQL details logged")
			os.Exit(1)
		}
		log.Print("local migrations and runtime grants ready")
		return
	}
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

	mux := http.NewServeMux()
	mux.Handle("/health/", health.NewHandler(pool, cfg.allowedOrigins))
	if os.Getenv("LOCAL_AUTH_PROTOTYPE") == "1" {
		limit, err := strconv.Atoi(os.Getenv("LOCAL_VERIFICATION_IP_LIMIT"))
		if err != nil || limit < 1 || limit > 1000 {
			return errors.New("invalid local quota")
		}
		repo := identitypg.NewIdentityRepository(pool)
		service, err := identity.NewAuthenticationService(repo, password.Hasher{}, devauth.Mailer{Address: os.Getenv("LOCAL_SMTP_ADDR")}, devauth.NewIPLimiter(limit), credentials.Generator{}, time.Now)
		if err != nil {
			return errors.New("local authentication initialization failed")
		}
		privacyService, err := privacy.NewService(repo)
		if err != nil {
			return errors.New("local privacy initialization failed")
		}
		mux.Handle("/api/v1/", identityhttp.NewHandler(service, repo, cfg.allowedOrigins, privacyService))
		verificationRepository := verificationpg.New(pool)
		verificationService, err := verification.NewService(verificationRepository, credentials.Generator{}, verification.LocalFixtureProvider{}, time.Now)
		if err != nil {
			return errors.New("local verification initialization failed")
		}
		evidenceRoot := os.Getenv("M03_EVIDENCE_DIR")
		evidenceStore, err := evidencefs.New(evidenceRoot)
		if err != nil {
			return errors.New("local private evidence storage is unavailable")
		}
		evidenceService, err := verification.NewEvidenceService(verificationRepository, verificationRepository, evidenceStore, credentials.Generator{}, time.Now)
		if err != nil {
			return errors.New("local evidence initialization failed")
		}
		verificationHandler := verificationhttp.NewHandler(service, verificationService, cfg.allowedOrigins, evidenceService)
		mux.Handle("/api/v1/verifications", verificationHandler)
		mux.Handle("/api/v1/verifications/", verificationHandler)
		mux.Handle("/api/v1/admin/verifications", verificationHandler)
		mux.Handle("/api/v1/admin/verifications/", verificationHandler)
		spacesService, err := spacesdomain.NewService(spacespg.New(pool, credentials.Generator{}))
		if err != nil {
			return errors.New("spaces initialization failed")
		}
		calendarService, err := occupancy.NewService(occupancypg.New(pool), credentials.Generator{})
		if err != nil {
			return errors.New("calendar initialization failed")
		}
		spacesHandler := spaceshttp.NewHandler(service, spacesService, cfg.allowedOrigins, calendarService)
		mux.Handle("/api/v1/spaces", spacesHandler)
		mux.Handle("/api/v1/spaces/", spacesHandler)
		mux.HandleFunc("GET /openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/yaml")
			http.ServeFile(w, r, "/openapi.yaml")
		})
	}
	server := &http.Server{
		Addr:              cfg.httpAddr,
		Handler:           mux,
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
