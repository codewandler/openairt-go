package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

type HandlerFunc func(data []byte) error

func Json[T any](j func(x T) error) HandlerFunc {
	return func(data []byte) error {
		var t T
		if err := json.Unmarshal(data, &t); err != nil {
			return err
		}

		return j(t)
	}
}

type ClientConfig struct {
	URL         string
	DialTimeout time.Duration
	Headers     http.Header
	OnText      func(data []byte) error
	OnBinary    func(data []byte) error
	Logger      *slog.Logger
}

type Client struct {
	out      chan wsutil.Message
	done     chan struct{}
	doneOnce sync.Once
	logger   *slog.Logger
}

func (c *Client) setDone() {
	c.doneOnce.Do(func() {
		close(c.done)
	})
}

func (c *Client) WriteText(data []byte) {
	c.Write(ws.OpText, data)
}

func (c *Client) WriteBinary(data []byte) {
	c.Write(ws.OpBinary, data)
}

func (c *Client) Ping(data []byte) {
	c.Write(ws.OpPing, data)
}

func (c *Client) SendClose(code ws.StatusCode, reason string) {
	c.Write(ws.OpClose, ws.NewCloseFrameBody(code, reason))
}

func (c *Client) Close(ctx context.Context) error {
	c.SendClose(ws.StatusNormalClosure, "closing")
	select {
	case <-c.done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("close failed: %w", ctx.Err())
	}
}

func (c *Client) Write(opcode ws.OpCode, data []byte) {
	c.out <- wsutil.Message{OpCode: opcode, Payload: data}
}

func Connect(ctx context.Context, config ClientConfig) (*Client, error) {

	logger := config.Logger
	if logger == nil {
		logger = slog.Default()
	}
	logger = logger.With(
		slog.String("url", config.URL),
	)

	dialTimeout := config.DialTimeout
	if dialTimeout == 0 {
		dialTimeout = 10 * time.Second
	}

	// 1) Handshake timeout only:
	hsCtx, cancel := context.WithTimeout(ctx, dialTimeout)
	defer cancel()

	// 2) Dial + WebSocket handshake
	d := ws.Dialer{
		Timeout: config.DialTimeout,
		Header:  ws.HandshakeHeaderHTTP(config.Headers),
	}
	conn, buf, hs, err := d.Dial(hsCtx, config.URL)
	if err != nil {
		return nil, err
	}
	logger.Debug("Handshake complete with response:", slog.Any("handshake", hs))

	// Make sure to recycle the buffer if non-nil:
	if buf != nil {
		defer ws.PutReader(buf)
	}

	logger.Info("Connected to websocket", slog.Any("url", config.URL))

	var (
		input  = make(chan wsutil.Message, 1000)
		output = make(chan wsutil.Message, 1000)
	)

	client := &Client{
		out:    output,
		done:   make(chan struct{}),
		logger: logger,
	}

	onTextFunc := config.OnText
	if onTextFunc == nil {
		onTextFunc = func(data []byte) error {
			return nil
		}
	}
	onBinaryFunc := config.OnBinary
	if onBinaryFunc == nil {
		onBinaryFunc = func(data []byte) error {
			return nil
		}
	}

	go func() {
		defer client.setDone()
		for {

			messages, err := wsutil.ReadServerMessage(conn, nil)
			if err != nil {
				if errors.Is(err, io.EOF) {
					return
				}

				logger.Error("ws read failed", slog.Any("err", err))

				return
			}
			for _, msg := range messages {
				input <- msg
			}
		}
	}()

	// output channel -> websocket
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case msg := <-output:
				err := wsutil.WriteClientMessage(conn, msg.OpCode, msg.Payload)
				if err != nil {
					logger.Error("Message write error:", slog.Any("err", err))
					return
				}

			}
		}
	}()

	// input channel processing
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-client.done:
				return
			case msg := <-input:

				// handle control
				if ws.OpCode.IsControl(msg.OpCode) {
					logger.Debug("rcv: control", slog.Any("opcode", msg.OpCode), slog.Any("payload", msg.Payload))

					if err := wsutil.HandleServerControlMessage(conn, msg); err != nil {
						logger.Error("handling of control messages failed", slog.Any("err", err))
					}

					switch msg.OpCode {
					case ws.OpClose:
						logger.Debug("rcv: close. closing client", slog.String("reason", string(msg.Payload)))
						client.setDone()
					}

					continue
				}

				switch msg.OpCode {
				case ws.OpText:
					logger.Debug("rcv: text", slog.String("text", string(msg.Payload)))
					if err := onTextFunc(msg.Payload); err != nil {
						logger.Error("text message handler failed", slog.Any("err", err))
					}

				case ws.OpBinary:
					logger.Debug("rcv: binary", slog.Int("len", len(msg.Payload)))
					if err := onBinaryFunc(msg.Payload); err != nil {
						logger.Error("binary message handler failed", slog.Any("err", err))
					}
				}
			}
		}
	}()

	client.Ping([]byte("ping"))

	return client, nil
}
