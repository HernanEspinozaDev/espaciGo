package types_test

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/binary"
	"errors"
	"image"
	"image/jpeg"
	"testing"

	"github.com/pdfcpu/pdfcpu/pkg/filter"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

type settlementFailureBudget struct{ err error }

func (b settlementFailureBudget) ReserveDecode(int64) (int64, int64, error) {
	return 64, 64, nil
}

func (b settlementFailureBudget) FinishDecode(int64, int64, bool) error { return b.err }

func TestStreamDictDecodeReportsBudgetSettlementFailure(t *testing.T) {
	settlementErr := errors.New("synthetic reservation settlement failure")
	stream := types.NewStreamDict(types.NewDict(), 0, nil, nil, []types.PDFFilter{{Name: "UnknownFilter"}})
	stream.Raw = []byte("synthetic")
	err := stream.DecodeWithBudget(64, settlementFailureBudget{err: settlementErr})
	if !errors.Is(err, settlementErr) {
		t.Fatalf("decode error = %v, want settlement error to remain observable", err)
	}
}

func TestStreamDictDecodeWithBudgetPropagatesRequestCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	budget := model.NewDecodeBudget(64)
	budget.SetDecodeContext(ctx)
	stream := types.NewStreamDict(types.NewDict(), 0, nil, nil, []types.PDFFilter{{Name: filter.RunLength}})
	stream.Raw = []byte{0, 'a', 0x80}

	err := stream.DecodeWithBudget(16, budget)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("decode error = %v, want context cancellation", err)
	}
	if stream.Content != nil {
		t.Fatal("cancelled stream retained decoded content")
	}
}

func deflatePDFStream(t *testing.T, content []byte) []byte {
	t.Helper()
	var encoded bytes.Buffer
	writer := zlib.NewWriter(&encoded)
	if _, err := writer.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return encoded.Bytes()
}

func TestStreamDictDecodeWithBudgetEnforcesAggregateOutput(t *testing.T) {
	budget := model.NewDecodeBudget(15)
	newStream := func() *types.StreamDict {
		stream := types.NewStreamDict(types.NewDict(), 0, nil, nil, []types.PDFFilter{{Name: filter.Flate}})
		stream.Raw = deflatePDFStream(t, []byte("12345678"))
		return &stream
	}

	first := newStream()
	if err := first.DecodeWithBudget(32, budget); err != nil {
		t.Fatalf("first stream decode: %v", err)
	}
	if got := len(first.Content); got != 8 {
		t.Fatalf("first decoded length = %d, want 8", got)
	}

	second := newStream()
	if err := second.DecodeWithBudget(32, budget); !errors.Is(err, filter.ErrDecodeLimitExceeded) {
		t.Fatalf("second stream error = %v, want aggregate decode limit exceeded", err)
	}
	if got := budget.Used(); got < 8 || got > 15 {
		t.Fatalf("aggregate committed/reserved output = %d, want between 8 and 15", got)
	}
}

func TestStreamDictDecodeWithBudgetCountsIntermediateFilterOutput(t *testing.T) {
	for _, testCase := range []struct {
		name    string
		budget  int64
		wantErr bool
	}{
		{name: "one byte short", budget: 24, wantErr: true},
		{name: "exact aggregate limit", budget: 25},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			budget := model.NewDecodeBudget(testCase.budget)
			stream := types.NewStreamDict(types.NewDict(), 0, nil, nil, []types.PDFFilter{
				{Name: filter.Flate},
				{Name: filter.ASCIIHex},
			})
			stream.Raw = deflatePDFStream(t, []byte("3132333435363738>"))
			err := stream.DecodeWithBudget(64, budget)
			if testCase.wantErr {
				if !errors.Is(err, filter.ErrDecodeLimitExceeded) {
					t.Fatalf("decode error = %v, want aggregate limit exceeded", err)
				}
				if stream.Content != nil {
					t.Fatal("over-budget final content was retained")
				}
				return
			}
			if err != nil {
				t.Fatalf("exact-budget decode failed: %v", err)
			}
			if got := string(stream.Content); got != "12345678" {
				t.Fatalf("decoded content = %q, want 12345678", got)
			}
			if got := budget.Used(); got != testCase.budget {
				t.Fatalf("aggregate decoded output = %d, want %d", got, testCase.budget)
			}
		})
	}
}

