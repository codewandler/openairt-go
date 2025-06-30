package openairt

import (
	"bytes"
	"encoding/binary"
	"github.com/faiface/beep"
)

type PCMStreamer struct {
	data []int16
	pos  int
}

func NewPCMStreamer(b []byte) *PCMStreamer {
	samples := make([]int16, len(b)/2)
	for i := 0; i < len(samples); i++ {
		samples[i] = int16(binary.LittleEndian.Uint16(b[i*2:]))
	}
	return &PCMStreamer{data: samples}
}

func (s *PCMStreamer) Stream(samples [][2]float64) (n int, ok bool) {
	for i := range samples {
		if s.pos >= len(s.data) {
			return i, false
		}
		val := float64(s.data[s.pos]) / 32768.0
		samples[i][0] = val
		samples[i][1] = val // duplicate mono to stereo
		s.pos++
	}
	return len(samples), true
}

func (s *PCMStreamer) Err() error { return nil }

func ResamplePCM(pcmData []byte, fromRate, toRate int) ([]byte, error) {
	streamer := NewPCMStreamer(pcmData)

	resampler := beep.Resample(3, beep.SampleRate(fromRate), beep.SampleRate(toRate), streamer)

	// Buffer to collect the output
	buf := new(bytes.Buffer)
	sample := make([][2]float64, 1024)

	for {
		n, ok := resampler.Stream(sample)
		for i := 0; i < n; i++ {
			mono := (sample[i][0] + sample[i][1]) / 2.0
			int16Val := int16(mono * 32767)
			err := binary.Write(buf, binary.LittleEndian, int16Val)
			if err != nil {
				return nil, err
			}
		}
		if !ok {
			break
		}
	}

	return buf.Bytes(), nil
}
