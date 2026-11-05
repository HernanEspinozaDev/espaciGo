/*
Copyright 2018 The pdfcpu Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package filter

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

type cancelOnRead struct {
	reader io.Reader
	cancel context.CancelFunc
}

type byteCountingReader struct {
	reader io.Reader
	read   int
}

type singleByteCountingReader struct {
	reader io.Reader
	read   int
}

func (r *byteCountingReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	r.read += n
	return n, err
}

func (r *singleByteCountingReader) Read(p []byte) (int, error) {
	if len(p) > 1 {
		p = p[:1]
	}
	n, err := r.reader.Read(p)
	r.read += n
	return n, err
}

func (r cancelOnRead) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	r.cancel()
	return n, err
}

func TestCopyDecodedChecksRequestContextWhileCopyingOutput(t *testing.T) {
	base, cancel := context.WithCancel(context.Background())
	defer cancel()
	f := baseFilter{maxDecodeBytes: 128 << 10, ctx: base}
	decoded, err := f.copyDecoded(cancelOnRead{reader: strings.NewReader(strings.Repeat("x", 64<<10)), cancel: cancel}, -1)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("copyDecoded error = %v, want context cancellation", err)
	}
	if decoded != nil {
		t.Fatal("cancelled decode returned a partial output buffer")
	}
}

func TestRunLengthDecodeChecksRequestContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	defer cancel()
	fi, err := NewFilterWithContext(ctx, RunLength, nil, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if decoded, err := fi.Decode(bytes.NewReader(append(bytes.Repeat([]byte{0x7f}, 32<<10), 0x80))); !errors.Is(err, context.Canceled) || decoded != nil {
		t.Fatalf("RunLength decode=(%v,%v), want cancellation and no output", decoded, err)
	}
}

func TestASCIIHexDecodeStopsBeforeReadingBeyondOutputBudget(t *testing.T) {
	const limit int64 = 8
	input := strings.Repeat("A", 2<<20) + ">"
	reader := &byteCountingReader{reader: strings.NewReader(input)}
	fi, err := NewFilter(ASCIIHex, nil, limit)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := fi.Decode(reader)
	if !errors.Is(err, ErrDecodeLimitExceeded) {
		t.Fatalf("decode error = %v, want %v", err, ErrDecodeLimitExceeded)
	}
	if decoded != nil {
		t.Fatal("over-budget decode returned output")
	}
	if reader.read >= len(input) {
		t.Fatalf("decoder consumed the full %d-byte encoded stream before rejecting the over-budget output", reader.read)
	}
}

func TestASCIIHexDecodeReadsOnlyThroughFirstOverBudgetPair(t *testing.T) {
	const limit int64 = 8
	reader := &singleByteCountingReader{reader: strings.NewReader(strings.Repeat("A", 2<<20) + ">")}
	fi, err := NewFilter(ASCIIHex, nil, limit)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := fi.Decode(reader)
	if !errors.Is(err, ErrDecodeLimitExceeded) {
		t.Fatalf("decode error = %v, want %v", err, ErrDecodeLimitExceeded)
	}
	if decoded != nil {
		t.Fatal("over-budget decode returned output")
	}
	const wantConsumed = 2 * (limit + 1)
	if reader.read != int(wantConsumed) {
		t.Fatalf("encoded bytes consumed = %d, want %d through the first over-budget pair", reader.read, wantConsumed)
	}
}

func TestASCIIHexDecodeRejectsOddNibbleBeyondOutputBudget(t *testing.T) {
	const limit int64 = 8
	input := strings.Repeat("A", int(2*limit)) + "A>"
	reader := &singleByteCountingReader{reader: strings.NewReader(input)}
	fi, err := NewFilter(ASCIIHex, nil, limit)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := fi.Decode(reader)
	if !errors.Is(err, ErrDecodeLimitExceeded) {
		t.Fatalf("decode error = %v, want %v", err, ErrDecodeLimitExceeded)
	}
	if decoded != nil {
		t.Fatal("over-budget decode returned output")
	}
	const wantConsumed = int(2*limit + 2) // odd nibble and EOD marker
	if reader.read != wantConsumed {
		t.Fatalf("encoded bytes consumed = %d, want %d through the odd-nibble boundary", reader.read, wantConsumed)
	}
}

func TestASCIIHexDecodePreservesLengthAndOddNibbleSemantics(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int64
		want   []byte
	}{
		{name: "whole stream with whitespace and odd nibble", input: " 6 >ignored", maxLen: -1, want: []byte{0x60}},
		{name: "requested prefix", input: "48656c6c6f>", maxLen: 2, want: []byte("He")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fi, err := NewFilter(ASCIIHex, nil, 16)
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := fi.DecodeLength(strings.NewReader(test.input), test.maxLen)
			if err != nil {
				t.Fatal(err)
			}
			got, err := io.ReadAll(decoded)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, test.want) {
				t.Fatalf("decoded = %x, want %x", got, test.want)
			}
		})
	}
}

func TestASCIIHexDecodeLengthRejectsShortInput(t *testing.T) {
	fi, err := NewFilter(ASCIIHex, nil, 16)
	if err != nil {
		t.Fatal(err)
	}
	if decoded, err := fi.DecodeLength(strings.NewReader("4869>"), 3); !errors.Is(err, io.ErrUnexpectedEOF) || decoded != nil {
		t.Fatalf("DecodeLength = (%v, %v), want unexpected EOF and no output", decoded, err)
	}
}

func TestFlatePredictorChecksRequestContextBeforeEachRow(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	f := flate{baseFilter{maxDecodeBytes: 64, ctx: ctx}}
	_, err := f.decodePostProcessRows(bytes.NewReader([]byte{PNGNone, 0}), -1, 2, 1, PredictorNone, 1, 8, 1, 1)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("predictor decode error = %v, want context cancellation", err)
	}
}

func TestDecodeLimitWriterCapsTheMaterializedPrefix(t *testing.T) {
	var output bytes.Buffer
	writer := &decodeLimitWriter{w: &output, remaining: 4}

	written, err := writer.Write([]byte("123456"))
	if !errors.Is(err, ErrDecodeLimitExceeded) {
		t.Fatalf("Write error = %v, want decode limit exceeded", err)
	}
	if written != 4 {
		t.Fatalf("Write count = %d, want the allowed prefix of 4 bytes", written)
	}
	if got := output.String(); got != "1234" {
		t.Fatalf("materialized output = %q, want only the 4-byte budget", got)
	}
	if writer.remaining != 0 {
		t.Fatalf("remaining budget = %d, want 0", writer.remaining)
	}
}

func TestDecodeLimitWriterAcceptsExactBudgetThenRejectsMore(t *testing.T) {
	var output bytes.Buffer
	writer := &decodeLimitWriter{w: &output, remaining: 4}

	if n, err := writer.Write([]byte("1234")); err != nil || n != 4 {
		t.Fatalf("exact-limit Write = (%d, %v), want (4, nil)", n, err)
	}
	if n, err := writer.Write([]byte("5")); n != 0 || !errors.Is(err, ErrDecodeLimitExceeded) {
		t.Fatalf("over-limit Write = (%d, %v), want (0, decode limit exceeded)", n, err)
	}
	if got := output.String(); got != "1234" {
		t.Fatalf("materialized output = %q, want exactly 1234", got)
	}
}

func TestCopyDecodedWithinLimitDoesNotMaterializeOverflow(t *testing.T) {
	var output bytes.Buffer
	err := copyDecodedWithinLimit(&output, strings.NewReader("123456"), 4)
	if !errors.Is(err, ErrDecodeLimitExceeded) {
		t.Fatalf("copy error = %v, want decode limit exceeded", err)
	}
	if got := output.String(); got != "1234" {
		t.Fatalf("materialized output = %q, want only the 4-byte budget", got)
	}
}

func TestCopyDecodedWithinLimitAllowsExactAndShortOutput(t *testing.T) {
	for _, input := range []string{"1234", "123"} {
		var output bytes.Buffer
		if err := copyDecodedWithinLimit(&output, strings.NewReader(input), 4); err != nil {
			t.Fatalf("copy %q: %v", input, err)
		}
		if got := output.String(); got != input {
			t.Fatalf("copy %q materialized %q", input, got)
		}
	}
}
