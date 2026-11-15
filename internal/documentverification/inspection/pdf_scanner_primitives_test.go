package inspection

import (
	"errors"
	"testing"
)

func TestPDFDecodeLimits(t *testing.T) {
	if _, err := pdfDecodeLimits(0); !errors.Is(err, ErrRejected) {
		t.Fatalf("expected ErrRejected for total=0, got %v", err)
	}
	if _, err := pdfDecodeLimits(-10); !errors.Is(err, ErrRejected) {
		t.Fatalf("expected ErrRejected for total=-10, got %v", err)
	}
	got, err := pdfDecodeLimits(1024)
	if err != nil || got != 1024 {
		t.Fatalf("expected 1024, got %d, err=%v", got, err)
	}
	got, err = pdfDecodeLimits(maxPDFDecodedStream + 1000)
	if err != nil || got != maxPDFDecodedStream {
		t.Fatalf("expected clamped %d, got %d, err=%v", maxPDFDecodedStream, got, err)
	}
}

func TestPDFScannerWhitespaceAndDelimiters(t *testing.T) {
	whitespaces := []byte{0, 9, 10, 12, 13, 32}
	for _, b := range whitespaces {
		if !isPDFWhitespace(b) {
			t.Errorf("expected isPDFWhitespace(%d)=true", b)
		}
		if !isPDFDelimiter(b) {
			t.Errorf("expected isPDFDelimiter(%d)=true", b)
		}
	}
	delims := []byte{'(', ')', '<', '>', '[', ']', '{', '}', '/', '%'}
	for _, b := range delims {
		if isPDFWhitespace(b) {
			t.Errorf("expected isPDFWhitespace(%c)=false", b)
		}
		if !isPDFDelimiter(b) {
			t.Errorf("expected isPDFDelimiter(%c)=true", b)
		}
	}
	nonDelims := []byte{'a', 'Z', '0', '9', '_', '-', '@'}
	for _, b := range nonDelims {
		if isPDFWhitespace(b) || isPDFDelimiter(b) {
			t.Errorf("expected regular char %c to not be delimiter/whitespace", b)
		}
	}
}

func TestPDFScannerSkipSpaceAndComments(t *testing.T) {
	data := []byte("   \t\r\n% this is a comment\r\n  % another\n123")
	start := skipPDFSpaceAndComments(data, 0)
	if string(data[start:]) != "123" {
		t.Fatalf("expected to skip to '123', got %q", string(data[start:]))
	}
}

func TestScanPDFIntegerAndIndirectReference(t *testing.T) {
	data := []byte("123 0 R /Next")
	end, ok := scanPDFInteger(data, 0)
	if !ok || end != 3 {
		t.Fatalf("scanPDFInteger failed: ok=%v, end=%d", ok, end)
	}
	if !isPDFIndirectReferenceAt(data, 0) {
		t.Fatalf("expected isPDFIndirectReferenceAt=true at 0")
	}
	if isPDFIndirectReferenceAt([]byte("123 abc R"), 0) {
		t.Fatalf("expected false for non-integer generation")
	}
	if isPDFIndirectReferenceAt([]byte("abc 0 R"), 0) {
		t.Fatalf("expected false for non-integer object number")
	}
	if isPDFIndirectReferenceAt([]byte("123 0 X"), 0) {
		t.Fatalf("expected false for missing 'R'")
	}
}

func TestSkipPDFLiteralString(t *testing.T) {
	literal := []byte(`(hello \(world\) and (nested) text\r\n) after`)
	end, ok := skipPDFLiteralString(literal, 0)
	if !ok || string(literal[:end]) != `(hello \(world\) and (nested) text\r\n)` {
		t.Fatalf("skipPDFLiteralString failed: ok=%v, end=%d, val=%q", ok, end, string(literal[:end]))
	}
	unclosed := []byte("(unclosed string")
	_, ok = skipPDFLiteralString(unclosed, 0)
	if ok {
		t.Fatalf("expected unclosed string to return false")
	}
	_, ok = skipPDFLiteralString([]byte("not a string"), 0)
	if ok {
		t.Fatalf("expected false for non-string start")
	}
}

