package media

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"image"
	"image/color"
	"image/png"
	"reflect"
	"strings"
	"testing"
	"time"
)

const (
	uploadTestOwnerID = "00000000-0000-4000-8000-000000000001"
	uploadTestSpaceID = "00000000-0000-4000-8000-000000000002"
	uploadTestAssetID = "00000000-0000-4000-8000-000000000003"
)

type uploadTestIDs struct{}

func (uploadTestIDs) ID() (string, error) { return uploadTestAssetID, nil }

type uploadTestRepository struct {
	Repository
	events               []string
	variants             []Variant
	reserved             Reservation
	planErr              error
	planErrVariant       VariantName
	cancelAfterPlan      context.CancelFunc
	finishErr            error
	finishErrVariant     VariantName
	recordErr            error
	finalizeErr          error
	failReservationID    string
	failReservationToken int64
	failReservationAt    time.Time
	failCalls            int
}

func (r *uploadTestRepository) Reserve(_ context.Context, id, owner, spaceID string, purpose Purpose, key, inputHash string, at, lease time.Time, options ReservationOptions) (Reservation, bool, error) {
	r.events = append(r.events, "reserve")
	r.reserved = Reservation{ID: id, OwnerID: owner, SpaceID: spaceID, Purpose: purpose, Token: 7, LeaseTill: lease, State: "pending", InputHash: inputHash}
	if key == "" || !lease.Equal(at.Add(15*time.Minute)) || options != (ReservationOptions{}) {
		return Reservation{}, false, errors.New("unexpected reservation parameters")
	}
	return r.reserved, false, nil
}

func (r *uploadTestRepository) PlanVariant(_ context.Context, assetID string, token int64, name VariantName, key string) error {
	if assetID != uploadTestAssetID || token != 7 || !strings.HasSuffix(key, "_"+string(name)+".webp") {
		return errors.New("unexpected variant intent")
	}
	r.events = append(r.events, "plan:"+string(name))
	if r.cancelAfterPlan != nil {
		r.cancelAfterPlan()
	}
	if r.planErr != nil && (r.planErrVariant == "" || r.planErrVariant == name) {
		return r.planErr
	}
	return nil
}

func (r *uploadTestRepository) FinishVariantWrite(_ context.Context, assetID string, name VariantName, key string) error {
	if assetID != uploadTestAssetID || !strings.HasSuffix(key, "_"+string(name)+".webp") {
		return errors.New("unexpected completed variant write")
	}
	r.events = append(r.events, "finish:"+string(name))
	if r.finishErr != nil && (r.finishErrVariant == "" || r.finishErrVariant == name) {
		return r.finishErr
	}
	return nil
}

func (r *uploadTestRepository) RecordVariant(_ context.Context, assetID string, token int64, variant Variant) error {
	if assetID != uploadTestAssetID || token != 7 || variant.MIME != "image/webp" || variant.Generation != 1 || variant.Size <= 0 || len(variant.SHA256) != 64 || variant.ObjectKey == "" {
		return errors.New("unexpected recorded variant")
	}
	r.events = append(r.events, "record:"+string(variant.Name))
	if r.recordErr != nil {
		return r.recordErr
	}
	r.variants = append(r.variants, variant)
	return nil
}

func (r *uploadTestRepository) FinalizeSpace(_ context.Context, assetID string, token int64, owner, spaceID, expectedRevision string, _ time.Time) (Photo, error) {
	if assetID != uploadTestAssetID || token != 7 || owner != uploadTestOwnerID || spaceID != uploadTestSpaceID || expectedRevision != "" || len(r.variants) != 2 {
		return Photo{}, errors.New("unexpected space finalization")
	}
	r.events = append(r.events, "finalize")
	if r.finalizeErr != nil {
		return Photo{}, r.finalizeErr
	}
	return Photo{ID: assetID, SpaceID: spaceID, OwnerID: owner, State: "ready", Position: 1, Variants: append([]Variant(nil), r.variants...)}, nil
}

func (r *uploadTestRepository) FailReservation(_ context.Context, id string, token int64, at time.Time) error {
	r.events = append(r.events, "fail")
	r.failReservationID = id
	r.failReservationToken = token
	r.failReservationAt = at
	r.failCalls++
	return nil
}

