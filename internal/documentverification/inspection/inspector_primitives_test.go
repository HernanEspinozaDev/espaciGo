package inspection

import (
	"context"
	"errors"
	"testing"

	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

func TestJPEGAndPDFInfoValidationPrimitives(t *testing.T) {
	// 1. jpegOrientation
	// Short / truncated (< 7 bytes, loop doesn't run)
	if got := jpegOrientation([]byte{0xff, 0xd8}); got != 1 {
		t.Fatalf("expected 1 for truncated jpeg, got %d", got)
	}
	// Non-0xff byte sequence inside loop (len >= 7)
	if got := jpegOrientation([]byte{0xff, 0xd8, 0x00, 0x01, 0x02, 0x03, 0x04}); got != 1 {
		t.Fatalf("expected 1 for non-0xff marker, got %d", got)
	}
	// EOI marker 0xd9 (len >= 7)
	if got := jpegOrientation([]byte{0xff, 0xd8, 0xff, 0xd9, 0x00, 0x00, 0x00}); got != 1 {
		t.Fatalf("expected 1 for EOI marker, got %d", got)
	}
	// SOS marker 0xda (len >= 7)
	if got := jpegOrientation([]byte{0xff, 0xd8, 0xff, 0xda, 0x00, 0x00, 0x00}); got != 1 {
		t.Fatalf("expected 1 for SOS marker, got %d", got)
	}
	// Invalid size (< 2) inside loop (len >= 7)
	if got := jpegOrientation([]byte{0xff, 0xd8, 0xff, 0xe1, 0x00, 0x01, 0x00, 0x00}); got != 1 {
		t.Fatalf("expected 1 for size < 2, got %d", got)
	}
	// Size exceeding slice length (len >= 7)
	if got := jpegOrientation([]byte{0xff, 0xd8, 0xff, 0xe1, 0x00, 0x20, 0x00, 0x00}); got != 1 {
		t.Fatalf("expected 1 for size exceeding length, got %d", got)
	}
	// Marker 0xe1 with Exif header -> rejected (returns 0)
	exifPayload := append([]byte{0xff, 0xd8, 0xff, 0xe1, 0x00, 0x08}, []byte("Exif\x00\x00")...)
	if got := jpegOrientation(exifPayload); got != 0 {
		t.Fatalf("expected 0 for Exif marker, got %d", got)
	}
	// Marker 0xe2 without Exif -> returns 1
	nonExifPayload := []byte{0xff, 0xd8, 0xff, 0xe2, 0x00, 0x04, 0x11, 0x22}
	if got := jpegOrientation(nonExifPayload); got != 1 {
		t.Fatalf("expected 1 for non-Exif marker, got %d", got)
	}

	// 2. hasZeroEmbeddedPDFFiles
	if !hasZeroEmbeddedPDFFiles("0 embedded files") {
		t.Fatal("expected true for '0 embedded files'")
	}
	if !hasZeroEmbeddedPDFFiles("  0   Embedded   Files  \n") {
		t.Fatal("expected true for '  0   Embedded   Files  \n'")
	}
	if hasZeroEmbeddedPDFFiles("1 embedded files") {
		t.Fatal("expected false for count=1")
	}
	if hasZeroEmbeddedPDFFiles("0 embedded") {
		t.Fatal("expected false for missing word")
	}
	if hasZeroEmbeddedPDFFiles("0 embedded files found") {
		t.Fatal("expected false for extra words")
	}
	if hasZeroEmbeddedPDFFiles("") {
		t.Fatal("expected false for empty string")
	}

	// 3. hasEmptyPDFInfoURLList
	if !hasEmptyPDFInfoURLList("Page  Type  URL\n") {
		t.Fatal("expected true for exact header")
	}
	if !hasEmptyPDFInfoURLList("  PAGE   TYPE   URL  \n") {
		t.Fatal("expected true for case-insensitive header")
	}
	if !hasEmptyPDFInfoURLList("\n\nPage Type URL\n\n") {
		t.Fatal("expected true with leading/trailing newlines")
	}
	if hasEmptyPDFInfoURLList("Page Type\n") {
		t.Fatal("expected false for missing url column")
	}
	if hasEmptyPDFInfoURLList("Page Type URL\n1 URI http://example.com\n") {
		t.Fatal("expected false when URLs present")
	}
	if hasEmptyPDFInfoURLList("") {
		t.Fatal("expected false for empty string")
	}
	if hasEmptyPDFInfoURLList("\n  \n") {
		t.Fatal("expected false for only whitespace")
	}

	// 4. trustedPDFExecutable and trustedPDFDirectories bounds
	if trustedPDFExecutable("pdfinfo", "pdfinfo") {
		t.Fatal("expected false for non-absolute path")
	}
	if trustedPDFExecutable("/usr//bin/pdfinfo", "pdfinfo") {
		t.Fatal("expected false for dirty path")
	}
	if trustedPDFExecutable("/usr/bin/bash", "pdfinfo") {
		t.Fatal("expected false for name mismatch")
	}
	if trustedPDFExecutable("/nonexistent/directory/pdfinfo", "pdfinfo") {
		t.Fatal("expected false for nonexistent file")
	}
	if trustedPDFDirectories("relative/dir") {
		t.Fatal("expected false for relative dir")
	}
	if trustedPDFDirectories("/nonexistent/directory") {
		t.Fatal("expected false for nonexistent dir")
	}
}

func TestPDFStructureScannerPrimitiveEntries(t *testing.T) {
	s := &pdfStructureScanner{
		ctx:            context.Background(),
		maxImagePixels: 10_000,
	}

	// 1. positiveIntegerEntry
	dict := types.Dict{}
	// Key not found
	if _, err := s.positiveIntegerEntry(dict, "Width"); !errors.Is(err, ErrRejected) {
		t.Fatalf("expected ErrRejected for missing key, got %v", err)
	}
	// Key has non-integer value
	dict["Width"] = types.Name("not-an-int")
	if _, err := s.positiveIntegerEntry(dict, "Width"); !errors.Is(err, ErrRejected) {
		t.Fatalf("expected ErrRejected for non-integer value, got %v", err)
	}
	// Key has integer <= 0
	dict["Width"] = types.Integer(0)
	if _, err := s.positiveIntegerEntry(dict, "Width"); !errors.Is(err, ErrRejected) {
		t.Fatalf("expected ErrRejected for zero integer, got %v", err)
	}
	dict["Width"] = types.Integer(-5)
	if _, err := s.positiveIntegerEntry(dict, "Width"); !errors.Is(err, ErrRejected) {
		t.Fatalf("expected ErrRejected for negative integer, got %v", err)
	}
	// Valid integer
	dict["Width"] = types.Integer(100)
	val, err := s.positiveIntegerEntry(dict, "Width")
	if err != nil || val != 100 {
		t.Fatalf("expected 100, got %d, err=%v", val, err)
	}

	// 2. nameEntry
	// Not found
	name, found, err := s.nameEntry(dict, "NonExistent")
	if err != nil || found || name != "" {
		t.Fatalf("expected not found, got name=%q, found=%v, err=%v", name, found, err)
	}
	// Non-name object
	dict["Subtype"] = types.Integer(123)
	_, _, err = s.nameEntry(dict, "Subtype")
	if !errors.Is(err, ErrRejected) {
		t.Fatalf("expected ErrRejected for non-name object, got %v", err)
	}
	// Valid name
	dict["Subtype"] = types.Name("Form")
	name, found, err = s.nameEntry(dict, "Subtype")
	if err != nil || !found || name != "Form" {
		t.Fatalf("expected 'Form', got %q, found=%v, err=%v", name, found, err)
	}

	// 3. validateImagePixelLimit
	sNoMax := &pdfStructureScanner{maxImagePixels: 0}
	if err := sNoMax.validateImagePixelLimit(dict); !errors.Is(err, ErrRejected) {
		t.Fatalf("expected ErrRejected for maxImagePixels=0, got %v", err)
	}
	// Missing Height
	dict["Width"] = types.Integer(100)
	if err := s.validateImagePixelLimit(dict); !errors.Is(err, ErrRejected) {
		t.Fatalf("expected ErrRejected for missing Height, got %v", err)
	}
	// Oversize pixels: 200 * 200 = 40,000 > maxImagePixels(10,000)
	dict["Height"] = types.Integer(200)
	dict["Width"] = types.Integer(200)
	if err := s.validateImagePixelLimit(dict); !errors.Is(err, ErrRejected) {
		t.Fatalf("expected ErrRejected for pixel bomb, got %v", err)
	}
	// Valid pixel dimensions: 50 * 50 = 2,500 <= 10,000
	dict["Width"] = types.Integer(50)
	dict["Height"] = types.Integer(50)
	if err := s.validateImagePixelLimit(dict); err != nil {
		t.Fatalf("expected nil for valid image dimensions, got %v", err)
	}

	// 4. validateFormInlineImagePixelLimit
	// Non-form subtype
	dict["Subtype"] = types.Name("Image")
	if err := s.validateFormInlineImagePixelLimit(dict, []byte("BI ... ID ... EI")); err != nil {
		t.Fatalf("expected nil for non-Form subtype, got %v", err)
	}
	// Subtype Form with nil xref -> ErrRejected
	dict["Subtype"] = types.Name("Form")
	if err := s.validateFormInlineImagePixelLimit(dict, []byte("BI ... ID ... EI")); !errors.Is(err, ErrRejected) {
		t.Fatalf("expected ErrRejected for Form with nil xref, got %v", err)
	}

	// 5. dereference
	obj, err := s.dereference(types.Integer(999))
	if err != nil || obj != types.Integer(999) {
		t.Fatalf("expected Integer(999), got %v, err=%v", obj, err)
	}
}
