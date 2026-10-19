package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/database/cloudsql"
	"github.com/HernanEspinozaDev/espaciGo/internal/gcp04w7b1"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/secretmanager/v1"
	"google.golang.org/api/sqladmin/v1beta4"
)

const scope = "https://www.googleapis.com/auth/cloud-platform"

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, out, errOut io.Writer) int {
	f := flag.NewFlagSet("gcp04w7b1", flag.ContinueOnError)
	f.SetOutput(errOut)
	mode := f.String("mode", "plan", "plan (offline default), create, probe, reconcile, or cleanup")
	releasePath := f.String("release-file", "", "private stage-specific release under .local/testing/gcp04-w7-b1")
	confirm := f.String("confirm-release-sha256", "", "exact SHA-256 of release file")
	executeCreate := f.Bool("execute-b1-create", false, "required in addition to a valid b1_create release")
	executeCleanup := f.Bool("execute-b1-cleanup", false, "required in addition to a valid b1_cleanup release")
	passwordFile := f.String("password-file", "", "protected mounted credential file; never pass the payload in argv")
	queryPath := f.String("query-file", "db/query/gcp04_w7_b1_actor_probe.sql", "pinned B1 SQL source")
	if f.Parse(args) != nil || f.NArg() != 0 {
		return 2
	}
	if *mode == "plan" {
		if *releasePath != "" || *confirm != "" || *passwordFile != "" || *executeCreate || *executeCleanup {
			fmt.Fprintln(errOut, "plan mode is offline and accepts no release or credential")
			return 2
		}
		_ = json.NewEncoder(out).Encode(gcp04w7b1.StaticPlan())
		return 0
	}
	if *releasePath == "" || *confirm == "" {
		fmt.Fprintln(errOut, "an exact private stage release and SHA-256 confirmation are required")
		return 2
	}
	if (*mode == "create" && !*executeCreate) || (*mode == "cleanup" && !*executeCleanup) || (*mode != "create" && *executeCreate) || (*mode != "cleanup" && *executeCleanup) {
		fmt.Fprintln(errOut, "the requested B1 stage switch is missing or mismatched")
		return 2
	}
	stage := map[string]string{"create": "b1_create", "probe": "b1_probe", "reconcile": "b1_reconcile", "cleanup": "b1_cleanup"}[*mode]
	if stage == "" {
		fmt.Fprintln(errOut, "unsupported B1 mode")
		return 2
	}
	releaseBytes, err := readReleaseFile(*releasePath)
	if err != nil {
		fmt.Fprintln(errOut, "B1 release file unavailable or not private")
		return 1
	}
	release, err := gcp04w7b1.ParseRelease(releaseBytes, *confirm, stage, time.Now())
	clear(releaseBytes)
	if err != nil {
		fmt.Fprintln(errOut, "B1 release is invalid, expired, or not exact for this stage")
		return 1
	}
	if *mode == "probe" && *passwordFile == "" {
		fmt.Fprintln(errOut, "probe requires a protected mounted credential file")
		return 2
	}
	if *mode != "probe" && *passwordFile != "" {
		fmt.Fprintln(errOut, "credential file is accepted only by the probe stage")
		return 2
	}
	ctx, cancel := boundedReleaseContext(context.Background(), release, time.Now())
	defer cancel()
	api, err := newCloudAPI(ctx)
	if err != nil {
		fmt.Fprintln(errOut, "authorized Cloud API client unavailable")
		return 1
	}
	inv, err := gcp04w7b1.Collect(ctx, api)
	if err != nil {
		fmt.Fprintln(errOut, "read-only B1 target inventory failed")
		return 1
	}
	journal := gcp04w7b1.AppendJournal{Dir: ".local/testing/gcp04-w7-b1/journal"}
	leases := ".local/testing/gcp04-w7-b1/leases"
	switch *mode {
	case "create":
		if _, err = gcp04w7b1.BuildPlan(inv); err != nil {
			fmt.Fprintln(errOut, "B1 creation collision or target drift; no resource adopted")
			return 1
		}
		version, createErr := gcp04w7b1.CreateB1(ctx, inv, release, api, api, journal, leases, time.Now(), nil)
		if createErr != nil {
			fmt.Fprintln(errOut, "B1 create incomplete or uncertain; reconcile exact run before any retry")
			return 1
		}
		resourceReleaseID := release.ResourceReleaseID
		if resourceReleaseID == "" {
			resourceReleaseID = release.ReleaseID
		}
		records, journalErr := journal.ReadRecords(resourceReleaseID)
		operationID, proofErr := gcp04w7b1.CreateOperationIDFromJournal(records, resourceReleaseID)
		if journalErr != nil || proofErr != nil {
			fmt.Fprintln(errOut, "B1 created but exact operation proof could not be recovered from the private journal")
			return 1
		}
		created, inventoryErr := gcp04w7b1.Collect(ctx, api)
		status, reconcileErr := gcp04w7b1.ReconcileInventory(created, resourceReleaseID, version)
		if inventoryErr != nil || reconcileErr != nil || status["status"] != "bootstrap_actor_created_probe_not_accredited" || created.Secret.ETag == "" || len(created.Versions) != 1 || created.Versions[0].Number != version || created.Versions[0].State != "ENABLED" || created.Versions[0].ETag == "" {
			fmt.Fprintln(errOut, "B1 created but exact post-create resource inventory could not be reconciled")
			return 1
		}
		_ = json.NewEncoder(out).Encode(map[string]any{"format": "espacigo.gcp04.w7.b1-create.v1", "status": "created", "login": gcp04w7b1.Bootstrap, "database_role_requested": gcp04w7b1.AdminRole, "secret_resource": parent(), "secret_etag": created.Secret.ETag, "secret_version": version, "secret_version_etag": created.Versions[0].ETag, "created_secret_version_etag": created.Versions[0].ETag, "create_operation_id": operationID, "mutation_authorized": true})
		return 0
	case "probe":
		status, err := gcp04w7b1.ReconcileInventory(inv, release.ResourceReleaseID, release.Version)
		if err != nil || status["status"] != "bootstrap_actor_created_probe_not_accredited" || inv.Secret.ETag != release.SecretETag || len(inv.Versions) != 1 || inv.Versions[0].ETag != release.VersionETag {
			fmt.Fprintln(errOut, "B1 resource identity or inventory does not match probe release")
			return 1
		}
		password, err := gcp04w7b1.ReadCredentialFile(*passwordFile)
		if err != nil {
			fmt.Fprintln(errOut, "B1 credential file is not protected or unavailable")
			return 1
		}
		defer clear(password)
		source, err := os.ReadFile(*queryPath)
		if err != nil {
			fmt.Fprintln(errOut, "B1 SQL source unavailable")
			return 1
		}
		dsn := "postgres://" + url.PathEscape(gcp04w7b1.Bootstrap) + "@localhost:5432/" + url.PathEscape(gcp04w7b1.Database) + "?sslmode=disable"
		conn, closeConnector, err := cloudsql.Connect(ctx, dsn, string(password), gcp04w7b1.Connection, true)
		if err != nil {
			fmt.Fprintln(errOut, "B1 private SQL connection failed")
			return 1
		}
		defer closeConnector()
		defer conn.Close(context.Background())
		result, err := gcp04w7b1.RunProbeOnce(ctx, release, conn, source, journal, leases, time.Now())
		if err != nil {
			fmt.Fprintln(errOut, "B1 probe failed or is incomplete; reconcile before retry")
			return 1
		}
		_ = json.NewEncoder(out).Encode(result)
		return 0
	case "reconcile":
		result, err := gcp04w7b1.ReconcileInventory(inv, release.ResourceReleaseID, release.Version)
		if err != nil {
			fmt.Fprintln(errOut, "B1 inventory is inconsistent")
			return 1
		}
		_ = json.NewEncoder(out).Encode(result)
		return 0
	case "cleanup":
		if err = gcp04w7b1.CleanupB1(ctx, inv, release, api, api, api, journal, leases, time.Now()); err != nil {
			fmt.Fprintln(errOut, "B1 cleanup failed or is uncertain; reconcile exact user, secret, and version before retry")
			return 1
		}
		_ = json.NewEncoder(out).Encode(map[string]any{"format": "espacigo.gcp04.w7.b1-cleanup.v1", "status": "user_delete_done_absent_secret_version_destroyed_shell_absent", "login": gcp04w7b1.Bootstrap, "secret_resource": parent(), "secret_version": release.Version})
		return 0
	default:
		return 2
	}
}

