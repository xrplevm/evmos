package server

import (
	"context"

	"github.com/cometbft/cometbft/libs/log"
	"golang.org/x/exp/slog"

	ethlog "github.com/ethereum/go-ethereum/log"
)

var _ slog.Handler = &CustomHandler{}

// slog custom handler to handle logger levels with server logger.
// It implements the slog.Handler interface
type CustomHandler struct {
	serverLogger log.Logger
}

// Handles the record to log the message with the server logger.
// It keeps the logic of the previous logger found in json_rpc.go.
func (h *CustomHandler) Handle(ctx context.Context, r slog.Record) error {
	switch r.Level {
		case slog.Level(ethlog.LevelTrace), slog.Level(ethlog.LevelDebug):
			h.serverLogger.Debug(r.Message, ctx)
		case slog.Level(ethlog.LevelInfo), slog.Level(ethlog.LevelWarn):
			h.serverLogger.Info(r.Message, ctx)
		case slog.Level(ethlog.LevelError), slog.Level(ethlog.LevelCrit):
			h.serverLogger.Error(r.Message, ctx)
	}
	return nil	
}

func (h *CustomHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return ethlog.DiscardHandler().Enabled(ctx, level)
}

func (h *CustomHandler) WithAttrs(attr []slog.Attr) slog.Handler {
	return ethlog.DiscardHandler().WithAttrs(attr)
}

func (h *CustomHandler) WithGroup(name string) slog.Handler {
	return ethlog.DiscardHandler().WithGroup(name)
}