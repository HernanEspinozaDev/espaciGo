package cloudstorage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strconv"

	"cloud.google.com/go/storage"
	"github.com/HernanEspinozaDev/espaciGo/internal/privacy"
	"google.golang.org/api/googleapi"
)

const replayRegistryMaxBytes = 16 << 20

// ReplayRegistry stores one small append-only manifest object in a private
// bucket. Every update is guarded by the previously observed GCS generation.
type ReplayRegistry struct {
	client *storage.Client
	object *storage.ObjectHandle
}

func NewReplayRegistry(ctx context.Context, bucketName, prefix string) (*ReplayRegistry, error) {
	if bucketName == "" {
		return nil, errors.New("privacy replay bucket is required")
	}
	if err := validatePrefix(prefix); err != nil {
		return nil, err
	}
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, privacy.ErrReplayStoreUnavailable
	}
	return newReplayRegistryWithClient(client, bucketName, prefix), nil
}

func newReplayRegistryWithClient(client *storage.Client, bucketName, prefix string) *ReplayRegistry {
	name := "privacy-replay-v1.json"
	if prefix != "" {
		name = prefix + "/" + name
	}
	return &ReplayRegistry{client: client, object: client.Bucket(bucketName).Object(name)}
}

func (r *ReplayRegistry) Close() error {
	if r == nil || r.client == nil {
		return nil
	}
	return r.client.Close()
}

func (r *ReplayRegistry) Load(ctx context.Context) (privacy.SuppressionReplaySnapshot, error) {
	if err := ctx.Err(); err != nil {
		return privacy.SuppressionReplaySnapshot{}, err
	}
	reader, err := r.object.NewReader(ctx)
	if errors.Is(err, storage.ErrObjectNotExist) {
		return emptyReplaySnapshot(), nil
	}
	if err != nil {
		return privacy.SuppressionReplaySnapshot{}, privacy.ErrReplayStoreUnavailable
	}
	if reader.Attrs.Size < 0 || reader.Attrs.Size > replayRegistryMaxBytes {
		_ = reader.Close()
		return privacy.SuppressionReplaySnapshot{}, privacy.ErrReplayStoreConflict
	}
	data, err := io.ReadAll(io.LimitReader(reader, replayRegistryMaxBytes+1))
	closeErr := reader.Close()
	if err != nil || closeErr != nil || len(data) > replayRegistryMaxBytes {
		return privacy.SuppressionReplaySnapshot{}, privacy.ErrReplayStoreUnavailable
	}
	manifest, err := decodeReplayManifest(data)
	if err != nil {
		return privacy.SuppressionReplaySnapshot{}, privacy.ErrReplayStoreConflict
	}
	return privacy.SuppressionReplaySnapshot{Revision: strconv.FormatInt(reader.Attrs.Generation, 10), Manifest: manifest}, nil
}

func (r *ReplayRegistry) CompareAndSwap(ctx context.Context, expectedRevision string, manifest privacy.SuppressionReplayManifest) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	normalized, err := privacy.NormalizeSuppressionReplayManifest(manifest)
	if err != nil || len(normalized.Entries) != len(manifest.Entries) {
		return "", privacy.ErrReplayStoreConflict
	}
	data, err := json.Marshal(normalized)
	if err != nil || len(data) > replayRegistryMaxBytes {
		return "", privacy.ErrReplayStoreConflict
	}
	conditions := storage.Conditions{}
	if expectedRevision == "" {
		conditions.DoesNotExist = true
	} else {
		generation, parseErr := strconv.ParseInt(expectedRevision, 10, 64)
		if parseErr != nil || generation <= 0 {
			return "", privacy.ErrReplayStoreConflict
		}
		conditions.GenerationMatch = generation
	}
	writer := r.object.If(conditions).NewWriter(ctx)
	writer.ContentType = "application/json"
	writer.ChunkSize = 0
	if _, err := writer.Write(data); err != nil {
		_ = writer.Close()
		return "", classifyReplayWriteError(err)
	}
	if err := writer.Close(); err != nil {
		return "", classifyReplayWriteError(err)
	}
	snapshot, err := r.Load(ctx)
	if err != nil || snapshot.Revision == "" {
		return "", privacy.ErrReplayStoreUnavailable
	}
	return snapshot.Revision, nil
}

func emptyReplaySnapshot() privacy.SuppressionReplaySnapshot {
	return privacy.SuppressionReplaySnapshot{
		Manifest: privacy.SuppressionReplayManifest{Version: 1, Entries: []privacy.SuppressionReplayEntry{}},
	}
}

func decodeReplayManifest(data []byte) (privacy.SuppressionReplayManifest, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var manifest privacy.SuppressionReplayManifest
	if err := decoder.Decode(&manifest); err != nil {
		return privacy.SuppressionReplayManifest{}, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return privacy.SuppressionReplayManifest{}, privacy.ErrInvalid
	}
	normalized, err := privacy.NormalizeSuppressionReplayManifest(manifest)
	if err != nil || manifest.Version != 1 || len(normalized.Entries) != len(manifest.Entries) {
		return privacy.SuppressionReplayManifest{}, privacy.ErrInvalid
	}
	return normalized, nil
}

func classifyReplayWriteError(err error) error {
	var apiErr *googleapi.Error
	if errors.As(err, &apiErr) && apiErr.Code == 412 {
		return privacy.ErrReplayStoreConflict
	}
	return privacy.ErrReplayStoreUnavailable
}