func TestStreamDictDecodeWithBudgetCountsOpaqueTerminalMaterialization(t *testing.T) {
	for _, testCase := range []struct {
		name    string
		budget  int64
		wantErr bool
	}{
		{name: "one byte short", budget: 15, wantErr: true},
		{name: "exact aggregate limit", budget: 16},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			budget := model.NewDecodeBudget(testCase.budget)
			stream := types.NewStreamDict(types.NewDict(), 0, nil, nil, []types.PDFFilter{
				{Name: filter.ASCIIHex},
				{Name: filter.JPX},
			})
			stream.Raw = []byte("3132333435363738>")
			err := stream.DecodeWithBudget(64, budget)
			if testCase.wantErr {
				if !errors.Is(err, filter.ErrDecodeLimitExceeded) {
					t.Fatalf("decode error = %v, want aggregate limit exceeded", err)
				}
				if stream.Content != nil {
					t.Fatal("over-budget content was retained")
				}
				return
			}
			if err != nil {
				t.Fatalf("exact-budget decode failed: %v", err)
			}
			if got := string(stream.Content); got != "12345678" {
				t.Fatalf("decoded content = %q, want 12345678", got)
			}
			if got := budget.Used(); got != testCase.budget {
				t.Fatalf("aggregate materialized output = %d, want %d", got, testCase.budget)
			}
		})
	}
}

func TestStreamDictDecodeWithBudgetAccountsSolePreservedImageFilterOutput(t *testing.T) {
	for _, filterName := range []string{filter.DCT, filter.JPX, filter.JBIG2} {
		t.Run(filterName, func(t *testing.T) {
			raw := []byte("synthetic encoded image")
			for _, testCase := range []struct {
				name      string
				budgetCap int64
				streamCap int64
				wantUsed  int64
				wantErr   bool
			}{
				{name: "one byte short aggregate", budgetCap: int64(len(raw) - 1), streamCap: int64(len(raw)), wantUsed: int64(len(raw) - 1), wantErr: true},
				{name: "one byte short per stream", budgetCap: int64(len(raw)), streamCap: int64(len(raw) - 1), wantUsed: int64(len(raw) - 1), wantErr: true},
				{name: "exact aggregate limit", budgetCap: int64(len(raw)), streamCap: int64(len(raw))},
			} {
				t.Run(testCase.name, func(t *testing.T) {
					budget := model.NewDecodeBudget(testCase.budgetCap)
					stream := types.NewStreamDict(types.NewDict(), 0, nil, nil, []types.PDFFilter{{Name: filterName}})
					stream.Raw = raw

					err := stream.DecodeWithBudget(testCase.streamCap, budget)
					if testCase.wantErr {
						if !errors.Is(err, filter.ErrDecodeLimitExceeded) {
							t.Fatalf("decode error = %v, want aggregate decode limit exceeded", err)
						}
						if stream.Content != nil {
							t.Fatal("over-budget opaque content was retained")
						}
						if got := budget.Used(); got != testCase.wantUsed {
							t.Fatalf("conservative aggregate charge = %d, want reserved cap %d", got, testCase.wantUsed)
						}
						return
					}
					if err != nil {
						t.Fatalf("exact-budget decode failed: %v", err)
					}
					if !bytes.Equal(stream.Content, raw) {
						t.Fatalf("preserved content = %q, want raw bytes %q", stream.Content, raw)
					}
					if &stream.Content[0] != &raw[0] {
						t.Fatal("opaque pass-through allocated a second content buffer")
					}
					if got := budget.Used(); got != int64(len(raw)) {
						t.Fatalf("aggregate materialized output = %d, want %d", got, len(raw))
					}
					if err := stream.DecodeWithBudget(int64(len(raw)), budget); err != nil {
						t.Fatalf("cached decode with same budget: %v", err)
					}
					if got := budget.Used(); got != int64(len(raw)) {
						t.Fatalf("aggregate output after cached decode = %d, want %d without double charging", got, len(raw))
					}
					if err := stream.DecodeWithBudget(int64(len(raw)-1), budget); !errors.Is(err, filter.ErrDecodeLimitExceeded) {
						t.Fatalf("cached decode below per-stream limit = %v, want decode limit exceeded", err)
					}
					if got := budget.Used(); got != int64(len(raw)) {
						t.Fatalf("aggregate output after rejected cached decode = %d, want %d", got, len(raw))
					}
					otherBudget := model.NewDecodeBudget(int64(len(raw)))
					if err := stream.DecodeWithBudget(int64(len(raw)), otherBudget); !errors.Is(err, filter.ErrDecodeLimitExceeded) {
						t.Fatalf("cached decode with a different budget = %v, want decode limit exceeded", err)
					}
					if got := otherBudget.Used(); got != 0 {
						t.Fatalf("different budget charged cached output without re-decoding: used=%d", got)
					}

					second := types.NewStreamDict(types.NewDict(), 0, nil, nil, []types.PDFFilter{{Name: filterName}})
					second.Raw = raw
					if err := second.DecodeWithBudget(int64(len(raw)), budget); !errors.Is(err, filter.ErrDecodeLimitExceeded) {
						t.Fatalf("second opaque stream error = %v, want aggregate decode limit exceeded", err)
					}
				})
			}
		})
	}
}

