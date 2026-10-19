package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/database/cloudsql"
	"github.com/HernanEspinozaDev/espaciGo/internal/gcp04w7b1"
)

const credentialMount = "/var/run/secrets/b1/password"

func main() {
	os.Exit(run())
}

// The runtime accepts one approved probe release through environment overrides.
// It has no Cloud SQL Admin or Secret Manager API client; its only cloud access
// is the connector's Cloud SQL client permission.
func run() int {
	raw := []byte(os.Getenv("GCP04_B1_RELEASE_JSON"))
	digest := os.Getenv("GCP04_B1_RELEASE_SHA256")
	release, err := gcp04w7b1.ParseRelease(raw, digest, "b1_probe", time.Now())
	clear(raw)
	if err != nil {
		return fail("approved B1 probe release unavailable or invalid")
	}

	mounted, err := readMountedCredential(credentialMount)
	if err != nil {
		return fail("mounted B1 credential unavailable")
	}
	defer clear(mounted)
	staged, err := stageCredential(mounted)
	if err != nil {
		return fail("private B1 credential staging failed")
	}
	defer os.Remove(staged)
	password, err := gcp04w7b1.ReadCredentialFile(staged)
	if err != nil {
		return fail("private B1 credential validation failed")
	}
	defer clear(password)

	now := time.Now()
	deadline := now.Add(time.Duration(release.MaxDurationSeconds) * time.Second)
	if release.ExpiresAt.Before(deadline) {
		deadline = release.ExpiresAt
	}
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	dsn := (&url.URL{Scheme: "postgres", User: url.User(gcp04w7b1.Bootstrap), Host: gcp04w7b1.PrivateIP + ":5432", Path: "/" + gcp04w7b1.Database, RawQuery: "sslmode=disable"}).String()
	conn, closeConnector, err := cloudsql.Connect(ctx, dsn, string(password), gcp04w7b1.Connection, true)
	if err != nil {
		return fail("private Cloud SQL connection failed")
	}
	defer closeConnector()
	defer conn.Close(context.Background())

	source, err := os.ReadFile("/gcp04_w7_b1_actor_probe.sql")
	if err != nil {
		return fail("pinned B1 query unavailable")
	}
	queryHash := sha256.Sum256(source)
	if hex.EncodeToString(queryHash[:]) != gcp04w7b1.QuerySHA256 {
		return fail("pinned B1 query digest mismatch")
	}
	journal := gcp04w7b1.AppendJournal{Dir: "/tmp/gcp04-w7-b1/journal"}
	result, err := gcp04w7b1.RunProbeOnce(ctx, release, conn, source, journal, "/tmp/gcp04-w7-b1/leases", now)
	if err != nil {
		return fail("B1 probe failed; reconcile before any retry")
	}
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		return fail("B1 report output failed")
	}
	return 0
}

func readMountedCredential(path string) ([]byte, error) {
	fi, err := os.Lstat(path)
	if err != nil || !fi.Mode().IsRegular() || fi.Mode()&os.ModeSymlink != 0 || fi.Mode().Perm() != 0400 || fi.Size() == 0 || fi.Size() > 4096 {
		return nil, errors.New("mounted credential must be a root-owned 0400 regular file")
	}
	return os.ReadFile(filepath.Clean(path))
}

func stageCredential(secret []byte) (string, error) {
	f, err := os.CreateTemp("/tmp", "gcp04-w7-b1-credential-")
	if err != nil {
		return "", err
	}
	path := f.Name()
	ok := false
	defer func() {
		_ = f.Close()
		if !ok {
			_ = os.Remove(path)
		}
	}()
	if err := f.Chmod(0600); err != nil {
		return "", err
	}
	if _, err := f.Write(secret); err != nil {
		return "", err
	}
	if err := f.Sync(); err != nil {
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	ok = true
	return path, nil
}

func fail(message string) int {
	fmt.Fprintln(os.Stderr, message)
	return 1
}
