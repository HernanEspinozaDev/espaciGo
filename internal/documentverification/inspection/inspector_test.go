package inspection

import (
	"bytes"
	"compress/zlib"
	"context"
	"crypto/md5"
	"crypto/rc4"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/filter"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
	"golang.org/x/sys/unix"
)

func tinyPNG(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	im := image.NewRGBA(image.Rect(0, 0, 2, 2))
	im.Set(0, 0, color.RGBA{R: 255, A: 255})
	if err := png.Encode(&b, im); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}
func inspector() *Inspector {
	return newTestInspector(Config{MaxPixels: 100, PDFInfoPath: "/does/not/exist", PDFDetachPath: "/does/not/exist"})
}

func newTestInspector(c Config) *Inspector {
	if c.MaxPixels <= 0 {
		c.MaxPixels = 40_000_000
	}
	return &Inspector{cfg: c}
}

func TestNewRejectsUntrustedExecutableConfiguration(t *testing.T) {
	dir := t.TempDir()
	pdfinfo := filepath.Join(dir, "pdfinfo")
	pdfdetach := filepath.Join(dir, "pdfdetach")
	for _, path := range []string{pdfinfo, pdfdetach} {
		if err := os.WriteFile(path, []byte("synthetic executable"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	if got := New(Config{PDFInfoPath: pdfinfo, PDFDetachPath: pdfdetach}); got != nil {
		t.Fatal("accepted executables in a user-writable directory")
	}
}

func TestNewAcceptsAdministratorManagedPopplerExecutables(t *testing.T) {
	pdfinfo, pdfdetach := popplerBinaries(t)
	if got := New(Config{PDFInfoPath: pdfinfo, PDFDetachPath: pdfdetach}); got == nil {
		t.Fatal("rejected administrator-managed Poppler executables")
	}
}
func TestInspectPNGNormalizesAndTrustsMetadata(t *testing.T) {
	b := tinyPNG(t)
	f, out, err := inspector().Inspect(dv.PurposeIdentity, "opaque-id", b)
	if err != nil {
		t.Fatal(err)
	}
	if f.ID != "opaque-id" || f.MIME != "image/png" || f.Size != int64(len(out)) || f.State != dv.FilePending || f.Pages != 1 {
		t.Fatalf("metadata: %#v", f)
	}
	if got, want := f.ContentSHA256, sha256.Sum256(out); got != want {
		t.Fatalf("content hash=%x, want normalized content hash %x", got, want)
	}
	if len(out) == 0 {
		t.Fatal("empty normalized output")
	}
	if _, format, err := image.Decode(bytes.NewReader(out)); err != nil || format != "png" {
		t.Fatal("normalized PNG failed to decode")
	}
}
func TestInspectRejectsUnsupportedAndOversize(t *testing.T) {
	i := inspector()
	for _, tc := range []struct {
		name string
		b    []byte
	}{{"unsupported", []byte("text")}, {"oversize", bytes.Repeat([]byte{0}, 10*1024*1024+1)}} {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := i.Inspect(dv.PurposeSpace, "id", tc.b); err == nil {
				t.Fatal("accepted invalid input")
			}
		})
	}
}
func TestInspectRejectsPixelBomb(t *testing.T) {
	var b bytes.Buffer
	im := image.NewRGBA(image.Rect(0, 0, 11, 10))
	_ = png.Encode(&b, im)
	if _, _, err := inspector().Inspect(dv.PurposeSpace, "id", b.Bytes()); err == nil {
		t.Fatal("accepted pixel bomb")
	}
}
func TestInspectRejectsBadPurposeAndID(t *testing.T) {
	i := inspector()
	if _, _, e := i.Inspect(dv.Purpose("other"), "id", tinyPNG(t)); e == nil {
		t.Fatal("accepted purpose")
	}
	if _, _, e := i.Inspect(dv.PurposeIdentity, "", tinyPNG(t)); e == nil {
		t.Fatal("accepted id")
	}
}
func TestPDFInspectorRejectsEscapedJavaScriptActionUsingPoppler(t *testing.T) {
	pdfinfo, pdfdetach := popplerBinaries(t)
	data := syntheticPDF(t, " /Open#41ction << /S /Java#53cript /J#53 (app.alert\\(1\\)) >>")
	cmd := exec.Command(pdfinfo, "-")
	cmd.Stdin = bytes.NewReader(data)
	info, err := cmd.CombinedOutput()
	if err != nil || !hasPDFInfoField(info, "JavaScript", "yes") {
		t.Fatalf("Poppler did not structurally identify synthetic JavaScript: err=%v output=%s", err, info)
	}
	if _, _, err := New(Config{PDFInfoPath: pdfinfo, PDFDetachPath: pdfdetach}).Inspect(dv.PurposeIdentity, "id", data); err == nil {
		t.Fatal("accepted PDF with escaped structural JavaScript action")
	}
}

func TestPDFInspectorRejectsEscapedAcroFormUsingPoppler(t *testing.T) {
	pdfinfo, pdfdetach := popplerBinaries(t)
	data := syntheticPDF(t, " /Acro#46orm << /Fields [] >>")
	cmd := exec.Command(pdfinfo, "-")
	cmd.Stdin = bytes.NewReader(data)
	info, err := cmd.CombinedOutput()
	if err != nil || !hasPDFInfoField(info, "Form", "AcroForm") {
		t.Fatalf("Poppler did not structurally identify synthetic AcroForm: err=%v output=%s", err, info)
	}
	if _, _, err := New(Config{PDFInfoPath: pdfinfo, PDFDetachPath: pdfdetach}).Inspect(dv.PurposeIdentity, "id", data); err == nil {
		t.Fatal("accepted PDF with escaped AcroForm")
	}
}

func TestPDFInspectorRejectsEscapedURIActionUsingPoppler(t *testing.T) {
	pdfinfo, pdfdetach := popplerBinaries(t)
	data := syntheticPDFWithObjects(t,
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 10 10] /Resources << >> /Contents 4 0 R /Annots [5 0 R] >>",
		"<< /Length 0 >>\nstream\n\nendstream",
		"<< /Type /Annot /Subtype /Link /Rect [0 0 1 1] /A << /S /U#52I /U#52I (https://example.invalid) >> >>",
	)
	cmd := exec.Command(pdfinfo, "-url", "-")
	cmd.Stdin = bytes.NewReader(data)
	urls, err := cmd.CombinedOutput()
	if err != nil || !bytes.Contains(urls, []byte("https://example.invalid")) {
		t.Fatalf("Poppler did not structurally identify synthetic URI action: err=%v output=%s", err, urls)
	}
	if _, _, err := New(Config{PDFInfoPath: pdfinfo, PDFDetachPath: pdfdetach}).Inspect(dv.PurposeIdentity, "id", data); err == nil {
		t.Fatal("accepted PDF with escaped URI action")
	}
}

func TestPDFInspectorRejectsEscapedLaunchAction(t *testing.T) {
	pdfinfo, pdfdetach := popplerBinaries(t)
	data := syntheticPDF(t, " /Open#41ction << /S /Laun#63h /F (synthetic) >>")
	if _, _, err := New(Config{PDFInfoPath: pdfinfo, PDFDetachPath: pdfdetach}).Inspect(dv.PurposeIdentity, "id", data); err == nil {
		t.Fatal("accepted PDF with escaped Launch action")
	}
}

