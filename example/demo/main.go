package main

import (
	"context"
	"fmt"
	"github.com/codewandler/openairt-go"
	"github.com/codewandler/openairt-go/events"
	"github.com/gordonklaus/portaudio"
	"log/slog"
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

	slog.SetLogLoggerLevel(slog.LevelDebug)

	// audio
	must(portaudio.Initialize())
	defer portaudio.Terminate()
	audioIO, err := NewAudioIO(24_000)
	if err != nil {
		panic(err)
	}

	// openAI client
	client := openairt.NewDefault()
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
			slog.Info("session updated", slog.Any("session", x.Session))
		case *events.SessionCreatedEvent:
			slog.Info("session created", slog.Any("session", x.Session.ID))
		case *events.SpeechStartedEvent:
			println("-- start --")
		default:
			fmt.Printf("%+v\n", x)

		}
	})

	err = client.Open(ctx)
	if err != nil {
		panic(err)
	}

	//must(client.UserInput("My name is timo. what is my name?", true))

	go func() {
		buf := make([]byte, 640)
		for {
			n, err := client.Read(buf)
			if err != nil {
				if err.Error() == "reset called" {
					<-time.After(100 * time.Millisecond)
					continue
				}
				panic(err)
			}

			//println(">", n)

			_, err = audioIO.Write(buf[:n])
			if err != nil {
				panic(err)
			}
		}
	}()

	// send mic input -> openAI
	go func() {
		buf := make([]byte, 10*1024)
		for {
			n, err := audioIO.Read(buf)
			if err != nil {
				panic(err)
			}

			_, err = client.Write(buf[:n])
			if err != nil {
				panic(err)
			}
		}
	}()

	<-ctx.Done()
}
