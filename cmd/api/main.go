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
	_ "time/tzdata"

	clamavadapter "github.com/HernanEspinozaDev/espaciGo/internal/adapters/clamav"
	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/devauth"
	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/fakebooking"
	password "github.com/HernanEspinozaDev/espaciGo/internal/adapters/password/bcrypt"
	bookingpg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/booking"
	contractpg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/contracts"
	conversationpg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/conversation"
	damageclaimpg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/damageclaim"
	disputepg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/dispute"
	documentverificationpg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/documentverification"
	gallerypg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/gallery"
	identitypg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/identity"
	localnoticepg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/localnotice"
	mediapg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/media"
	occupancypg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/occupancy"
	operationspg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/operations"
	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/ownerexport"
	pricingpg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/pricing"
	publiccatalogpg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/publiccatalog"
	reputationpg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/reputation"
	spacespg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/spaces"
	verificationpg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/verification"
	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/zohoauth"
	"github.com/HernanEspinozaDev/espaciGo/internal/adminglobal"
	adminglobalhttp "github.com/HernanEspinozaDev/espaciGo/internal/adminglobal/transport/http"
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
	"github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
	"github.com/HernanEspinozaDev/espaciGo/internal/documentverification/inspection"
	documentverificationhttp "github.com/HernanEspinozaDev/espaciGo/internal/documentverification/transport/http"
	"github.com/HernanEspinozaDev/espaciGo/internal/gallery"
	galleryhttp "github.com/HernanEspinozaDev/espaciGo/internal/gallery/transport/http"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
	identityhttp "github.com/HernanEspinozaDev/espaciGo/internal/identity/transport/http"
	"github.com/HernanEspinozaDev/espaciGo/internal/identityverification"
	"github.com/HernanEspinozaDev/espaciGo/internal/localnotice"
	noticehttp "github.com/HernanEspinozaDev/espaciGo/internal/localnotice/transport/http"
	"github.com/HernanEspinozaDev/espaciGo/internal/m02local"
	"github.com/HernanEspinozaDev/espaciGo/internal/media"
	"github.com/HernanEspinozaDev/espaciGo/internal/occupancy"
	"github.com/HernanEspinozaDev/espaciGo/internal/operation"
	operationhttp "github.com/HernanEspinozaDev/espaciGo/internal/operation/transport/http"
	"github.com/HernanEspinozaDev/espaciGo/internal/platform/businessclock"
	"github.com/HernanEspinozaDev/espaciGo/internal/platform/health"
	"github.com/HernanEspinozaDev/espaciGo/internal/pricing"
	"github.com/HernanEspinozaDev/espaciGo/internal/privacy"
	"github.com/HernanEspinozaDev/espaciGo/internal/publiccatalog"
	publiccataloghttp "github.com/HernanEspinozaDev/espaciGo/internal/publiccatalog/transport/http"
	"github.com/HernanEspinozaDev/espaciGo/internal/reputation"
	reputationhttp "github.com/HernanEspinozaDev/espaciGo/internal/reputation/transport/http"
	spacesdomain "github.com/HernanEspinozaDev/espaciGo/internal/spaces"
	spaceshttp "github.com/HernanEspinozaDev/espaciGo/internal/spaces/transport/http"
	"github.com/HernanEspinozaDev/espaciGo/internal/verification"
	verificationhttp "github.com/HernanEspinozaDev/espaciGo/internal/verification/transport/http"
)

const readyURL = "http://127.0.0.1:8080/health/ready"

const (
	apiReadHeaderTimeout = 5 * time.Second
	// 11 MiB multipart payloads get a two-minute body window (~96 KiB/s).
	apiReadTimeout = 2 * time.Minute
	// WriteTimeout starts with request processing; four minutes allows the
	// full body window plus two minutes for conversion, storage, and response.
	apiWriteTimeout = 4 * time.Minute
)

func newHTTPServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: apiReadHeaderTimeout,
		ReadTimeout:       apiReadTimeout,
		WriteTimeout:      apiWriteTimeout,
		IdleTimeout:       60 * time.Second,
	}
}