func TestPDFInspectorRejectsCompressedLaunchAction(t *testing.T) {
	pdfinfo, pdfdetach := popplerBinaries(t)
	data := syntheticPDFWithCompressedLaunchAction(t)
	cmd := exec.Command(pdfinfo, "-")
	cmd.Stdin = bytes.NewReader(data)
	info, err := cmd.CombinedOutput()
	if err != nil || !hasPDFInfoField(info, "Pages", "1") {
		t.Fatalf("compressed-action PDF is not structurally parseable: err=%v output=%s", err, info)
	}
	if bytes.Contains(data, []byte("/OpenAction")) || bytes.Contains(data, []byte("/Launch")) {
		t.Fatal("compressed-action fixture must hide active PDF names from raw scanning")
	}
	if err := validatePDFStructure(data, 40_000_000, 1); !errors.Is(err, errPDFActiveContent) {
		t.Fatalf("structural parser did not identify the compressed Launch action: %v", err)
	}
	if _, _, err := New(Config{PDFInfoPath: pdfinfo, PDFDetachPath: pdfdetach}).Inspect(dv.PurposeIdentity, "id", data); err == nil {
		t.Fatal("accepted compressed OpenAction/Launch dictionary")
	}
}

func TestPDFInspectorRejectsUnreferencedCompressedLaunchAction(t *testing.T) {
	data := syntheticPDFWithUnreferencedCompressedLaunchAction(t)
	if bytes.Contains(data, []byte("/Launch")) || bytes.Contains(data, []byte("/OpenAction")) {
		t.Fatal("unreferenced compressed-action fixture must hide active PDF names from raw scanning")
	}
	if err := validatePDFStructure(data, 40_000_000, 1); !errors.Is(err, errPDFActiveContent) {
		t.Fatalf("unreferenced compressed Launch action was not rejected: %v", err)
	}
}

func TestPDFInspectorRejectsUnknownActionType(t *testing.T) {
	data := syntheticPDFWithObjects(t,
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 10 10] /Resources << >> /Contents 4 0 R /Annots [5 0 R] >>",
		"<< /Length 0 >>\nstream\n\nendstream",
		"<< /Type /Annot /Subtype /Link /Rect [0 0 1 1] /A << /S /FutureAction /Payload (synthetic) >> >>",
	)
	conf := model.NewStatelessConfiguration()
	conf.ValidationMode = model.ValidationStrict
	conf.Offline = true
	conf.Optimize = false
	parsed, parseErr := api.ReadAndValidate(context.Background(), bytes.NewReader(data), conf)
	if parseErr != nil {
		if !strings.Contains(parseErr.Error(), `unsupported action type "FutureAction"`) {
			t.Fatalf("unknown-action fixture failed for an unexpected parser reason: %v", parseErr)
		}
		return
	}
	if parsed == nil {
		t.Fatal("strict parser returned no context without an error")
	}
	if err := validatePDFStructure(data, 40_000_000, 1); !errors.Is(err, errPDFActiveContent) {
		t.Fatalf("unknown action was not rejected structurally: %v", err)
	}
}

func TestValidatePDFStructureEnforcesAggregateDecodeBudgetBeforeParserMaterialization(t *testing.T) {
	// The xref and catalog metadata streams each fit the configured per-stream
	// limit, but their combined decoded output exceeds the smaller document budget.
	const metadataDecodedBytes = 64 << 10
	const xrefEntryCount = 8 << 10
	const aggregateBudget = 96 << 10
	const perStreamLimit = metadataDecodedBytes + 1
	const prefix = `<x:xmpmeta xmlns:x="adobe:ns:meta/">`
	const suffix = `</x:xmpmeta>`
	metadata := append([]byte(prefix), bytes.Repeat([]byte(" "), metadataDecodedBytes-len(prefix)-len(suffix))...)
	metadata = append(metadata, suffix...)
	var compressed bytes.Buffer
	writer := zlib.NewWriter(&compressed)
	if _, err := writer.Write(metadata); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if compressed.Len() >= metadataDecodedBytes/20 {
		t.Fatalf("metadata fixture is not highly compressed: compressed=%d decoded=%d", compressed.Len(), metadataDecodedBytes)
	}
	metadataStream := fmt.Sprintf("<< /Type /Metadata /Subtype /XML /Filter /FlateDecode /Length %d >>\nstream\n%s\nendstream", compressed.Len(), compressed.Bytes())
	data := syntheticPDFWithXRefStream(t, xrefEntryCount,
		"<< /Type /Catalog /Pages 2 0 R /Metadata 5 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 10 10] /Resources << >> /Contents 4 0 R >>",
		"<< /Length 0 >>\nstream\n\nendstream",
		metadataStream,
	)
	xrefDecodedBytes := xrefEntryCount * 7
	if metadataDecodedBytes >= perStreamLimit || xrefDecodedBytes >= perStreamLimit {
		t.Fatal("fixture streams must each fit below the individual decode limit")
	}
	if metadataDecodedBytes+xrefDecodedBytes <= aggregateBudget {
		t.Fatalf("fixture aggregate=%d does not exceed budget=%d", metadataDecodedBytes+xrefDecodedBytes, aggregateBudget)
	}
	underLimitConf := model.NewStatelessConfiguration()
	underLimitConf.ValidationMode = model.ValidationStrict
	underLimitConf.Offline = true
	underLimitConf.Limits.MaxDecodeBytes = metadataDecodedBytes - 1
	if _, err := api.ReadAndValidate(context.Background(), bytes.NewReader(data), underLimitConf); !errors.Is(err, filter.ErrDecodeLimitExceeded) {
		t.Fatalf("metadata decode at individual limit %d: error=%v, want decode-limit error", metadataDecodedBytes-1, err)
	}
	conf := model.NewStatelessConfiguration()
	conf.ValidationMode = model.ValidationStrict
	conf.Offline = true
	conf.Limits.MaxDecodeBytes = perStreamLimit
	parsed, err := api.ReadAndValidate(context.Background(), bytes.NewReader(data), conf)
	if err != nil {
		t.Fatalf("ReadAndValidate rejected individually in-limit metadata and xref streams: %v", err)
	}
	if parsed == nil || parsed.XRefTable == nil || parsed.PageCount != 1 {
		t.Fatal("ReadAndValidate returned an incomplete PDF context")
	}

	if err := validatePDFStructureWithBudget(data, 40_000_000, 1, 3<<20); err != nil {
		t.Fatalf("valid PDF below the aggregate decoded-byte budget was rejected: %v", err)
	}
	if err := validatePDFStructureWithBudget(data, 40_000_000, 1, aggregateBudget); !errors.Is(err, ErrRejected) {
		t.Fatalf("aggregate decoded output exceeded budget %d but parser accepted it: %v", aggregateBudget, err)
	}
}

