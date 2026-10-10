package main

import (
	"context"
	"encoding/hex"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/devauth"
	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/evidencefs"
	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/fakebooking"
	password "github.com/HernanEspinozaDev/espaciGo/internal/adapters/password/bcrypt"
	bookingpg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/booking"
	contractpg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/contracts"
	conversationpg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/conversation"
	damageclaimpg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/damageclaim"
	disputepg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/dispute"
	gallerypg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/gallery"
	identitypg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/identity"
	localnoticepg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/localnotice"
	occupancypg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/occupancy"
	operationspg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/operations"
	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/ownerexport"
	pricingpg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/pricing"
	reputationpg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/reputation"
	spacespg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/spaces"
	verificationpg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/verification"
	"github.com/HernanEspinozaDev/espaciGo/internal/adminlocal"
	adminlocalhttp "github.com/HernanEspinozaDev/espaciGo/internal/adminlocal/transport/http"
	"github.com/HernanEspinozaDev/espaciGo/internal/booking"
	bookinghttp "github.com/HernanEspinozaDev/espaciGo/internal/booking/transport/http"
	"github.com/HernanEspinozaDev/espaciGo/internal/contract"
	contracthttp "github.com/HernanEspinozaDev/espaciGo/internal/contract/transport/http"
	"github.com/HernanEspinozaDev/espaciGo/internal/conversation"
	"github.com/HernanEspinozaDev/espaciGo/internal/damageclaim"
	damageclaimhttp "github.com/HernanEspinozaDev/espaciGo/internal/damageclaim/transport/http"
	"github.com/HernanEspinozaDev/espaciGo/internal/dispute"
	disputehttp "github.com/HernanEspinozaDev/espaciGo/internal/dispute/transport/http"
	"github.com/HernanEspinozaDev/espaciGo/internal/gallery"
	galleryhttp "github.com/HernanEspinozaDev/espaciGo/internal/gallery/transport/http"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
	identityhttp "github.com/HernanEspinozaDev/espaciGo/internal/identity/transport/http"
	"github.com/HernanEspinozaDev/espaciGo/internal/localnotice"
	noticehttp "github.com/HernanEspinozaDev/espaciGo/internal/localnotice/transport/http"
	"github.com/HernanEspinozaDev/espaciGo/internal/m02local"
	"github.com/HernanEspinozaDev/espaciGo/internal/occupancy"
	"github.com/HernanEspinozaDev/espaciGo/internal/operation"
	operationhttp "github.com/HernanEspinozaDev/espaciGo/internal/operation/transport/http"
	"github.com/HernanEspinozaDev/espaciGo/internal/platform/health"
	"github.com/HernanEspinozaDev/espaciGo/internal/pricing"
	"github.com/HernanEspinozaDev/espaciGo/internal/privacy"
	"github.com/HernanEspinozaDev/espaciGo/internal/reputation"
	reputationhttp "github.com/HernanEspinozaDev/espaciGo/internal/reputation/transport/http"
	spacesdomain "github.com/HernanEspinozaDev/espaciGo/internal/spaces"
	spaceshttp "github.com/HernanEspinozaDev/espaciGo/internal/spaces/transport/http"
	"github.com/HernanEspinozaDev/espaciGo/internal/verification"
	verificationhttp "github.com/HernanEspinozaDev/espaciGo/internal/verification/transport/http"
	"github.com/jackc/pgx/v5/pgxpool"
)

const readyURL = "http://127.0.0.1:8080/health/ready"