type uploadTestStore struct {
	events            *[]string
	objects           map[string][]byte
	putErr            error
	putErrVariant     VariantName
	deleteCalls       int
	deletedKey        string
	deletedGeneration int64
}

func (s *uploadTestStore) Put(_ context.Context, key string, body []byte, mime string) (StoredObject, error) {
	if mime != "image/webp" || len(body) < 12 || string(body[:4]) != "RIFF" || string(body[8:12]) != "WEBP" {
		return StoredObject{}, errors.New("unexpected object payload")
	}
	name := ""
	for _, candidate := range []VariantName{VariantDisplay, VariantThumbnail} {
		if strings.HasSuffix(key, "_"+string(candidate)+".webp") {
			name = string(candidate)
		}
	}
	if name == "" {
		return StoredObject{}, errors.New("unexpected object key")
	}
	*s.events = append(*s.events, "put:"+name)
	if s.objects == nil {
		s.objects = make(map[string][]byte)
	}
	s.objects[key] = append([]byte(nil), body...)
	if s.putErr != nil && (s.putErrVariant == "" || s.putErrVariant == VariantName(name)) {
		return StoredObject{}, s.putErr
	}
	digest := sha256.Sum256(body)
	return StoredObject{ObjectKey: key, Provider: "memory", Bucket: "synthetic", Generation: 1, Size: int64(len(body)), SHA256: hex.EncodeToString(digest[:])}, nil
}

func (*uploadTestStore) Get(context.Context, string, int64) ([]byte, error) {
	return nil, ErrNotFound
}
func (*uploadTestStore) Generation(context.Context, string) (int64, error) {
	return 0, ErrNotFound
}
func (s *uploadTestStore) Delete(_ context.Context, key string, generation int64) error {
	*s.events = append(*s.events, "delete")
	s.deleteCalls++
	s.deletedKey = key
	s.deletedGeneration = generation
	delete(s.objects, key)
	return nil
}

func TestUploadSpaceRejectsInvalidImageBeforeReservation(t *testing.T) {
	repo := &uploadTestRepository{}
	store := &uploadTestStore{events: &repo.events}
	service, err := NewService(repo, store, uploadTestIDs{}, time.Now)
	if err != nil {
		t.Fatal(err)
	}

	result, err := service.UploadSpace(context.Background(), uploadTestOwnerID, uploadTestSpaceID, "synthetic-invalid-upload", []byte("not an image"))
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("UploadSpace() error = %v, want ErrInvalid", err)
	}
	if !reflect.DeepEqual(result, UploadResult{}) {
		t.Fatalf("UploadSpace() result = %+v, want zero result", result)
	}
	if len(repo.events) != 0 || repo.failCalls != 0 || store.deleteCalls != 0 || len(store.objects) != 0 {
		t.Fatalf("invalid image caused side effects: events=%v failCalls=%d deletes=%d objects=%d", repo.events, repo.failCalls, store.deleteCalls, len(store.objects))
	}
}

func TestUploadSpacePlanFailureFailsReservationBeforeObjectWrite(t *testing.T) {
	var input bytes.Buffer
	if err := png.Encode(&input, image.NewNRGBA(image.Rect(0, 0, 8, 6))); err != nil {
		t.Fatal(err)
	}
	fixedNow := time.Date(2026, 11, 12, 12, 30, 0, 0, time.UTC)
	planErr := errors.New("synthetic plan failure")
	repo := &uploadTestRepository{planErr: planErr}
	store := &uploadTestStore{events: &repo.events}
	service, err := NewService(repo, store, uploadTestIDs{}, func() time.Time { return fixedNow })
	if err != nil {
		t.Fatal(err)
	}

	result, err := service.UploadSpace(context.Background(), uploadTestOwnerID, uploadTestSpaceID, "synthetic-upload-key-05", input.Bytes())
	if !errors.Is(err, planErr) {
		t.Fatalf("UploadSpace() error = %v, want plan error", err)
	}
	if !reflect.DeepEqual(result, UploadResult{}) {
		t.Fatalf("UploadSpace() result = %+v, want zero result", result)
	}
	wantEvents := []string{"reserve", "plan:display", "fail"}
	if !reflect.DeepEqual(repo.events, wantEvents) || len(store.objects) != 0 || store.deleteCalls != 0 {
		t.Fatalf("plan failure events=%v objects=%d deletes=%d", repo.events, len(store.objects), store.deleteCalls)
	}
	if repo.failCalls != 1 || repo.failReservationID != uploadTestAssetID || repo.failReservationToken != 7 || !repo.failReservationAt.Equal(fixedNow) {
		t.Fatalf("reservation failure calls=%d id=%q token=%d at=%s", repo.failCalls, repo.failReservationID, repo.failReservationToken, repo.failReservationAt)
	}
}

