package websocket

import (
	"context"
	"github.com/stretchr/testify/require"
	"log/slog"
	"testing"
	"time"
)

func TestClient(t *testing.T) {
	slog.SetLogLoggerLevel(slog.LevelDebug)
	slog.Debug("test")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	client, err := Connect(ctx, ClientConfig{
		URL: "wss://babelforce-rtvbp-proxy.fly.dev",
		//URL:         "ws://127.0.0.1:8080/",
		DialTimeout: time.Second,
		OnText: Json(func(x map[string]any) error {
			slog.Debug("text received", slog.Any("data", x))
			return nil
		}),
	})
	require.NoError(t, err)
	require.NotNil(t, client)

	<-ctx.Done()
}
