package openairt

import (
	"bytes"
	"io"
	"sync"
)

type BlockingBuffer struct {
	buf    bytes.Buffer
	mu     sync.Mutex
	cond   *sync.Cond
	closed bool
}

func NewBlockingBuffer() *BlockingBuffer {
	bb := &BlockingBuffer{}
	bb.cond = sync.NewCond(&bb.mu)
	return bb
}

// Write is non-blocking
func (b *BlockingBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return 0, io.ErrClosedPipe
	}

	n, err := b.buf.Write(p)
	b.cond.Signal() // notify one waiting reader
	return n, err
}

// Read blocks until data is available or closed
func (b *BlockingBuffer) Read(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for {
		if b.buf.Len() > 0 {
			return b.buf.Read(p)
		}
		if b.closed {
			return 0, io.EOF
		}
		b.cond.Wait()
	}
}

func (b *BlockingBuffer) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = true
	b.cond.Broadcast() // wake up all waiting readers
	return nil
}