func TestUploadSpaceThumbnailPlanFailureFailsReservationAfterDisplayWrite(t *testing.T) {
	var input bytes.Buffer
	if err := png.Encode(&input, image.NewNRGBA(image.Rect(0, 0, 8, 6))); err != nil {
		t.Fatal(err)
	}
	fixedNow := time.Date(2026, 11, 12, 12, 30, 0, 0, time.UTC)
	planErr := errors.New("synthetic thumbnail plan failure")
	repo := &uploadTestRepository{planErr: planErr, planErrVariant: VariantThumbnail}
	store := &uploadTestStore{events: &repo.events}
	service, err := NewService(repo, store, uploadTestIDs{}, func() time.Time { return fixedNow })
	if err != nil {
		t.Fatal(err)
	}

	result, err := service.UploadSpace(context.Background(), uploadTestOwnerID, uploadTestSpaceID, "synthetic-upload-key-07", input.Bytes())
	if !errors.Is(err, planErr) {
		t.Fatalf("UploadSpace() error = %v, want thumbnail plan error", err)
	}
	if !reflect.DeepEqual(result, UploadResult{}) {
		t.Fatalf("UploadSpace() result = %+v, want zero result", result)
	}
	wantEvents := []string{
		"reserve",
		"plan:display", "put:display", "finish:display", "record:display",
		"plan:thumbnail", "fail",
	}
	if !reflect.DeepEqual(repo.events, wantEvents) || len(repo.variants) != 1 || repo.variants[0].Name != VariantDisplay || len(store.objects) != 1 || store.deleteCalls != 0 {
		t.Fatalf("thumbnail plan failure events=%v variants=%v objects=%d deletes=%d", repo.events, repo.variants, len(store.objects), store.deleteCalls)
	}
	for key := range store.objects {
		if !strings.HasSuffix(key, "_display.webp") {
			t.Fatalf("unexpected object after thumbnail plan failure: %q", key)
		}
	}
	if repo.failCalls != 1 || repo.failReservationID != uploadTestAssetID || repo.failReservationToken != 7 || !repo.failReservationAt.Equal(fixedNow) {
		t.Fatalf("reservation failure calls=%d id=%q token=%d at=%s", repo.failCalls, repo.failReservationID, repo.failReservationToken, repo.failReservationAt)
	}
}
func TestUploadSpaceCancellationAfterPlanningStopsBeforeObjectWrite(t *testing.T) {
	var input bytes.Buffer
	if err := png.Encode(&input, image.NewNRGBA(image.Rect(0, 0, 8, 6))); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	repo := &uploadTestRepository{cancelAfterPlan: cancel}
	store := &uploadTestStore{events: &repo.events}
	service, err := NewService(repo, store, uploadTestIDs{}, time.Now)
	if err != nil {
		t.Fatal(err)
	}

	result, err := service.UploadSpace(ctx, uploadTestOwnerID, uploadTestSpaceID, "synthetic-upload-cancel", input.Bytes())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("UploadSpace() error = %v, want context.Canceled", err)
	}
	if !reflect.DeepEqual(result, UploadResult{}) {
		t.Fatalf("UploadSpace() result = %+v, want zero result", result)
	}
	wantEvents := []string{"reserve", "plan:display", "fail"}
	if !reflect.DeepEqual(repo.events, wantEvents) || len(store.objects) != 0 || store.deleteCalls != 0 {
		t.Fatalf("canceled upload events=%v objects=%d deletes=%d", repo.events, len(store.objects), store.deleteCalls)
	}
	if repo.failCalls != 1 || repo.failReservationID != uploadTestAssetID || repo.failReservationToken != 7 {
		t.Fatalf("canceled reservation failure calls=%d id=%q token=%d", repo.failCalls, repo.failReservationID, repo.failReservationToken)
	}
}

