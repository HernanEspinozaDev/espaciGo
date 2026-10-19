package gcp04w7b1

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"google.golang.org/api/googleapi"
)

var (
	ErrRunAlreadyClaimed = errors.New("W7 B1 runtime release is already claimed")
	ErrRunClaimUncertain = errors.New("W7 B1 runtime claim result is uncertain; reconcile before retry")
	runtimeDigestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

type RuntimePins struct {
	ImageDigest  string `json:"image_digest"`
	BinarySHA256 string `json:"binary_sha256"`
}

type RuntimeRelease struct {
	Release Release     `json:"release"`
	Pins    RuntimePins `json:"pins"`
}

// ParseRuntimeRelease validates the private release envelope and all pins that
// must agree with the mounted secret and Cloud Run image before any GCS/SQL IO.
func ParseRuntimeRelease(data []byte, confirmation, secretVersion, imageDigest, binarySHA256 string, now time.Time) (RuntimeRelease, error) {
	var envelope RuntimeRelease
	if len(data) == 0 || ReleaseDigest(data) != strings.ToLower(strings.TrimSpace(confirmation)) {
		return RuntimeRelease{}, errors.New("B1 runtime release digest mismatch")
	}
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&envelope); err != nil || dec.Decode(&struct{}{}) != io.EOF {
		return RuntimeRelease{}, errors.New("B1 runtime release envelope invalid")
	}
	if err := ValidateRelease(envelope.Release, "b1_probe", now); err != nil {
		return RuntimeRelease{}, err
	}
	if secretVersion == "" || envelope.Release.Version != secretVersion || envelope.Release.VersionETag == "" ||
		envelope.Release.ReleaseID != envelope.Release.ResourceReleaseID ||
		envelope.Pins.ImageDigest == "" || envelope.Pins.ImageDigest != imageDigest ||
		!runtimeDigestPattern.MatchString(envelope.Pins.BinarySHA256) || envelope.Pins.BinarySHA256 != binarySHA256 {
		return RuntimeRelease{}, errors.New("B1 runtime release pins differ from the job configuration")
	}
	return envelope, nil
}

type RunClaim struct {
	ReleaseID         string `json:"release_id"`
	ResourceReleaseID string `json:"resource_release_id"`
	ReleaseSHA256     string `json:"release_sha256"`
	Stage             string `json:"stage"`
	Reviewer          string `json:"reviewer"`
	ExpiresAt         string `json:"expires_at"`
	CreateOperationID string `json:"create_operation_id"`
	Target            string `json:"target"`
	Database          string `json:"database"`
	Login             string `json:"login"`
	DatabaseRole      string `json:"database_role"`
	SecretID          string `json:"secret_id"`
	SecretVersion     string `json:"secret_version"`
	SecretETag        string `json:"secret_etag"`
	VersionETag       string `json:"version_etag"`
	QuerySHA256       string `json:"query_sha256"`
	ImageDigest       string `json:"image_digest"`
	BinarySHA256      string `json:"binary_sha256"`
	PlanSHA256        string `json:"terraform_plan_sha256"`
}

type ClaimStore interface {
	Create(context.Context, string, []byte) error
}

type GCSClaimStore struct {
	client *storage.Client
	bucket string
	prefix string
}

func NewGCSClaimStore(ctx context.Context, bucket, prefix string) (*GCSClaimStore, error) {
	if bucket == "" || !safeObjectPrefix(prefix) {
		return nil, errors.New("W7 B1 run marker bucket and exact prefix required")
	}
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, errors.New("W7 B1 run marker store unavailable")
	}
	return &GCSClaimStore{client: client, bucket: bucket, prefix: prefix}, nil
}

func (s *GCSClaimStore) Close() error {
	if s == nil || s.client == nil {
		return nil
	}
	return s.client.Close()
}

func (s *GCSClaimStore) Claim(ctx context.Context, envelope RuntimeRelease, releaseSHA, planSHA string) error {
	if s == nil || s.client == nil || !safeID(envelope.Release.ResourceReleaseID) ||
		!runtimeDigestPattern.MatchString(releaseSHA) || !runtimeDigestPattern.MatchString(planSHA) {
		return errors.New("W7 B1 run claim pins invalid")
	}
	r := envelope.Release
	claim := RunClaim{ReleaseID: r.ReleaseID, ResourceReleaseID: r.ResourceReleaseID, ReleaseSHA256: releaseSHA,
		Stage: r.Stage, Reviewer: r.Reviewer, ExpiresAt: r.ExpiresAt.UTC().Format(time.RFC3339), CreateOperationID: r.CreateOperationID,
		Target: r.Target, Database: Database, Login: r.Login, DatabaseRole: r.Role,
		SecretID: r.SecretID, SecretVersion: r.Version, SecretETag: r.SecretETag, VersionETag: r.VersionETag,
		QuerySHA256: r.QuerySHA256, ImageDigest: envelope.Pins.ImageDigest,
		BinarySHA256: envelope.Pins.BinarySHA256, PlanSHA256: planSHA}
	body, err := json.Marshal(claim)
	if err != nil {
		return ErrRunClaimUncertain
	}
	object := fmt.Sprintf("%s/%s/probe.once.json", s.prefix, r.ResourceReleaseID)
	return s.Create(ctx, object, body)
}

// Create uses an object create precondition and never reads, overwrites, deletes,
// or expires the marker. Any non-precondition error is uncertain and fails closed.
func (s *GCSClaimStore) Create(ctx context.Context, object string, body []byte) error {
	if s == nil || s.client == nil || !strings.HasPrefix(object, s.prefix+"/") || len(body) == 0 {
		return errors.New("W7 B1 run claim request invalid")
	}
	writer := s.client.Bucket(s.bucket).Object(object).If(storage.Conditions{DoesNotExist: true}).NewWriter(ctx)
	writer.ContentType = "application/json"
	writer.ChunkSize = 0
	if _, err := writer.Write(body); err != nil {
		_ = writer.Close()
		return classifyClaimError(err)
	}
	if err := writer.Close(); err != nil {
		return classifyClaimError(err)
	}
	return nil
}

func classifyClaimError(err error) error {
	var apiErr *googleapi.Error
	if errors.As(err, &apiErr) && apiErr.Code == 412 {
		return ErrRunAlreadyClaimed
	}
	return fmt.Errorf("%w: %v", ErrRunClaimUncertain, err)
}

func safeObjectPrefix(prefix string) bool {
	if prefix == "" || strings.HasPrefix(prefix, "/") || strings.HasSuffix(prefix, "/") || strings.Contains(prefix, "..") {
		return false
	}
	for _, part := range strings.Split(prefix, "/") {
		if !regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`).MatchString(part) {
			return false
		}
	}
	return true
}