func TestPDFParserBudgetCachesSharedStreamBeforeValidation(t *testing.T) {
	const lookupReferences = 100
	const lookupDecodedBytes = 3 * 256
	const aggregateBudget = 64 << 10
	lookup := bytes.Repeat([]byte{0x42}, lookupDecodedBytes)
	var compressed bytes.Buffer
	writer := zlib.NewWriter(&compressed)
	if _, err := writer.Write(lookup); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	lookupStream := fmt.Sprintf("<< /Filter /FlateDecode /Length %d >>\nstream\n%s\nendstream", compressed.Len(), compressed.Bytes())

	var colorSpaces strings.Builder
	colorSpaces.WriteString("<< ")
	for index := range lookupReferences {
		fmt.Fprintf(&colorSpaces, "/CS%d [ /Indexed /DeviceRGB 255 5 0 R ] ", index)
	}
	colorSpaces.WriteString(">>")
	data := syntheticPDFWithXRefStream(t, 16,
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 10 10] /Resources << /ColorSpace %s >> /Contents 4 0 R >>", colorSpaces.String()),
		"<< /Length 0 >>\nstream\n\nendstream",
		lookupStream,
	)

	parserLimit, _, scannerBudget, err := pdfDecodeLimits(data, aggregateBudget, 1)
	if err != nil {
		t.Fatalf("compute parser limit: %v", err)
	}
	if int64(lookupDecodedBytes) >= parserLimit {
		t.Fatalf("lookup output %d must fit each parser stream limit %d", lookupDecodedBytes, parserLimit)
	}
	if lookupReferences*lookupDecodedBytes <= int(aggregateBudget) {
		t.Fatal("fixture repeated decoder output does not exceed the aggregate budget")
	}

	conf := model.NewStatelessConfiguration()
	conf.ValidationMode = model.ValidationStrict
	conf.Offline = true
	conf.Limits.MaxDecodeBytes = parserLimit
	conf.Cmd = model.VALIDATE
	if _, err := api.ReadAndValidate(context.Background(), bytes.NewReader(data), conf); err != nil {
		t.Fatalf("fixture must reach repeated parser decoding under the per-stream limit: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), pdfParseTimeout)
	defer cancel()
	parsed, usage, err := readAndValidatePDFWithinBudget(ctx, data, conf, parserLimit, aggregateBudget-scannerBudget)
	if err != nil {
		t.Fatalf("budgeted parser rejected valid shared lookup stream: %v", err)
	}
	if parsed == nil || parsed.PageCount != 1 {
		t.Fatal("budgeted parser returned an incomplete PDF context")
	}
	if usage.predecodedBytes < lookupDecodedBytes || usage.predecodedBytes >= lookupReferences*lookupDecodedBytes {
		t.Fatalf("shared stream was not counted once before validation: decoded=%d references=%d stream-bytes=%d", usage.predecodedBytes, lookupReferences, lookupDecodedBytes)
	}
	if usage.total() > aggregateBudget-scannerBudget {
		t.Fatalf("parser budget exceeded: usage=%d budget=%d", usage.total(), aggregateBudget-scannerBudget)
	}
}

func syntheticPDFWithXRefStream(t *testing.T, xrefEntryCount int, objects ...string) []byte {
	t.Helper()
	const xrefEntryWidth = 7 // /W [1 4 2]
	xrefObjectNumber := len(objects) + 1
	if xrefEntryCount <= xrefObjectNumber {
		t.Fatalf("xref entry count %d does not include object %d", xrefEntryCount, xrefObjectNumber)
	}

	var b bytes.Buffer
	b.WriteString("%PDF-1.7\n")
	offsets := make([]int, xrefObjectNumber+1)
	for index, object := range objects {
		objectNumber := index + 1
		offsets[objectNumber] = b.Len()
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", objectNumber, object)
	}
	xrefOffset := b.Len()
	offsets[xrefObjectNumber] = xrefOffset

	xref := make([]byte, xrefEntryCount*xrefEntryWidth)
	xref[5], xref[6] = 0xff, 0xff // object zero is permanently free
	for objectNumber := 1; objectNumber <= xrefObjectNumber; objectNumber++ {
		entry := xref[objectNumber*xrefEntryWidth:]
		entry[0] = 1
		binary.BigEndian.PutUint32(entry[1:5], uint32(offsets[objectNumber]))
	}
	for objectNumber := xrefObjectNumber + 1; objectNumber < xrefEntryCount; objectNumber++ {
		entry := xref[objectNumber*xrefEntryWidth:]
		entry[5], entry[6] = 0xff, 0xff
	}

	var encodedXRef bytes.Buffer
	xrefWriter := zlib.NewWriter(&encodedXRef)
	if _, err := xrefWriter.Write(xref); err != nil {
		t.Fatal(err)
	}
	if err := xrefWriter.Close(); err != nil {
		t.Fatal(err)
	}
	fmt.Fprintf(&b, "%d 0 obj\n<< /Type /XRef /Size %d /Root 1 0 R /W [1 4 2] /Filter /FlateDecode /Length %d >>\nstream\n", xrefObjectNumber, xrefEntryCount, encodedXRef.Len())
	b.Write(encodedXRef.Bytes())
	b.WriteString("\nendstream\nendobj\nstartxref\n")
	fmt.Fprintf(&b, "%d\n%%%%EOF\n", xrefOffset)
	return b.Bytes()
}

func TestPDFParserBudgetIncludesCompressedObjectStreamRead(t *testing.T) {
	data := syntheticPDFWithIndirectLengthObjectStream(t, 8<<10)
	for _, testCase := range []struct {
		name         string
		budget       int64
		wantRejected bool
	}{
		{name: "object stream exceeds parser share", budget: 64 << 10, wantRejected: true},
		{name: "object stream fits parser share", budget: 1 << 20},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			parserLimit, _, scannerBudget, err := pdfDecodeLimits(data, testCase.budget, 1)
			if err != nil {
				t.Fatalf("compute parser limit: %v", err)
			}
			conf := model.NewStatelessConfiguration()
			conf.Reader15 = true
			conf.ValidationMode = model.ValidationStrict
			conf.Offline = true
			conf.Optimize = false
			conf.Limits.MaxInputBytes = maxBytes
			conf.Limits.MaxObjectBytes = maxBytes
			conf.Limits.MaxStreamBytes = maxBytes
			conf.Limits.MaxDecodeBytes = parserLimit
			conf.Limits.MaxObjectCount = maxPDFObjects
			conf.Limits.MaxObjectStreamCount = maxPDFObjectStreams
			conf.Limits.MaxObjectStreamFirst = min(maxPDFObjectProlog, parserLimit)
			conf.Limits.MaxXRefEntries = maxPDFObjects
			conf.Limits.MaxRecursionDepth = maxPDFRecursion
			conf.Cmd = model.VALIDATE

			readCtx, cancelRead := context.WithTimeout(context.Background(), pdfParseTimeout)
			readParsed, readErr := api.ReadContext(readCtx, bytes.NewReader(data), conf.Clone())
			cancelRead()
			if testCase.wantRejected {
				if !errors.Is(readErr, filter.ErrDecodeLimitExceeded) {
					t.Fatalf("compressed object stream read error=%v, want decode-limit error", readErr)
				}
			} else {
				if readErr != nil {
					t.Fatalf("ReadContext rejected in-budget compressed object stream: %v", readErr)
				}
				entry := readParsed.Table[6]
				if !readParsed.Read.UsingObjectStreams || entry == nil || entry.Object != types.Integer(4) {
					t.Fatal("ReadContext did not materialize the compressed indirect stream length")
				}
			}

			ctx, cancel := context.WithTimeout(context.Background(), pdfParseTimeout)
			defer cancel()
			parsed, usage, err := readAndValidatePDFWithinBudget(ctx, data, conf, parserLimit, testCase.budget-scannerBudget)
			if testCase.wantRejected {
				if !errors.Is(err, ErrRejected) {
					t.Fatalf("oversized compressed object stream was not rejected before scanning: parsed=%v err=%v", parsed != nil, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("valid compressed object stream was rejected: %v", err)
			}
			if parsed == nil || parsed.Read == nil || !parsed.Read.UsingObjectStreams || parsed.PageCount != 1 {
				t.Fatal("budgeted parser did not validate the compressed-object-stream fixture")
			}
			entry, found := parsed.Table[6]
			if !found || entry == nil || entry.Object != types.Integer(4) {
				t.Fatalf("indirect stream length was not materialized from the object stream: entry=%#v", entry)
			}
			if usage.total() > testCase.budget-scannerBudget {
				t.Fatalf("parser budget exceeded: usage=%d budget=%d", usage.total(), testCase.budget-scannerBudget)
			}
		})
	}
}

func syntheticPDFWithIndirectLengthObjectStream(t *testing.T, paddingBytes int) []byte {
	t.Helper()
	objectStreamBody := "4 (" + strings.Repeat("A", paddingBytes) + ")"
	objectStreamProlog := fmt.Sprintf("6 0 7 %d ", len("4 "))
	objectStreamContent := objectStreamProlog + objectStreamBody
	var compressedObjectStream bytes.Buffer
	objectStreamWriter := zlib.NewWriter(&compressedObjectStream)
	if _, err := objectStreamWriter.Write([]byte(objectStreamContent)); err != nil {
		t.Fatal(err)
	}
	if err := objectStreamWriter.Close(); err != nil {
		t.Fatal(err)
	}

	var pdf bytes.Buffer
	pdf.WriteString("%PDF-1.7\n")
	offsets := make(map[int]int)
	writeObject := func(objectNumber int, body string) {
		offsets[objectNumber] = pdf.Len()
		fmt.Fprintf(&pdf, "%d 0 obj\n%s\nendobj\n", objectNumber, body)
	}
	writeObject(1, "<< /Type /Catalog /Pages 2 0 R >>")
	writeObject(2, "<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	writeObject(3, "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 10 10] /Resources << >> /Contents 8 0 R >>")
	writeObject(5, fmt.Sprintf("<< /Type /ObjStm /N 2 /First %d /Filter /FlateDecode /Length %d >>\nstream\n%s\nendstream", len(objectStreamProlog), compressedObjectStream.Len(), compressedObjectStream.Bytes()))
	writeObject(8, "<< /Length 6 0 R >>\nstream\nq Q\nendstream")
	xrefOffset := pdf.Len()
	offsets[9] = xrefOffset

	const entryWidth = 7
	xref := make([]byte, 10*entryWidth)
	setFree := func(objectNumber int, generation uint16) {
		entry := xref[objectNumber*entryWidth:]
		entry[0] = 0
		binary.BigEndian.PutUint16(entry[5:7], generation)
	}
	setFree(0, 65535)
	setFree(4, 0)
	for _, objectNumber := range []int{1, 2, 3, 5, 8, 9} {
		entry := xref[objectNumber*entryWidth:]
		entry[0] = 1
		binary.BigEndian.PutUint32(entry[1:5], uint32(offsets[objectNumber]))
	}
	for index, objectNumber := range []int{6, 7} {
		entry := xref[objectNumber*entryWidth:]
		entry[0] = 2
		binary.BigEndian.PutUint32(entry[1:5], 5)
		binary.BigEndian.PutUint16(entry[5:7], uint16(index))
	}

	var compressedXRef bytes.Buffer
	xrefWriter := zlib.NewWriter(&compressedXRef)
	if _, err := xrefWriter.Write(xref); err != nil {
		t.Fatal(err)
	}
	if err := xrefWriter.Close(); err != nil {
		t.Fatal(err)
	}
	fmt.Fprintf(&pdf, "9 0 obj\n<< /Type /XRef /Size 10 /Root 1 0 R /W [1 4 2] /Filter /FlateDecode /Length %d >>\nstream\n", compressedXRef.Len())
	pdf.Write(compressedXRef.Bytes())
	pdf.WriteString("\nendstream\nendobj\nstartxref\n")
	fmt.Fprintf(&pdf, "%d\n%%%%EOF\n", xrefOffset)
	return pdf.Bytes()
}

func TestPDFReadAndValidateRejectsCatalogMetadataAboveDecodeLimit(t *testing.T) {
	const decodedBytes = 128 << 10
	const prefix = `<x:xmpmeta xmlns:x="adobe:ns:meta/">`
	const suffix = `</x:xmpmeta>`
	metadata := append([]byte(prefix), bytes.Repeat([]byte(" "), decodedBytes-len(prefix)-len(suffix))...)
	metadata = append(metadata, suffix...)
	var compressed bytes.Buffer
	writer := zlib.NewWriter(&compressed)
	if _, err := writer.Write(metadata); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if compressed.Len() >= decodedBytes/20 {
		t.Fatalf("metadata fixture is not highly compressed: compressed=%d decoded=%d", compressed.Len(), decodedBytes)
	}
	data := syntheticPDFWithObjects(t,
		"<< /Type /Catalog /Pages 2 0 R /Metadata 5 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 10 10] /Resources << >> /Contents 4 0 R >>",
		"<< /Length 0 >>\nstream\n\nendstream",
		fmt.Sprintf("<< /Type /Metadata /Subtype /XML /Filter /FlateDecode /Length %d >>\nstream\n%s\nendstream", compressed.Len(), compressed.Bytes()),
	)
	conf := model.NewStatelessConfiguration()
	conf.ValidationMode = model.ValidationStrict
	conf.Offline = true
	conf.Limits.MaxDecodeBytes = 4 << 10
	if _, err := api.ReadAndValidate(context.Background(), bytes.NewReader(data), conf); !errors.Is(err, filter.ErrDecodeLimitExceeded) {
		t.Fatalf("ReadAndValidate error=%v, want configured decode-limit error", err)
	}
}

type unsupportedPDFObject struct{}

func (unsupportedPDFObject) Clone() types.Object { return unsupportedPDFObject{} }
func (unsupportedPDFObject) String() string      { return "unsupported" }
func (unsupportedPDFObject) PDFString() string   { return "unsupported" }

func TestPDFStructureScannerRejectsUnsupportedObjectType(t *testing.T) {
	scanner := pdfStructureScanner{}
	if err := scanner.walk(unsupportedPDFObject{}, 0); !errors.Is(err, ErrRejected) {
		t.Fatalf("unsupported PDF object type was not rejected: %v", err)
	}
}

func TestPDFStructureScannerStopsWhenContextIsCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	scanner := pdfStructureScanner{ctx: ctx}
	if err := scanner.walk(types.Array{types.Integer(1)}, 0); !errors.Is(err, ErrRejected) {
		t.Fatalf("scanner continued after its deadline context was cancelled: %v", err)
	}
}

