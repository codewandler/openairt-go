package main

import (
	"encoding/binary"
	"errors"
	"sync"
	"time"

	"github.com/MarkKremer/microphone/v2"
	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/speaker"
)

const (
	bytesPerSample  = 2                      // 16-bit mono PCM
	playLatency     = 200 * time.Millisecond // speaker buffer = 200 ms
	captureFrames   = 1024                   // mic pull size
	playChannelSize = 48_000                 // 1 s @ 48 kHz
)

// NewAudioIO returns an io.ReadWriter that speaks 16-bit MONO PCM.
// ctx / framesPerBuffer are ignored for API compatibility.
func NewAudioIO(
	sampleRate float64,
) (*AudioIO, error) {

	sr := beep.SampleRate(int(sampleRate))

	// --------------- playback side ------------------------------------------
	if err := speaker.Init(sr, sr.N(playLatency)); err != nil {
		return nil, err
	}

	// channel feeding the one global streamer
	playCh := make(chan [2]float64, playChannelSize)
	// kick the streamer exactly once
	speaker.Play(newChanStreamer(playCh))

	// --------------- capture side -------------------------------------------
	mic, _, err := microphone.OpenDefaultStream(sr, 1) // 1 = mono
	if err != nil {
		return nil, err
	}
	mic.Start()

	a := &AudioIO{
		mic:        mic,
		playCh:     playCh,
		readBuf:    make([]byte, 0, 8192),
		sampleRate: sr,
	}

	go a.captureLoop()
	return a, nil
}

// ---------------------------------------------------------------------------

type AudioIO struct {
	mic        *microphone.Streamer
	sampleRate beep.SampleRate

	playCh chan [2]float64 // writer side pushes here

	readMu  sync.Mutex
	readBuf []byte
}

// --------------------------- io.Reader --------------------------------------

func (a *AudioIO) Read(p []byte) (int, error) {
	for {
		a.readMu.Lock()
		if len(a.readBuf) > 0 {
			n := copy(p, a.readBuf)
			a.readBuf = a.readBuf[n:]
			a.readMu.Unlock()
			return n, nil
		}
		a.readMu.Unlock()
		time.Sleep(3 * time.Millisecond)
	}
}

// --------------------------- io.Writer --------------------------------------

func (a *AudioIO) Write(b []byte) (int, error) {
	if len(b)%bytesPerSample != 0 {
		return 0, errors.New("audioio: Write expects 16-bit mono PCM")
	}

	for i := 0; i < len(b); i += bytesPerSample {
		v := int16(binary.LittleEndian.Uint16(b[i:]))
		f := float64(v) / 32768.0    // range -1..1
		a.playCh <- [2]float64{f, f} // duplicate to stereo
	}
	return len(b), nil
}

// ---------------------------------------------------------------------------

func (a *AudioIO) captureLoop() {
	frames := make([][2]float64, captureFrames)

	for {
		n, ok := a.mic.Stream(frames)
		if !ok {
			return
		}

		mono := stereoSamplesToPCM16Mono(frames[:n])

		a.readMu.Lock()
		a.readBuf = append(a.readBuf, mono...)
		a.readMu.Unlock()
	}
}

func (a *AudioIO) Clear() {
	// 1.  Empty our own buffered channel.
	for {
		select {
		case <-a.playCh: // discard one frame
		default: // channel drained
			goto drained
		}
	}
drained:

	// 2.  Flush Beep’s mixer (needs the global lock).
	speaker.Lock()
	speaker.Clear()
	speaker.Unlock()
}

// ---------------------- conversion helpers ---------------------------------

func stereoSamplesToPCM16Mono(s [][2]float64) []byte {
	b := make([]byte, len(s)*bytesPerSample)
	for i, v := range s {
		m := int16(clamp(v[0]) * 32767) // take left channel
		binary.LittleEndian.PutUint16(b[i*2:], uint16(m))
	}
	return b
}

func clamp(f float64) float64 {
	switch {
	case f > 1:
		return 1
	case f < -1:
		return -1
	default:
		return f
	}
}

// ------------------------- chanStreamer ------------------------------------

// chanStreamer pulls samples from a channel. When the channel is empty it
// plays silence, avoiding glitches while waiting for more data.
type chanStreamer struct {
	ch <-chan [2]float64
}

func newChanStreamer(ch <-chan [2]float64) *chanStreamer { return &chanStreamer{ch: ch} }

func (c *chanStreamer) Stream(buf [][2]float64) (int, bool) {
	for i := range buf {
		select {
		case smp := <-c.ch:
			buf[i] = smp
		default: // no sample available yet – play silence
			buf[i] = [2]float64{}
		}
	}
	return len(buf), true
}
func (c *chanStreamer) Err() error { return nil }
