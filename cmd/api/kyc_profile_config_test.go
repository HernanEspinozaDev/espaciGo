package main

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/devauth"
	password "github.com/HernanEspinozaDev/espaciGo/internal/adapters/password/bcrypt"
	identitypg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/identity"
	identitydeclarationpg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/identitydeclaration"
	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/zohoauth"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
)

func TestLoadRuntimeConfigAllowsPrivatePassfileForLocalKYCProfile(t *testing.T) {
	passfile := filepath.Join(privatePassfileDir(t), "api.pgpass")
	if err := os.WriteFile(passfile, []byte("127.0.0.1:5432:espacigo_dms_stage_20261020:espacigo_api_login:synthetic-only"), 0o600); err != nil {
		t.Fatal(err)
	}
	values := map[string]string{
		"PGPASSFILE":               passfile,
		"DATABASE_CONNECTION_MODE": "direct",
		"DATABASE_HOST":            "127.0.0.1",
		"DATABASE_PORT":            "5432",
		"DATABASE_NAME":            "espacigo_dms_stage_20261020",
		"DATABASE_USER":            "espacigo_api_login",
		"DATABASE_SET_ROLE":        "espacigo_runtime",
		"HTTP_ADDR":                "127.0.0.1:18090",
	}
	cfg, err := loadRuntimeConfig(func(key string) string { return values[key] }, runtimeProfileKYCWorkerFree)
	if err != nil {
		t.Fatalf("loadRuntimeConfig rejected the local KYC API passfile: %v", err)
	}
	cfg.close()
}

func TestLocalKYCProfileStartupPreflight(t *testing.T) {
	if os.Getenv("LOCAL_KYC_STARTUP_PREFLIGHT") != "1" {
		t.Skip("explicitly opt in to read-only local KYC startup preflight")
	}
	profile, err := runtimeProfile(os.Getenv)
	if err != nil || profile != runtimeProfileKYCWorkerFree {
		t.Fatalf("stage=profile error_present=%t profile_matches=%t", err != nil, profile == runtimeProfileKYCWorkerFree)
	}
	cfg, err := loadRuntimeConfig(os.Getenv, profile)
	if err != nil {
		t.Fatalf("stage=config error=%q", err.Error())
	}
	defer cfg.close()
	if keyring, err := loadIdentityDeclarationKeyring(os.Getenv); err != nil || keyring == nil {
		t.Fatalf("stage=keyring error_present=%t keyring_present=%t", err != nil, keyring != nil)
	}
	limit, err := strconv.Atoi(os.Getenv("LOCAL_VERIFICATION_IP_LIMIT"))
	if err != nil || limit < 1 || limit > 1000 {
		t.Fatalf("stage=local-quota error_present=%t", err != nil)
	}
	pool, closeConnector, err := newDatabasePool(context.Background(), cfg)
	if err != nil {
		t.Fatal("stage=database-pool error_present=true")
	}
	defer closeConnector()
	defer pool.Close()
	if err := pool.Ping(context.Background()); err != nil {
		t.Fatal("stage=database-ping error_present=true")
	}
	var databaseName, sessionUser, currentUser, serverAddress string
	var serverPort int
	if err := pool.QueryRow(context.Background(), `SELECT current_database(),session_user,current_user,inet_server_addr()::text,inet_server_port()`).Scan(&databaseName, &sessionUser, &currentUser, &serverAddress, &serverPort); err != nil {
		t.Fatal("stage=database-target-readback error_present=true")
	}
	serverIP, _, cidrErr := net.ParseCIDR(serverAddress)
	if cidrErr != nil {
		serverIP = net.ParseIP(serverAddress)
	}
	addressMatch := serverIP != nil && serverIP.To4() != nil && serverIP.To4().IsLoopback()
	if databaseName != "espacigo_dms_stage_20261020" || sessionUser != "espacigo_api_login" || currentUser != "espacigo_runtime" || !addressMatch || serverPort != 5432 {
		t.Fatalf("stage=database-target-readback database_match=%t login_match=%t role_match=%t address_match=%t port_match=%t", databaseName == "espacigo_dms_stage_20261020", sessionUser == "espacigo_api_login", currentUser == "espacigo_runtime", addressMatch, serverPort == 5432)
	}
	authService, err := identity.NewAuthenticationService(identitypg.NewIdentityRepository(pool), password.Hasher{}, zohoauth.UnavailableMailer{}, devauth.NewIPLimiter(limit), credentials.Generator{}, time.Now)
	if err != nil || authService == nil {
		t.Fatalf("stage=auth-service error_present=%t service_present=%t", err != nil, authService != nil)
	}
	declarationRepository := identitydeclarationpg.New(pool)
	declarationService, err := newRuntimeIdentityDeclarationService(os.Getenv, declarationRepository, time.Now)
	if err != nil || declarationService == nil {
		t.Fatalf("stage=declaration-service error_present=%t service_present=%t", err != nil, declarationService != nil)
	}
}