func TestReadPDFNameAndDecodeEscapes(t *testing.T) {
	raw := []byte("/Flate#44ecode /ASCII#38#35Decode /NormalName ")
	name1, end1 := readPDFName(raw, 0)
	if name1 != "FlateDecode" {
		t.Fatalf("readPDFName escape decode failed: got %q, want FlateDecode", name1)
	}
	name2, end2 := readPDFName(raw, end1+1)
	if name2 != "ASCII85Decode" {
		t.Fatalf("readPDFName escape decode failed: got %q, want ASCII85Decode", name2)
	}
	name3, _ := readPDFName(raw, end2+1)
	if name3 != "NormalName" {
		t.Fatalf("readPDFName normal failed: got %q, want NormalName", name3)
	}
	if _, ok := decodePDFNameEscape('G', 'H'); ok {
		t.Fatalf("expected invalid hex escape to return false")
	}
	if _, ok := decodePDFNameEscape('0', 'Z'); ok {
		t.Fatalf("expected invalid low hex to return false")
	}
	if val, ok := decodePDFNameEscape('6', 'f'); !ok || val != 'o' {
		t.Fatalf("expected lowercase hex 6f to decode to 'o', got %c (ok=%v)", val, ok)
	}
	if val, ok := decodePDFNameEscape('a', 'b'); !ok || val != 0xab {
		t.Fatalf("expected lowercase hex ab to decode to 0xab, got 0x%x (ok=%v)", val, ok)
	}
	if val, ok := decodePDFNameEscape('E', 'F'); !ok || val != 0xef {
		t.Fatalf("expected uppercase hex EF to decode to 0xef, got 0x%x (ok=%v)", val, ok)
	}

	// Name ending at EOF without trailing delimiter
	nameEOF, endEOF := readPDFName([]byte("/DirectNameAtEOF"), 0)
	if nameEOF != "DirectNameAtEOF" || endEOF != len("/DirectNameAtEOF") {
		t.Fatalf("readPDFName at EOF failed: %q, %d", nameEOF, endEOF)
	}

	// Name with unmatched/invalid '#' escape
	nameBadHash, _ := readPDFName([]byte("/Name#ZZ#"), 0)
	if nameBadHash != "Name#ZZ#" {
		t.Fatalf("readPDFName with bad hash failed: %q", nameBadHash)
	}
}

func TestIsPDFDecoderName(t *testing.T) {
	decoders := []string{
		"ASCII85Decode", "A85", "ASCIIHexDecode", "AHx", "RunLengthDecode", "RL",
		"LZWDecode", "LZW", "FlateDecode", "Fl", "CCITTFaxDecode", "CCF",
		"DCTDecode", "DCT", "JBIG2Decode", "JPXDecode", "Crypt",
	}
	for _, d := range decoders {
		if !isPDFDecoderName(d) {
			t.Errorf("expected isPDFDecoderName(%q)=true", d)
		}
	}
	if isPDFDecoderName("CustomDecoder") || isPDFDecoderName("Filter") || isPDFDecoderName("") {
		t.Errorf("expected non-decoder name to return false")
	}
}

