package openairt

import (
	"log/slog"
	"os"
)

type clientConfig struct {
	model       string
	apiKey      string
	instruction string
	language    string
	temperature float64
	logger      *slog.Logger
}

type ClientOption func(*clientConfig)

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
		WithInstruction("You are a helpcenter agent and help the user."),
		WithTemperature(0.7),
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
