package openairt

import (
	"fmt"
	"github.com/codewandler/openairt-go/tool"
	"log/slog"
	"os"
	"time"
)

const (
	ApiKeyEnvVarNameShort = "OPENAI_KEY"
	ApiKeyEnvVarNameLong  = "OPENAI_API_KEY"
)

type clientConfig struct {
	model       string
	apiKey      string
	instruction string
	language    string
	voice       string
	temperature float64
	speed       float64
	sampleRate  int
	latencyMS   int
	logger      *slog.Logger
	tools       []tool.Tool
}

func (c *clientConfig) latency() time.Duration {
	return time.Duration(c.latencyMS) * time.Millisecond
}

func (c *clientConfig) validate() error {
	if c.apiKey == "" {
		return fmt.Errorf("missing api key")
	}
	return nil
}

type ClientOption func(*clientConfig)

func WithTools(tools ...tool.Tool) ClientOption {
	return func(config *clientConfig) {
		config.tools = tools
	}
}

func WithVoice(voice string) ClientOption {
	return func(config *clientConfig) {
		config.voice = voice
	}
}

func WithSpeed(speed float64) ClientOption {
	return func(config *clientConfig) {
		config.speed = speed
	}
}

func WithSampleRate(sr int) ClientOption {
	return func(config *clientConfig) {
		config.sampleRate = sr
	}
}

func WithLogger(logger *slog.Logger) ClientOption {
	return func(o *clientConfig) {
		o.logger = logger
	}
}

func WithDefaultLogger() ClientOption {
	return WithLogger(slog.Default())
}

func WithTemperature(temperature float64) ClientOption {
	return func(o *clientConfig) {
		o.temperature = temperature
	}
}

func WithModel(model string) ClientOption {
	return func(o *clientConfig) {
		o.model = model
	}
}

func WithKey(apiKey string) ClientOption {
	return func(o *clientConfig) {
		o.apiKey = apiKey
	}
}

func WithEnvKey(vars ...string) ClientOption {
	return func(o *clientConfig) {
		for _, envVarName := range vars {
			if k := os.Getenv(envVarName); k != "" {
				o.apiKey = k
				return
			}
		}
	}
}

func WithOptions(opts ...ClientOption) ClientOption {
	return func(o *clientConfig) {
		for _, opt := range opts {
			opt(o)
		}
	}
}

func withDefaults() ClientOption {
	return WithOptions(
		WithLogger(slog.New(slog.DiscardHandler)),
		WithLanguage("en"),
		WithVoice("coral"),
		WithInstruction("You are a helpcenter agent and help the user."),
		WithTemperature(0.7),
		WithSampleRate(24_000),
		WithLatency(200),
		WithSpeed(1.3),
		WithModel("gpt-4o-realtime-preview-2025-06-03"),
		WithEnvKey(ApiKeyEnvVarNameShort, ApiKeyEnvVarNameLong),
	)
}

func WithLanguage(language string) ClientOption {
	return func(o *clientConfig) {
		o.language = language
	}
}

func WithInstruction(instruction string) ClientOption {
	return func(o *clientConfig) {
		o.instruction = instruction
	}
}

// WithLatency sets the latency in milliseconds.
func WithLatency(latencyMS int) ClientOption {
	return func(o *clientConfig) {
		o.latencyMS = latencyMS
	}
}