func main() {
	if len(os.Args) >= 2 && (os.Args[1] == "local-privacy-retention-purge" || os.Args[1] == "local-privacy-replay-export" || os.Args[1] == "local-privacy-replay-apply") {
		if err := runLocalPrivacyCommand(os.Args[1], os.Args[2:]); err != nil {
			log.Print("local privacy operation failed; verify the command, restored database, and administrator authorization")
			os.Exit(1)
		}
		return
	}
	if len(os.Args) >= 2 && os.Args[1] == "local-booking-pagination-fixtures" {
		if err := createLocalCatalogPaginationFixtures(os.Args[2:]); err != nil {
			log.Print("synthetic pagination examples were not added; no account or database details logged")
			os.Exit(1)
		}
		return
	}
	if len(os.Args) >= 2 && os.Args[1] == "local-booking-interval-selector-fixtures" {
		if err := createLocalIntervalSelectorFixtures(os.Args[2:]); err != nil {
			log.Print("synthetic interval selector fixtures were not added; no account or database details logged")
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
	var localPaymentService *booking.Service
	var localIdentityService *identity.AuthenticationService
	var localPrivacyService *privacy.Service
	var localM02Service *m02local.Service
	var localGalleryService *gallery.Service
	var localContractService *contract.Service
	var localOperationService *operation.Service
	var localNoticeService *localnotice.Service
	var privacyReplayRegistryPath string
	var privacyEvidenceCleaner privacy.SyntheticEvidenceCleaner
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
		localIdentityService = service
		verificationRepository := verificationpg.New(pool)
		evidenceRoot := os.Getenv("M03_EVIDENCE_DIR")
		evidenceStore, err := evidencefs.New(evidenceRoot)
		if err != nil {
			return errors.New("local private evidence storage is unavailable")
		}
		privacyService, err := privacy.NewService(repo, ownerexport.New(pool, evidenceStore))
		if err != nil {
			return errors.New("local privacy initialization failed")
		}
		localPrivacyService = privacyService
		privacyReplayRegistryPath = os.Getenv("LOCAL_PRIVACY_REPLAY_FILE")
		disputeService, err := dispute.NewService(disputepg.New(pool), time.Now)
		if err != nil {
			return errors.New("local dispute initialization failed")
		}
		disputeHandler := disputehttp.NewHandler(service, disputeService, cfg.allowedOrigins)
		registerDisputeRoutes(mux, disputeHandler)
		verificationService, err := verification.NewService(verificationRepository, credentials.Generator{}, verification.LocalFixtureProvider{}, time.Now)
		if err != nil {
			return errors.New("local verification initialization failed")
		}
		privacyEvidenceCleaner = evidenceStore
		m02Service, err := m02local.New(pool, evidenceStore, time.Now)
		if err != nil {
			return errors.New("local M02 initialization failed")
		}
		localM02Service = m02Service
		m02Handler := m02local.NewHandler(service, m02Service, cfg.allowedOrigins)
		mux.Handle("/api/v1/profile/photo", m02Handler)
		mux.Handle("/api/v1/profile/photo/", m02Handler)
		mux.Handle("/api/v1/payout-account", m02Handler)
		mux.Handle("/api/v1/", identityhttp.NewHandlerWithSuppressionRegistry(service, repo, cfg.allowedOrigins, privacyService, evidenceStore, privacyReplayRegistryPath))
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
		galleryService, err := gallery.NewService(gallerypg.New(pool), evidenceStore, credentials.Generator{}, time.Now)
		if err != nil {
			return errors.New("local synthetic gallery initialization failed")
		}
		localGalleryService = galleryService
		galleryHandler := galleryhttp.NewHandler(service, galleryService, cfg.allowedOrigins)
		mux.Handle("/api/v1/spaces/{spaceID}/gallery", galleryHandler)
		mux.Handle("/api/v1/spaces/{spaceID}/gallery/{photoID}", galleryHandler)
		mux.Handle("/api/v1/spaces/{spaceID}/gallery/{photoID}/content", galleryHandler)
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
			secretBytes, err := os.ReadFile(os.Getenv("LOCAL_PAYMENT_WEBHOOK_SECRET_FILE"))
			if err != nil {
				return errors.New("local payment event authentication secret is unavailable")
			}
			paymentKey, err := hex.DecodeString(strings.TrimSpace(string(secretBytes)))
			if err != nil || len(paymentKey) < 32 {
				return errors.New("local payment event authentication secret is invalid")
			}
			paymentAdapter, err := fakebooking.NewWithStore(paymentKey, bookingpg.New(pool))
			if err != nil {
				return errors.New("local payment event authentication initialization failed")
			}
			bookingService, err := booking.NewServiceWithTTLs(bookingpg.New(pool), credentials.Generator{}, time.Now, paymentAdapter, quoteTTL, payTTL, hostTTL)
			if err != nil {
				return errors.New("local booking trial initialization failed")
			}
			bookingService.SetLocalRefundAdapter(paymentAdapter)
			bookingService.SetLocalNoticeSender(devauth.Mailer{Address: os.Getenv("LOCAL_SMTP_ADDR")})
			guaranteeOutcome := strings.TrimSpace(os.Getenv("LOCAL_GUARANTEE_FAKE_OUTCOME"))
			bookingService.SetLocalGuaranteeOutcome(nil) // The mock explicitly starts the independent authorization after rent payment.
			if guaranteeOutcome != "" && guaranteeOutcome != "exito" && guaranteeOutcome != "rechazo" && guaranteeOutcome != "sin_respuesta" {
				return errors.New("invalid local guarantee fake outcome")
			}
			if guaranteeOutcome != "" {
				bookingService.SetLocalGuaranteeOutcome(func() string { return guaranteeOutcome })
			}
			reputationService, err := reputation.New(reputationpg.New(pool), credentials.Generator{}, time.Now)
			if err != nil {
				return errors.New("local synthetic reputation initialization failed")
			}
			reputationHandler := reputationhttp.NewHandler(service, reputationService, cfg.allowedOrigins)
			mux.Handle("GET /api/v1/spaces/{spaceID}/reviews", reputationHandler)
			registerReputationReservationRoutes(mux, reputationHandler)
			mux.Handle("GET /api/v1/local/reputation/me", reputationHandler)
			localNoticeService, err = localnotice.New(localnoticepg.New(pool), devauth.Mailer{Address: os.Getenv("LOCAL_SMTP_ADDR")}, time.Now)
			if err != nil {
				return errors.New("local durable notice initialization failed")
			}
			mux.Handle("/api/v1/admin/local/notices", noticehttp.NewHandler(service, localNoticeService, cfg.allowedOrigins))
			mux.Handle("/api/v1/admin/local/notices/", noticehttp.NewHandler(service, localNoticeService, cfg.allowedOrigins))
			localPaymentService = bookingService
			conversationService, err := conversation.NewService(conversationpg.New(pool), credentials.Generator{}, time.Now)
			if err != nil {
				return errors.New("local booking conversation initialization failed")
			}
			bookingHandler := bookinghttp.NewHandler(service, bookingService, cfg.allowedOrigins, conversationService)
			adminLocalService, err := adminlocal.New(pool, time.Now)
			if err != nil {
				return errors.New("local administration service initialization failed")
			}
			adminLocalHandler := adminlocalhttp.NewHandler(service, adminLocalService, cfg.allowedOrigins)
			mux.Handle("/api/v1/admin/local/accounts/", adminLocalHandler)
			mux.Handle("/api/v1/admin/local/reports/", adminLocalHandler)
			mux.Handle("/api/v1/local/booking-trial/", bookingHandler)
			mux.Handle("/api/v1/admin/local/reservations", bookingHandler)
			mux.Handle("/api/v1/admin/local/reservations/", bookingHandler)
			mux.Handle("/api/v1/admin/local/audit-events", bookingHandler)
			mux.Handle("/api/v1/admin/local/audit-events/export", bookingHandler)
			contractKeyText, err := os.ReadFile(os.Getenv("LOCAL_CONTRACT_ENCRYPTION_KEY_FILE"))
			if err != nil {
				return errors.New("local contract encryption key is unavailable")
			}
			contractKey, err := hex.DecodeString(strings.TrimSpace(string(contractKeyText)))
			if err != nil || len(contractKey) != 32 {
				return errors.New("local contract encryption key is invalid")
			}
			contractRepo, err := contractpg.New(pool, contractKey)
			if err != nil {
				return errors.New("local contract repository initialization failed")
			}
			contractService, err := contract.NewService(contractRepo, time.Now)
			if err != nil {
				return errors.New("local contract rehearsal initialization failed")
			}
			localContractService = contractService
			registerContractRoutes(mux, contracthttp.NewHandler(service, contractService, cfg.allowedOrigins))
			operationService, err := operation.NewService(operationspg.New(pool), evidenceStore, credentials.Generator{}, time.Now)
			if err != nil {
				return errors.New("local rental operation initialization failed")
			}
			localOperationService = operationService
			registerOperationRoutes(mux, operationhttp.NewHandler(service, operationService, cfg.allowedOrigins))
			claimService, err := damageclaim.NewWithEvidenceStore(damageclaimpg.New(pool), evidenceStore, time.Now)
			if err != nil {
				return errors.New("local damage claim initialization failed")
			}
			registerDamageClaimRoutes(mux, damageclaimhttp.NewHandler(service, claimService, cfg.allowedOrigins))
		}
		mux.HandleFunc("GET /openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/yaml")
			http.ServeFile(w, r, "/openapi.yaml")
		})
	}
	server := &http.Server{
		Addr:              cfg.httpAddr,
		Handler:           identityhttp.RestrictedAccountMiddleware(localIdentityService, mux),
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
	if localPaymentService != nil {
		go localPaymentService.RunPaymentReconciler(ctx, 5*time.Second)
		go localPaymentService.RunGuaranteeReconciler(ctx, 5*time.Second)
	}
	if localContractService != nil {
		go func() {
			ticker := time.NewTicker(5 * time.Second)
			defer ticker.Stop()
			_, _ = localContractService.EnsureApproved(ctx)
			_, _ = localContractService.ExpireDue(ctx)
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					_, _ = localContractService.EnsureApproved(ctx)
					_, _ = localContractService.ExpireDue(ctx)
				}
			}
		}()
	}
	if localOperationService != nil {
		go func() {
			ticker := time.NewTicker(15 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					_ = localOperationService.CleanRetiredOnce(ctx, 50)
				}
			}
		}()
	}
	if localIdentityService != nil {
		go localIdentityService.RunCredentialNoticeWorker(ctx, time.Second)
	}
	if localNoticeService != nil {
		go localNoticeService.Run(ctx, time.Second)
		go func() {
			ticker := time.NewTicker(time.Hour)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					_, _ = localNoticeService.Purge(ctx, 100)
				}
			}
		}()
	}
	if localPrivacyService != nil {
		go localPrivacyService.RunSuppressionCleanupWorker(ctx, 10*time.Second, privacyEvidenceCleaner, privacyReplayRegistryPath)
		if localM02Service != nil {
			go func() {
				ticker := time.NewTicker(15 * time.Second)
				defer ticker.Stop()
				for {
					select {
					case <-ctx.Done():
						return
					case <-ticker.C:
						_ = localM02Service.CleanRetiredPhotos(ctx, 20)
					}
				}
			}()
		}
		go localPrivacyService.RunReservationRetentionWorker(ctx, time.Hour, 100)
	}
	if localGalleryService != nil {
		go func() {
			ticker := time.NewTicker(15 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					_ = localGalleryService.CleanRetiredOnce(ctx, 50)
				}
			}
		}()
	}
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

