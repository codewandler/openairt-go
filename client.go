package openairt

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/codewandler/openairt-go/events"
	"github.com/codewandler/openairt-go/internal/websocket"
	nanoid "github.com/matoous/go-nanoid/v2"
	"github.com/smallnest/ringbuffer"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"
)

const (
	API_URL = "https://api.openairt.com"
)

type ClientConfig struct {
	ApiKey string
	Model  string
}

type Client struct {
	config     *ClientConfig
	ws         *websocket.Client
	onEvent    func(e any)
	onError    func(e *events.ErrorEvent)
	logger     *slog.Logger
	update     chan struct{}
	audioIn    chan []byte
	audioClear chan struct{}
	audio      *ringbuffer.RingBuffer
}

func (c *Client) Read(data []byte) (int, error) {
	return c.audio.Read(data)
}

func (c *Client) Write(data []byte) (int, error) {
	id, err := nanoid.New()
	if err != nil {
		return 0, err
	}

	evtData, err := json.Marshal(map[string]any{
		"event_id": id,
		"type":     "input_audio_buffer.append",
		"audio":    base64.StdEncoding.EncodeToString(data),
	})
	if err != nil {
		return 0, err
	}

	c.ws.WriteText(evtData)

	return len(data), nil
}

var _ io.Writer = (*Client)(nil)

func (c *Client) SetLogger(logger *slog.Logger) {
	c.logger = logger
}

func (c *Client) OnEvent(h func(e any)) {
	c.onEvent = h
}

func (c *Client) OnError(h func(e *events.ErrorEvent)) {
	c.onError = h
}

func (c *Client) Send(evt any) error {

	data, err := json.Marshal(evt)
	if err != nil {
		return err
	}

	fmt.Printf("evt --> %+v\n", string(data))

	c.ws.WriteText(data)

	return nil
}

func (c *Client) CreateResponseWithPayload(p events.ResponseCreatePayload) error {
	return c.Send(events.ResponseCreateEvent{
		BaseEvent: events.NewBaseEvent("response.create"),
		Response:  p,
	})
}

func (c *Client) CreateResponse() error {
	return c.Send(events.ResponseCreateEvent{
		BaseEvent: events.NewBaseEvent("response.create"),
		Response:  events.ResponseCreatePayload{},
	})
}

func dispatchEvent[T any](f func(any), data []byte) {
	evt, err := events.Parse[T](data)
	if err != nil {
		slog.Error("failed to parse event", slog.Any("err", err))
		return
	}

	if f != nil {
		f(evt)
	}
}

func (c *Client) SessionUpdate(session events.SessionUpdate) error {
	evt := events.SessionUpdateEvent{
		BaseEvent: events.NewBaseEvent("session.update"),
		Session:   session,
	}

	if err := c.Send(evt); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	select {
	case <-ctx.Done():
		return fmt.Errorf("timeout waiting for session update")
	case <-c.update:
	}

	return nil
}

func (c *Client) UserInput(text string, respond bool) (err error) {
	id, _ := nanoid.New()
	err = c.Send(events.ConversationItemCreateEvent{
		BaseEvent: events.NewBaseEvent("conversation.item.create"),
		Item: events.ConversationItem{
			ID:   id,
			Type: "message",
			Role: "user",
			Content: []events.ConversationItemContent{
				{Type: "input_text", Text: text},
			},
		},
	})
	if err != nil {
		return err
	}

	if respond {
		return c.CreateResponse()
	}

	return nil

}

