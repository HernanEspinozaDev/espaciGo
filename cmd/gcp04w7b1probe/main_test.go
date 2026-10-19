package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/gcp04w7b1"
	"github.com/jackc/pgx/v5"
)

type claimRecorder struct {
	calls int
	err   error
}

func (c *claimRecorder) claim(context.Context, gcp04w7b1.RuntimeRelease, string, string) error {
	c.calls++
	return c.err
}

func testConfig(t *testing.T) (runtimeConfig, *claimRecorder, *int) {
	t.Helper()
	exe, err := os.ReadFile("/proc/self/exe")
	if err != nil {
		t.Fatal(err)
	}
	binaryHash := sha256.Sum256(exe)
	clear(exe)
	binarySHA := hex.EncodeToString(binaryHash[:])
	image := "southamerica-west1-docker.pkg.dev/espacigo-testprod/espacigo-testprod/gcp04-w7-b1@sha256:" + strings.Repeat("a", 64)
	envelope := gcp04w7b1.RuntimeRelease{
		Release: gcp04w7b1.Release{Decision: "approved", Stage: "b1_probe", ReleaseID: "b1-run-1", ResourceReleaseID: "b1-run-1", Reviewer: "HernanEspinozaDev", ExpiresAt: time.Now().Add(10 * time.Minute), Target: gcp04w7b1.ProjectID + "/" + gcp04w7b1.Instance, Login: gcp04w7b1.Bootstrap, Role: gcp04w7b1.AdminRole, SecretID: gcp04w7b1.SecretID, QuerySHA256: gcp04w7b1.QuerySHA256, Runs: 1, MaxDurationSeconds: 120, SecretETag: "secret-etag", CreateOperationID: "create-op-1", Version: "7", VersionETag: "version-etag"},
		Pins:    gcp04w7b1.RuntimePins{ImageDigest: image, BinarySHA256: binarySHA},
	}
	data, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	query := filepath.Join("..", "..", "db", "query", "gcp04_w7_b1_actor_probe.sql")
	claim := &claimRecorder{}
	connected := 0
	return runtimeConfig{releaseJSON: data, releaseSHA: gcp04w7b1.ReleaseDigest(data), secretVersion: "7", imageDigest: image, binarySHA: binarySHA, planSHA: strings.Repeat("c", 64), credential: filepath.Join(t.TempDir(), "missing-secret"), queryPath: query}, claim, &connected
}

func noConnect(connected *int) connectFunc {
	return func(context.Context, string) (*pgx.Conn, func(), error) {
		*connected++
		return nil, nil, errors.New("unexpected connector invocation")
	}
}

func TestRuntimeGatesRejectInvalidReleaseBeforeClaimOrConnector(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*runtimeConfig)
	}{
		{name: "release digest", mutate: func(c *runtimeConfig) { c.releaseSHA = strings.Repeat("0", 64) }},
		{name: "secret version mismatch", mutate: func(c *runtimeConfig) { c.secretVersion = "8" }},
		{name: "image pin mismatch", mutate: func(c *runtimeConfig) {
			c.imageDigest = strings.Replace(c.imageDigest, strings.Repeat("a", 64), strings.Repeat("d", 64), 1)
		}},
		{name: "binary pin malformed", mutate: func(c *runtimeConfig) { c.binarySHA = "bad" }},
		{name: "execution plan hash malformed", mutate: func(c *runtimeConfig) { c.planSHA = "bad" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, claim, connected := testConfig(t)
			tc.mutate(&cfg)
			if got := runWith(cfg, claim.claim, noConnect(connected)); got != 1 {
				t.Fatalf("exit=%d", got)
			}
			if claim.calls != 0 || *connected != 0 {
				t.Fatalf("gate failure reached claim/connector: claims=%d connects=%d", claim.calls, *connected)
			}
		})
	}
}

func TestRuntimeGatesRejectExpiredReleaseBeforeClaimOrConnector(t *testing.T) {
	cfg, claim, connected := testConfig(t)
	var envelope gcp04w7b1.RuntimeRelease
	if err := json.Unmarshal(cfg.releaseJSON, &envelope); err != nil {
		t.Fatal(err)
	}
	envelope.Release.ExpiresAt = time.Now().Add(-time.Minute)
	cfg.releaseJSON, _ = json.Marshal(envelope)
	cfg.releaseSHA = gcp04w7b1.ReleaseDigest(cfg.releaseJSON)
	if got := runWith(cfg, claim.claim, noConnect(connected)); got != 1 {
		t.Fatalf("exit=%d", got)
	}
	if claim.calls != 0 || *connected != 0 {
		t.Fatalf("expired release reached claim/connector: claims=%d connects=%d", claim.calls, *connected)
	}
}

func TestRuntimeCredentialFileGatesRejectMissingSymlinkAndWrongMode(t *testing.T) {
	for _, kind := range []string{"missing", "symlink", "0444", "directory"} {
		t.Run(kind, func(t *testing.T) {
			cfg, claim, connected := testConfig(t)
			dir := t.TempDir()
			path := filepath.Join(dir, "password")
			switch kind {
			case "symlink":
				target := filepath.Join(dir, "target")
				if err := os.WriteFile(target, []byte("fixture"), 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, path); err != nil {
					t.Fatal(err)
				}
			case "0444":
				if err := os.WriteFile(path, []byte("fixture"), 0400); err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod(path, 0444); err != nil {
					t.Fatal(err)
				}
			case "directory":
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			}
			cfg.credential = path
			if got := runWith(cfg, claim.claim, noConnect(connected)); got != 1 {
				t.Fatalf("exit=%d", got)
			}
			if claim.calls != 0 || *connected != 0 {
				t.Fatalf("invalid file reached claim/connector: claims=%d connects=%d", claim.calls, *connected)
			}
		})
	}
}

func TestRuntimeNeverConnectsWhenDurableClaimIsUncertain(t *testing.T) {
	cfg, claim, connected := testConfig(t)
	path := filepath.Join(t.TempDir(), "password")
	if err := os.WriteFile(path, []byte("fixture"), 0400); err != nil {
		t.Fatal(err)
	}
	cfg.credential = path
	claim.err = gcp04w7b1.ErrRunClaimUncertain
	if got := runWith(cfg, claim.claim, noConnect(connected)); got != 1 {
		t.Fatalf("exit=%d", got)
	}
	if claim.calls != 1 || *connected != 0 {
		t.Fatalf("uncertain claim reached connector: claims=%d connects=%d", claim.calls, *connected)
	}
}
