package events

import "github.com/codewandler/openairt-go/tool"

type Session struct {
	ID                       string         `json:"id,omitempty"`
	Object                   string         `json:"object,omitempty"`
	ExpiresAt                int64          `json:"expires_at,omitempty"`
	InputAudioNoiseReduction *string        `json:"input_audio_noise_reduction,omitempty"`
	TurnDetection            *TurnDetection `json:"turn_detection,omitempty"`
	InputAudioFormat         string         `json:"input_audio_format,omitempty"`
	InputAudioTranscription  *string        `json:"input_audio_transcription,omitempty"`
	ClientSecret             *string        `json:"client_secret,omitempty"`
	Include                  *[]string      `json:"include,omitempty"`
	Model                    string         `json:"model,omitempty"`
	Modalities               []string       `json:"modalities,omitempty"`
	Instructions             string         `json:"instructions,omitempty"`
	Voice                    string         `json:"voice,omitempty"`
	OutputAudioFormat        string         `json:"output_audio_format,omitempty"`
	ToolChoice               string         `json:"tool_choice,omitempty"`
	Temperature              float64        `json:"temperature,omitempty"`
	MaxResponseOutputTokens  string         `json:"max_response_output_tokens,omitempty"`
	Speed                    float64        `json:"speed,omitempty"`
	Tracing                  *string        `json:"tracing,omitempty"`
	Tools                    *[]interface{} `json:"tools,omitempty"`
}

type SessionUpdate struct {
	TurnDetection           *TurnDetection `json:"turn_detection,omitempty"`
	InputAudioFormat        AudioFormat    `json:"input_audio_format,omitempty"`
	Model                   string         `json:"model,omitempty"`
	Modalities              []string       `json:"modalities,omitempty"`
	Instructions            string         `json:"instructions,omitempty"`
	Voice                   string         `json:"voice,omitempty"`
	OutputAudioFormat       AudioFormat    `json:"output_audio_format,omitempty"`
	Temperature             float64        `json:"temperature,omitempty"`
	MaxResponseOutputTokens string         `json:"max_response_output_tokens,omitempty"`
	Speed                   float64        `json:"speed,omitempty"`
	Tracing                 *string        `json:"tracing,omitempty"`
	Tools                   []tool.Tool    `json:"tools,omitempty"`
	ToolChoice              tool.Choice    `json:"tool_choice,omitempty"`
}

// TurnDetection holds the VAD configuration.
type TurnDetection struct {
	Type              string  `json:"type,omitempty"`
	Threshold         float64 `json:"threshold,omitempty"`
	PrefixPaddingMs   int     `json:"prefix_padding_ms,omitempty"`
	SilenceDurationMs int     `json:"silence_duration_ms,omitempty"`
	CreateResponse    bool    `json:"create_response,omitempty"`
	InterruptResponse bool    `json:"interrupt_response,omitempty"`
}