func TestUploadSpaceFinalizeFailureFailsReservationAfterRecordingVariants(t *testing.T) {
	var input bytes.Buffer
	if err := png.Encode(&input, image.NewNRGBA(image.Rect(0, 0, 8, 6))); err != nil {
		t.Fatal(err)
	}
	fixedNow := time.Date(2026, 11, 12, 12, 30, 0, 0, time.UTC)
	finalizeErr := errors.New("synthetic finalize failure")
	repo := &uploadTestRepository{finalizeErr: finalizeErr}
	store := &uploadTestStore{events: &repo.events}
	service, err := NewService(repo, store, uploadTestIDs{}, func() time.Time { return fixedNow })
	if err != nil {
		t.Fatal(err)
	}

	result, err := service.UploadSpace(context.Background(), uploadTestOwnerID, uploadTestSpaceID, "synthetic-upload-key-06", input.Bytes())
	if !errors.Is(err, finalizeErr) {
		t.Fatalf("UploadSpace() error = %v, want finalize error", err)
	}
	if !reflect.DeepEqual(result, UploadResult{}) {
		t.Fatalf("UploadSpace() result = %+v, want zero result", result)
	}
	wantEvents := []string{
		"reserve",
		"plan:display", "put:display", "finish:display", "record:display",
		"plan:thumbnail", "put:thumbnail", "finish:thumbnail", "record:thumbnail",
		"finalize", "fail",
	}
	if !reflect.DeepEqual(repo.events, wantEvents) || len(store.objects) != 2 || store.deleteCalls != 0 {
		t.Fatalf("finalize failure events=%v objects=%d deletes=%d", repo.events, len(store.objects), store.deleteCalls)
	}
	if repo.failCalls != 1 || repo.failReservationID != uploadTestAssetID || repo.failReservationToken != 7 || !repo.failReservationAt.Equal(fixedNow) {
		t.Fatalf("reservation failure calls=%d id=%q token=%d at=%s", repo.failCalls, repo.failReservationID, repo.failReservationToken, repo.failReservationAt)
	}
}

func TestUploadSpaceCreatesReadyPhotoFromSyntheticBytes(t *testing.T) {
	var input bytes.Buffer
	fixture := image.NewNRGBA(image.Rect(0, 0, 8, 6))
	fixture.Set(2, 3, color.NRGBA{R: 180, G: 90, B: 30, A: 255})
	if err := png.Encode(&input, fixture); err != nil {
		t.Fatal(err)
	}
	inputHash := sha256.Sum256(input.Bytes())
	fixedNow := time.Date(2026, 11, 12, 12, 30, 0, 0, time.UTC)
	repo := &uploadTestRepository{}
	store := &uploadTestStore{events: &repo.events}
	service, err := NewService(repo, store, uploadTestIDs{}, func() time.Time { return fixedNow })
	if err != nil {
		t.Fatal(err)
	}

	result, err := service.UploadSpace(context.Background(), uploadTestOwnerID, uploadTestSpaceID, "  synthetic-upload-key-01  ", input.Bytes())
	if err != nil {
		t.Fatalf("UploadSpace() error = %v", err)
	}
	if result.Pending || result.Reused || result.Photo.ID != uploadTestAssetID || result.Photo.OwnerID != uploadTestOwnerID || result.Photo.SpaceID != uploadTestSpaceID || result.Photo.State != "ready" || result.Photo.Position != 1 {
		t.Fatalf("UploadSpace() result = %+v", result)
	}
	if repo.reserved.InputHash != hex.EncodeToString(inputHash[:]) || repo.reserved.Purpose != PurposeSpace || repo.reserved.OwnerID != uploadTestOwnerID || repo.reserved.SpaceID != uploadTestSpaceID {
		t.Fatalf("reservation = %+v", repo.reserved)
	}
	wantEvents := []string{
		"reserve",
		"plan:display", "put:display", "finish:display", "record:display",
		"plan:thumbnail", "put:thumbnail", "finish:thumbnail", "record:thumbnail",
		"finalize",
	}
	if !reflect.DeepEqual(repo.events, wantEvents) {
		t.Fatalf("upload events = %v, want %v", repo.events, wantEvents)
	}
	if len(result.Photo.Variants) != 2 {
		t.Fatalf("ready photo has %d variants, want 2", len(result.Photo.Variants))
	}
	for index, name := range []VariantName{VariantDisplay, VariantThumbnail} {
		variant := result.Photo.Variants[index]
		wantURL := "/api/v1/spaces/" + uploadTestSpaceID + "/gallery/" + uploadTestAssetID + "/variants/" + string(name)
		if variant.Name != name || variant.MIME != "image/webp" || variant.URL != wantURL || variant.Provider != "memory" || variant.Bucket != "synthetic" || variant.Generation != 1 || int64(len(store.objects[variant.ObjectKey])) != variant.Size {
			t.Errorf("variant[%d] = %+v", index, variant)
		}
		digest := sha256.Sum256(store.objects[variant.ObjectKey])
		if variant.SHA256 != hex.EncodeToString(digest[:]) {
			t.Errorf("variant[%d] SHA-256 does not match stored synthetic bytes", index)
		}
	}
}

