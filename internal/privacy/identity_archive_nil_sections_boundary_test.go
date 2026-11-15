package privacy

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"
)

type identityArchiveNilSectionsModulesFake struct {
	calls     int
	accountID string
	files     []ExportFile
}

func (f *identityArchiveNilSectionsModulesFake) ExportAdditionalOwnData(_ context.Context, accountID string) (map[string]json.RawMessage, []ExportFile, []string, error) {
	f.calls++
	f.accountID = accountID
	return nil, f.files, nil, nil
}

func TestExportOwnArchiveRejectsEmptyAccountIDBeforeReachingSources(t *testing.T) {
	modules := &identityArchiveNilSectionsModulesFake{}
	service, err := NewService(&memoryRepository{}, modules)
	if err != nil {
		t.Fatal(err)
	}

	archiveBytes, err := service.ExportOwnArchive(context.Background(), "")
	if archiveBytes != nil || !errors.Is(err, ErrInvalid) {
		t.Fatalf("archive bytes=%d err=%v, want nil archive and ErrInvalid", len(archiveBytes), err)
	}
	if modules.calls != 0 {
		t.Fatalf("module calls=%d, want 0 calls when account ID is empty", modules.calls)
	}
}

func TestExportOwnArchiveReturnsUnavailableWhenCompleteModulesNil(t *testing.T) {
	const ownerID = "owner@example.invalid"
	service, err := NewService(&memoryRepository{})
	if err != nil {
		t.Fatal(err)
	}

	archiveBytes, err := service.ExportOwnArchive(context.Background(), ownerID)
	if archiveBytes != nil || !errors.Is(err, ErrExportUnavailable) {
		t.Fatalf("archive bytes=%d err=%v, want nil archive and ErrExportUnavailable", len(archiveBytes), err)
	}
}

func TestExportOwnArchiveNormalizesNilSectionsAndSortsManifestFiles(t *testing.T) {
	const ownerID = "owner@example.invalid"
	const payloadA = "payload-alpha-doc"
	const payloadZ = "payload-zulu-doc"

	modules := &identityArchiveNilSectionsModulesFake{
		files: []ExportFile{
			{
				Name:        "files/zulu-doc.pdf",
				MediaType:   "application/pdf",
				Description: "Documento zulu sintético",
				Content:     []byte(payloadZ),
			},
			{
				Name:        "files/alpha-doc.pdf",
				MediaType:   "application/pdf",
				Description: "Documento alpha sintético",
				Content:     []byte(payloadA),
			},
		},
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

	manifestJSON, ok := entries["manifest.json"]
	if !ok {
		t.Fatal("archive is missing manifest.json")
	}
	dataJSON, ok := entries["data.json"]
	if !ok {
		t.Fatal("archive is missing data.json")
	}
	if !bytes.Contains(dataJSON, []byte("\"sections\": {}")) {
		t.Fatalf("data.json=%s, want normalized empty sections map {", string(dataJSON))
	}
	if bytes.Contains(dataJSON, []byte("\"sections\": null")) {
		t.Fatal("data.json serialized nil sections as null")
	}

	var manifest ExportManifest
	if err := json.Unmarshal(manifestJSON, &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Sections) != 1 || manifest.Sections[0] != "identity" {
		t.Fatalf("manifest sections=%v, want exactly [\"identity\"]", manifest.Sections)
	}
	if len(manifest.Files) != 2 {
		t.Fatalf("manifest files count=%d, want 2", len(manifest.Files))
	}
	if manifest.Files[0].Name != "files/alpha-doc.pdf" || manifest.Files[1].Name != "files/zulu-doc.pdf" {
		t.Fatalf("manifest files ordering=%+v, want lexicographical sort [alpha, zulu]", manifest.Files)
	}
}
