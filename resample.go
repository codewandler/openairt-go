package openairt

import (
	"encoding/binary"
	"errors"
	"io"
	"math"
)

type Resampler interface {
	Resample(input []int16, fromRate, toRate int) []int16
}

// LinearResampler uses linear interpolation to resample audio
type LinearResampler struct{}

func (r LinearResampler) Resample(input []int16, fromRate, toRate int) []int16 {
	if fromRate == toRate {
		return input
	}

	ratio := float64(toRate) / float64(fromRate)
	outputLen := int(float64(len(input)) * ratio)
	output := make([]int16, outputLen)

	for i := 0; i < outputLen; i++ {
		srcPos := float64(i) / ratio
		i0 := int(math.Floor(srcPos))
		i1 := i0 + 1
		if i1 >= len(input) {
			i1 = len(input) - 1
		}

		frac := srcPos - float64(i0)
		output[i] = int16((1-frac)*float64(input[i0]) + frac*float64(input[i1]))
	}
	return output
}

// ResampleReader reads from an underlying reader, resamples audio, and returns PCM16 bytes
type ResampleReader struct {
	Source    io.Reader
	FromRate  int
	ToRate    int
	Resampler Resampler
}

func (r *ResampleReader) Read(p []byte) (int, error) {
	temp := make([]byte, len(p))
	n, err := r.Source.Read(temp)
	if err != nil && err != io.EOF {
		return 0, err
	}

	sampleCount := n / 2
	samples := make([]int16, sampleCount)
	for i := 0; i < sampleCount; i++ {
		samples[i] = int16(binary.LittleEndian.Uint16(temp[i*2:]))
	}

	resampled := r.Resampler.Resample(samples, r.FromRate, r.ToRate)
	byteLen := len(resampled) * 2
	if byteLen > len(p) {
		resampled = resampled[:len(p)/2]
		byteLen = len(resampled) * 2
	}

	for i, s := range resampled {
		binary.LittleEndian.PutUint16(p[i*2:], uint16(s))
	}

	return byteLen, nil
}

// ResampleWriter takes PCM16 data, resamples it, and writes to an underlying writer
type ResampleWriter struct {
	Sink      io.Writer
	FromRate  int
	ToRate    int
	Resampler Resampler
}

func (w *ResampleWriter) Write(p []byte) (int, error) {
	if len(p)%2 != 0 {
		return 0, errors.New("unaligned PCM16 byte stream")
	}

	sampleCount := len(p) / 2
	samples := make([]int16, sampleCount)
	for i := 0; i < sampleCount; i++ {
		samples[i] = int16(binary.LittleEndian.Uint16(p[i*2:]))
	}

	resampled := w.Resampler.Resample(samples, w.FromRate, w.ToRate)
	outBuf := make([]byte, len(resampled)*2)
	for i, s := range resampled {
		binary.LittleEndian.PutUint16(outBuf[i*2:], uint16(s))
	}

	return w.Sink.Write(outBuf)
}
