package openairt

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/codewandler/openairt-go/events"
	"github.com/codewandler/openairt-go/internal/websocket"
	"github.com/codewandler/openairt-go/tool"
	nanoid "github.com/matoous/go-nanoid/v2"
	"github.com/smallnest/ringbuffer"
	"io"
	"log/slog"
	"net/http"
	"time"
)

type AudioIO struct {
	agentBuffer       *ringbuffer.RingBuffer
	userInputWriter   io.Writer // userInputWriter is where to write audio to the agent.
	userOutputReader  io.Reader // userOutputReader is where to read audio from the agent.
	agentInputReader  io.Reader // agentInputReader is where to read audio from the user.
	agentOutputWriter io.Writer // agentOutputWriter is where to write audio to the user.
}

func (io *AudioIO) ClearOutputBuffer() {
	// TODO: locking
	io.agentBuffer.Reset()
}

func NewAudioIO(userSampleRate int, latency time.Duration) *AudioIO {

	userBufferSize := getChunkSize(24_000, latency, 2, 1) * 2
	userBuffer := ringbuffer.New(userBufferSize).SetBlocking(true)

	agentBufferSize := getChunkSize(24_000, 60*time.Second, 2, 1) * 2
	agentBuffer := ringbuffer.New(agentBufferSize).SetBlocking(true)

	return &AudioIO{
		// agent
		agentBuffer:      agentBuffer,
		agentInputReader: newChunkReader(userBuffer, 24_000, latency),
		agentOutputWriter: &ResampleWriter{
			Sink:      agentBuffer,
			FromRate:  24_000,
			ToRate:    userSampleRate,
			Resampler: LinearResampler{},
		},
		// user
		userOutputReader: newChunkReader(agentBuffer, userSampleRate, latency),
		userInputWriter: &ResampleWriter{
			Sink:      userBuffer,
			FromRate:  userSampleRate,
			ToRate:    24_000,
			Resampler: LinearResampler{},
		},
	}
}

type Client struct {
	config     *clientConfig
	ws         *websocket.Client
	onEvent    func(e any)
	onError    func(e *events.ErrorEvent)
	onToolCall func(name string, args map[string]any) (any, error)
	logger     *slog.Logger
	update     chan struct{}
	io         *AudioIO
}

func (c *Client) Audio() (io.Reader, io.Writer) {
	return c.io.userOutputReader, c.io.userInputWriter
}

func (c *Client) OnEvent(h func(e any)) {
	c.onEvent = h
}

func (c *Client) OnError(h func(e *events.ErrorEvent)) {
	c.onError = h
}

func (c *Client) OnToolCall(h func(name string, args map[string]any) (any, error)) {
	c.onToolCall = h
}

// Send sends any kind of event to the websocket
func (c *Client) Send(evt any) error {
	data, err := json.Marshal(evt)
	if err != nil {
		return err
	}

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
	if err := c.config.validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

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

					toolChoice := tool.ChoiceNone
					if len(c.config.tools) > 0 {
						toolChoice = tool.ChoiceAuto
					}

					initialized <- c.SessionUpdate(events.SessionUpdate{
						Voice:             c.config.voice,
						InputAudioFormat:  events.AudioFormatPCM16,
						OutputAudioFormat: events.AudioFormatPCM16,
						Temperature:       c.config.temperature,
						Speed:             c.config.speed,
						Instructions:      c.config.instruction,
						Modalities:        []string{"text", "audio"},
						ToolChoice:        toolChoice,
						Tools:             c.config.tools,
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
			case "response.done":
				evt, err := events.Parse[events.ResponseDoneEvent](data)
				if err != nil {
					slog.Error("failed to parse response done event", slog.Any("err", err))
				}

				if c.onToolCall != nil {
					for _, o := range evt.Response.Output {
						if o.Type == "function_call" && o.Status == "completed" {
							var args map[string]any
							if err := json.Unmarshal([]byte(o.Arguments), &args); err != nil {
								return err
							}

							res, err := c.onToolCall(o.Name, args)
							c.logger.Debug("tool call", slog.Any("name", o.Name), slog.Any("args", args), slog.Any("res", res), slog.Any("err", err))
							// TODO: add to conversation

							var toolOutput = func() string {
								if err != nil {
									d, _ := json.Marshal(map[string]any{
										"error": err.Error(),
									})
									return string(d)
								} else if res != nil {
									d, _ := json.Marshal(res)
									return string(d)
								} else {
									d, _ := json.Marshal(map[string]any{
										"success": true,
									})
									return string(d)
								}
							}()

							_ = c.Send(events.ConversationItemCreateEvent{
								BaseEvent: events.NewBaseEvent("conversation.item.create"),
								Item: events.ConversationItem{
									ID:     o.CallID,
									Type:   "function_call_output",
									CallID: o.CallID,
									Output: toolOutput,
								},
							})
							_ = c.CreateResponse()
						}
					}
				}

				// dispatch
				dispatchEvent[events.ResponseDoneEvent](c.onEvent, data)

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
				} else {
					data, err := base64.StdEncoding.DecodeString(evt.Delta)
					if err != nil {
						slog.Error("failed to decode base64 data", slog.Any("err", err))
					}
					if _, err = c.io.agentBuffer.Write(data); err != nil {
						c.logger.Error("failed to write to audio read buffer", slog.Any("err", err))
					}
				}

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

				c.io.ClearOutputBuffer()

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
		cs := getChunkSize(24_000, c.config.latency(), 2, 1)
		buf := make([]byte, cs)

		for {
			n, err := c.io.agentInputReader.Read(buf)
			if err != nil {
				if err == io.EOF {
					return
				}
				c.logger.Error("failed to read from agent audio buffer", slog.Any("err", err))
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
		config: config,
		logger: slog.Default(),
		update: make(chan struct{}, 1),
		io:     NewAudioIO(config.sampleRate, time.Duration(config.latencyMS)*time.Millisecond),
	}
}

type InterruptEvent struct {
}

func newChunkReader(r io.Reader, sampleRate int, latency time.Duration) io.Reader {
	return NewFixedAudioChunkReader(r, sampleRate, latency, 2, 1)
}
