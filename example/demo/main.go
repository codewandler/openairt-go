package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/codewandler/audio-go"
	"github.com/codewandler/openairt-go"
	"github.com/codewandler/openairt-go/events"
	"github.com/codewandler/openairt-go/tool"
	"github.com/gordonklaus/portaudio"
	"log"
	"log/slog"
	"os"
	"time"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var (
		phone       = false
		debug       = false
		sr          = 24_000
		instruction = "You are a help-center agent and help the user. You speak english language."
	)

	flag.StringVar(&instruction, "instruction", instruction, "instruction to send to the agent.")
	flag.IntVar(&sr, "sample-rate", sr, "sample rate")
	flag.BoolVar(&phone, "phone", false, "enabled 8khz audio emulation.")
	flag.BoolVar(&debug, "debug", false, "enable debug logs")
	flag.Parse()

	if phone {
		sr = 8_000
	}

	slog.SetLogLoggerLevel(slog.LevelError)
	if debug {
		slog.SetLogLoggerLevel(slog.LevelDebug)
	}

	// audio
	must(portaudio.Initialize())
	defer portaudio.Terminate()

	// emulate 8khz
	audioDevice, err := audio.NewDevice(sr, 1)
	if err != nil {
		log.Panicf("failed to create audio device: %s", err)
	}

	// openAI client
	client := openairt.New(
		openairt.WithDefaultLogger(),
		openairt.WithInstruction(instruction),
		openairt.WithTools(
			tool.Tool{
				Type:        "function",
				Description: "Use to end the conversation for various reasons. User may have asked for it. You see a dead end or you think the case is closed. If you think you should end the conversation, ask the user if its okay, then before you end, say good bye and only after the user confirmed with good bye end it.",
				Name:        "conversation_end",
				Parameters: tool.Parameters{
					Type: "object",
					Properties: tool.Properties{
						"summary": {
							Type:        "string",
							Description: `Concise summary of the conversation`,
						},
						"reason": {
							Type:        "string",
							Description: "The reason for ending the conversation. If you don't specify a reason, the default reason is 'user'.",
						},
					},
					Required: []string{
						"summary",
						"reason",
					},
				},
			},
			tool.Tool{
				Type:        "function",
				Description: "Get current time",
				Name:        "get_time",
				Parameters: tool.Parameters{
					Type:       "object",
					Properties: tool.Properties{},
					Required:   []string{},
				},
			},
		),
	)
	client.OnToolCall(func(name string, args map[string]any) (any, error) {
		switch name {
		case "get_time":
			return time.Now().Format(time.RFC3339), nil
		case "conversation_end":
			fmt.Printf("agent> end conversation: %s", args["reason"])
			fmt.Printf("summary>\n%s", args["summary"])
			os.Exit(0)
			return "OK", nil
		}

		return nil, fmt.Errorf("unknown tool: %s", name)
	})
	client.OnError(func(e *events.ErrorEvent) {
		slog.Error("error", slog.Any("error", e))
	})
	client.OnEvent(func(e any) {
		switch x := e.(type) {
		case *events.ResponseAudioTranscriptDeltaEvent:
			print(".")
		case *events.ResponseAudioTranscriptDoneEvent:
			println("agent>", x.Transcript)
		case *events.ResponseAudioDone:
			println("")
		case *events.SessionUpdateEvent:
			//slog.Info("session updated", slog.Any("session", x.Session))
		case *events.SessionCreatedEvent:
			//slog.Info("session created", slog.Any("session", x.Session.ID))
		case *events.ResponseDoneEvent:
			//slog.Info("response done", slog.Any("response", x.Response))
		case *events.SpeechStartedEvent:
			println("agent> reset buffer")
			//audioDevice.ClearOutputBuffer()
		}
	})

	err = client.Open(ctx)
	if err != nil {
		panic(err)
	}

	audioUser := client.Audio()

	//must(client.UserInput("Hi, my name is timo. Can you ", true))
	must(client.CreateResponse())

	//latency := time.Duration(latencyMs) * time.Millisecond

	// stream audio from openAI to device
	go func() {
		bufferSize := 3200
		buf := make([]byte, bufferSize)
		for {
			n, err := audioUser.Read(buf)
			if err != nil {
				if err.Error() == "reset called" {
					<-time.After(100 * time.Millisecond)
					println("-- reset --")
					continue
				}
				panic(err)
			}

			data := buf[:n]
			resampled, err := openairt.ResamplePCM(data, 24_000, sr)
			if err != nil {
				panic(err)
			}

			n, err = audioDevice.Write(resampled)
			if err != nil {
				panic(err)
			}
		}
	}()

	// send mic input -> openAI
	go func() {
		bufferSize := 3200
		buf := make([]byte, bufferSize)
		for {
			n, err := audioDevice.Read(buf)
			if err != nil {
				panic(err)
			}

			data := buf[:n]
			resampled, err := openairt.ResamplePCM(data, sr, 24_000)
			if err != nil {
				panic(err)
			}

			_, err = audioUser.Write(resampled)
			if err != nil {
				panic(err)
			}
		}
	}()

	<-ctx.Done()
}
