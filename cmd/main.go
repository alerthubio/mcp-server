package main

import (
	"context"
	"log/slog"
	"github.com/alerthubio/mcp-server/internal/mcp"
	"os"
)


var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	opts := &slog.HandlerOptions{
		AddSource: true,
		Level:     slog.LevelError,
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, opts))
	ctx := context.Background()

	if err := mcp.Run(ctx, logger, version); err != nil {
		logger.ErrorContext(ctx, "mcp server terminated", slog.String("error", err.Error()))
		os.Exit(1)
	}
}