// registerDisputeRoutes uses method-aware exact patterns so the dispute handler
// cannot shadow the existing booking endpoints under the reservations prefix.
func registerDisputeRoutes(mux *http.ServeMux, handler http.Handler) {
	mux.Handle("GET /api/v1/local/booking-trial/reservations/{reservation_id}/disputes", handler)
	mux.Handle("POST /api/v1/local/booking-trial/reservations/{reservation_id}/disputes", handler)
	mux.Handle("GET /api/v1/local/booking-trial/disputes/{dispute_id}/history", handler)
	mux.Handle("GET /api/v1/admin/disputes", handler)
	mux.Handle("POST /api/v1/admin/disputes/{dispute_id}/close", handler)
}

func registerContractRoutes(mux *http.ServeMux, handler http.Handler) {
	mux.Handle("POST /api/v1/local/booking-trial/reservations/{reservation_id}/contract", handler)
	mux.Handle("GET /api/v1/local/booking-trial/contracts/{contract_id}", handler)
	mux.Handle("POST /api/v1/local/booking-trial/contracts/{contract_id}/sign", handler)
	mux.Handle("POST /api/v1/local/booking-trial/contracts/{contract_id}/reject", handler)
	mux.Handle("GET /api/v1/local/booking-trial/contracts/{contract_id}/document", handler)
}

