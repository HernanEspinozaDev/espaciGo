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
	"regexp"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/database/cloudsql"
	"github.com/HernanEspinozaDev/espaciGo/internal/gcp04w7b1"
	"github.com/jackc/pgx/v5"
)

const credentialMount = "/var/run/secrets/b1/password"

var imagePattern = regexp.MustCompile(`^southamerica-west1-docker\.pkg\.dev/espacigo-testprod/espacigo-testprod/gcp04-w7-b1@sha256:[0-9a-f]{64}$`)

type runtimeConfig struct {
	releaseJSON   []byte
	releaseSHA    string
	secretVersion string
	imageDigest   string
	binarySHA     string
	planSHA       string
	credential    string
	queryPath     string
	markerBucket  string
	markerPrefix  string
}

type claimFunc func(context.Context, gcp04w7b1.RuntimeRelease, string, string) error
type connectFunc func(context.Context, string) (*pgx.Conn, func(), error)

func main() { os.Exit(run()) }

func run() int {
	cfg := runtimeConfig{
		releaseJSON: []byte(os.Getenv("GCP04_B1_RELEASE_JSON")), releaseSHA: os.Getenv("GCP04_B1_RELEASE_SHA256"),
		secretVersion: os.Getenv("GCP04_B1_SECRET_VERSION"), imageDigest: os.Getenv("GCP04_B1_IMAGE_DIGEST"),
		binarySHA: os.Getenv("GCP04_B1_BINARY_SHA256"), planSHA: os.Getenv("GCP04_B1_TERRAFORM_PLAN_SHA256"),
		credential: credentialMount, queryPath: "/gcp04_w7_b1_actor_probe.sql",
		markerBucket: os.Getenv("GCP04_B1_MARKER_BUCKET"), markerPrefix: os.Getenv("GCP04_B1_MARKER_PREFIX"),
	}
	claim := func(ctx context.Context, envelope gcp04w7b1.RuntimeRelease, releaseSHA, planSHA string) error {
		store, err := gcp04w7b1.NewGCSClaimStore(ctx, cfg.markerBucket, cfg.markerPrefix)
		if err != nil {
			return err
		}
		defer store.Close()
		return store.Claim(ctx, envelope, releaseSHA, planSHA)
	}
	return runWith(cfg, claim, connectCloudSQL)
}

// runWith validates every pin and the mounted file before making the durable
// create-only claim. SQL is unreachable unless the GCS marker is confirmed.
func runWith(cfg runtimeConfig, claim claimFunc, connect connectFunc) int {
	now := time.Now()
	envelope, err := gcp04w7b1.ParseRuntimeRelease(cfg.releaseJSON, cfg.releaseSHA, cfg.secretVersion, cfg.imageDigest, cfg.binarySHA, now)
	if err != nil || !imagePattern.MatchString(cfg.imageDigest) || !digest(cfg.planSHA) {
		return fail("approved B1 runtime release or pins unavailable or invalid")
	}
	binary, err := os.ReadFile("/proc/self/exe")
	if err != nil {
		return fail("B1 runtime executable unavailable")
	}
	binarySum := sha256.Sum256(binary)
	clear(binary)
	if hex.EncodeToString(binarySum[:]) != cfg.binarySHA {
		return fail("B1 runtime executable digest mismatch")
	}
	source, err := os.ReadFile(cfg.queryPath)
	if err != nil {
		return fail("pinned B1 query unavailable")
	}
	queryHash := sha256.Sum256(source)
	if hex.EncodeToString(queryHash[:]) != gcp04w7b1.QuerySHA256 {
		return fail("pinned B1 query digest mismatch")
	}
	if _, err := gcp04w7b1.CompileQuery(source); err != nil {
		return fail("pinned B1 query validation failed")
	}
	if err := validateMountedCredential(cfg.credential); err != nil {
		return fail("mounted B1 credential unavailable")
	}
	if claim == nil || connect == nil {
		return fail("B1 runtime dependencies unavailable")
	}
	ctx, cancel := releaseContext(envelope.Release, now)
	defer cancel()
	if err := claim(ctx, envelope, cfg.releaseSHA, cfg.planSHA); err != nil {
		return fail("B1 durable run claim failed; reconcile before retry")
	}

	mounted, err := os.ReadFile(filepath.Clean(cfg.credential))
	if err != nil || len(mounted) == 0 || len(mounted) > 4096 {
		return fail("mounted B1 credential unavailable after run claim; reconcile before retry")
	}
	defer clear(mounted)
	staged, err := stageCredential(mounted)
	if err != nil {
		return fail("private B1 credential staging failed; reconcile before retry")
	}
	defer os.Remove(staged)
	password, err := gcp04w7b1.ReadCredentialFile(staged)
	if err != nil {
		return fail("private B1 credential validation failed; reconcile before retry")
	}
	defer clear(password)

	conn, closeConnector, err := connect(ctx, string(password))
	if err != nil {
		return fail("private Cloud SQL connection failed; reconcile before retry")
	}
	defer closeConnector()
	defer conn.Close(context.Background())
	journal := gcp04w7b1.AppendJournal{Dir: "/tmp/gcp04-w7-b1/journal"}
	result, err := gcp04w7b1.RunProbeOnce(ctx, envelope.Release, conn, source, journal, "/tmp/gcp04-w7-b1/leases", now)
	if err != nil {
		return fail("B1 probe failed; reconcile before any retry")
	}
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		return fail("B1 report output failed")
	}
	return 0
}

func connectCloudSQL(ctx context.Context, password string) (*pgx.Conn, func(), error) {
	dsn := (&url.URL{Scheme: "postgres", User: url.User(gcp04w7b1.Bootstrap), Host: gcp04w7b1.PrivateIP + ":5432", Path: "/" + gcp04w7b1.Database, RawQuery: "sslmode=disable"}).String()
	return cloudsql.Connect(ctx, dsn, password, gcp04w7b1.Connection, true)
}

func releaseContext(release gcp04w7b1.Release, now time.Time) (context.Context, context.CancelFunc) {
	deadline := now.Add(time.Duration(release.MaxDurationSeconds) * time.Second)
	if release.ExpiresAt.Before(deadline) {
		deadline = release.ExpiresAt
	}
	return context.WithDeadline(context.Background(), deadline)
}

func validateMountedCredential(path string) error {
	fi, err := os.Lstat(path)
	if err != nil || !fi.Mode().IsRegular() || fi.Mode()&os.ModeSymlink != 0 || fi.Mode().Perm() != 0400 || fi.Size() == 0 || fi.Size() > 4096 {
		return errors.New("mounted credential must be a root-owned 0400 regular file")
	}
	return nil
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

func digest(s string) bool { return regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(s) }

func fail(message string) int {
	fmt.Fprintln(os.Stderr, message)
	return 1
}