func TestUploadSpacePutFailureFinishesAmbiguousWriteAndFailsReservation(t *testing.T) {
	var input bytes.Buffer
	if err := png.Encode(&input, image.NewNRGBA(image.Rect(0, 0, 8, 6))); err != nil {
		t.Fatal(err)
	}
	fixedNow := time.Date(2026, 11, 12, 12, 30, 0, 0, time.UTC)
	repo := &uploadTestRepository{}
	store := &uploadTestStore{events: &repo.events, putErr: errors.New("synthetic ambiguous write")}
	service, err := NewService(repo, store, uploadTestIDs{}, func() time.Time { return fixedNow })
	if err != nil {
		t.Fatal(err)
	}

	result, err := service.UploadSpace(context.Background(), uploadTestOwnerID, uploadTestSpaceID, "synthetic-upload-key-02", input.Bytes())
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("UploadSpace() error = %v, want ErrUnavailable", err)
	}
	if result.Pending || result.Reused || result.Photo.ID != "" || len(result.Photo.Variants) != 0 {
		t.Fatalf("UploadSpace() result = %+v, want empty result", result)
	}
	wantEvents := []string{"reserve", "plan:display", "put:display", "finish:display", "fail"}
	if !reflect.DeepEqual(repo.events, wantEvents) {
		t.Fatalf("upload failure events = %v, want %v", repo.events, wantEvents)
	}
	if len(store.objects) != 1 || repo.failCalls != 1 || repo.failReservationID != uploadTestAssetID || repo.failReservationToken != 7 || !repo.failReservationAt.Equal(fixedNow) {
		t.Fatalf("ambiguous failure cleanup objects=%d failCalls=%d reservation=%q/%d at=%s", len(store.objects), repo.failCalls, repo.failReservationID, repo.failReservationToken, repo.failReservationAt)
	}
}

