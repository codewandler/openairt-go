package events

import "fmt"

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

type ResponseCreateEvent struct {
	BaseEvent
	Response ResponseCreatePayload `json:"response"`
}

type ResponseCreatePayload struct {
	Modalities        []string    `json:"modalities,omitempty"`
	Instructions      string      `json:"instructions,omitempty"`
	Voice             string      `json:"voice,omitempty"`
	OutputAudioFormat AudioFormat `json:"output_audio_format,omitempty"`
	Tools             []Tool      `json:"tools,omitempty"`
	ToolChoice        string      `json:"tool_choice,omitempty"`
	Temperature       float64     `json:"temperature,omitempty"`
	MaxOutputTokens   int         `json:"max_output_tokens,omitempty"`
}

type Tool struct {
	Type        string     `json:"type"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Parameters  Parameters `json:"parameters"`
}

type Parameters struct {
	Type       string     `json:"type"`
	Properties Properties `json:"properties"`
	Required   []string   `json:"required"`
}

type Properties map[string]Schema

type Schema struct {
	Type string `json:"type"`
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