func TestHasIndirectPDFFilter(t *testing.T) {
	direct := []byte("/Filter /FlateDecode")
	if hasIndirectPDFFilter(direct, len("/Filter")) {
		t.Fatalf("direct filter returned indirect=true")
	}
	indirect := []byte("/Filter 12 0 R")
	if !hasIndirectPDFFilter(indirect, len("/Filter")) {
		t.Fatalf("indirect reference filter returned indirect=false")
	}
	directArr := []byte("/Filter [/ASCII85Decode /FlateDecode]")
	if hasIndirectPDFFilter(directArr, len("/Filter")) {
		t.Fatalf("direct array filter returned indirect=true")
	}
	indirectArr := []byte("/Filter [/ASCII85Decode 15 0 R]")
	if !hasIndirectPDFFilter(indirectArr, len("/Filter")) {
		t.Fatalf("array with indirect reference returned indirect=false")
	}

	// Out of bounds
	if hasIndirectPDFFilter([]byte("/Filter "), len("/Filter ")) {
		t.Fatalf("expected false when valueStart >= len(data)")
	}

	// Comment in array
	commentArr := []byte("/Filter [ /FlateDecode % comment\n /ASCII85Decode ]")
	if hasIndirectPDFFilter(commentArr, len("/Filter")) {
		t.Fatalf("comment array returned indirect=true")
	}

	// Literal string in array (closed and unclosed)
	strArr := []byte("/Filter [ (string param) /FlateDecode ]")
	if hasIndirectPDFFilter(strArr, len("/Filter")) {
		t.Fatalf("literal string array returned indirect=true")
	}
	unclosedStrArr := []byte("/Filter [ (unclosed /FlateDecode ]")
	if !hasIndirectPDFFilter(unclosedStrArr, len("/Filter")) {
		t.Fatalf("unclosed literal string in array returned indirect=false")
	}

	// Dict and hex string in array
	hexAndDictArr := []byte("/Filter [ << /Key /Val >> <4142> /FlateDecode ]")
	if hasIndirectPDFFilter(hexAndDictArr, len("/Filter")) {
		t.Fatalf("hex and dict array returned indirect=true")
	}
	unclosedHexArr := []byte("/Filter [ <4142 /FlateDecode ]")
	if !hasIndirectPDFFilter(unclosedHexArr, len("/Filter")) {
		t.Fatalf("unclosed hex string returned indirect=false")
	}

	// Nested array and unclosed array
	nestedArr := []byte("/Filter [ [ /FlateDecode ] ]")
	if hasIndirectPDFFilter(nestedArr, len("/Filter")) {
		t.Fatalf("nested array returned indirect=true")
	}
	unclosedArr := []byte("/Filter [ /FlateDecode")
	if !hasIndirectPDFFilter(unclosedArr, len("/Filter")) {
		t.Fatalf("unclosed array returned indirect=false")
	}
}

func TestCountPDFDecoderStagesAndXRefReferences(t *testing.T) {
	pdf := []byte(`
%PDF-1.4
1 0 obj
<< /Filter [/FlateDecode /ASCII85Decode] /Length 100 >>
stream
...
endstream
endobj
2 0 obj
<< /Filter 10 0 R >>
endobj
startxref
1234
%%EOF
startxref
5678
`)
	_, err := countPDFDecoderStages(pdf)
	if !errors.Is(err, ErrRejected) {
		t.Fatalf("expected ErrRejected on indirect filter, got %v", err)
	}
	cleanPDF := []byte(`<< /Filter [/FlateDecode /ASCII85Decode] /Length 100 >> stream ... endstream`)
	count, err := countPDFDecoderStages(cleanPDF)
	if err != nil || count != 2 {
		t.Fatalf("expected count=2, got %d, err=%v", count, err)
	}
	xrefCount := countPDFXRefReferences(pdf)
	if xrefCount < 2 {
		t.Fatalf("expected at least 2 startxref occurrences, got %d", xrefCount)
	}
	streamMarkers := countPDFStreamMarkers(pdf)
	if streamMarkers != 2 {
		t.Fatalf("expected 2 stream markers (including endstream substring match), got %d", streamMarkers)
	}
}

