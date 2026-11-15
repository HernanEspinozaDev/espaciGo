package privacy

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

type identityArchivePartialErrorRepository struct {
	*memoryRepository
	accountID string
	data      OwnData
	err       error
	calls     int
}

func (r *identityArchivePartialErrorRepository) ExportOwnData(_ context.Context, accountID string) (OwnData, error) {
	r.calls++
	r.accountID = accountID
	return r.data, r.err
}

type identityArchiveModuleCallRecorder struct {
	calls     int
	accountID string
}

func (r *identityArchiveModuleCallRecorder) ExportAdditionalOwnData(_ context.Context, accountID string) (map[string]json.RawMessage, []ExportFile, []string, error) {
	r.calls++
	r.accountID = accountID
	return map[string]json.RawMessage{}, nil, nil, nil
}

func TestExportOwnArchiveStopsAfterIdentitySourceReturnsDataAndError(t *testing.T) {
	const ownerID = "owner@example.invalid"
	wantErr := errors.New("synthetic identity source failure after partial result")
	repo := &identityArchivePartialErrorRepository{
		memoryRepository: &memoryRepository{},
		data: OwnData{
			Account: ExportAccount{Email: "partial-owner@example.invalid"},
			Roles:   []string{"synthetic-operator-role"},
		},
		err: wantErr,
	}
	modules := &identityArchiveModuleCallRecorder{}
	service, err := NewService(repo, modules)
	if err != nil {
		t.Fatal(err)
	}

	archiveBytes, err := service.ExportOwnArchive(context.Background(), ownerID)
	if archiveBytes != nil || !errors.Is(err, wantErr) {
		t.Fatalf("archive bytes=%d err=%v, want nil archive and original repository error", len(archiveBytes), err)
	}
	if repo.calls != 1 || repo.accountID != ownerID {
		t.Fatalf("identity repository calls=%d account=%q, want one call scoped to owner", repo.calls, repo.accountID)
	}
	if modules.calls != 0 {
		t.Fatalf("module source calls=%d account=%q, want zero after identity source error", modules.calls, modules.accountID)
	}
}
