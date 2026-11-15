package privacy

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"testing"
	"time"
)

type identityArchiveModulesFake struct {
	accountID string
	sections  map[string]json.RawMessage
	files     []ExportFile
	excluded  []string
	err       error
	calls     int
}

func (f *identityArchiveModulesFake) ExportAdditionalOwnData(_ context.Context, accountID string) (map[string]json.RawMessage, []ExportFile, []string, error) {
	f.calls++
	f.accountID = accountID
	return f.sections, f.files, f.excluded, f.err
}

type identityArchiveCancellationModulesFake struct {
	entered   chan struct{}
	abort     <-chan struct{}
	calls     int
	accountID string
}

func (f *identityArchiveCancellationModulesFake) ExportAdditionalOwnData(ctx context.Context, accountID string) (map[string]json.RawMessage, []ExportFile, []string, error) {
	f.calls++
	f.accountID = accountID
	close(f.entered)
	select {
	case <-ctx.Done():
		return nil, nil, nil, ctx.Err()
	case <-f.abort:
		return nil, nil, nil, errors.New("synthetic test cleanup")
	}
}

func TestExportOwnArchiveOmitsOperationalRolesAndKeepsFileBytesOutOfMetadata(t *testing.T) {
	const ownerID = "owner@example.invalid"
	const payload = "synthetic-identity-file-payload-7291"

	modules := &identityArchiveModulesFake{
		sections: map[string]json.RawMessage{"identity_review": json.RawMessage(`{"state":"en_revision"}`)},
		files: []ExportFile{{
			Name:        "files/identity-document.pdf",
			MediaType:   "application/pdf",
			Description: "Documento propio sintético",
			Content:     []byte(payload),
		}},
	}
	service, err := NewService(&memoryRepository{}, modules)
	if err != nil {
		t.Fatal(err)
	}

	archiveBytes, err := service.ExportOwnArchive(context.Background(), ownerID)
	if err != nil {
		t.Fatal(err)
	}
	if modules.calls != 1 || modules.accountID != ownerID {
		t.Fatalf("module calls=%d account=%q, want one call scoped to owner", modules.calls, modules.accountID)
	}

	archive, err := zip.NewReader(bytes.NewReader(archiveBytes), int64(len(archiveBytes)))
	if err != nil {
		t.Fatal(err)
	}
	entries := make(map[string][]byte, len(archive.File))
	for _, file := range archive.File {
		reader, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		body, readErr := io.ReadAll(reader)
		closeErr := reader.Close()
		if readErr != nil {
			t.Fatal(readErr)
		}
		if closeErr != nil {
			t.Fatal(closeErr)
		}
		entries[file.Name] = body
	}
	if len(entries) != 3 {
		t.Fatalf("archive entries=%v, want manifest, data and one private file", reflect.ValueOf(entries).MapKeys())
	}

	manifestJSON, ok := entries["manifest.json"]
	if !ok {
		t.Fatal("archive is missing manifest.json")
	}
	dataJSON, ok := entries["data.json"]
	if !ok {
		t.Fatal("archive is missing data.json")
	}
	fileBytes, ok := entries["files/identity-document.pdf"]
	if !ok || string(fileBytes) != payload {
		t.Fatalf("private file entry=%q, present=%t; want exact synthetic payload", fileBytes, ok)
	}
	if bytes.Contains(manifestJSON, []byte(payload)) || bytes.Contains(dataJSON, []byte(payload)) {
		t.Fatal("binary file payload leaked into JSON metadata entries")
	}

	var data CompleteOwnData
	if err := json.Unmarshal(dataJSON, &data); err != nil {
		t.Fatal(err)
	}
	if data.Identity.Account.Email != ownerID || len(data.Identity.Roles) != 0 {
		t.Fatalf("identity account=%q roles=%v, want owner account and no operational roles", data.Identity.Account.Email, data.Identity.Roles)
	}

	var manifest ExportManifest
	if err := json.Unmarshal(manifestJSON, &manifest); err != nil {
		t.Fatal(err)
	}
	requireIdentityArchiveJSONKeys(t, manifestJSON, "schema_version", "data_file", "sections", "files", "exclusions", "exported_at")
	var manifestWire map[string]json.RawMessage
	if err := json.Unmarshal(manifestJSON, &manifestWire); err != nil {
		t.Fatal(err)
	}
	var fileMetadata []json.RawMessage
	if err := json.Unmarshal(manifestWire["files"], &fileMetadata); err != nil {
		t.Fatal(err)
	}
	if len(fileMetadata) != 1 {
		t.Fatalf("manifest file metadata count=%d, want one", len(fileMetadata))
	}
	requireIdentityArchiveJSONKeys(t, fileMetadata[0], "name", "media_type", "description")
	if len(manifest.Files) != 1 || manifest.Files[0] != (ExportFileInfo{
		Name:        "files/identity-document.pdf",
		MediaType:   "application/pdf",
		Description: "Documento propio sintético",
	}) {
		t.Fatalf("manifest files=%+v, want metadata only for the included file", manifest.Files)
	}
}

