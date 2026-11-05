package inspection

import (
	"bytes"
	"compress/zlib"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
)

func TestInspectorRejectsOverBudgetPDFBeforePoppler(t *testing.T) {
	dir := t.TempDir()
	pdfInfoMarker := filepath.Join(dir, "pdfinfo-called")
	pdfDetachMarker := filepath.Join(dir, "pdfdetach-called")
	t.Setenv("PDFINFO_CALL_MARKER", pdfInfoMarker)
	t.Setenv("PDFDETACH_CALL_MARKER", pdfDetachMarker)

	pdfInfoPath := filepath.Join(dir, "pdfinfo")
	pdfDetachPath := filepath.Join(dir, "pdfdetach")
	pdfInfoScript := "#!/bin/sh\nprintf x >> \"$PDFINFO_CALL_MARKER\"\ncase \"$1\" in\n  -js) cat >/dev/null ;;\n  -url) cat >/dev/null; printf 'Page Type URL\\n' ;;\n  -) cat >/dev/null; printf 'Pages: 1\\nEncrypted: no\\nJavaScript: no\\nForm: none\\n' ;;\n  *) exit 99 ;;\nesac\n"
	pdfDetachScript := "#!/bin/sh\nprintf x >> \"$PDFDETACH_CALL_MARKER\"\nprintf '0 embedded files\\n'\n"
	if err := os.WriteFile(pdfInfoPath, []byte(pdfInfoScript), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pdfDetachPath, []byte(pdfDetachScript), 0700); err != nil {
		t.Fatal(err)
	}
	pdfInfo, err := os.Open(pdfInfoPath)
	if err != nil {
		t.Fatal(err)
	}
	defer pdfInfo.Close()
	pdfDetach, err := os.Open(pdfDetachPath)
	if err != nil {
		t.Fatal(err)
	}
	defer pdfDetach.Close()

	inspector := &Inspector{
		cfg:       Config{MaxPixels: 40_000_000, PDFInfoPath: pdfInfoPath, PDFDetachPath: pdfDetachPath},
		pdfInfo:   pdfInfo,
		pdfDetach: pdfDetach,
	}

	underBudget := syntheticPDFWithXRefStream(t, 32,
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 10 10] /Resources << >> /Contents [4 0 R 5 0 R] >>",
		compressedWhitespaceStream(t, 1<<10),
		compressedWhitespaceStream(t, 1<<10),
	)
	if _, _, err := inspector.Inspect(dv.PurposeIdentity, "under-budget", underBudget); err != nil {
		t.Fatalf("in-budget control PDF was rejected: %v", err)
	}
	if got, err := os.ReadFile(pdfInfoMarker); err != nil || string(got) != "xxx" {
		t.Fatalf("in-budget control did not invoke pdfinfo three times: calls=%q err=%v", got, err)
	}
	if got, err := os.ReadFile(pdfDetachMarker); err != nil || string(got) != "x" {
		t.Fatalf("in-budget control did not reach pdfdetach: calls=%q err=%v", got, err)
	}
	if err := os.Remove(pdfInfoMarker); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(pdfDetachMarker); err != nil {
		t.Fatal(err)
	}

	// Two individually permitted 128 MiB content streams plus the decoded xref
	// stream exceed Inspector's 256 MiB aggregate document budget.
	largeStream := compressedWhitespaceStream(t, maxPDFDecodedStream)
	overBudget := syntheticPDFWithXRefStream(t, 32,
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 10 10] /Resources << >> /Contents [4 0 R 5 0 R] >>",
		largeStream,
		largeStream,
	)
	if len(overBudget) > maxBytes {
		t.Fatalf("compressed over-budget fixture exceeds input cap: input=%d cap=%d", len(overBudget), maxBytes)
	}
	if _, _, err := inspector.Inspect(dv.PurposeIdentity, "over-budget", overBudget); !errors.Is(err, ErrRejected) {
		t.Fatalf("aggregate-over-budget PDF returned %v, want rejection", err)
	}
	for _, marker := range []string{pdfInfoMarker, pdfDetachMarker} {
		if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("Poppler was invoked before aggregate-budget rejection: marker=%s stat=%v", filepath.Base(marker), err)
		}
	}
}

func compressedWhitespaceStream(t *testing.T, decodedBytes int64) string {
	t.Helper()
	const chunkSize = 64 << 10
	chunk := bytes.Repeat([]byte{' '}, chunkSize)
	var compressed bytes.Buffer
	writer := zlib.NewWriter(&compressed)
	for remaining := decodedBytes; remaining > 0; {
		count := int64(len(chunk))
		if remaining < count {
			count = remaining
		}
		if _, err := writer.Write(chunk[:int(count)]); err != nil {
			t.Fatal(err)
		}
		remaining -= count
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("<< /Filter /FlateDecode /Length %d >>\nstream\n%s\nendstream", compressed.Len(), compressed.Bytes())
}