func main() {
	if len(os.Args) == 2 && os.Args[1] == "local-readonly-probe" {
		if runLocalReadOnlyProbe() != nil {
			log.Print("local read-only PostgreSQL probe failed; no connection details logged")
			os.Exit(1)
		}
		return
	}
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
	runtimeProfileValue, err := runtimeProfile(os.Getenv)
	if err != nil {
		return errors.New("invalid API runtime profile")
	}
	cfg, err := loadConfig(os.Getenv)
	if err != nil {
		return errors.New("invalid API configuration")
	}
	businessNow, err := businessclock.FromEnv(os.Getenv)
	if err != nil {
		return errors.New("invalid isolated business clock configuration")
	}
	isolatedDatasetClock, err := isolatedDatasetClockFromEnv(os.Getenv)
	if err != nil {
		return errors.New("invalid dataset isolated clock configuration")
	}
	prototypeEnabled, err := authPrototypeEnabled(os.Getenv)
	if err != nil {
		return errors.New("invalid API profile configuration")
	}

	pool, closeConnector, err := newDatabasePool(context.Background(), cfg)
	if err != nil {
		return errors.New("could not initialize PostgreSQL connection pool")
	}
	defer closeConnector()
	defer pool.Close()

	mux := http.NewServeMux()
	var localPaymentService *booking.Service
	var localIdentityService *identity.AuthenticationService
	var localPrivacyService *privacy.Service
	var localM02Service *m02local.Service
	var localGalleryService *gallery.Service
	var localMediaService *media.Service
	var documentScanProcessor *documentverificationpg.ScanProcessor
	var documentRetentionProcessor *documentverificationpg.RetentionProcessor
	var localContractService *contract.Service
	var localOperationService *operation.Service
	var localNoticeService *localnotice.Service
	var localContractRepository *contractpg.Repository
	var publicCatalogMasterKey []byte
	var privacyReplayRegistryPath string
	var privacyReplayStore privacy.SuppressionReplayStore
	var privacyEvidenceCleaner privacy.SyntheticEvidenceCleaner
	var authMailer identity.AuthenticationMailer = devauth.Mailer{Address: os.Getenv("LOCAL_SMTP_ADDR")}
	var localNoticeSender interface {
		SendLocalBookingNotice(context.Context, string, string, string) error
	} = devauth.Mailer{Address: os.Getenv("LOCAL_SMTP_ADDR")}
	mux.Handle("/health/", health.NewHandler(pool, cfg.allowedOrigins))
	if prototypeEnabled {
		limit, err := strconv.Atoi(os.Getenv("LOCAL_VERIFICATION_IP_LIMIT"))
		if err != nil || limit < 1 || limit > 1000 {
			return errors.New("invalid local quota")
		}
		repo := identitypg.NewIdentityRepository(pool)
		if os.Getenv("APP_PROFILE") == "cloud-test" {
			zohoMailer, mailErr := zohoauth.New(zohoauth.Config{
				Host: os.Getenv("AUTH_SMTP_HOST"), Port: os.Getenv("AUTH_SMTP_PORT"), Security: os.Getenv("AUTH_SMTP_SECURITY"),
				Username: os.Getenv("AUTH_SMTP_USERNAME"), PasswordFile: os.Getenv("AUTH_SMTP_PASSWORD_FILE"),
				From: os.Getenv("AUTH_SMTP_FROM"), FromName: "EspaciGo (pruebas)", PublicOrigin: cfg.frontendPublicOrigin,
			})
			if mailErr != nil {
				authMailer = zohoauth.UnavailableMailer{}
				localNoticeSender = zohoauth.UnavailableMailer{}
			} else {
				authMailer = zohoMailer
				localNoticeSender = zohoMailer
			}
		}
		service, err := identity.NewAuthenticationService(repo, password.Hasher{}, authMailer, devauth.NewIPLimiter(limit), credentials.Generator{}, time.Now)
		if err != nil {
			return errors.New("local authentication initialization failed")
		}
		localIdentityService = service
		documentVerificationRepository := documentverificationpg.New(pool)
		documentVerificationService, err := documentverification.NewService(documentVerificationRepository)
		if err != nil {
			return errors.New("local document verification initialization failed")
		}
		evidenceRoot := os.Getenv("M03_EVIDENCE_DIR")
		documentStore, err := newRuntimeDocumentStore(os.Getenv, evidenceRoot)
		if err != nil {
			return errors.New("private document storage is unavailable")
		}
		var documentUploadService *documentverification.UploadService
		var documentDownloadHandler http.Handler
		if runtimeProfileValue == runtimeProfileDocumentScanOnly {
			if documentStore == nil {
				return errors.New("document scan profile requires private document storage")
			}
			scanConfig, enabled, scanErr := documentScanConfigFromEnv(os.Getenv)
			if scanErr != nil || !enabled {
				return errors.New("document scanner configuration is invalid")
			}
			scanner, scannerErr := clamavadapter.New(clamavadapter.Config{
				Address: scanConfig.address, Timeout: 10 * time.Second,
				MaxBytes: documentverification.MaxUploadBytes, MaxSignatureAge: scanConfig.maxSignatureAge,
			})
			if scannerErr != nil {
				return errors.New("document scanner initialization failed")
			}
			documentScanProcessor = documentverificationpg.NewScanProcessor(documentVerificationRepository, documentStore, scanner, 2*time.Minute, 30*time.Second, scanConfig.maxSignatureAge)
			documentRetentionProcessor = documentverificationpg.NewRetentionProcessor(documentVerificationRepository, documentStore, 2*time.Minute, 30*time.Second)
			if documentScanProcessor == nil || documentRetentionProcessor == nil {
				return errors.New("document scan processor initialization failed")
			}
			documentInspector := inspection.New(inspection.Config{PDFInfoPath: "/usr/bin/pdfinfo", PDFDetachPath: "/usr/bin/pdfdetach"})
			if documentInspector == nil {
				return errors.New("trusted document inspection tools are unavailable")
			}
			documentUploadService, err = documentverification.NewUploadService(documentVerificationService, documentInspector, documentStore)
			if err != nil {
				return errors.New("document upload service initialization failed")
			}
			downloadService, downloadErr := documentverification.NewSpaceDownloadService(documentVerificationRepository, documentStore)
			if downloadErr != nil {
				return errors.New("document download service initialization failed")
			}
			documentDownloadHandler = documentverificationhttp.NewOwnerDownloadHandler(service, downloadService)
		}
		registerDocumentVerificationRoutes(mux, documentverificationhttp.NewOwnerCasesHandlerWithUpload(service, documentVerificationService, documentUploadService), documentDownloadHandler)
		verificationRepository := verificationpg.New(pool)
		evidenceStore, err := newRuntimeFileStore(context.Background(), os.Getenv, evidenceRoot)
		if err != nil {
			return errors.New("private evidence storage is unavailable")
		}
		if closer, ok := evidenceStore.(interface{ Close() error }); ok {
			defer closer.Close()
		}
		mediaStore, err := newRuntimeMediaStore(context.Background(), os.Getenv, evidenceRoot)
		if err != nil {
			return errors.New("private media storage is unavailable")
		}
		if closer, ok := mediaStore.(interface{ Close() error }); ok {
			defer closer.Close()
		}
		if keyPath := strings.TrimSpace(os.Getenv("LOCAL_CONTRACT_ENCRYPTION_KEY_FILE")); keyPath != "" {
			contractKeyText, readErr := os.ReadFile(keyPath)
			if readErr != nil {
				return errors.New("local contract encryption key is unavailable")
			}
			contractKey, decodeErr := hex.DecodeString(strings.TrimSpace(string(contractKeyText)))
			if decodeErr != nil || len(contractKey) != 32 {
				return errors.New("local contract encryption key is invalid")
			}
			publicCatalogMasterKey = append([]byte(nil), contractKey...)
			localContractRepository, err = contractpg.New(pool, contractKey)
			if err != nil {
				return errors.New("local contract repository initialization failed")
			}
		}
		var archiveRepository *ownerexport.Repository
		if localContractRepository != nil {
			archiveRepository = ownerexport.NewWithMediaStore(pool, evidenceStore, mediaStore, localContractRepository)
		} else {
			archiveRepository = ownerexport.NewWithMediaStore(pool, evidenceStore, mediaStore)
		}
		privacyService, err := privacy.NewService(repo, archiveRepository)
		if err != nil {
			return errors.New("local privacy initialization failed")
		}
		localPrivacyService = privacyService
		privacyReplayRegistryPath = os.Getenv("LOCAL_PRIVACY_REPLAY_FILE")
		privacyReplayStore, err = newRuntimePrivacyReplayStore(context.Background(), os.Getenv)
		if err != nil {
			return errors.New("privacy replay storage configuration is invalid")
		}
		if closer, ok := privacyReplayStore.(interface{ Close() error }); ok {
			defer closer.Close()
		}
		disputeService, err := dispute.NewService(disputepg.New(pool), businessNow)
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
		mux.Handle("/api/v1/", identityhttp.NewHandlerWithSuppressionRegistry(service, repo, cfg.allowedOrigins, privacyService, evidenceStore, privacyReplayRegistryPath, privacyReplayStore))
		globalAdminService, err := adminglobal.New(pool, time.Now)
		if err != nil {
			return errors.New("global administration service initialization failed")
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
		globalAdminService.SetPublicationRetirer(spacesService)
		mux.Handle("/api/v1/admin/global/", adminglobalhttp.NewHandler(service, globalAdminService, cfg.allowedOrigins))
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
		ownerWeeklyHoursService := booking.NewOwnerWeeklyHoursService(bookingpg.New(pool))
		ownerWeeklyHoursHandler := bookinghttp.NewOwnerWeeklyHoursHandler(service, ownerWeeklyHoursService, cfg.allowedOrigins)
		registerOwnerWeeklyHoursRoutes(mux, ownerWeeklyHoursHandler)
		galleryService, err := gallery.NewService(gallerypg.New(pool), evidenceStore, credentials.Generator{}, time.Now)
		if err != nil {
			return errors.New("local synthetic gallery initialization failed")
		}
		localGalleryService = galleryService
		globalAdminService.SetPublicationGallery(galleryService)
		mediaService, err := media.NewService(mediapg.New(pool), mediaStore, credentials.Generator{}, time.Now)
		if err != nil {
			return errors.New("media service initialization failed")
		}
		localMediaService = mediaService
		galleryHandler := galleryhttp.NewHandlerWithMedia(service, galleryService, mediaService, cfg.allowedOrigins)
		m02Handler := m02local.NewHandlerWithMedia(service, m02Service, mediaService, cfg.allowedOrigins)
		mux.Handle("/api/v1/profile/photo", m02Handler)
		mux.Handle("/api/v1/profile/photo/", m02Handler)
		mux.Handle("/api/v1/payout-account", m02Handler)
		mux.Handle("/api/v1/spaces/{spaceID}/gallery", galleryHandler)
		mux.Handle("/api/v1/spaces/{spaceID}/cover", galleryHandler)
		mux.Handle("/api/v1/spaces/{spaceID}/gallery/{photoID}", galleryHandler)
		mux.Handle("/api/v1/spaces/{spaceID}/gallery/{photoID}/content", galleryHandler)
		mux.Handle("/api/v1/spaces/{spaceID}/gallery/{photoID}/variants/{variant}", galleryHandler)
		mux.Handle("/api/v1/spaces/{spaceID}/gallery/{photoID}/publication", galleryHandler)
		if os.Getenv("LOCAL_BOOKING_TRIAL") == "1" {
			publicCatalogService, err := publiccatalog.NewWithImageAndMediaStore(publiccatalogpg.New(pool), publicCatalogMasterKey, evidenceStore, mediaStore)
			if err != nil {
				return errors.New("public catalog initialization failed")
			}
			publicCatalogHandler := publiccataloghttp.New(publicCatalogService, cfg.allowedOrigins...)
			mux.Handle("/api/v1/public/spaces", publicCatalogHandler)
			mux.Handle("/api/v1/public/spaces/", publicCatalogHandler)
			mux.Handle("/api/v1/public/space-categories", publicCatalogHandler)
			mux.Handle("/api/v1/public/space-categories/", publicCatalogHandler)
			mux.Handle("/api/v1/public/locations", publicCatalogHandler)
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
			paymentAdapter, err := fakebooking.NewWithStoreAndClock(paymentKey, bookingpg.New(pool), businessNow)
			if err != nil {
				return errors.New("local payment event authentication initialization failed")
			}
			bookingService, err := booking.NewServiceWithTTLsAndGlobalExpirySweeps(bookingpg.New(pool), credentials.Generator{}, businessNow, paymentAdapter, quoteTTL, payTTL, hostTTL, !isolatedDatasetClock)
			if err != nil {
				return errors.New("local booking trial initialization failed")
			}
			identityVerifier, err := identityverification.NewService(documentverificationpg.New(pool))
			if err != nil {
				return errors.New("local documentary identity verifier initialization failed")
			}
			bookingService.SetBookingIdentityVerifier(identityVerifier)
			globalAdminService.SetWeeklyHoursManager(bookingService)
			bookingService.SetLocalRefundAdapter(paymentAdapter)
			bookingService.SetLocalSettlementAdapter(paymentAdapter)
			bookingService.SetLocalNoticeSender(localNoticeSender)
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
			mux.Handle("/api/v1/admin/local/review-reports", reputationHandler)
			mux.Handle("/api/v1/admin/local/review-reports/", reputationHandler)
			registerReputationReservationRoutes(mux, reputationHandler)
			mux.Handle("GET /api/v1/local/reputation/me", reputationHandler)
			localNoticeService, err = localnotice.NewWithFrontendOrigin(localnoticepg.New(pool), localNoticeSender, time.Now, cfg.frontendPublicOrigin)
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
			bookingHandler := bookinghttp.NewHandlerWithClock(service, bookingService, cfg.allowedOrigins, businessNow, conversationService)
			adminLocalService, err := adminlocal.New(pool, time.Now)
			if err != nil {
				return errors.New("local administration service initialization failed")
			}
			adminLocalHandler := adminlocalhttp.NewHandler(service, adminLocalService, cfg.allowedOrigins)
			mux.Handle("/api/v1/admin/local/accounts/", adminLocalHandler)
			mux.Handle("/api/v1/admin/local/reports/", adminLocalHandler)
			mux.Handle("/api/v1/admin/local/published-spaces", adminLocalHandler)
			mux.Handle("/api/v1/admin/local/published-spaces/", adminLocalHandler)
			mux.Handle("/api/v1/local/booking-trial/", bookingHandler)
			mux.Handle("/api/v1/admin/local/reservations", bookingHandler)
			mux.Handle("/api/v1/admin/local/reservations/", bookingHandler)
			mux.Handle("/api/v1/admin/local/audit-events", bookingHandler)
			mux.Handle("/api/v1/admin/local/audit-events/export", bookingHandler)
			if localContractRepository == nil {
				return errors.New("local contract encryption key is unavailable")
			}
			contractService, err := contract.NewService(localContractRepository, businessNow)
			if err != nil {
				return errors.New("local contract rehearsal initialization failed")
			}
			localContractService = contractService
			registerContractRoutes(mux, contracthttp.NewHandler(service, contractService, cfg.allowedOrigins))
			operationService, err := operation.NewService(operationspg.New(pool), evidenceStore, credentials.Generator{}, businessNow)
			if err != nil {
				return errors.New("local rental operation initialization failed")
			}
			localOperationService = operationService
			registerOperationRoutes(mux, operationhttp.NewHandler(service, operationService, cfg.allowedOrigins))
			claimService, err := damageclaim.NewWithEvidenceStore(damageclaimpg.New(pool), evidenceStore, businessNow)
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
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	documentWorkers := newTrackedWorkerGroup(ctx)
	defer documentWorkers.StopAndWait()

	// Keep every mutating background loop in this registry so the explicit
	// worker-free profile can suppress them as one enforceable startup gate.
	workers := make([]namedWorker, 0, 13)
	if localPaymentService != nil && !isolatedDatasetClock {
		workers = append(workers,
			namedWorker{"payment-reconciler", func() { go localPaymentService.RunPaymentReconciler(ctx, 5*time.Second) }},
			namedWorker{"guarantee-reconciler", func() { go localPaymentService.RunGuaranteeReconciler(ctx, 5*time.Second) }},
			namedWorker{"settlement-reconciler", func() { go localPaymentService.RunSettlementReconciler(ctx, 5*time.Second) }},
		)
	}
	if localContractService != nil && !isolatedDatasetClock {
		workers = append(workers, namedWorker{"contract-lifecycle", func() {
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
		}})
	}
	if localMediaService != nil && !isolatedDatasetClock {
		workers = append(workers, namedWorker{"media-cleanup", func() {
			go func() {
				ticker := time.NewTicker(time.Minute)
				defer ticker.Stop()
				for {
					select {
					case <-ctx.Done():
						return
					case <-ticker.C:
						// Reconciliation is bounded and logs no media/account identifiers.
						if err := localMediaService.CleanOnce(ctx, 50); err != nil {
							log.Print("private media cleanup pass failed")
						}
					}
				}
			}()
		}})
	}
	if localOperationService != nil && !isolatedDatasetClock {
		workers = append(workers, namedWorker{"operation-cleanup", func() {
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
		}})
	}
	if localIdentityService != nil {
		workers = append(workers, namedWorker{"credential-notices", func() {
			go localIdentityService.RunCredentialNoticeWorker(ctx, time.Second)
		}})
	}
	if localNoticeService != nil {
		workers = append(workers,
			namedWorker{"notice-delivery", func() { go localNoticeService.Run(ctx, time.Second) }},
			namedWorker{"notice-purge", func() {
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
			}},
		)
	}
	if localPrivacyService != nil {
		workers = append(workers,
			namedWorker{"privacy-suppression-replay-export", func() {
				go localPrivacyService.RunSuppressionReplayExportWorker(ctx, 10*time.Second, privacyReplayRegistryPath, privacyReplayStore)
			}},
			namedWorker{"local-privacy-retention", func() {
				go localPrivacyService.RunLocalRetentionWorker(ctx, time.Hour, 100, privacyEvidenceCleaner)
			}},
		)
		if localM02Service != nil {
			workers = append(workers, namedWorker{"retired-photo-cleanup", func() {
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
			}})
		}
	}
	if localGalleryService != nil {
		workers = append(workers, namedWorker{"gallery-retention", func() {
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
		}})
	}
	if documentScanProcessor != nil {
		workers = append(workers, namedWorker{"document-verification-scan", func() {
			documentWorkers.Go(func(workerCtx context.Context) {
				if err := documentScanProcessor.Run(workerCtx, time.Second, func(error) {
					log.Print("document verification scan pass failed")
				}); err != nil {
					log.Print("document verification scan worker stopped")
				}
			})
		}})
		workers = append(workers, namedWorker{"document-verification-retention", func() {
			documentWorkers.Go(func(workerCtx context.Context) {
				if err := documentRetentionProcessor.Run(workerCtx, time.Second, func(error) {
					log.Print("document verification retention pass failed")
				}); err != nil {
					log.Print("document verification retention worker stopped")
				}
			})
		}})
	}
	if err := validateWorkerRegistry(runtimeProfileValue, workers); err != nil {
		return errors.New("background worker registry is invalid")
	}
	apiHandler := http.Handler(mux)
	if runtimeProfileValue == runtimeProfileDocumentScanOnly {
		apiHandler = restrictDocumentVerificationRoutes(apiHandler)
	}
	server := newHTTPServer(cfg.httpAddr, identityhttp.RestrictedAccountMiddleware(localIdentityService, apiHandler, cfg.allowedOrigins...))
	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- server.ListenAndServe()
	}()
	if err := launchWorkers(runtimeProfileValue, workers); err != nil {
		return errors.New("background worker registry is invalid")
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

func isolatedDatasetClockFromEnv(getenv func(string) string) (bool, error) {
	switch strings.TrimSpace(getenv("LOCAL_DATASET_DEALS_ISOLATED_CLOCK")) {
	case "", "0":
		return false, nil
	case "1":
		if getenv("LOCAL_BOOKING_TRIAL") != "1" || strings.TrimSpace(getenv("LOCAL_DEMO_BUSINESS_CLOCK_FILE")) != "/run/fe14-demo/business-clock.json" {
			return false, errors.New("isolated dataset clock requires the local trial and run-scoped clock file")
		}
		return true, nil
	default:
		return false, errors.New("isolated dataset clock flag must be 0 or 1")
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
	mux.Handle("POST /api/v1/local/booking-trial/reservations/{reservationID}/damage-claim/defense/evidence", handler)
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
