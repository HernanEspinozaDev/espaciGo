// Command gcp06-sql-inspect performs a fixed catalog-only read transaction on
// the GCP-06 candidate. It has no SQL-input, DDL, DML, or automatic retry path.
package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"regexp"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/database/cloudsql"
	"github.com/HernanEspinozaDev/espaciGo/internal/gcp06sqlinspect"
)

var credentialLoginPattern = regexp.MustCompile(`^gcp06_inspect_[a-f0-9]{12}$`)

func main() {
	if err := run(os.Args[1:], os.Getenv, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "GCP-06 SQL catalog inspection failed:", err)
		os.Exit(1)
	}
}

func run(args []string, getenv func(string) string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("gcp06-sql-inspect", flag.ContinueOnError)
	flags.SetOutput(stderr)
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected positional arguments")
	}
	if getenv("GCP06_RUN_ID") != gcp06sqlinspect.RunID || getenv("CLOUD_SQL_INSTANCE_CONNECTION_NAME") != gcp06sqlinspect.ConnectionName || getenv("DATABASE_NAME") != gcp06sqlinspect.DatabaseName {
		return errors.New("SQL inspector target must match the pinned GCP-06 candidate")
	}
	if err := cloudsql.ValidatePGEnvironment(getenv); err != nil {
		return errors.New("PostgreSQL environment overrides are not allowed")
	}
	credentials, err := readCredentials(gcp06sqlinspect.CredentialFile)
	if err != nil {
		return errors.New("one-time SQL inspector credential mount is unavailable")
	}
	defer clear(credentials.Password)
	if err := gcp06sqlinspect.ValidateTarget(
		getenv("GCP06_RUN_ID"), getenv("CLOUD_SQL_INSTANCE_CONNECTION_NAME"), getenv("DATABASE_NAME"), credentials.Username,
	); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	conn, closeConnector, err := cloudsql.Connect(ctx, databaseURL(credentials.Username), string(credentials.Password), gcp06sqlinspect.ConnectionName, true)
	if err != nil {
		return errors.New("private candidate connection failed")
	}
	defer closeConnector()
	defer conn.Close(context.Background())
	report, err := gcp06sqlinspect.Inspect(ctx, conn, credentials.Username)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(stdout)
	encoder.SetEscapeHTML(true)
	if err := encoder.Encode(report); err != nil {
		return errors.New("could not encode sanitized catalog evidence")
	}
	return nil
}

func readCredentials(path string) (gcp06sqlinspect.Credentials, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0400 || info.Size() < 1 || info.Size() > 4096 {
		return gcp06sqlinspect.Credentials{}, errors.New("credential must be a bounded regular mode-0400 Secret Manager mount")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return gcp06sqlinspect.Credentials{}, errors.New("credential mount cannot be read")
	}
	defer clear(data)
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var envelope struct {
		Username string          `json:"username"`
		Password json.RawMessage `json:"password"`
	}
	if err := decoder.Decode(&envelope); err != nil {
		return gcp06sqlinspect.Credentials{}, errors.New("credential mount has invalid JSON")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return gcp06sqlinspect.Credentials{}, errors.New("credential mount must contain one JSON object")
	}
	if !credentialLoginPattern.MatchString(envelope.Username) || len(envelope.Password) != 45 || envelope.Password[0] != '"' || envelope.Password[len(envelope.Password)-1] != '"' {
		clear(envelope.Password)
		return gcp06sqlinspect.Credentials{}, errors.New("credential mount does not satisfy the one-run login contract")
	}
	password := append([]byte(nil), envelope.Password[1:len(envelope.Password)-1]...)
	clear(envelope.Password)
	if len(password) != 43 {
		clear(password)
		return gcp06sqlinspect.Credentials{}, errors.New("credential mount does not satisfy the one-run password contract")
	}
	for _, char := range password {
		if !((char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '_' || char == '-') {
			clear(password)
			return gcp06sqlinspect.Credentials{}, errors.New("credential mount does not satisfy the one-run password contract")
		}
	}
	decoded := make([]byte, 32)
	decodedLength, decodeErr := base64.RawURLEncoding.Strict().Decode(decoded, password)
	clear(decoded)
	if decodeErr != nil || decodedLength != 32 {
		clear(password)
		return gcp06sqlinspect.Credentials{}, errors.New("credential mount does not satisfy the one-run password contract")
	}
	return gcp06sqlinspect.Credentials{Username: envelope.Username, Password: password}, nil
}

func databaseURL(login string) string {
	return (&url.URL{Scheme: "postgres", User: url.User(login), Host: "localhost:5432", Path: "/" + gcp06sqlinspect.DatabaseName, RawQuery: "sslmode=disable"}).String()
}
