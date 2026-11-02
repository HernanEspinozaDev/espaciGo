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

// Package filter contains PDF filter implementations.
package filter

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/pdfcpu/pdfcpu/pkg/log"
)

// PDF defines the following filters. See also 7.4 in the PDF spec.
const (
	ASCII85   = "ASCII85Decode"
	ASCIIHex  = "ASCIIHexDecode"
	RunLength = "RunLengthDecode"
	LZW       = "LZWDecode"
	Flate     = "FlateDecode"
	CCITTFax  = "CCITTFaxDecode"
	JBIG2     = "JBIG2Decode" // TODO
	DCT       = "DCTDecode"
	JPX       = "JPXDecode" // TODO
)

// ErrUnsupportedFilter signals unsupported filter encountered.
var ErrUnsupportedFilter = errors.New("filter not supported")

// ErrDecodeLimitExceeded signals that decoded filter output exceeds the configured decode limit.
var ErrDecodeLimitExceeded = errors.New("filter decode limit exceeded")

// DefaultMaxDecodeBytes is the default maximum decoded stream size.
const DefaultMaxDecodeBytes int64 = 512 << 20 // 512 MiB

const maxInt = int(^uint(0) >> 1)
const maxInt64 = int64(^uint64(0) >> 1)

// Filter defines an interface for encoding/decoding PDF object streams.
type Filter interface {
	// Encode returns a reader for the encoded contents of r.
	Encode(r io.Reader) (io.Reader, error)
	// Decode returns a reader for the decoded contents of r.
	Decode(r io.Reader) (io.Reader, error)
	// DecodeLength will decode at least maxLen bytes. For filters where decoding
	// parts doesn't make sense (e.g. DCT), the whole stream is decoded.
	// If maxLen < 0 is passed, the whole stream is decoded.
	DecodeLength(r io.Reader, maxLen int64) (io.Reader, error)
}

// NewFilter returns a filter for given filterName and an optional parameter dictionary.
func NewFilter(filterName string, parms map[string]int, maxDecodeBytes ...int64) (filter Filter, err error) {
	return NewFilterWithContext(context.Background(), filterName, parms, maxDecodeBytes...)
}

// NewFilterWithContext returns a filter that observes cancellation while decoding.
func NewFilterWithContext(ctx context.Context, filterName string, parms map[string]int, maxDecodeBytes ...int64) (filter Filter, err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	limit := DefaultMaxDecodeBytes
	if len(maxDecodeBytes) > 0 {
		limit = maxDecodeBytes[0]
	}
	base := baseFilter{parms: parms, maxDecodeBytes: limit, ctx: ctx}
	switch filterName {

	case ASCII85:
		filter = ascii85Decode{base}

	case ASCIIHex:
		filter = asciiHexDecode{base}

	case RunLength:
		filter = runLengthDecode{base}

	case LZW:
		filter = lzwDecode{base}

	case Flate:
		filter = flate{base}

	case CCITTFax:
		filter = ccittDecode{base}

	case DCT:
		filter = dctDecode{base}

	case JBIG2:
		// Unsupported
		fallthrough

	case JPX:
		// Unsupported
		if log.InfoEnabled() {
			log.Info.Printf("Filter not supported: <%s>", filterName)
		}
		err = ErrUnsupportedFilter

	default:
		err = fmt.Errorf("invalid filter: <%s>", filterName)
	}

	return filter, err
}

// List return the list of all supported PDF filters.
func List() []string {
	// Exclude CCITTFax, DCT, JBIG2 & JPX since they only makes sense in the context of image processing.
	return []string{ASCII85, ASCIIHex, RunLength, LZW, Flate}
}

type baseFilter struct {
	parms          map[string]int
	maxDecodeBytes int64
	ctx            context.Context
}

// SupportsDecodeParms returns true if filterName supports decode parameters.
func SupportsDecodeParms(f string) bool {
	return f == CCITTFax || f == LZW || f == Flate
}