func TestPDFStructureScannerEnforcesNodeBudget(t *testing.T) {
	values := make(types.Array, maxPDFObjects*6)
	scanner := pdfStructureScanner{}
	if err := scanner.walk(values, 0); !errors.Is(err, ErrRejected) {
		t.Fatalf("scanner accepted a structure exceeding its node budget: %v", err)
	}
}

func TestPDFStructureScannerRejectsNilSecurityNameValues(t *testing.T) {
	for _, key := range []string{"Type", "Subtype", "S"} {
		t.Run(key, func(t *testing.T) {
			scanner := pdfStructureScanner{}
			if err := scanner.walkDict(types.Dict{key: nil}, 0); !errors.Is(err, ErrRejected) {
				t.Fatalf("accepted null value for security-sensitive /%s field: %v", key, err)
			}
		})
	}
}

func TestReserveDecodedPDFBytesEnforcesAggregateLimit(t *testing.T) {
	if used, ok := reserveDecodedPDFBytes(6, 4, 10); !ok || used != 10 {
		t.Fatalf("exact budget boundary: used=%d accepted=%v", used, ok)
	}
	if used, ok := reserveDecodedPDFBytes(6, 5, 10); ok || used != 6 {
		t.Fatalf("over-budget reservation: used=%d accepted=%v", used, ok)
	}
	if used, ok := reserveDecodedPDFBytes(1<<63-1, 1, 1<<63-1); ok || used != 1<<63-1 {
		t.Fatalf("overflowing reservation: used=%d accepted=%v", used, ok)
	}
}