func TestObjectStreamPrologAndLazyMaterializationShareDecodeBudget(t *testing.T) {
	content := []byte("1 0 1234567890")
	prologLen := len("1 0 ")
	budget := model.NewDecodeBudget(int64(prologLen + len(content) - 1))
	osd := types.NewObjectStreamDict()
	osd.Raw = deflatePDFStream(t, content)
	osd.FirstObjOffset = prologLen
	osd.MaxDecodeBytes = int64(len(content))
	osd.SetDecodeBudget(budget)

	prolog, err := osd.DecodeLengthWithLimit(int64(osd.FirstObjOffset), osd.MaxDecodeBytes)
	if err != nil {
		t.Fatalf("decode object-stream prolog: %v", err)
	}
	if got, want := string(prolog), "1 0 "; got != want {
		t.Fatalf("prolog = %q, want %q", got, want)
	}
	if got, want := budget.Used(), int64(prologLen); got != want {
		t.Fatalf("budget after prolog decode = %d, want %d", got, want)
	}

	lazyObject := types.NewLazyObjectStreamObject(osd, osd.FirstObjOffset, -1, nil).(types.LazyObjectStreamObject)
	if _, err := lazyObject.GetData(); !errors.Is(err, filter.ErrDecodeLimitExceeded) {
		t.Fatalf("lazy full materialization error = %v, want aggregate decode limit exceeded", err)
	}
	if got, want := budget.Used(), int64(prologLen+len(content)-1); got != want {
		t.Fatalf("budget after rejected lazy materialization = %d, want conservative charge %d", got, want)
	}
}

func TestStreamDictDecodeWithBudgetReservesBeforeDCTImageDecode(t *testing.T) {
	var encoded bytes.Buffer
	if err := jpeg.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 8, 8)), nil); err != nil {
		t.Fatal(err)
	}
	data := encoded.Bytes()
	marker := bytes.Index(data, []byte{0xff, 0xda})
	if marker < 0 || marker+4 > len(data) {
		t.Fatal("encoded JPEG is missing its scan header")
	}
	scanHeaderEnd := marker + 2 + int(binary.BigEndian.Uint16(data[marker+2:marker+4]))
	if scanHeaderEnd > len(data) {
		t.Fatal("encoded JPEG scan header exceeds input")
	}
	truncated := bytes.Clone(data[:scanHeaderEnd])
	if _, err := jpeg.DecodeConfig(bytes.NewReader(truncated)); err != nil {
		t.Fatalf("truncated JPEG should retain a readable image config: %v", err)
	}
	if _, err := jpeg.Decode(bytes.NewReader(truncated)); err == nil {
		t.Fatal("truncated JPEG unexpectedly decoded without image data")
	}

	budget := model.NewDecodeBudget(63)
	stream := types.NewStreamDict(types.NewDict(), 0, nil, nil, []types.PDFFilter{{Name: filter.DCT}})
	stream.Raw = truncated
	stream.CSComponents = 4
	err := stream.DecodeWithBudget(4096, budget)
	if !errors.Is(err, filter.ErrDecodeLimitExceeded) {
		t.Fatalf("DCT decode error = %v, want aggregate budget rejection before JPEG decode", err)
	}
	if stream.Content != nil {
		t.Fatal("over-budget DCT stream retained decoded content")
	}
	if got := budget.Used(); got != 63 {
		t.Fatalf("aggregate budget after rejected DCT decode = %d, want reserved capacity 63", got)
	}
}