func TestBoundedOutputLimits(t *testing.T) {
	var b boundedOutput
	n, err := b.Write([]byte("hello world"))
	if err != nil || n != 11 || b.String() != "hello world" {
		t.Fatalf("unexpected Write result: n=%d, err=%v, str=%q", n, err, b.String())
	}

	beforeLen := b.buffer.Len()
	oversized := make([]byte, maxParserOutputBytes+1)
	_, err = b.Write(oversized)
	if !errors.Is(err, errParserOutputLimit) {
		t.Fatalf("expected errParserOutputLimit on overflow, got %v", err)
	}
	if b.buffer.Len() != beforeLen {
		t.Fatalf("expected buffer to remain unmutated on overflow, got len %d want %d", b.buffer.Len(), beforeLen)
	}

	// Exact limit write
	var bExact boundedOutput
	exactChunk := make([]byte, maxParserOutputBytes)
	nExact, errExact := bExact.Write(exactChunk)
	if errExact != nil || nExact != maxParserOutputBytes || bExact.buffer.Len() != maxParserOutputBytes {
		t.Fatalf("exact limit write failed: n=%d, len=%d, err=%v", nExact, bExact.buffer.Len(), errExact)
	}
	// Writing 1 more byte fails
	_, errOver := bExact.Write([]byte{1})
	if !errors.Is(errOver, errParserOutputLimit) {
		t.Fatalf("expected errParserOutputLimit after reaching exact limit, got %v", errOver)
	}
}

func TestSkipPDFLiteralStringBounds(t *testing.T) {
	// start >= len(data)
	idx, ok := skipPDFLiteralString([]byte("hello"), 10)
	if ok || idx != 10 {
		t.Fatalf("expected false and idx=10 for out of bounds, got %v, %d", ok, idx)
	}

	// Escaped char at EOF
	idx, ok = skipPDFLiteralString([]byte("(hello\\"), 0)
	if ok || idx != len("(hello\\") {
		t.Fatalf("expected unclosed string with trailing escape, got %v, %d", ok, idx)
	}
}

func TestCountPDFNameOccurrencesDirect(t *testing.T) {
	data := []byte("/Prev 100 /XRefStm 200 /Prev 300 /Other /Prev")
	prevCount := countPDFNameOccurrences(data, "Prev")
	if prevCount != 3 {
		t.Fatalf("expected 3 Prev occurrences, got %d", prevCount)
	}
	xrefCount := countPDFNameOccurrences(data, "XRefStm")
	if xrefCount != 1 {
		t.Fatalf("expected 1 XRefStm occurrence, got %d", xrefCount)
	}
	missingCount := countPDFNameOccurrences(data, "NotFound")
	if missingCount != 0 {
		t.Fatalf("expected 0 NotFound occurrences, got %d", missingCount)
	}
}

func TestValidatePDFStructureWithBudgetBoundaries(t *testing.T) {
	// expectedPages < 1
	if err := validatePDFStructureWithBudget([]byte("%PDF"), 1000, 0, 1024); !errors.Is(err, ErrRejected) {
		t.Fatalf("expected ErrRejected for expectedPages=0, got %v", err)
	}
	// expectedPages > maxPDFPages
	if err := validatePDFStructureWithBudget([]byte("%PDF"), 1000, maxPDFPages+1, 1024); !errors.Is(err, ErrRejected) {
		t.Fatalf("expected ErrRejected for expectedPages > maxPDFPages, got %v", err)
	}
	// empty data
	if err := validatePDFStructureWithBudget(nil, 1000, 1, 1024); !errors.Is(err, ErrRejected) {
		t.Fatalf("expected ErrRejected for nil data, got %v", err)
	}
	// maxPixels <= 0
	if err := validatePDFStructureWithBudget([]byte("%PDF-1.4"), 0, 1, 1024); !errors.Is(err, ErrRejected) {
		t.Fatalf("expected ErrRejected for maxPixels=0, got %v", err)
	}
	// decodedBudget <= 0
	if err := validatePDFStructureWithBudget([]byte("%PDF-1.4"), 1000, 1, 0); !errors.Is(err, ErrRejected) {
		t.Fatalf("expected ErrRejected for decodedBudget=0, got %v", err)
	}
}