func TestUploadSpaceThumbnailPutFailureFinishesAmbiguousWriteAndFailsReservation(t *testing.T) {
	var input bytes.Buffer
	if err := png.Encode(&input, image.NewNRGBA(image.Rect(0, 0, 8, 6))); err != nil {
		t.Fatal(err)
	}
	fixedNow := time.Date(2026, 11, 12, 12, 30, 0, 0, time.UTC)
	repo := &uploadTestRepository{}
	store := &uploadTestStore{
		events: &repo.events, putErr: errors.New("synthetic ambiguous thumbnail write"),
		putErrVariant: VariantThumbnail,
	}
	service, err := NewService(repo, store, uploadTestIDs{}, func() time.Time { return fixedNow })
	if err != nil {
		t.Fatal(err)
	}

	result, err := service.UploadSpace(context.Background(), uploadTestOwnerID, uploadTestSpaceID, "synthetic-upload-key-08", input.Bytes())
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("UploadSpace() error = %v, want ErrUnavailable", err)
	}
	if !reflect.DeepEqual(result, UploadResult{}) {
		t.Fatalf("UploadSpace() result = %+v, want zero result", result)
	}
	wantEvents := []string{
		"reserve",
		"plan:display", "put:display", "finish:display", "record:display",
		"plan:thumbnail", "put:thumbnail", "finish:thumbnail", "fail",
	}
	if !reflect.DeepEqual(repo.events, wantEvents) || len(repo.variants) != 1 || repo.variants[0].Name != VariantDisplay || len(store.objects) != 2 || store.deleteCalls != 0 {
		t.Fatalf("thumbnail PUT failure events=%v variants=%v objects=%d deletes=%d", repo.events, repo.variants, len(store.objects), store.deleteCalls)
	}
	for key := range store.objects {
		if !strings.HasSuffix(key, "_display.webp") && !strings.HasSuffix(key, "_thumbnail.webp") {
			t.Fatalf("unexpected object after thumbnail PUT failure: %q", key)
		}
	}
	if repo.failCalls != 1 || repo.failReservationID != uploadTestAssetID || repo.failReservationToken != 7 || !repo.failReservationAt.Equal(fixedNow) {
		t.Fatalf("reservation failure calls=%d id=%q token=%d at=%s", repo.failCalls, repo.failReservationID, repo.failReservationToken, repo.failReservationAt)
	}
}

func TestUploadSpaceRecordFailureDeletesWrittenGenerationAndFailsReservation(t *testing.T) {
	var input bytes.Buffer
	if err := png.Encode(&input, image.NewNRGBA(image.Rect(0, 0, 8, 6))); err != nil {
		t.Fatal(err)
	}
	fixedNow := time.Date(2026, 11, 12, 12, 30, 0, 0, time.UTC)
	repo := &uploadTestRepository{recordErr: errors.New("synthetic metadata failure")}
	store := &uploadTestStore{events: &repo.events}
	service, err := NewService(repo, store, uploadTestIDs{}, func() time.Time { return fixedNow })
	if err != nil {
		t.Fatal(err)
	}

	result, err := service.UploadSpace(context.Background(), uploadTestOwnerID, uploadTestSpaceID, "synthetic-upload-key-03", input.Bytes())
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("UploadSpace() error = %v, want ErrUnavailable", err)
	}
	if result.Pending || result.Reused || result.Photo.ID != "" || len(result.Photo.Variants) != 0 {
		t.Fatalf("UploadSpace() result = %+v, want empty result", result)
	}
	wantEvents := []string{"reserve", "plan:display", "put:display", "finish:display", "record:display", "delete", "fail"}
	if !reflect.DeepEqual(repo.events, wantEvents) {
		t.Fatalf("record failure events = %v, want %v", repo.events, wantEvents)
	}
	if store.deleteCalls != 1 || store.deletedGeneration != 1 || !strings.HasSuffix(store.deletedKey, "_display.webp") || len(store.objects) != 0 || repo.failCalls != 1 || repo.failReservationID != uploadTestAssetID || repo.failReservationToken != 7 || !repo.failReservationAt.Equal(fixedNow) {
		t.Fatalf("record failure cleanup deletes=%d key=%q generation=%d objects=%d failCalls=%d reservation=%q/%d at=%s", store.deleteCalls, store.deletedKey, store.deletedGeneration, len(store.objects), repo.failCalls, repo.failReservationID, repo.failReservationToken, repo.failReservationAt)
	}
}

