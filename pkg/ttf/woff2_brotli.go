//go:build !js

package ttf

import (
	"bytes"
	"errors"
	"io"

	"github.com/andybalholm/brotli"
)

// woff2Decompress is woff2_decompress (BrotliDecoderDecompress, which
// ignores input after the stream's end).
func woff2Decompress(dstSize uint64, src []byte) ([]byte, error) {
	r := brotli.NewReader(bytes.NewReader(src))
	dst := make([]byte, 0, dstSize)
	buf := make([]byte, 32*1024)
	for {
		n, err := r.Read(buf)
		if uint64(len(dst))+uint64(n) > dstSize {
			return nil, errInvalidTable
		}
		dst = append(dst, buf[:n]...)
		if errors.Is(err, io.EOF) || (err != nil && err.Error() == "brotli: excessive input") {
			break
		}
		if err != nil {
			return nil, errInvalidTable
		}
	}
	if uint64(len(dst)) != dstSize {
		return nil, errInvalidTable
	}
	return dst, nil
}
