package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/codewandler/openairt-go"
	"github.com/codewandler/openairt-go/events"
	"github.com/codewandler/openairt-go/tool"
	"github.com/gordonklaus/portaudio"
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
		srMic       = 24_000
		srSpeaker   = 24_000
		instruction = "You are a helpcenter agent and help the user."
	)

	flag.StringVar(&instruction, "instruction", instruction, "instruction to send to the agent.")
	flag.IntVar(&srMic, "mic-sample-rate", srMic, "microphone sample rate")
	flag.IntVar(&srSpeaker, "speaker-sample-rate", srMic, "speaker sample rate")
	flag.BoolVar(&phone, "phone", false, "enabled 8khz audio emulation.")
	flag.BoolVar(&debug, "debug", false, "enable debug logs")
	flag.Parse()

	if phone {
		srMic = 8_000
		srSpeaker = 8_000
	}

	slog.SetLogLoggerLevel(slog.LevelError)
	if debug {
		slog.SetLogLoggerLevel(slog.LevelDebug)
	}

	// audio
	must(portaudio.Initialize())
	defer portaudio.Terminate()

	// emulate 8khz
	audioIO, err := NewAudioIO(srSpeaker, srMic)
	if err != nil {
		panic(err)
	}

	// openAI client
	client := openairt.New(
		openairt.WithDefaultLogger(),
		openairt.WithInstruction(instruction),
		openairt.WithTools(
			tool.Tool{
				Type:        "function",
				Description: "End the conversation",
				Name:        "conversation_end",
				Parameters: tool.Parameters{
					Type:       "object",
					Properties: make(tool.Properties),
					Required:   []string{},
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
			//println("-- start --")
			audioIO.ClearOutputBuffer()
		}
	})

	err = client.Open(ctx)
	if err != nil {
		panic(err)
	}

	cr, cw := client.Audio(srSpeaker, srMic)

	//must(client.UserInput("Hi, my name is timo. Can you ", true))
	must(client.CreateResponse())

	go func() {

		buf := make([]byte, 640)
		for {
			n, err := cr.Read(buf)
			if err != nil {
				if err.Error() == "reset called" {
					<-time.After(100 * time.Millisecond)
					continue
				}
				panic(err)
			}

			_, err = audioIO.Write(buf[:n])
			if err != nil {
				panic(err)
			}
		}
	}()

	// send mic input -> openAI
	go func() {

		buf := make([]byte, 1024)
		for {
			n, err := audioIO.Read(buf)
			if err != nil {
				panic(err)
			}

			_, err = cw.Write(buf[:n])
			if err != nil {
				panic(err)
			}
		}
	}()

	<-ctx.Done()
}
