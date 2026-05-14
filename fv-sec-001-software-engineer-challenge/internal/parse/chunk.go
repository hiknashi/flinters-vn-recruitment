package parse

import (
	"bufio"
	"errors"
	"io"
	"os"
)

// ChunkResult reports how many rows a single chunk yielded.
type ChunkResult struct {
	Parsed  int64
	Skipped int64
}

// readBufferSize is the bufio buffer used per chunk. It comfortably exceeds any
// realistic CSV line, so ReadSlice returns whole lines without copying.
const readBufferSize = 1 << 20 // 1 MiB

// SplitRanges divides a file of the given size into n contiguous, start-
// inclusive / end-exclusive byte ranges. The split points are arbitrary byte
// offsets; ProcessChunk realigns them to line boundaries so that every row is
// handled by exactly one chunk.
func SplitRanges(size int64, n int) [][2]int64 {
	if n < 1 {
		n = 1
	}
	if size == 0 {
		return [][2]int64{{0, 0}}
	}
	if int64(n) > size {
		n = int(size)
	}
	ranges := make([][2]int64, 0, n)
	step := size / int64(n)
	var start int64
	for i := 0; i < n; i++ {
		end := start + step
		if i == n-1 {
			end = size
		}
		ranges = append(ranges, [2]int64{start, end})
		start = end
	}
	return ranges
}

// ProcessChunk reads the byte range [start, end) of the file at path and
// invokes onRow for every valid CSV row whose first byte falls within the
// range. Malformed lines are counted in ChunkResult.Skipped instead.
//
// Boundary handling: a line that begins inside the range but extends past end
// is still read in full, and a chunk that does not start at byte 0 discards
// its leading partial line (which belongs to the previous chunk). Together
// these guarantee every row is processed exactly once. The file header line is
// skipped by the chunk that owns byte 0.
func ProcessChunk(path string, start, end int64, onRow func(Row)) (ChunkResult, error) {
	var res ChunkResult
	f, err := os.Open(path)
	if err != nil {
		return res, err
	}
	defer f.Close()

	if _, err := f.Seek(start, io.SeekStart); err != nil {
		return res, err
	}
	r := bufio.NewReaderSize(f, readBufferSize)
	offset := start

	// The first line read is never this chunk's to process: for chunk 0 it is
	// the CSV header, and for any later chunk it is the tail of a line owned by
	// the previous chunk.
	skipped, err := r.ReadSlice('\n')
	offset += int64(len(skipped))
	if err != nil {
		if err == io.EOF {
			return res, nil // header-only file, or a chunk with no line break
		}
		return res, err
	}

	for offset < end {
		line, err := r.ReadSlice('\n')
		if errors.Is(err, bufio.ErrBufferFull) {
			// A line longer than the read buffer: drain it and skip it rather
			// than misparse a fragment.
			offset += int64(len(line))
			n, derr := drainLine(r)
			offset += int64(n)
			res.Skipped++
			if derr == nil {
				continue
			}
			if derr == io.EOF {
				break
			}
			return res, derr
		}
		offset += int64(len(line))
		if trimmed := trimEOL(line); len(trimmed) > 0 {
			if row, perr := ParseRow(trimmed); perr == nil {
				onRow(row)
				res.Parsed++
			} else {
				res.Skipped++
			}
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return res, err
		}
	}
	return res, nil
}

// trimEOL removes a trailing "\n", "\r" or "\r\n" from a line.
func trimEOL(line []byte) []byte {
	n := len(line)
	for n > 0 && (line[n-1] == '\n' || line[n-1] == '\r') {
		n--
	}
	return line[:n]
}

// drainLine consumes bytes up to and including the next newline, returning the
// number of bytes consumed. It is used to discard an over-long line.
func drainLine(r *bufio.Reader) (int, error) {
	total := 0
	for {
		chunk, err := r.ReadSlice('\n')
		total += len(chunk)
		if errors.Is(err, bufio.ErrBufferFull) {
			continue
		}
		return total, err
	}
}
