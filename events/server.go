package events

import (
	"fmt"
	"github.com/codewandler/openairt-go/tool"
)

type AudioFormat string

const (
	AudioFormatPCM16 AudioFormat = "pcm16"
)

type ErrorEvent struct {
	BaseEvent
	ErrorDetail ErrorDetail `json:"error"`
}

func (e *ErrorEvent) Error() string {
	return e.ErrorDetail.Error()
}

// ErrorDetail holds the details of the error.
type ErrorDetail struct {
	Type    string `json:"type"`
	Code    string `json:"code"`
	Message string `json:"message"`
	Param   string `json:"param"`
	EventID string `json:"event_id"`
}

func (e *ErrorDetail) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

type SessionCreatedEvent struct {
	BaseEvent
	Session Session `json:"session"`
}

// ResponseCreateEvent creates an out-of-band response
// see: https://platform.openai.com/docs/guides/realtime-conversations#create-responses-outside-the-default-conversation
type ResponseCreateEvent struct {
	BaseEvent
	Response ResponseCreatePayload `json:"response"`
}

type ResponseCreatePayload struct {
	Modalities        []string       `json:"modalities,omitempty"`
	Instructions      string         `json:"instructions,omitempty"`
	Voice             string         `json:"voice,omitempty"`
	Conversation      string         `json:"conversation,omitempty"`
	MetaData          map[string]any `json:"metadata,omitempty"`
	OutputAudioFormat AudioFormat    `json:"output_audio_format,omitempty"`
	Tools             []tool.Tool    `json:"tools,omitempty"`
	ToolChoice        tool.Choice    `json:"tool_choice,omitempty"`
	Temperature       float64        `json:"temperature,omitempty"`
	MaxOutputTokens   int            `json:"max_output_tokens,omitempty"`
}

type SpeechStartedEvent struct {
	BaseEvent
}

type SpeechStoppedEvent struct {
	BaseEvent
}

type ResponseAudioDeltaEvent struct {
	BaseEvent
	ResponseId  string `json:"response_id"`
	OutputIndex int    `json:"output_index"`
	InputIndex  int    `json:"input_index"`
	ItemID      string `json:"item_id"`
	Delta       string `json:"delta"`
}

type ResponseAudioDone struct {
	BaseEvent
	ResponseId  string `json:"response_id"`
	OutputIndex int    `json:"output_index"`
	InputIndex  int    `json:"input_index"`
	ItemID      string `json:"item_id"`
}

type ResponseAudioTranscriptDeltaEvent struct {
	BaseEvent
	ResponseId  string `json:"response_id"`
	OutputIndex int    `json:"output_index"`
	InputIndex  int    `json:"input_index"`
	ItemID      string `json:"item_id"`
	Delta       string `json:"delta"`
}

type ResponseAudioTranscriptDoneEvent struct {
	BaseEvent
	ResponseId  string `json:"response_id"`
	OutputIndex int    `json:"output_index"`
	InputIndex  int    `json:"input_index"`
	ItemID      string `json:"item_id"`
	Transcript  string `json:"transcript"`
}

type ResponseDoneEvent struct {
	BaseEvent
	ResponseId string               `json:"response_id"`
	Response   ResponseDoneResponse `json:"response"`
}

type ResponseDoneResponse struct {
	Object   string               `json:"object"`
	ID       string               `json:"id"`
	Status   string               `json:"status"`
	Output   []ResponseDoneOutput `json:"output"`
	MetaData map[string]any       `json:"metadata"`
}

type ResponseDoneOutput struct {
	Object    string `json:"object"`
	ID        string `json:"id"`
	Type      string `json:"type"`
	Status    string `json:"status"`
	Name      string `json:"name"`
	CallID    string `json:"call_id"`
	Arguments string `json:"arguments"`
}
