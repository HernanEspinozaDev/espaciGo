package inspection

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/filter"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

func TestReadContextRejectsAliasedObjectStreamOffsetsBeforeDecode(t *testing.T) {
	data := syntheticPDFAliasedObjectStreamOffsets(t, 6_000)
	conf := model.NewStatelessConfiguration()
	conf.Reader15 = true
	conf.ValidationMode = model.ValidationStrict
	conf.Offline = true
	conf.Limits.MaxInputBytes = 10 << 20
	conf.Limits.MaxObjectBytes = 10 << 20
	conf.Limits.MaxStreamBytes = 10 << 20
	conf.Limits.MaxDecodeBytes = 10_000
	conf.Limits.MaxTotalDecodeBytes = 10_000
	conf.Limits.MaxObjectCount = 100_000
	conf.Limits.MaxObjectStreamCount = 10_000
	conf.Limits.MaxObjectStreamFirst = 10_000
	conf.Limits.MaxXRefEntries = 100_000
	conf.Limits.MaxRecursionDepth = 64
	conf.Cmd = model.VALIDATE

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	parsed, err := api.ReadContext(ctx, bytes.NewReader(data), conf)
	if parsed != nil {
		t.Fatal("ReadContext returned a context for object streams sharing one physical offset")
	}
	if err == nil || errors.Is(err, filter.ErrDecodeLimitExceeded) || !strings.Contains(err.Error(), "duplicate object stream offset") {
		t.Fatalf("aliased object-stream offsets were not rejected before decoding: %v", err)
	}
}

func TestReadContextEnforcesAggregateBudgetAcrossXRefAndObjectStreams(t *testing.T) {
	data, xrefBytes, objectStreamPrefixBytes, objectStreamBytes := syntheticPDFAggregateXRefAndObjectStreams(t)
	total := xrefBytes + objectStreamPrefixBytes + objectStreamBytes
	conf := model.NewStatelessConfiguration()
	conf.Reader15 = true
	conf.ValidationMode = model.ValidationStrict
	conf.Offline = true
	conf.Limits.MaxInputBytes = 10 << 20
	conf.Limits.MaxObjectBytes = 10 << 20
	conf.Limits.MaxStreamBytes = 10 << 20
	conf.Limits.MaxDecodeBytes = total
	conf.Limits.MaxTotalDecodeBytes = total - 1
	conf.Limits.MaxObjectCount = 100_000
	conf.Limits.MaxObjectStreamCount = 10_000
	conf.Limits.MaxObjectStreamFirst = 10_000
	conf.Limits.MaxXRefEntries = 100_000
	conf.Limits.MaxRecursionDepth = 64
	conf.Cmd = model.VALIDATE

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	parsed, err := api.ReadContext(ctx, bytes.NewReader(data), conf)
	if err != nil || parsed == nil {
		t.Fatalf("ReadContext before validation = (%v,%v), want lazy object streams parsed", parsed, err)
	}
	if got, want := parsed.XRefTable.DecodeBudget.Used(), xrefBytes+objectStreamPrefixBytes; got != want {
		t.Fatalf("aggregate budget after xref and object-stream prolog = %d, want %d", got, want)
	}
	if err := api.ValidateContext(ctx, parsed); !errors.Is(err, filter.ErrDecodeLimitExceeded) {
		t.Fatalf("ValidateContext error = %v, want aggregate decoded-byte limit exceeded", err)
	}
	if got, want := parsed.XRefTable.DecodeBudget.Used(), total-1; got != want {
		t.Fatalf("aggregate budget after rejected full object-stream decode = %d, want %d", got, want)
	}

	conf.Limits.MaxTotalDecodeBytes = total
	parsed, err = api.ReadContext(ctx, bytes.NewReader(data), conf)
	if err != nil || parsed == nil {
		t.Fatalf("ReadContext at exact aggregate output budget = (%v,%v), want parsed context", parsed, err)
	}
	if err := api.ValidateContext(ctx, parsed); err != nil {
		t.Fatalf("ValidateContext at exact aggregate output budget: %v", err)
	}
	if got := parsed.XRefTable.DecodeBudget.Used(); got != total {
		t.Fatalf("aggregate budget at exact xref/object-stream boundary = %d, want %d", got, total)
	}
}

