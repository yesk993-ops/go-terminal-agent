package logger

import (
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
)

var (
	defaultLogger *slog.Logger
	initOnce      sync.Once
)

func Init(level, format, output string) {
	var w io.Writer = os.Stderr
	switch output {
	case "stdout":
		w = os.Stdout
	case "stderr", "":
		w = os.Stderr
	default:
		f, err := os.OpenFile(output, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err == nil {
			w = f
		}
	}

	var lvl slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lvl = slog.LevelDebug
	case "info":
		lvl = slog.LevelInfo
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: lvl}

	var h slog.Handler
	switch strings.ToLower(format) {
	case "json":
		h = slog.NewJSONHandler(w, opts)
	default:
		h = slog.NewTextHandler(w, opts)
	}

	defaultLogger = slog.New(h)
	slog.SetDefault(defaultLogger)
}

func L() *slog.Logger {
	initOnce.Do(func() {
		if defaultLogger == nil {
			Init("info", "text", "stderr")
		}
	})
	return defaultLogger
}
