package privacy

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

type identityArchiveCancellationBoundaryRepository struct {
	calls       int
	accountID   string
	afterExport func()
	err         error
}

func (*identityArchiveCancellationBoundaryRepository) GetProfile(context.Context, string) (Profile, error) {
	return Profile{}, ErrNotFound
}

func (*identityArchiveCancellationBoundaryRepository) SaveProfile(context.Context, string, string, *string) (Profile, error) {
	return Profile{}, ErrInvalid
}

func (*identityArchiveCancellationBoundaryRepository) CreateRightsRequest(context.Context, string, string, string) (RightsRequest, error) {
	return RightsRequest{}, ErrInvalid
}

func (*identityArchiveCancellationBoundaryRepository) ListOwnRightsRequests(context.Context, string) ([]RightsRequest, error) {
	return nil, ErrInvalid
}

func (r *identityArchiveCancellationBoundaryRepository) ExportOwnData(_ context.Context, accountID string) (OwnData, error) {
	r.calls++
	r.accountID = accountID
	if r.afterExport != nil {
		r.afterExport()
	}
	return OwnData{Account: ExportAccount{Email: accountID}, Roles: []string{}}, r.err
}

type identityArchiveCancellationBoundaryModules struct {
	calls       int
	accountID   string
	afterExport func()
	err         error
}

func (m *identityArchiveCancellationBoundaryModules) ExportAdditionalOwnData(_ context.Context, accountID string) (map[string]json.RawMessage, []ExportFile, []string, error) {
	m.calls++
	m.accountID = accountID
	if m.afterExport != nil {
		m.afterExport()
	}
	// Deliberately return success even if the callback cancelled the context;
	// the service must fence work after a dependency returns.
	return map[string]json.RawMessage{}, nil, nil, m.err
}

func TestExportOwnArchiveSkipsSourcesWhenContextIsAlreadyCancelled(t *testing.T) {
	const ownerID = "owner@example.invalid"
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	repo := &identityArchiveCancellationBoundaryRepository{}
	modules := &identityArchiveCancellationBoundaryModules{}
	service, err := NewService(repo, modules)
	if err != nil {
		t.Fatal(err)
	}

	archive, err := service.ExportOwnArchive(ctx, ownerID)
	if archive != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("archive bytes=%d err=%v, want nil archive and context cancellation", len(archive), err)
	}
	if repo.calls != 0 || modules.calls != 0 {
		t.Fatalf("identity reads=%d module reads=%d, want no source calls", repo.calls, modules.calls)
	}
}

func TestExportOwnArchiveStopsAfterIdentitySourceCancelsContext(t *testing.T) {
	const ownerID = "owner@example.invalid"
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	repo := &identityArchiveCancellationBoundaryRepository{afterExport: cancel}
	modules := &identityArchiveCancellationBoundaryModules{}
	service, err := NewService(repo, modules)
	if err != nil {
		t.Fatal(err)
	}

	archive, err := service.ExportOwnArchive(ctx, ownerID)
	if archive != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("archive bytes=%d err=%v, want nil archive and context cancellation", len(archive), err)
	}
	if repo.calls != 1 || repo.accountID != ownerID {
		t.Fatalf("identity reads=%d account=%q, want one read scoped to owner", repo.calls, repo.accountID)
	}
	if modules.calls != 0 {
		t.Fatalf("module reads=%d, want no module read after identity-source cancellation", modules.calls)
	}
}

func TestExportOwnArchiveStopsAfterModuleSourceCancelsContext(t *testing.T) {
	const ownerID = "owner@example.invalid"
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	repo := &identityArchiveCancellationBoundaryRepository{}
	modules := &identityArchiveCancellationBoundaryModules{afterExport: cancel}
	service, err := NewService(repo, modules)
	if err != nil {
		t.Fatal(err)
	}

	archive, err := service.ExportOwnArchive(ctx, ownerID)
	if archive != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("archive bytes=%d err=%v, want nil archive and context cancellation", len(archive), err)
	}
	if repo.calls != 1 || repo.accountID != ownerID {
		t.Fatalf("identity reads=%d account=%q, want one read scoped to owner", repo.calls, repo.accountID)
	}
	if modules.calls != 1 || modules.accountID != ownerID {
		t.Fatalf("module reads=%d account=%q, want one read scoped to owner", modules.calls, modules.accountID)
	}
}

func TestExportOwnArchivePreservesSourceErrorWhenSourceCancelsContext(t *testing.T) {
	const ownerID = "owner@example.invalid"
	for _, tc := range []struct {
		name                 string
		cancelIdentitySource bool
		wantModuleCalls      int
	}{
		{name: "identity source", cancelIdentitySource: true, wantModuleCalls: 0},
		{name: "module source", wantModuleCalls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			wantErr := errors.New("synthetic source error")
			repo := &identityArchiveCancellationBoundaryRepository{}
			modules := &identityArchiveCancellationBoundaryModules{}
			if tc.cancelIdentitySource {
				repo.afterExport = cancel
				repo.err = wantErr
			} else {
				modules.afterExport = cancel
				modules.err = wantErr
			}
			service, err := NewService(repo, modules)
			if err != nil {
				t.Fatal(err)
			}

			archive, err := service.ExportOwnArchive(ctx, ownerID)
			if archive != nil || !errors.Is(err, wantErr) || errors.Is(err, context.Canceled) {
				t.Fatalf("archive bytes=%d err=%v, want nil archive and source error without cancellation taking precedence", len(archive), err)
			}
			if !errors.Is(ctx.Err(), context.Canceled) {
				t.Fatalf("context error=%v, want canceled by source fake", ctx.Err())
			}
			if repo.calls != 1 || repo.accountID != ownerID {
				t.Fatalf("identity reads=%d account=%q, want one read scoped to owner", repo.calls, repo.accountID)
			}
			if modules.calls != tc.wantModuleCalls {
				t.Fatalf("module reads=%d, want %d", modules.calls, tc.wantModuleCalls)
			}
			if modules.calls > 0 && modules.accountID != ownerID {
				t.Fatalf("module account=%q, want owner-scoped read", modules.accountID)
			}
		})
	}
}

func TestExportOwnArchiveDoesNotReturnArchiveWhenContextIsCancelledDuringAssembly(t *testing.T) {
	const ownerID = "owner@example.invalid"
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	repo := &identityArchiveCancellationBoundaryRepository{}
	modules := &identityArchiveCancellationBoundaryModules{}
	service, err := NewService(repo, modules)
	if err != nil {
		t.Fatal(err)
	}
	service.now = func() time.Time {
		cancel()
		return time.Date(2026, time.November, 13, 12, 0, 0, 0, time.UTC)
	}

	archive, err := service.ExportOwnArchive(ctx, ownerID)
	if archive != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("archive bytes=%d err=%v, want nil archive and context cancellation", len(archive), err)
	}
	if repo.calls != 1 || repo.accountID != ownerID {
		t.Fatalf("identity reads=%d account=%q, want one read scoped to owner", repo.calls, repo.accountID)
	}
	if modules.calls != 1 || modules.accountID != ownerID {
		t.Fatalf("module reads=%d account=%q, want one read scoped to owner", modules.calls, modules.accountID)
	}
}
