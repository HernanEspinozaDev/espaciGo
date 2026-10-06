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
	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/fakebooking"
	password "github.com/HernanEspinozaDev/espaciGo/internal/adapters/password/bcrypt"
	bookingpg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/booking"
	conversationpg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/conversation"
	identitypg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/identity"
	occupancypg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/occupancy"
	pricingpg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/pricing"
	spacespg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/spaces"
	verificationpg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/verification"
	"github.com/HernanEspinozaDev/espaciGo/internal/booking"
	bookinghttp "github.com/HernanEspinozaDev/espaciGo/internal/booking/transport/http"
	"github.com/HernanEspinozaDev/espaciGo/internal/conversation"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
	identityhttp "github.com/HernanEspinozaDev/espaciGo/internal/identity/transport/http"
	"github.com/HernanEspinozaDev/espaciGo/internal/occupancy"
	"github.com/HernanEspinozaDev/espaciGo/internal/platform/health"
	"github.com/HernanEspinozaDev/espaciGo/internal/pricing"
	"github.com/HernanEspinozaDev/espaciGo/internal/privacy"
	spacesdomain "github.com/HernanEspinozaDev/espaciGo/internal/spaces"
	spaceshttp "github.com/HernanEspinozaDev/espaciGo/internal/spaces/transport/http"
	"github.com/HernanEspinozaDev/espaciGo/internal/verification"
	verificationhttp "github.com/HernanEspinozaDev/espaciGo/internal/verification/transport/http"
	"github.com/jackc/pgx/v5/pgxpool"
)

const readyURL = "http://127.0.0.1:8080/health/ready"

func main() {
	if len(os.Args) == 2 && os.Args[1] == "local-booking-pagination-fixtures" {
		if err := createLocalCatalogPaginationFixtures(); err != nil {
			log.Print("synthetic pagination examples were not added; no account or database details logged")
			os.Exit(1)
		}
		return
	}
	if (len(os.Args) == 4 || len(os.Args) == 5) && os.Args[1] == "local-booking-fixture" {
		category := "sala_multiproposito"
		if len(os.Args) == 5 {
			category = os.Args[4]
		}
		if err := createLocalBookingFixture(os.Args[2], os.Args[3], category); err != nil {
			log.Print("local fixture was not enabled; no account or database details logged")
			os.Exit(1)
		}
		return
	}
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
		pricingService, err := pricing.NewService(pricingpg.New(pool), calendarService, credentials.Generator{})
		if err != nil {
			return errors.New("pricing initialization failed")
		}
		spacesHandler := spaceshttp.NewHandlerWithPricing(service, spacesService, cfg.allowedOrigins, calendarService, pricingService)
		mux.Handle("/api/v1/spaces", spacesHandler)
		mux.Handle("/api/v1/spaces/", spacesHandler)
		if os.Getenv("LOCAL_BOOKING_TRIAL") == "1" {
			quoteTTL, err := localTrialDuration("LOCAL_BOOKING_QUOTE_TTL", 15*time.Minute)
			if err != nil {
				return errors.New("invalid local quote lifetime")
			}
			payTTL, err := localTrialDuration("LOCAL_BOOKING_PAY_TTL", 15*time.Minute)
			if err != nil {
				return errors.New("invalid local payment lifetime")
			}
			hostTTL, err := localTrialDuration("LOCAL_BOOKING_HOST_TTL", 24*time.Hour)
			if err != nil {
				return errors.New("invalid local host response lifetime")
			}
			bookingService, err := booking.NewServiceWithTTLs(bookingpg.New(pool), credentials.Generator{}, time.Now, fakebooking.New(), quoteTTL, payTTL, hostTTL)
			if err != nil {
				return errors.New("local booking trial initialization failed")
			}
			conversationService, err := conversation.NewService(conversationpg.New(pool), credentials.Generator{}, time.Now)
			if err != nil {
				return errors.New("local booking conversation initialization failed")
			}
			bookingHandler := bookinghttp.NewHandler(service, bookingService, cfg.allowedOrigins, conversationService)
			mux.Handle("/api/v1/local/booking-trial/", bookingHandler)
		}
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
