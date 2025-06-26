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
	"time"
)

type Client struct {
	config      *clientConfig
	ws          *websocket.Client
	onEvent     func(e any)
	onError     func(e *events.ErrorEvent)
	logger      *slog.Logger
	update      chan struct{}
	audioInChan chan []byte
	audioClear  chan struct{}
	audioOut    *ringbuffer.RingBuffer
	audioIn     *ringbuffer.RingBuffer
}

// Audio returns a reader and writer for the audio stream.
// The reader is used to read the audio stream from the server.
// The writer is used to write the audio stream to the server.
// The sample rate of the audio stream can be specified.
// If the sample rate is not specified, the default sample rate is 24_000 Hz.
// The sample rate of the audio stream must be 24_000 Hz.
// If the sample rate is not 24_000 Hz, the audio stream will be resampled.
// The resampling algorithm is linear.
func (c *Client) Audio(outSampleRate, inputSampleRate int) (io.Reader, io.Writer) {
	var (
		r io.Reader = c.audioOut
		w io.Writer = c.audioIn
	)

	if outSampleRate != 24_000 {
		r = &ResampleReader{
			Source:    c.audioOut,
			Resampler: LinearResampler{},
			FromRate:  24_000,
			ToRate:    outSampleRate,
		}
	}

	if inputSampleRate != 24_000 {
		w = &ResampleWriter{
			Sink:      c.audioIn,
			Resampler: LinearResampler{},
			FromRate:  inputSampleRate,
			ToRate:    24_000,
		}
	}

	return r, w
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
	headers.Add("Authorization", fmt.Sprintf("Bearer %s", c.config.apiKey))
	headers.Add("OpenAI-Beta", "realtime=v1")

	initialized := make(chan error)

	if ws, err := websocket.Connect(ctx, websocket.ClientConfig{
		Logger:  slog.New(slog.DiscardHandler),
		URL:     fmt.Sprintf("wss://api.openai.com/v1/realtime?model=%s", c.config.model),
		Headers: headers,
		OnText: func(data []byte) error {
			var x struct {
				Type    string `json:"type"`
				EventID string `json:"event_id"`
			}

			if err := json.Unmarshal(data, &x); err != nil {
				return err
			}

			println("<-- evt:", x.Type, x.EventID)

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
				c.audioInChan <- data

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
			case <-ctx.Done():
				return
			case <-c.audioClear:
				c.audioOut.Reset()
			case data := <-c.audioInChan:
				_, err := c.audioOut.Write(data)
				if err != nil {
					c.logger.Error("failed to write to audio read buffer", slog.Any("err", err))
				}
			}
		}
	}()

	go func() {
		buf := make([]byte, 1024)

		for {
			n, err := c.audioIn.Read(buf)
			if err != nil {
				if err == io.EOF {
					return
				}
				return
			}
			data := buf[:n]
			id, _ := nanoid.New()

			evtData, err := json.Marshal(map[string]any{
				"event_id": id,
				"type":     "input_audio_buffer.append",
				"audio":    base64.StdEncoding.EncodeToString(data),
			})
			if err != nil {
				c.logger.Error("failed to marshal input audio buffer append event", slog.Any("err", err))
				return
			}

			c.ws.WriteText(evtData)
		}
	}()

	return <-initialized

}

func New(opts ...ClientOption) *Client {
	config := &clientConfig{}
	withDefaults()(config)
	WithOptions(opts...)(config)
	return &Client{
		config:      config,
		logger:      slog.Default(),
		update:      make(chan struct{}, 1),
		audioOut:    ringbuffer.New(1024 * 1024).SetBlocking(true),
		audioIn:     ringbuffer.New(1024 * 1024 * 10).SetBlocking(true),
		audioClear:  make(chan struct{}, 1),
		audioInChan: make(chan []byte, 100),
	}
}

type InterruptEvent struct {
}
