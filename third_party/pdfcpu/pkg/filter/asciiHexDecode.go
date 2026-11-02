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
	"bufio"
	"bytes"
	"encoding/hex"
	"errors"
	"io"
)

type asciiHexDecode struct {
	baseFilter
}

const eodHexDecode = '>'

// Encode implements encoding for an ASCIIHexDecode filter.
func (f asciiHexDecode) Encode(r io.Reader) (io.Reader, error) {
	bb, err := getReaderBytes(r)
	if err != nil {
		return nil, err
	}

	if len(bb) > maxInt/2 {
		return nil, errors.New("ASCIIHex encode: length overflow")
	}

	dst := make([]byte, hex.EncodedLen(len(bb)))
	hex.Encode(dst, bb)

	// eod marker
	dst = append(dst, eodHexDecode)

	return bytes.NewBuffer(dst), nil
}

// Decode implements decoding for an ASCIIHexDecode filter.
func (f asciiHexDecode) Decode(r io.Reader) (io.Reader, error) {
	return f.DecodeLength(r, -1)
}

// DecodeLength implements decoding for an ASCIIHexDecode filter with a maximum output length.
func (f asciiHexDecode) DecodeLength(r io.Reader, maxLen int64) (io.Reader, error) {
	if err := f.checkContext(); err != nil {
		return nil, err
	}
	r = contextReader{ctx: f.ctx, reader: r}
	if err := f.checkDecodeLength(maxLen); err != nil {
		return nil, err
	}
	if maxLen > int64(maxInt) || maxLen > maxInt64/2 {
		return nil, errors.New("ASCIIHex decode: length overflow")
	}
	if maxLen == 0 {
		return bytes.NewBuffer(nil), nil
	}

	outputLimit := maxLen
	if maxLen < 0 {
		outputLimit = f.decodeLimit(-1)
	}
	reader := bufio.NewReader(r)
	var dst bytes.Buffer
	var pair [2]byte
	nibbleCount := 0
	written := int64(0)
	writePair := func() (bool, error) {
		if outputLimit >= 0 && written >= outputLimit {
			if maxLen >= 0 {
				return true, nil
			}
			return false, ErrDecodeLimitExceeded
		}
		if written >= int64(maxInt) {
			return false, errors.New("ASCIIHex decode: length overflow")
		}
		var decoded [1]byte
		if _, err := hex.Decode(decoded[:], pair[:]); err != nil {
			return false, err
		}
		if err := dst.WriteByte(decoded[0]); err != nil {
			return false, err
		}
		written++
		nibbleCount = 0
		return maxLen >= 0 && written >= maxLen, nil
	}

	for {
		if err := f.checkContext(); err != nil {
			return nil, err
		}
		b, err := reader.ReadByte()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if b == eodHexDecode {
			break
		}
		switch b {
		case 0x00, 0x09, 0x0A, 0x0C, 0x0D, 0x20:
			continue
		}
		pair[nibbleCount] = b
		nibbleCount++
		if nibbleCount != len(pair) {
			continue
		}
		done, err := writePair()
		if err != nil {
			return nil, err
		}
		if done {
			return &dst, nil
		}
	}

	if nibbleCount == 1 {
		pair[1] = '0'
		done, err := writePair()
		if err != nil {
			return nil, err
		}
		if done {
			return &dst, nil
		}
	}
	if maxLen >= 0 && written < maxLen {
		return nil, io.ErrUnexpectedEOF
	}
	return &dst, nil
}