func getReaderBytes(r io.Reader) ([]byte, error) {
	var bb []byte
	if buf, ok := r.(*bytes.Buffer); ok {
		bb = buf.Bytes()
	} else {
		var buf bytes.Buffer
		if _, err := io.Copy(&buf, r); err != nil {
			return nil, err
		}

		bb = buf.Bytes()
	}

	return bb, nil
}

func (f baseFilter) decodeLimit(maxLen int64) int64 {
	if maxLen >= 0 {
		return maxLen
	}
	if f.maxDecodeBytes == 0 {
		return DefaultMaxDecodeBytes
	}
	return f.maxDecodeBytes
}

func (f baseFilter) checkDecodeLength(maxLen int64) error {
	if maxLen >= 0 {
		if limit := f.decodeLimit(-1); limit >= 0 && maxLen > limit {
			return ErrDecodeLimitExceeded
		}
	}
	return nil
}

func (f baseFilter) checkContext() error {
	if f.ctx == nil {
		return nil
	}
	return f.ctx.Err()
}

func (f baseFilter) copyDecoded(r io.Reader, maxLen int64) (*bytes.Buffer, error) {
	if err := f.checkContext(); err != nil {
		return nil, err
	}
	if err := f.checkDecodeLength(maxLen); err != nil {
		return nil, err
	}
	if maxLen >= 0 {
		var b bytes.Buffer
		_, err := io.CopyN(&b, contextReader{ctx: f.ctx, reader: r}, maxLen)
		return &b, err
	}

	limit := f.decodeLimit(maxLen)
	if limit < 0 {
		var b bytes.Buffer
		_, err := io.Copy(&b, contextReader{ctx: f.ctx, reader: r})
		return &b, err
	}
	if limit == maxInt64 {
		var b bytes.Buffer
		_, err := io.Copy(&b, contextReader{ctx: f.ctx, reader: r})
		return &b, err
	}

	var b bytes.Buffer
	if err := copyDecodedWithinLimit(&b, contextReader{ctx: f.ctx, reader: r}, limit); err != nil {
		return nil, err
	}
	return &b, nil
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if r.ctx != nil {
		if err := r.ctx.Err(); err != nil {
			return 0, err
		}
	}
	n, err := r.reader.Read(p)
	if r.ctx != nil {
		if ctxErr := r.ctx.Err(); ctxErr != nil {
			return n, ctxErr
		}
	}
	return n, err
}

type contextWriter struct {
	ctx    context.Context
	writer io.Writer
}

func (w contextWriter) Write(p []byte) (int, error) {
	if w.ctx != nil {
		if err := w.ctx.Err(); err != nil {
			return 0, err
		}
	}
	n, err := w.writer.Write(p)
	if w.ctx != nil {
		if ctxErr := w.ctx.Err(); ctxErr != nil {
			return n, ctxErr
		}
	}
	return n, err
}

func copyDecodedWithinLimit(dst io.Writer, r io.Reader, limit int64) error {
	if limit < 0 || limit == maxInt64 {
		_, err := io.Copy(dst, r)
		return err
	}

	writer := &decodeLimitWriter{w: dst, remaining: limit}
	copied, err := io.Copy(writer, io.LimitReader(r, limit))
	if err != nil {
		return err
	}
	if copied < limit {
		return nil
	}

	var extra [1]byte
	n, err := io.ReadFull(r, extra[:])
	if n > 0 {
		return ErrDecodeLimitExceeded
	}
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return err
	}
	return nil
}

type decodeLimitWriter struct {
	w         io.Writer
	remaining int64
}

func (w *decodeLimitWriter) Write(p []byte) (int, error) {
	if w.remaining < 0 {
		return w.w.Write(p)
	}
	if int64(len(p)) <= w.remaining {
		n, err := w.w.Write(p)
		w.remaining -= int64(n)
		return n, err
	}

	allowed := int(w.remaining)
	n, err := w.w.Write(p[:allowed])
	w.remaining -= int64(n)
	if err != nil {
		return n, err
	}
	if n < allowed {
		return n, io.ErrShortWrite
	}
	return n, ErrDecodeLimitExceeded
}