func syntheticPDFAggregateXRefAndObjectStreams(t *testing.T) ([]byte, int64, int64, int64) {
	t.Helper()
	var pdf bytes.Buffer
	pdf.WriteString("%PDF-1.7\n")
	offsets := make([]int, 8)
	writeObject := func(objectNumber int, body string) {
		offsets[objectNumber] = pdf.Len()
		fmt.Fprintf(&pdf, "%d 0 obj\n%s\nendobj\n", objectNumber, body)
	}
	writeObject(1, "<< /Type /Catalog /Pages 9 0 R >>")
	writeObject(2, "<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	writeObject(3, "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 10 10] /Resources << >> /Contents 4 0 R >>")
	writeObject(4, "<< /Length 0 >>\nstream\n\nendstream")

	firstObject := "<< /Type /Pages /Kids [10 0 R] /Count 1 >> "
	secondObject := "<< /Type /Page /Parent 9 0 R /MediaBox [0 0 10 10] /Resources << >> /Contents 4 0 R >> "
	prolog := fmt.Sprintf("9 0 10 %d ", len(firstObject))
	decodedObjectStream := prolog + firstObject + secondObject
	var encodedObjectStream bytes.Buffer
	objectStreamWriter := zlib.NewWriter(&encodedObjectStream)
	if _, err := objectStreamWriter.Write([]byte(decodedObjectStream)); err != nil {
		t.Fatal(err)
	}
	if err := objectStreamWriter.Close(); err != nil {
		t.Fatal(err)
	}
	objectStreamOffset := pdf.Len()
	offsets[5] = objectStreamOffset
	fmt.Fprintf(&pdf, "5 0 obj\n<< /Type /ObjStm /N 2 /First %d /Length %d /Filter /FlateDecode >>\nstream\n", len(prolog), encodedObjectStream.Len())
	pdf.Write(encodedObjectStream.Bytes())
	pdf.WriteString("\nendstream\nendobj\n")

	xrefOffset := pdf.Len()
	const xrefEntries = 11
	const entryWidth = 7
	xref := make([]byte, xrefEntries*entryWidth)
	putEntry := func(objectNumber int, kind byte, field2 uint32, field3 uint16) {
		entry := xref[objectNumber*entryWidth : (objectNumber+1)*entryWidth]
		entry[0] = kind
		binary.BigEndian.PutUint32(entry[1:5], field2)
		binary.BigEndian.PutUint16(entry[5:7], field3)
	}
	putEntry(0, 0, 0, 65535)
	for objectNumber := 1; objectNumber <= 4; objectNumber++ {
		putEntry(objectNumber, 1, uint32(offsets[objectNumber]), 0)
	}
	putEntry(5, 1, uint32(objectStreamOffset), 0)
	putEntry(6, 0, 0, 0)
	putEntry(7, 1, uint32(xrefOffset), 0)
	putEntry(8, 0, 0, 0)
	putEntry(9, 2, 5, 0)
	putEntry(10, 2, 5, 1)

	var encodedXRef bytes.Buffer
	xrefWriter := zlib.NewWriter(&encodedXRef)
	if _, err := xrefWriter.Write(xref); err != nil {
		t.Fatal(err)
	}
	if err := xrefWriter.Close(); err != nil {
		t.Fatal(err)
	}
	fmt.Fprintf(&pdf, "7 0 obj\n<< /Type /XRef /Size %d /Root 1 0 R /W [1 4 2] /Filter /FlateDecode /Length %d >>\nstream\n", xrefEntries, encodedXRef.Len())
	pdf.Write(encodedXRef.Bytes())
	pdf.WriteString("\nendstream\nendobj\nstartxref\n")
	fmt.Fprintf(&pdf, "%d\n%%%%EOF\n", xrefOffset)
	return pdf.Bytes(), int64(len(xref)), int64(len(prolog)), int64(len(decodedObjectStream))
}

