package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
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
		return errors.New("usage: gadm-admin bootstrap --account-id UUID | recover --account-id UUID")
	}
	command := args[0]
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	target := flags.String("account-id", "", "verified account UUID")
	credentialFile := flags.String("credentials", ".local/testing/gadm01.env", "0600 local test credential file")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 || command != "bootstrap" && command != "recover" || *target == "" {
		return errors.New("bootstrap/recover requires --account-id UUID")
	}
	root, err := filepath.Abs(*credentialFile)
	if err != nil {
		return errors.New("invalid credential file path")
	}
	localDir, _ := filepath.Abs(".local/testing")
	if !strings.HasPrefix(root, localDir+string(os.PathSeparator)) {
		return errors.New("credential file must be under .local/testing")
	}
	info, err := os.Stat(root)
	if err != nil {
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
	if err = adminglobal.ValidateBootstrapTarget(os.Getenv("GADM_BOOTSTRAP_ENV"), dsn, dbName); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return errors.New("invalid local test database config")
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return errors.New("unable to connect to local test database")
	}
	defer pool.Close()
	var actual string
	if err = pool.QueryRow(ctx, "SELECT current_database()").Scan(&actual); err != nil || actual != dbName {
		return errors.New("connected database does not match explicit test target")
	}
	correlation, err := (credentials.Generator{}).ID()
	if err != nil {
		return errors.New("unable to create audit correlation")
	}
	if err = adminglobal.Bootstrap(ctx, pool, *target, command, correlation); err != nil {
		return err
	}
	fmt.Println("global administrator bootstrap completed")
	return nil
}