func registerOperationRoutes(mux *http.ServeMux, handler http.Handler) {
	mux.Handle("POST /api/v1/local/booking-trial/reservations/{reservationID}/check-in", handler)
	mux.Handle("POST /api/v1/local/booking-trial/reservations/{reservationID}/check-out", handler)
	mux.Handle("POST /api/v1/local/booking-trial/reservations/{reservationID}/reception", handler)
	mux.Handle("GET /api/v1/local/booking-trial/reservations/{reservationID}/operations", handler)
	mux.Handle("GET /api/v1/local/booking-trial/reservations/{reservationID}/evidence/{evidenceID}", handler)
}

func registerDamageClaimRoutes(mux *http.ServeMux, handler http.Handler) {
	mux.Handle("GET /api/v1/local/booking-trial/reservations/{reservationID}/damage-claim", handler)
	mux.Handle("POST /api/v1/local/booking-trial/reservations/{reservationID}/damage-claim", handler)
	mux.Handle("POST /api/v1/local/booking-trial/reservations/{reservationID}/damage-claim/defense", handler)
	mux.Handle("GET /api/v1/admin/local/damage-claims", handler)
	mux.Handle("GET /api/v1/admin/local/damage-claims/{claimID}", handler)
	mux.Handle("POST /api/v1/admin/local/damage-claims/{claimID}/resolution", handler)
	mux.Handle("GET /api/v1/admin/local/damage-claims/{claimID}/evidence/{evidenceID}", handler)
}

// registerReputationReservationRoutes mounts only review operations, preserving
// the booking router's ownership of reservation detail, payment and cancellation.
func registerReputationReservationRoutes(mux *http.ServeMux, handler http.Handler) {
	mux.Handle("GET /api/v1/local/booking-trial/reservations/{reservationID}/reviews", handler)
	mux.Handle("POST /api/v1/local/booking-trial/reservations/{reservationID}/reviews", handler)
	mux.Handle("POST /api/v1/local/booking-trial/reservations/{reservationID}/reviews/{reviewID}/report", handler)
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