func syntheticPDFAliasedObjectStreamOffsets(t *testing.T, padding int) []byte {
	t.Helper()
	var pdf bytes.Buffer
	pdf.WriteString("%PDF-1.7\n")
	offsets := make([]int, 8)
	writeObject := func(objectNumber int, body string) {
		offsets[objectNumber] = pdf.Len()
		fmt.Fprintf(&pdf, "%d 0 obj\n%s\nendobj\n", objectNumber, body)
	}
	writeObject(1, "<< /Type /Catalog /Pages 2 0 R >>")
	writeObject(2, "<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	writeObject(3, "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 10 10] /Resources << >> /Contents 4 0 R >>")
	writeObject(4, "<< /Length 0 >>\nstream\n\nendstream")

	firstObject := "<< /Example /One >> "
	secondObject := "<< /Example /Two >> "
	prologPrefix := fmt.Sprintf("9 0 10 %d ", len(firstObject))
	if padding < len(prologPrefix) {
		t.Fatalf("object-stream prolog target %d is shorter than its fields", padding)
	}
	prolog := prologPrefix + strings.Repeat(" ", padding-len(prologPrefix))
	decodedObjectStream := prolog + firstObject + secondObject
	var encodedObjectStream bytes.Buffer
	objectStreamWriter := zlib.NewWriter(&encodedObjectStream)
	if _, err := objectStreamWriter.Write([]byte(decodedObjectStream)); err != nil {
		t.Fatal(err)
	}
	if err := objectStreamWriter.Close(); err != nil {
		t.Fatal(err)
	}
	objectStreamOffset := pdf.Len()
	offsets[5] = objectStreamOffset
	fmt.Fprintf(&pdf, "5 0 obj\n<< /Type /ObjStm /N 2 /First %d /Length %d /Filter /FlateDecode >>\nstream\n", len(prolog), encodedObjectStream.Len())
	pdf.Write(encodedObjectStream.Bytes())
	pdf.WriteString("\nendstream\nendobj\n")

	xrefOffset := pdf.Len()
	const xrefEntries = 11
	const entryWidth = 7
	xref := make([]byte, xrefEntries*entryWidth)
	putEntry := func(objectNumber int, kind byte, field2 uint32, field3 uint16) {
		entry := xref[objectNumber*entryWidth : (objectNumber+1)*entryWidth]
		entry[0] = kind
		binary.BigEndian.PutUint32(entry[1:5], field2)
		binary.BigEndian.PutUint16(entry[5:7], field3)
	}
	putEntry(0, 0, 0, 65535)
	for objectNumber := 1; objectNumber <= 4; objectNumber++ {
		putEntry(objectNumber, 1, uint32(offsets[objectNumber]), 0)
	}
	putEntry(5, 1, uint32(objectStreamOffset), 0)
	putEntry(6, 1, uint32(objectStreamOffset), 0)
	putEntry(7, 1, uint32(xrefOffset), 0)
	putEntry(8, 0, 0, 65535)
	putEntry(9, 2, 5, 0)
	putEntry(10, 2, 6, 1)

	var encodedXRef bytes.Buffer
	xrefWriter := zlib.NewWriter(&encodedXRef)
	if _, err := xrefWriter.Write(xref); err != nil {
		t.Fatal(err)
	}
	if err := xrefWriter.Close(); err != nil {
		t.Fatal(err)
	}
	fmt.Fprintf(&pdf, "7 0 obj\n<< /Type /XRef /Size %d /Root 1 0 R /W [1 4 2] /Filter /FlateDecode /Length %d >>\nstream\n", xrefEntries, encodedXRef.Len())
	pdf.Write(encodedXRef.Bytes())
	pdf.WriteString("\nendstream\nendobj\nstartxref\n")
	fmt.Fprintf(&pdf, "%d\n%%%%EOF\n", xrefOffset)
	return pdf.Bytes()
}