func TestExportOwnArchiveReturnsNoPartialArchiveWhenModuleSourceFails(t *testing.T) {
	const ownerID = "owner@example.invalid"
	wantErr := errors.New("synthetic identity module source failure")
	modules := &identityArchiveModulesFake{err: wantErr}
	service, err := NewService(&memoryRepository{}, modules)
	if err != nil {
		t.Fatal(err)
	}

	archiveBytes, err := service.ExportOwnArchive(context.Background(), ownerID)
	if archiveBytes != nil || !errors.Is(err, wantErr) {
		t.Fatalf("archive bytes=%d err=%v, want nil archive and original module error", len(archiveBytes), err)
	}
	if modules.calls != 1 || modules.accountID != ownerID {
		t.Fatalf("module calls=%d account=%q, want one call scoped to owner", modules.calls, modules.accountID)
	}
}

func TestExportOwnArchiveReturnsNoArchiveWhenModuleExportContextIsCancelled(t *testing.T) {
	const ownerID = "owner@example.invalid"
	abort := make(chan struct{})
	modules := &identityArchiveCancellationModulesFake{entered: make(chan struct{}), abort: abort}
	service, err := NewService(&memoryRepository{}, modules)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
		close(abort)
	}()
	type exportResult struct {
		archive []byte
		err     error
	}
	done := make(chan exportResult, 1)
	go func() {
		archive, err := service.ExportOwnArchive(ctx, ownerID)
		done <- exportResult{archive: archive, err: err}
	}()

	select {
	case <-modules.entered:
		cancel()
	case <-time.After(5 * time.Second):
		cancel()
		t.Fatal("module source was not reached")
	}

	select {
	case result := <-done:
		if result.archive != nil || !errors.Is(result.err, context.Canceled) {
			t.Fatalf("archive bytes=%d err=%v, want no archive and context cancellation", len(result.archive), result.err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("export did not stop after module context cancellation")
	}
	if modules.calls != 1 || modules.accountID != ownerID {
		t.Fatalf("module calls=%d account=%q, want one call scoped to owner", modules.calls, modules.accountID)
	}
}

func TestExportOwnArchiveReturnsNoPartialArchiveWhenModuleSourceReturnsDataAndError(t *testing.T) {
	const ownerID = "owner@example.invalid"
	wantErr := errors.New("synthetic identity module source failed after partial results")
	modules := &identityArchiveModulesFake{
		sections: map[string]json.RawMessage{"identity_review": json.RawMessage(`{"state":"synthetic"}`)},
		files: []ExportFile{{
			Name:        "files/partial-identity-document.pdf",
			MediaType:   "application/pdf",
			Description: "Synthetic partial result",
			Content:     []byte("synthetic partial payload"),
		}},
		excluded: []string{"synthetic excluded section"},
		err:      wantErr,
	}
	service, err := NewService(&memoryRepository{}, modules)
	if err != nil {
		t.Fatal(err)
	}

	archiveBytes, err := service.ExportOwnArchive(context.Background(), ownerID)
	if archiveBytes != nil || !errors.Is(err, wantErr) {
		t.Fatalf("archive bytes=%d err=%v, want nil archive and original module error", len(archiveBytes), err)
	}
	if modules.calls != 1 || modules.accountID != ownerID {
		t.Fatalf("module calls=%d account=%q, want one call scoped to owner", modules.calls, modules.accountID)
	}
}

func TestExportOwnArchiveReturnsNoPartialArchiveWhenModuleSectionIsInvalidJSON(t *testing.T) {
	const ownerID = "owner@example.invalid"
	modules := &identityArchiveModulesFake{
		sections: map[string]json.RawMessage{"identity_review": json.RawMessage(`{"state":`)},
	}
	service, err := NewService(&memoryRepository{}, modules)
	if err != nil {
		t.Fatal(err)
	}

	archiveBytes, err := service.ExportOwnArchive(context.Background(), ownerID)
	if archiveBytes != nil || err == nil {
		t.Fatalf("archive bytes=%d err=%v, want nil archive and serialization error", len(archiveBytes), err)
	}
	if modules.calls != 1 || modules.accountID != ownerID {
		t.Fatalf("module calls=%d account=%q, want one call scoped to owner", modules.calls, modules.accountID)
	}
}

func TestExportOwnArchiveRejectsDirectoryAndDuplicateFilePaths(t *testing.T) {
	tests := []struct {
		name  string
		files []ExportFile
	}{
		{
			name:  "directory entry",
			files: []ExportFile{{Name: "files/"}},
		},
		{
			name:  "trailing separator",
			files: []ExportFile{{Name: "files/identity-document/"}},
		},
		{
			name:  "dot segment",
			files: []ExportFile{{Name: "files/./identity-document.pdf"}},
		},
		{
			name:  "backslash separator",
			files: []ExportFile{{Name: `files/sub\identity-document.pdf`}},
		},
		{
			name:  "parent traversal into archive metadata",
			files: []ExportFile{{Name: "files/../manifest.json", Content: []byte("synthetic traversal payload")}},
		},
		{
			name:  "NUL byte in filename",
			files: []ExportFile{{Name: "files/identity\x00-document.pdf", Content: []byte("synthetic NUL filename payload")}},
		},
		{
			name: "duplicate entry names",
			files: []ExportFile{
				{Name: "files/identity-document.pdf", Content: []byte("first synthetic payload")},
				{Name: "files/identity-document.pdf", Content: []byte("second synthetic payload")},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			const ownerID = "owner@example.invalid"
			modules := &identityArchiveModulesFake{files: test.files}
			service, err := NewService(&memoryRepository{}, modules)
			if err != nil {
				t.Fatal(err)
			}

			archiveBytes, err := service.ExportOwnArchive(context.Background(), ownerID)
			if archiveBytes != nil || !errors.Is(err, ErrInvalid) {
				t.Fatalf("archive bytes=%d err=%v, want nil archive and ErrInvalid", len(archiveBytes), err)
			}
			if modules.calls != 1 || modules.accountID != ownerID {
				t.Fatalf("module calls=%d account=%q, want one call scoped to owner", modules.calls, modules.accountID)
			}
		})
	}
}

func requireIdentityArchiveJSONKeys(t *testing.T, encoded []byte, expected ...string) {
	t.Helper()
	var object map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &object); err != nil {
		t.Fatal(err)
	}
	if len(object) != len(expected) {
		t.Fatalf("JSON object keys=%v, want exactly %v", object, expected)
	}
	for _, key := range expected {
		if _, ok := object[key]; !ok {
			t.Fatalf("JSON object is missing expected key %q: %v", key, object)
		}
	}
}
