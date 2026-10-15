package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
	"github.com/HernanEspinozaDev/espaciGo/internal/adminglobal"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: gadm-admin activate-local --action bootstrap|recover --account-id UUID --database espacigo_local --confirm 'ACTIVATE <action> <UUID> IN espacigo_local'")
	}
	command := args[0]
	if command != "activate-local" && command != "bootstrap" && command != "recover" {
		return errors.New("unknown command; use activate-local or local-test bootstrap/recover")
	}
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	target := flags.String("account-id", "", "verified account UUID")
	action := flags.String("action", "", "bootstrap or recover (activate-local)")
	database := flags.String("database", "", "explicit database name (activate-local)")
	confirmation := flags.String("confirm", "", "exact activation confirmation phrase")
	credentialFile := flags.String("credentials", ".local/testing/gadm01.env", "0600 local credential file")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 || *target == "" {
		return errors.New("a valid --account-id UUID is required")
	}
	bootstrapAction := command
	if command == "activate-local" {
		bootstrapAction = *action
		if (bootstrapAction != "bootstrap" && bootstrapAction != "recover") || *database != "espacigo_local" || *confirmation != fmt.Sprintf("ACTIVATE %s %s IN %s", bootstrapAction, *target, *database) {
			return errors.New("activation requires --action bootstrap|recover, --database espacigo_local, and the exact confirmation phrase")
		}
	} else if _, ok := actionForCommand(command); !ok || os.Getenv("GADM_BOOTSTRAP_ENV") != "local-test" {
		return errors.New("bootstrap/recover are restricted to local-test mode")
	}
	root, err := filepath.Abs(*credentialFile)
	if err != nil {
		return errors.New("invalid credential file path")
	}
	localDir, _ := filepath.Abs(".local/testing")
	if !strings.HasPrefix(root, localDir+string(os.PathSeparator)) {
		return errors.New("credential file must be under .local/testing")
	}
	info, err := os.Lstat(root)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("unable to read local test credentials")
	}
	if info.Mode().Perm() != 0600 {
		return errors.New("local test credential file must have mode 0600")
	}
	values := map[string]string{}
	data, err := os.ReadFile(root)
	if err != nil {
		return errors.New("unable to read local test credentials")
	}
	for _, line := range strings.Split(string(data), "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if ok {
			values[key] = strings.TrimSpace(value)
		}
	}
	dsn, dbName := values["DATABASE_URL"], values["EXPECTED_DATABASE"]
	if command == "activate-local" {
		err = adminglobal.ValidateLocalActivationTarget(os.Getenv("GADM_BOOTSTRAP_ENV"), dsn, dbName)
	} else {
		err = adminglobal.ValidateBootstrapTarget(os.Getenv("GADM_BOOTSTRAP_ENV"), dsn, dbName)
	}
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return errors.New("invalid local test database config")
	}
	if command == "activate-local" {
		cfg.AfterConnect = adminglobal.SetRestrictedLocalBootstrapRole
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return errors.New("unable to connect to local test database")
	}
	defer pool.Close()
	if command == "activate-local" {
		connection, acquireErr := pool.Acquire(ctx)
		if acquireErr != nil {
			return errors.New("activate-local requires a temporary login with only the restricted GADM bootstrap role")
		}
		connection.Release()
	}
	var actual string
	var serverAddress, clientAddress net.IP
	if err = pool.QueryRow(ctx, "SELECT current_database(), inet_server_addr(), inet_client_addr()").Scan(&actual, &serverAddress, &clientAddress); err != nil || actual != dbName {
		return errors.New("connected database does not match explicit target")
	}
	if command == "activate-local" && (!adminglobal.IsLocalDatabaseAddress(serverAddress) || !adminglobal.IsLocalDatabaseAddress(clientAddress)) {
		return errors.New("connected database endpoint is not loopback")
	}
	correlation, err := (credentials.Generator{}).ID()
	if err != nil {
		return errors.New("unable to create audit correlation")
	}
	if err = adminglobal.Bootstrap(ctx, pool, *target, bootstrapAction, correlation); err != nil {
		return err
	}
	fmt.Printf("global administrator %s completed for %s\n", bootstrapAction, dbName)
	return nil
}

func actionForCommand(command string) (string, bool) {
	switch command {
	case "bootstrap":
		return "bootstrap", true
	case "recover":
		return "recover", true
	default:
		return "", false
	}
}