func TestPDFStructureScannerEnforcesAggregateObjectStreamBudget(t *testing.T) {
	generation := 0
	streamOneNumber, streamTwoNumber := 5, 7
	newStream := func(objectNumber int) *types.ObjectStreamDict {
		prolog := fmt.Sprintf("%d 0 ", objectNumber)
		stream := &types.ObjectStreamDict{
			StreamDict:     types.StreamDict{Content: []byte(prolog + "1")},
			MaxDecodeBytes: maxPDFDecodedStream,
			ObjCount:       1,
			FirstObjOffset: len(prolog),
		}
		stream.ObjArray = types.Array{
			types.NewLazyObjectStreamObject(stream, 0, -1, func(context.Context, string) (types.Object, error) {
				return types.Dict{}, nil
			}),
		}
		return stream
	}
	streamOne, streamTwo := newStream(6), newStream(8)
	newLazy := func(stream *types.ObjectStreamDict) types.Object {
		return types.NewLazyObjectStreamObject(stream, 0, -1, func(context.Context, string) (types.Object, error) {
			return types.Dict{}, nil
		})
	}
	objectStreamIndex := 0
	entries := map[int]*model.XRefTableEntry{
		streamOneNumber: {Generation: &generation, Object: *streamOne},
		6:               {Generation: &generation, Object: newLazy(streamOne), ObjectStream: &streamOneNumber, ObjectStreamInd: &objectStreamIndex},
		streamTwoNumber: {Generation: &generation, Object: *streamTwo},
		8:               {Generation: &generation, Object: newLazy(streamTwo), ObjectStream: &streamTwoNumber, ObjectStreamInd: &objectStreamIndex},
	}
	scanner := pdfStructureScanner{
		xref:                 &model.XRefTable{Table: entries},
		visited:              make(map[pdfObjectRef]struct{}),
		decodedObjectStreams: make(map[int]struct{}),
		decodedByteLimit:     9,
	}
	if err := scanner.walk(types.IndirectRef{ObjectNumber: 6, GenerationNumber: 0}, 0); err != nil {
		t.Fatalf("first object stream unexpectedly rejected: %v", err)
	}
	if err := scanner.walk(types.IndirectRef{ObjectNumber: 8, GenerationNumber: 0}, 0); !errors.Is(err, ErrRejected) {
		t.Fatalf("aggregate object-stream budget was not enforced: %v", err)
	}
}

func TestPDFStructureScannerReservesBudgetBeforeObjectStreamDecode(t *testing.T) {
	generation := 0
	prolog := []byte("6 0 ")
	decoded := append(append([]byte(nil), prolog...), []byte("123456")...)
	var compressed bytes.Buffer
	writer := zlib.NewWriter(&compressed)
	if _, err := writer.Write(decoded); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	stream := &types.ObjectStreamDict{
		StreamDict: types.StreamDict{
			Raw:            compressed.Bytes(),
			FilterPipeline: []types.PDFFilter{{Name: "FlateDecode"}},
		},
		MaxDecodeBytes: maxPDFDecodedStream,
		FirstObjOffset: len(prolog),
		ObjCount:       1,
	}
	stream.ObjArray = types.Array{
		types.NewLazyObjectStreamObject(stream, 0, -1, func(context.Context, string) (types.Object, error) {
			return types.Dict{}, nil
		}),
	}
	scanner := pdfStructureScanner{
		xref: &model.XRefTable{Table: map[int]*model.XRefTableEntry{
			5: {Generation: &generation, Object: stream},
		}},
		decodedObjectStreams: make(map[int]struct{}),
		decodedByteLimit:     5,
	}
	if err := scanner.accountObjectStream(5); !errors.Is(err, ErrRejected) {
		t.Fatalf("over-budget object stream was accepted: %v", err)
	}
	if stream.Content != nil {
		t.Fatalf("object-stream decoder materialized %d bytes before enforcing the remaining budget", len(stream.Content))
	}
}

func TestPDFStructureScannerPublishesBoundedObjectStreamDecodeToXRef(t *testing.T) {
	generation, objectStreamNumber, objectStreamIndex := 0, 5, 0
	prolog := []byte("6 0 ")
	decoded := append(append([]byte(nil), prolog...), []byte("<< /Type /Example >>")...)
	var compressed bytes.Buffer
	writer := zlib.NewWriter(&compressed)
	if _, err := writer.Write(decoded); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	stream := &types.ObjectStreamDict{
		StreamDict: types.StreamDict{
			Raw:            compressed.Bytes(),
			FilterPipeline: []types.PDFFilter{{Name: "FlateDecode"}},
		},
		MaxDecodeBytes: maxPDFDecodedStream,
		FirstObjOffset: len(prolog),
		ObjCount:       1,
	}
	lazy := types.NewLazyObjectStreamObject(stream, 0, -1, func(context.Context, string) (types.Object, error) {
		return types.Dict{"Type": types.Name("Example")}, nil
	})
	stream.ObjArray = types.Array{lazy}
	xref := &model.XRefTable{Table: map[int]*model.XRefTableEntry{
		objectStreamNumber: {Generation: &generation, Object: *stream},
		6:                  {Generation: &generation, Object: lazy, ObjectStream: &objectStreamNumber, ObjectStreamInd: &objectStreamIndex},
	}}
	scanner := pdfStructureScanner{xref: xref, decodedObjectStreams: make(map[int]struct{}), decodedByteLimit: maxPDFDecodedTotal}
	if err := scanner.accountObjectStream(objectStreamNumber); err != nil {
		t.Fatalf("object stream rejected: %v", err)
	}
	entry, found := xref.Find(objectStreamNumber)
	if !found || entry == nil {
		t.Fatal("decoded object stream entry disappeared")
	}
	cached, ok := entry.Object.(types.ObjectStreamDict)
	if !ok || len(cached.Content) != len(decoded) {
		t.Fatalf("XRefTable object stream was not populated with bounded decoded content: type=%T content_bytes=%d", entry.Object, func() int {
			if ok {
				return len(cached.Content)
			}
			return 0
		}())
	}
	if len(cached.ObjArray) != 1 {
		t.Fatalf("cached object count=%d, want 1", len(cached.ObjArray))
	}
	if _, lazy := cached.ObjArray[0].(types.LazyObjectStreamObject); lazy {
		t.Fatal("cached object stream retained lazy objects that could decode outside the aggregate budget")
	}
	if _, lazy := cached.ObjArray[0].(*types.LazyObjectStreamObject); lazy {
		t.Fatal("cached object stream retained pointer-backed lazy objects")
	}
}

