package openairt

import (
	"github.com/codewandler/openairt-go/tool"
	"log/slog"
	"os"
)

type clientConfig struct {
	model       string
	apiKey      string
	instruction string
	language    string
	voice       string
	temperature float64
	speed       float64
	logger      *slog.Logger
	tools       []tool.Tool
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

func WithEnvKey(envName string) ClientOption {
	return func(o *clientConfig) {
		o.apiKey = os.Getenv(envName)
		if o.apiKey == "" {
			panic("missing environment variable: " + envName)
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
		WithSpeed(1.3),
		WithModel("gpt-4o-realtime-preview-2025-06-03"),
		WithEnvKey("OPENAI_API_KEY"),
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
