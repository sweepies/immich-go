package exif

import (
	"bufio"
	"bytes"
	"io"
)

type sliceReader struct {
	bufio.Reader
}

func newSliceReader(r io.Reader) *sliceReader {
	return &sliceReader{
		Reader: *bufio.NewReader(r),
	}
}

func (r *sliceReader) ReadSlice(l int) ([]byte, error) {
	b := make([]byte, l)
	_, err := r.Read(b)
	return b, err
}

func searchPattern(r io.Reader, pattern []byte, buffer []byte) (*sliceReader, error) {
	var err error
	pos := 0
	ofs := 0

	var bytesRead int
	for {
		// Read a chunk of data into the buffer
		bytesRead, err = r.Read(buffer[ofs:])
		if err != nil {
			return nil, err
		}

		// Search for the pattern within the buffer
		index := bytes.Index(buffer[:ofs+bytesRead], pattern)
		if index >= 0 {
			return newSliceReader(io.MultiReader(bytes.NewReader(buffer[index:]), r)), nil
		}

		// Move the remaining bytes of the current buffer to the beginning.
		// Fewer than len(pattern)-1 bytes may be buffered (tiny or truncated
		// files); keep whatever there is instead of computing a negative index.
		total := bytesRead + ofs
		p := total - len(pattern) + 1
		if p < 0 {
			p = 0
		}

		copy(buffer, buffer[p:total])
		ofs = total - p
		pos += bytesRead
	}
}