func TestPDFStructureScannerRejectsResolvedReferenceWithMissingObject(t *testing.T) {
	generation := 0
	scanner := pdfStructureScanner{
		xref: &model.XRefTable{Table: map[int]*model.XRefTableEntry{
			42: {Generation: &generation, Valid: true},
		}},
		visited: make(map[pdfObjectRef]struct{}),
	}
	ref := types.IndirectRef{ObjectNumber: 42, GenerationNumber: 0}
	if err := scanner.walk(ref, 0); !errors.Is(err, ErrRejected) {
		t.Fatalf("reference with a missing object was accepted: %v", err)
	}
}

func TestPDFStructureScannerRejectsNilLazyObjectInObjectStream(t *testing.T) {
	generation := 0
	stream := &types.ObjectStreamDict{
		StreamDict:     types.StreamDict{Content: []byte("synthetic")},
		MaxDecodeBytes: maxPDFDecodedStream,
		ObjCount:       1,
		ObjArray:       types.Array{(*types.LazyObjectStreamObject)(nil)},
	}
	scanner := pdfStructureScanner{
		xref:                 &model.XRefTable{Table: map[int]*model.XRefTableEntry{5: {Generation: &generation, Object: stream}}},
		decodedObjectStreams: make(map[int]struct{}),
	}
	if err := scanner.accountObjectStream(5); !errors.Is(err, ErrRejected) {
		t.Fatalf("nil lazy object was not rejected safely: %v", err)
	}
}

func TestPDFStructureScannerStopsObjectStreamAccountingWhenContextIsCancelled(t *testing.T) {
	generation := 0
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	stream := &types.ObjectStreamDict{
		ObjCount: 1,
		ObjArray: types.Array{types.StringLiteral("synthetic")},
	}
	scanner := pdfStructureScanner{
		ctx: ctx,
		xref: &model.XRefTable{Table: map[int]*model.XRefTableEntry{
			5: {Generation: &generation, Object: stream},
		}},
	}
	if err := scanner.accountObjectStream(5); !errors.Is(err, ErrRejected) {
		t.Fatalf("object-stream accounting continued after cancellation: %v", err)
	}
}

func TestPDFStructureScannerTraversesCompressedObjectStreamValue(t *testing.T) {
	generation := 0
	objectStreamNumber := 41
	objectStreamIndex := 0
	prolog := "42 0 "
	stream := &types.ObjectStreamDict{
		StreamDict:     types.StreamDict{Content: []byte(prolog + "<< /OpenAction true >>")},
		MaxDecodeBytes: maxPDFDecodedStream,
		ObjCount:       1,
		FirstObjOffset: len(prolog),
	}
	lazy := types.NewLazyObjectStreamObject(stream, 0, -1, func(context.Context, string) (types.Object, error) { return types.Dict{}, nil })
	stream.ObjArray = types.Array{lazy}
	xref := &model.XRefTable{Table: map[int]*model.XRefTableEntry{
		objectStreamNumber: {Generation: &generation, Object: *stream},
		42:                 {Generation: &generation, Object: lazy, ObjectStream: &objectStreamNumber, ObjectStreamInd: &objectStreamIndex},
	}}
	scanner := pdfStructureScanner{xref: xref, visited: make(map[pdfObjectRef]struct{})}
	ref := types.IndirectRef{ObjectNumber: 42, GenerationNumber: 0}
	if err := scanner.walk(ref, 0); !errors.Is(err, errPDFActiveContent) {
		t.Fatalf("compressed object-stream action was not traversed: %v", err)
	}
}

func TestPDFStructureScannerRejectsObjectStreamObjectMissingXRefEntry(t *testing.T) {
	generation := 0
	prolog := "42 0 "
	stream := &types.ObjectStreamDict{
		StreamDict:     types.StreamDict{Content: []byte(prolog + "1")},
		MaxDecodeBytes: maxPDFDecodedStream,
		ObjCount:       1,
		FirstObjOffset: len(prolog),
	}
	stream.ObjArray = types.Array{types.NewLazyObjectStreamObject(stream, 0, -1, func(context.Context, string) (types.Object, error) {
		return types.Integer(1), nil
	})}
	scanner := pdfStructureScanner{xref: &model.XRefTable{Table: map[int]*model.XRefTableEntry{
		41: {Generation: &generation, Object: *stream},
	}}}
	if err := scanner.accountObjectStream(41); !errors.Is(err, ErrRejected) {
		t.Fatalf("object stream with an unindexed object was accepted: %v", err)
	}
}