func (c *Client) Open(ctx context.Context) error {
	headers := http.Header{}
	headers.Add("Authorization", fmt.Sprintf("Bearer %s", c.config.ApiKey))
	headers.Add("OpenAI-Beta", "realtime=v1")

	initialized := make(chan error)

	if ws, err := websocket.Connect(ctx, websocket.ClientConfig{
		Logger:  slog.New(slog.DiscardHandler),
		URL:     fmt.Sprintf("wss://api.openai.com/v1/realtime?model=%s", c.config.Model),
		Headers: headers,
		OnText: func(data []byte) error {
			var x struct {
				Type    string `json:"type"`
				EventID string `json:"event_id"`
			}

			if err := json.Unmarshal(data, &x); err != nil {
				return err
			}

			//println("<-- evt:", x.Type, x.EventID)

			switch x.Type {
			case "error":
				evt, err := events.Parse[events.ErrorEvent](data)
				if err != nil {
					slog.Error("failed to parse error event", slog.Any("err", err))
				} else if c.onError != nil {
					c.onError(evt)
				}
			// TODO: case "response.done":
			// TODO: case "input_audio_buffer.speech_started":
			// TODO: case "input_audio_buffer.speech_stopped":
			case "session.created":
				dispatchEvent[events.SessionCreatedEvent](c.onEvent, data)
				go func() {
					initialized <- c.SessionUpdate(events.SessionUpdate{
						Voice:             "alloy",
						InputAudioFormat:  events.AudioFormatPCM16,
						OutputAudioFormat: events.AudioFormatPCM16,
						Temperature:       0.6,
						Speed:             1.4,
						Instructions:      "Your name is Martha. Your main language is English. Your are helpful",
						Modalities:        []string{"text", "audio"},
						TurnDetection: &events.TurnDetection{
							CreateResponse:    true,
							InterruptResponse: true,
							Type:              "server_vad",
						},
					})
				}()
			case "session.updated":
				c.update <- struct{}{}
				dispatchEvent[events.SessionUpdateEvent](c.onEvent, data)
			case "response.audio_transcript.done":
				dispatchEvent[events.ResponseAudioTranscriptDoneEvent](c.onEvent, data)
			case "response.audio_transcript.delta":
				dispatchEvent[events.ResponseAudioTranscriptDeltaEvent](c.onEvent, data)
			case "response.audio.done":
				dispatchEvent[events.ResponseAudioDone](c.onEvent, data)
			case "response.audio.delta":

				evt, err := events.Parse[events.ResponseAudioDeltaEvent](data)
				if err != nil {
					slog.Error("failed to parse response audio delta event", slog.Any("err", err))
				}
				data, err := base64.StdEncoding.DecodeString(evt.Delta)
				if err != nil {
					slog.Error("failed to decode base64 data", slog.Any("err", err))
				}

				// write to buffer
				c.audioIn <- data

			case "input_audio_buffer.speech_started":
				/*id1, _ := nanoid.New()
				c.Send(map[string]any{
					"event_id": id1,
					"type":     "response.cancel",
				})*/
				/*id2, _ := nanoid.New()
				c.Send(map[string]any{
					"event_id": id2,
					"type":     "input_audio_buffer.clear",
				})*/

				c.audioClear <- struct{}{}

				dispatchEvent[events.SpeechStartedEvent](c.onEvent, data)
			case "input_audio_buffer.speech_stopped":
				dispatchEvent[events.SpeechStoppedEvent](c.onEvent, data)
			}

			return nil
		},
	}); err != nil {
		return err
	} else {
		c.ws = ws
	}

	go func() {
		for {
			select {
			case <-c.audioClear:
				c.audio.Reset()
			case data := <-c.audioIn:
				c.audio.Write(data)
			}
		}
	}()

	return <-initialized

}

func New(config *ClientConfig) *Client {
	return &Client{
		config:     config,
		logger:     slog.Default(),
		update:     make(chan struct{}, 1),
		audio:      ringbuffer.New(1024 * 1024 * 5).SetBlocking(true),
		audioClear: make(chan struct{}, 1),
		audioIn:    make(chan []byte, 100),
	}
}

func NewDefault() *Client {
	return New(&ClientConfig{
		ApiKey: os.Getenv("OPENAI_API_KEY"),
		Model:  "gpt-4o-realtime-preview-2025-06-03",
	})
}

type InterruptEvent struct {
}
