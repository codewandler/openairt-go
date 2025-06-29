package openairt

import (
	"fmt"
	"io"
	"time"
)

type FixedChunkReader struct {
	r         io.Reader
	buf       []byte
	chunkSize int
	eof       bool
}

func NewFixedChunkReader(r io.Reader, chunkSize int) *FixedChunkReader {
	return &FixedChunkReader{
		r:         r,
		chunkSize: chunkSize,
		buf:       make([]byte, 0, chunkSize*2),
	}
}

func getChunkSize(sampleRate int, sampleDuration time.Duration, bytesPerSample int, channels int) int {
	frames := int(float64(sampleRate) * sampleDuration.Seconds())
	chunkSize := frames * bytesPerSample * channels
	return chunkSize
}

func NewFixedAudioChunkReader(
	r io.Reader,
	sampleRate int,
	latency time.Duration,
	bytesPerSample int,
	channels int,
) *FixedChunkReader {
	return NewFixedChunkReader(r, getChunkSize(sampleRate, latency, bytesPerSample, channels))
}

func (f *FixedChunkReader) Read(p []byte) (int, error) {
	if len(p) < f.chunkSize {
		return 0, fmt.Errorf("buffer passed to Read must be at least %d bytes", f.chunkSize)
	}

	// Fill internal buffer until we can emit a full chunk or reach EOF
	for len(f.buf) < f.chunkSize && !f.eof {
		tmp := make([]byte, f.chunkSize)
		n, err := f.r.Read(tmp)
		if n > 0 {
			f.buf = append(f.buf, tmp[:n]...)
		}
		if err == io.EOF {
			f.eof = true
			break
		}
		if err != nil {
			return 0, err
		}
	}

	if len(f.buf) == 0 && f.eof {
		return 0, io.EOF
	}

	// Determine how much to copy (either a full chunk, or the remaining)
	n := f.chunkSize
	if len(f.buf) < f.chunkSize {
		n = len(f.buf)
	}

	copy(p, f.buf[:n])
	f.buf = f.buf[n:]

	return n, nil
}