func TestPDFInspectorFailsClosedWhenStructuralParserFails(t *testing.T) {
	dir := t.TempDir()
	pdfinfo := filepath.Join(dir, "pdfinfo")
	pdfdetach := filepath.Join(dir, "pdfdetach")
	if err := os.WriteFile(pdfinfo, []byte("#!/bin/sh\ncase \"$1\" in\n  -) printf 'Pages: 1\\nEncrypted: no\\nJavaScript: no\\nForm: none\\n' ;;\n  *) exit 9 ;;\nesac\n"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pdfdetach, []byte("#!/bin/sh\nprintf '0 embedded files\\n'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, _, err := newTestInspector(Config{PDFInfoPath: pdfinfo, PDFDetachPath: pdfdetach}).Inspect(dv.PurposeIdentity, "id", syntheticPDF(t, "")); err == nil {
		t.Fatal("accepted PDF after a required structural parser command failed")
	}
}

func TestPDFInspectorFailsClosedWhenPDFStructureIsMalformed(t *testing.T) {
	dir := t.TempDir()
	pdfinfo := filepath.Join(dir, "pdfinfo")
	pdfdetach := filepath.Join(dir, "pdfdetach")
	if err := os.WriteFile(pdfinfo, []byte("#!/bin/sh\ncase \"$1\" in\n  -) printf 'Pages: 1\\nEncrypted: no\\nJavaScript: no\\nForm: none\\n' ;;\n  -url) printf 'Page Type URL\\n' ;;\n  -js) exit 0 ;;\n  *) exit 9 ;;\nesac\n"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pdfdetach, []byte("#!/bin/sh\nprintf '0 embedded files\\n'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	malformed := []byte("%PDF-1.4\\nnot a valid PDF structure")
	if _, _, err := newTestInspector(Config{PDFInfoPath: pdfinfo, PDFDetachPath: pdfdetach}).Inspect(dv.PurposeIdentity, "id", malformed); err == nil {
		t.Fatal("accepted malformed PDF after Poppler returned a clean summary")
	}
}

func TestPDFInspectorRejectsMultiDigitAttachmentCount(t *testing.T) {
	dir := t.TempDir()
	pdfinfo := filepath.Join(dir, "pdfinfo")
	pdfdetach := filepath.Join(dir, "pdfdetach")
	if err := os.WriteFile(pdfinfo, []byte("#!/bin/sh\ncase \"$1\" in\n  -) printf 'Pages: 1\\nEncrypted: no\\nJavaScript: no\\nForm: none\\n' ;;\n  -url) printf 'Page Type URL\\n' ;;\n  -js) exit 0 ;;\n  *) exit 9 ;;\nesac\n"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pdfdetach, []byte("#!/bin/sh\nprintf '10 embedded files\\n'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, _, err := newTestInspector(Config{PDFInfoPath: pdfinfo, PDFDetachPath: pdfdetach}).Inspect(dv.PurposeIdentity, "id", syntheticPDF(t, "")); err == nil {
		t.Fatal("accepted nonzero attachment count with a leading zero")
	}
}

func TestPDFInspectorRejectsOversizedParserOutput(t *testing.T) {
	dir := t.TempDir()
	pdfinfo := filepath.Join(dir, "pdfinfo")
	pdfdetach := filepath.Join(dir, "pdfdetach")
	if err := os.WriteFile(pdfinfo, []byte("#!/bin/sh\ncase \"$1\" in\n  -) printf 'Pages: 1\\nEncrypted: no\\nJavaScript: no\\nForm: none\\n'; printf '%2000000s\\n' '' ;;\n  -url) printf 'Page Type URL\\n' ;;\n  -js) exit 0 ;;\n  *) exit 9 ;;\nesac\n"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pdfdetach, []byte("#!/bin/sh\nprintf '0 embedded files\\n'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, _, err := newTestInspector(Config{PDFInfoPath: pdfinfo, PDFDetachPath: pdfdetach}).Inspect(dv.PurposeIdentity, "id", syntheticPDF(t, "")); err == nil {
		t.Fatal("accepted unbounded output from pdfinfo")
	}
}

func TestPDFInspectorTimesOutParserCommand(t *testing.T) {
	dir := t.TempDir()
	pdfinfo := filepath.Join(dir, "pdfinfo")
	pdfdetach := filepath.Join(dir, "pdfdetach")
	if err := os.WriteFile(pdfinfo, []byte("#!/bin/sh\ncase \"$1\" in\n  -) sleep 6 >/dev/null 2>&1 ;;\n  -url) printf 'Page Type URL\\n' ;;\n  -js) exit 0 ;;\n  *) exit 9 ;;\nesac\n"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pdfdetach, []byte("#!/bin/sh\nprintf '0 embedded files\\n'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	if _, _, err := newTestInspector(Config{PDFInfoPath: pdfinfo, PDFDetachPath: pdfdetach}).Inspect(dv.PurposeIdentity, "id", syntheticPDF(t, "")); err == nil {
		t.Fatal("accepted PDF after parser command timeout")
	}
	if elapsed := time.Since(started); elapsed >= pdfParseTimeout+500*time.Millisecond {
		t.Fatalf("parser subprocess was not stopped by its deadline: elapsed %v", elapsed)
	}
}

func TestEmptyPDFInfoURLListAcceptsExtendedHeader(t *testing.T) {
	if !hasEmptyPDFInfoURLList("Page Type URL Object\n") {
		t.Fatal("rejected an empty URL table with an additional header column")
	}
	if hasEmptyPDFInfoURLList("Page Type URL\n1 Link https://example.invalid\n") {
		t.Fatal("accepted a URL row")
	}
}

func hasPDFInfoField(info []byte, key, expected string) bool {
	for _, line := range strings.Split(string(info), "\n") {
		name, value, ok := strings.Cut(line, ":")
		if ok && strings.TrimSpace(name) == key && strings.EqualFold(strings.TrimSpace(value), expected) {
			return true
		}
	}
	return false
}

func TestPDFInspectorRejectsEscapedEmbeddedFileUsingPoppler(t *testing.T) {
	pdfinfo, pdfdetach := popplerBinaries(t)
	data := syntheticPDFWithObjects(t,
		"<< /Type /Catalog /Pages 2 0 R /Names << /Embedded#46iles << /Names [(sample.txt) 5 0 R] >> >> >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 10 10] /Resources << >> /Contents 4 0 R >>",
		"<< /Length 0 >>\nstream\n\nendstream",
		"<< /Type /File#73pec /F (sample.txt) /UF (sample.txt) /EF << /F 6 0 R >> >>",
		"<< /Type /Embedded#46ile /Length 4 >>\nstream\ntest\nendstream",
	)
	fd, err := unix.MemfdCreate("synthetic-document-test", unix.MFD_CLOEXEC)
	if err != nil {
		t.Fatal(err)
	}
	file := os.NewFile(uintptr(fd), "synthetic-document")
	if file == nil {
		t.Fatal("could not wrap in-memory document descriptor")
	}
	t.Cleanup(func() { _ = file.Close() })
	if _, err := file.Write(data); err != nil {
		t.Fatal(err)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(pdfdetach, "-list", "/proc/self/fd/3")
	cmd.ExtraFiles = []*os.File{file}
	attachments, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(attachments), "1 embedded files") {
		t.Fatalf("Poppler did not structurally identify synthetic attachment: err=%v output=%s", err, attachments)
	}
	if _, _, err := New(Config{PDFInfoPath: pdfinfo, PDFDetachPath: pdfdetach}).Inspect(dv.PurposeIdentity, "id", data); err == nil {
		t.Fatal("accepted PDF with escaped embedded file")
	}
}

func TestPDFInspectorAcceptsSyntheticCleanPDF(t *testing.T) {
	pdfinfo, pdfdetach := popplerBinaries(t)
	data := syntheticPDF(t, "")
	if _, _, err := New(Config{PDFInfoPath: pdfinfo, PDFDetachPath: pdfdetach}).Inspect(dv.PurposeIdentity, "id", data); err != nil {
		t.Fatalf("clean synthetic PDF rejected: %v", err)
	}
}

func TestPDFInspectorRejectsEncryptedPDFUsingPoppler(t *testing.T) {
	pdfinfo, pdfdetach := popplerBinaries(t)
	data := syntheticEncryptedPDF(t)
	cmd := exec.Command(pdfinfo, "-")
	cmd.Stdin = bytes.NewReader(data)
	info, err := cmd.CombinedOutput()
	if err != nil || !hasPDFInfoFieldPrefix(info, "Encrypted", "yes") {
		t.Fatalf("Poppler did not structurally identify synthetic encryption: err=%v output=%s", err, info)
	}
	if err := validatePDFStructure(data, 40_000_000, 1); err == nil {
		t.Fatal("pdfcpu structural validation accepted an encrypted document")
	}
	if _, _, err := New(Config{PDFInfoPath: pdfinfo, PDFDetachPath: pdfdetach}).Inspect(dv.PurposeIdentity, "id", data); err == nil {
		t.Fatal("accepted encrypted PDF")
	}
}

func hasPDFInfoFieldPrefix(info []byte, key, prefix string) bool {
	for _, line := range strings.Split(string(info), "\n") {
		name, value, ok := strings.Cut(line, ":")
		if ok && strings.TrimSpace(name) == key && strings.HasPrefix(strings.ToLower(strings.TrimSpace(value)), strings.ToLower(prefix)) {
			return true
		}
	}
	return false
}

func popplerBinaries(t *testing.T) (string, string) {
	t.Helper()
	pdfinfo, err := exec.LookPath("pdfinfo")
	if err != nil {
		t.Skip("pdfinfo unavailable")
	}
	pdfdetach, err := exec.LookPath("pdfdetach")
	if err != nil {
		t.Skip("pdfdetach unavailable")
	}
	return pdfinfo, pdfdetach
}

func syntheticPDF(t *testing.T, catalogExtra string) []byte {
	return syntheticPDFWithObjects(t,
		"<< /Type /Catalog /Pages 2 0 R"+catalogExtra+" >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 10 10] /Resources << >> /Contents 4 0 R >>",
		"<< /Length 0 >>\nstream\n\nendstream",
	)
}

func syntheticPDFWithObjects(t *testing.T, objects ...string) []byte {
	return syntheticPDFWithTrailer(t, "", objects...)
}

func syntheticPDFWithTrailer(t *testing.T, trailerEntries string, objects ...string) []byte {
	t.Helper()
	var b bytes.Buffer
	b.WriteString("%PDF-1.5\n")
	offsets := []int{0}
	for n, obj := range objects {
		offsets = append(offsets, b.Len())
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", n+1, obj)
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(offsets))
	for _, off := range offsets[1:] {
		fmt.Fprintf(&b, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R%s >>\nstartxref\n%d\n%%%%EOF\n", len(offsets), trailerEntries, xref)
	return b.Bytes()
}

func syntheticPDFWithCompressedLaunchAction(t *testing.T) []byte {
	return syntheticPDFWithCompressedLaunchActionReference(t, true)
}

func syntheticPDFWithUnreferencedCompressedLaunchAction(t *testing.T) []byte {
	return syntheticPDFWithCompressedLaunchActionReference(t, false)
}

func syntheticPDFWithCompressedLaunchActionReference(t *testing.T, referenceAction bool) []byte {
	t.Helper()
	catalogText := "<< /Type /Catalog /Pages 2 0 R >>"
	if referenceAction {
		catalogText = "<< /Type /Catalog /Pages 2 0 R /OpenAction 6 0 R >>"
	}
	catalog := []byte(catalogText)
	action := []byte("<< /S /Launch /F (synthetic) >>")
	padding := bytes.Repeat([]byte(" "), 2048)
	header := []byte(fmt.Sprintf("1 0 6 %d ", len(catalog)+len(padding)))
	objectStream := append(append(append(append([]byte(nil), header...), catalog...), padding...), action...)
	var compressed bytes.Buffer
	zw := zlib.NewWriter(&compressed)
	if _, err := zw.Write(objectStream); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	var pdf bytes.Buffer
	pdf.WriteString("%PDF-1.5\n")
	offsets := make(map[int]uint32)
	writeObject := func(number int, body []byte) {
		offsets[number] = uint32(pdf.Len())
		fmt.Fprintf(&pdf, "%d 0 obj\n", number)
		pdf.Write(body)
		pdf.WriteString("\nendobj\n")
	}
	writeObject(2, []byte("<< /Type /Pages /Kids [3 0 R] /Count 1 >>"))
	writeObject(3, []byte("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 10 10] /Resources << >> /Contents 4 0 R >>"))
	writeObject(4, []byte("<< /Length 0 >>\nstream\n\nendstream"))
	objectStreamBody := append([]byte(fmt.Sprintf("<< /Type /ObjStm /N 2 /First %d /Length %d /Filter /FlateDecode >>\nstream\n", len(header), compressed.Len())), compressed.Bytes()...)
	objectStreamBody = append(objectStreamBody, []byte("\nendstream")...)
	writeObject(5, objectStreamBody)

	xrefOffset := uint32(pdf.Len())
	offsets[7] = xrefOffset
	var xref bytes.Buffer
	writeRecord := func(kind byte, field2 uint32, field3 uint16) {
		xref.WriteByte(kind)
		var fields [6]byte
		binary.BigEndian.PutUint32(fields[:4], field2)
		binary.BigEndian.PutUint16(fields[4:], field3)
		xref.Write(fields[:])
	}
	writeRecord(0, 0, 65535)
	writeRecord(2, 5, 0)
	for _, number := range []int{2, 3, 4, 5} {
		writeRecord(1, offsets[number], 0)
	}
	writeRecord(2, 5, 1)
	writeRecord(1, offsets[7], 0)
	fmt.Fprintf(&pdf, "7 0 obj\n<< /Type /XRef /Size 8 /Root 1 0 R /W [1 4 2] /Length %d >>\nstream\n", xref.Len())
	pdf.Write(xref.Bytes())
	fmt.Fprintf(&pdf, "\nendstream\nendobj\nstartxref\n%d\n%%%%EOF\n", xrefOffset)
	return pdf.Bytes()
}

func syntheticEncryptedPDF(t *testing.T) []byte {
	t.Helper()
	padding, err := hex.DecodeString("28bf4e5e4e758a4164004e56fffa01082e2e00b6d0683e802f0ca9fe6453697a")
	if err != nil {
		t.Fatal(err)
	}
	pad := func(password []byte) []byte {
		result := make([]byte, 32)
		copy(result, password)
		copy(result[len(password):], padding)
		return result
	}
	userPassword := pad(nil)
	ownerKey := md5.Sum(pad(nil))
	ownerCipher, err := rc4.NewCipher(ownerKey[:5])
	if err != nil {
		t.Fatal(err)
	}
	ownerEntry := make([]byte, len(userPassword))
	ownerCipher.XORKeyStream(ownerEntry, userPassword)
	fileID := []byte{0, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}
	permissions := make([]byte, 4)
	binary.LittleEndian.PutUint32(permissions, ^uint32(3))
	encryptionKeyInput := append(append(append([]byte(nil), userPassword...), ownerEntry...), permissions...)
	encryptionKeyInput = append(encryptionKeyInput, fileID...)
	key := md5.Sum(encryptionKeyInput)
	userCipher, err := rc4.NewCipher(key[:5])
	if err != nil {
		t.Fatal(err)
	}
	userEntry := make([]byte, len(padding))
	userCipher.XORKeyStream(userEntry, padding)
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 10 10] /Resources << >> /Contents 4 0 R >>",
		"<< /Length 0 >>\nstream\n\nendstream",
		fmt.Sprintf("<< /Filter /Standard /V 1 /R 2 /Length 40 /O <%s> /U <%s> /P -4 >>", hex.EncodeToString(ownerEntry), hex.EncodeToString(userEntry)),
	}
	extra := fmt.Sprintf(" /Encrypt 5 0 R /ID [<%s><%s>]", hex.EncodeToString(fileID), hex.EncodeToString(fileID))
	return syntheticPDFWithTrailer(t, extra, objects...)
}

func TestPDFInspectorRequiresConfiguredExecutable(t *testing.T) {
	if _, _, e := inspector().Inspect(dv.PurposeIdentity, "opaque", []byte("%PDF-1.7")); e == nil {
		t.Fatal("accepted PDF without parser")
	}
}
func TestJPEGImageCanBeReencoded(t *testing.T) {
	var b bytes.Buffer
	im := image.NewRGBA(image.Rect(0, 0, 2, 2))
	if e := jpeg.Encode(&b, im, nil); e != nil {
		t.Fatal(e)
	}
	f, out, e := inspector().Inspect(dv.PurposeIdentity, "opaque", b.Bytes())
	if e != nil {
		t.Fatal(e)
	}
	if f.MIME != "image/jpeg" || len(out) == 0 {
		t.Fatalf("%#v", f)
	}
}
func TestExecutablePathMustBeExplicit(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "fake")
	if e := os.WriteFile(p, []byte("#!/bin/sh\nexit 0\n"), 0700); e != nil {
		t.Fatal(e)
	}
	if _, _, e := New(Config{}).Inspect(dv.PurposeIdentity, "opaque", []byte("%PDF-1.7")); e == nil {
		t.Fatal("implicit parser accepted")
	}
	pdfinfo, pdfdetach := popplerBinaries(t)
	if _, _, e := New(Config{PDFInfoPath: filepath.Base(pdfinfo), PDFDetachPath: filepath.Base(pdfdetach)}).Inspect(dv.PurposeIdentity, "opaque", syntheticPDF(t, "")); e == nil {
		t.Fatal("resolved a parser executable by searching PATH")
	}
}
