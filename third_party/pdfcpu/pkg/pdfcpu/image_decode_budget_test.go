package pdfcpu

import (
	"bytes"
	"compress/zlib"
	"errors"
	"testing"

	"github.com/pdfcpu/pdfcpu/pkg/filter"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

func TestImageDecodeUsesDocumentAggregateBudget(t *testing.T) {
	const outputLimit = 128
	const streamLimit = 4096
	decoded := bytes.Repeat([]byte("x"), outputLimit+1)
	var compressed bytes.Buffer
	writer := zlib.NewWriter(&compressed)
	if _, err := writer.Write(decoded); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name   string
		decode func(*model.Context, *types.StreamDict) error
	}{
		{
			name: "extract image",
			decode: func(ctx *model.Context, sd *types.StreamDict) error {
				return decodeImage(ctx, sd, filter.Flate, filter.Flate, 1)
			},
		},
		{
			name: "write image bytes",
			decode: func(ctx *model.Context, sd *types.StreamDict) error {
				_, err := streamBytes(ctx.XRefTable, sd, "synthetic image")
				return err
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			conf := model.NewStatelessConfiguration()
			conf.Limits.MaxDecodeBytes = streamLimit
			conf.Limits.MaxTotalDecodeBytes = outputLimit
			xref := &model.XRefTable{
				Conf:         conf,
				DecodeBudget: model.NewDecodeBudget(outputLimit),
			}
			ctx := &model.Context{Configuration: conf, XRefTable: xref}
			stream := types.NewStreamDict(types.NewDict(), 0, nil, nil, []types.PDFFilter{{Name: filter.Flate}})
			stream.Raw = bytes.Clone(compressed.Bytes())

			if err := tc.decode(ctx, &stream); !errors.Is(err, filter.ErrDecodeLimitExceeded) {
				t.Fatalf("image decode error=%v, want aggregate decode limit exceeded", err)
			}
		})
	}
}