func boundedReleaseContext(ctx context.Context, release gcp04w7b1.Release, now time.Time) (context.Context, context.CancelFunc) {
	deadline := now.Add(time.Duration(release.MaxDurationSeconds) * time.Second)
	if release.ExpiresAt.Before(deadline) {
		deadline = release.ExpiresAt
	}
	if parent, ok := ctx.Deadline(); ok && parent.Before(deadline) {
		deadline = parent
	}
	return context.WithDeadline(ctx, deadline)
}

func newCloudAPI(ctx context.Context) (cloudAPI, error) {
	cred, err := google.FindDefaultCredentials(ctx, scope)
	if err != nil || cred == nil {
		return cloudAPI{}, errors.New("ADC unavailable")
	}
	opts := []option.ClientOption{option.WithCredentials(cred), option.WithQuotaProject(gcp04w7b1.ProjectID)}
	sql, err := sqladmin.NewService(ctx, opts...)
	if err != nil {
		return cloudAPI{}, err
	}
	sm, err := secretmanager.NewService(ctx, opts...)
	if err != nil {
		return cloudAPI{}, err
	}
	return cloudAPI{sql: sql, sm: sm}, nil
}
func readReleaseFile(path string) ([]byte, error) {
	if path == "" {
		return nil, errors.New("release must be stored in private W7 directory")
	}
	root, err := filepath.Abs(".local/testing/gcp04-w7-b1")
	if err != nil {
		return nil, err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	abs, err = filepath.EvalSymlinks(abs)
	if err != nil {
		return nil, err
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || filepath.IsAbs(rel) {
		return nil, errors.New("release must be stored in private W7 directory")
	}
	return readPrivateFile(abs, 64<<10)
}
func readPrivateFile(path string, limit int64) ([]byte, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	fi, err := os.Lstat(abs)
	if err != nil || !fi.Mode().IsRegular() || fi.Mode()&os.ModeSymlink != 0 || fi.Mode().Perm() != 0600 || fi.Size() == 0 || fi.Size() > limit {
		return nil, errors.New("private file mode or size invalid")
	}
	b, err := os.ReadFile(abs)
	if err != nil || len(b) == 0 || int64(len(b)) > limit {
		return nil, errors.New("private file read failed")
	}
	return b, nil
}