func TestUploadSpaceFinishFailureDeletesWrittenGenerationAndFailsReservation(t *testing.T) {
	var input bytes.Buffer
	if err := png.Encode(&input, image.NewNRGBA(image.Rect(0, 0, 8, 6))); err != nil {
		t.Fatal(err)
	}
	fixedNow := time.Date(2026, 11, 12, 12, 30, 0, 0, time.UTC)
	repo := &uploadTestRepository{finishErr: errors.New("synthetic finish failure")}
	store := &uploadTestStore{events: &repo.events}
	service, err := NewService(repo, store, uploadTestIDs{}, func() time.Time { return fixedNow })
	if err != nil {
		t.Fatal(err)
	}

	result, err := service.UploadSpace(context.Background(), uploadTestOwnerID, uploadTestSpaceID, "synthetic-upload-key-04", input.Bytes())
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("UploadSpace() error = %v, want ErrUnavailable", err)
	}
	if result.Pending || result.Reused || result.Photo.ID != "" || len(result.Photo.Variants) != 0 {
		t.Fatalf("UploadSpace() result = %+v, want empty result", result)
	}
	wantEvents := []string{"reserve", "plan:display", "put:display", "finish:display", "delete", "fail"}
	if !reflect.DeepEqual(repo.events, wantEvents) {
		t.Fatalf("finish failure events = %v, want %v", repo.events, wantEvents)
	}
	if store.deleteCalls != 1 || store.deletedGeneration != 1 || !strings.HasSuffix(store.deletedKey, "_display.webp") || len(store.objects) != 0 || repo.failCalls != 1 || repo.failReservationID != uploadTestAssetID || repo.failReservationToken != 7 || !repo.failReservationAt.Equal(fixedNow) {
		t.Fatalf("finish failure cleanup deletes=%d key=%q generation=%d objects=%d failCalls=%d reservation=%q/%d at=%s", store.deleteCalls, store.deletedKey, store.deletedGeneration, len(store.objects), repo.failCalls, repo.failReservationID, repo.failReservationToken, repo.failReservationAt)
	}
}

func TestUploadSpaceThumbnailFinishFailureDeletesOnlyThumbnailGeneration(t *testing.T) {
	var input bytes.Buffer
	if err := png.Encode(&input, image.NewNRGBA(image.Rect(0, 0, 8, 6))); err != nil {
		t.Fatal(err)
	}
	fixedNow := time.Date(2026, 11, 12, 12, 30, 0, 0, time.UTC)
	repo := &uploadTestRepository{
		finishErr: errors.New("synthetic thumbnail finish failure"), finishErrVariant: VariantThumbnail,
	}
	store := &uploadTestStore{events: &repo.events}
	service, err := NewService(repo, store, uploadTestIDs{}, func() time.Time { return fixedNow })
	if err != nil {
		t.Fatal(err)
	}

	result, err := service.UploadSpace(context.Background(), uploadTestOwnerID, uploadTestSpaceID, "synthetic-upload-key-09", input.Bytes())
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("UploadSpace() error = %v, want ErrUnavailable", err)
	}
	if !reflect.DeepEqual(result, UploadResult{}) {
		t.Fatalf("UploadSpace() result = %+v, want zero result", result)
	}
	wantEvents := []string{
		"reserve",
		"plan:display", "put:display", "finish:display", "record:display",
		"plan:thumbnail", "put:thumbnail", "finish:thumbnail", "delete", "fail",
	}
	if !reflect.DeepEqual(repo.events, wantEvents) || len(repo.variants) != 1 || repo.variants[0].Name != VariantDisplay {
		t.Fatalf("thumbnail finish failure events=%v variants=%v", repo.events, repo.variants)
	}
	if store.deleteCalls != 1 || store.deletedGeneration != 1 || !strings.HasSuffix(store.deletedKey, "_thumbnail.webp") || len(store.objects) != 1 || repo.failCalls != 1 || repo.failReservationID != uploadTestAssetID || repo.failReservationToken != 7 || !repo.failReservationAt.Equal(fixedNow) {
		t.Fatalf("thumbnail finish cleanup deletes=%d key=%q generation=%d objects=%d failCalls=%d reservation=%q/%d at=%s", store.deleteCalls, store.deletedKey, store.deletedGeneration, len(store.objects), repo.failCalls, repo.failReservationID, repo.failReservationToken, repo.failReservationAt)
	}
	for key := range store.objects {
		if !strings.HasSuffix(key, "_display.webp") {
			t.Fatalf("thumbnail finish failure removed or replaced display unexpectedly: remaining key=%q", key)
		}
	}
}
