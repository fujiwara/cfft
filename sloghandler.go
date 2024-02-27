package cfft

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
)

type HandlerOptions struct {
	slog.HandlerOptions
	Color bool
}
type logHandler struct {
	opts         *HandlerOptions
	preformatted []byte
	mu           *sync.Mutex
	w            io.Writer
}

func NewLogHandler(w io.Writer, opts *HandlerOptions) slog.Handler {
	return &logHandler{
		opts: opts,
		mu:   new(sync.Mutex),
		w:    w,
	}
}

func (h *logHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= h.opts.Level.Level()
}

func (h *logHandler) FprintfFunc(level slog.Level) func(io.Writer, string, ...interface{}) {
	if h.opts.Color {
		switch level {
		case slog.LevelWarn:
			return color.New(color.FgYellow).FprintfFunc()
		case slog.LevelError:
			return color.New(color.FgRed).FprintfFunc()
		}
	}
	return defaultFprintfFunc
}

var defaultFprintfFunc = func(w io.Writer, format string, args ...interface{}) {
	fmt.Fprintf(w, format, args...)
}

func (h *logHandler) Handle(ctx context.Context, record slog.Record) error {
	buf := new(bytes.Buffer)
	fprintf := h.FprintfFunc(record.Level)
	fprintf(buf, "%s", record.Time.Format(time.RFC3339))
	fprintf(buf, " [%s]", strings.ToLower(record.Level.String()))
	if len(h.preformatted) > 0 {
		buf.Write(h.preformatted)
	}
	record.Attrs(func(a slog.Attr) bool {
		fprintf(buf, " [%s:%v]", a.Key, a.Value)
		return true
	})
	fprintf(buf, " %s\n", record.Message)
	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := h.w.Write(buf.Bytes())
	return err
}

func (h *logHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	preformatted := []byte{}
	for _, a := range attrs {
		preformatted = append(preformatted, fmt.Sprintf(" [%s:%v]", a.Key, a.Value)...)
	}
	return &logHandler{
		opts:         h.opts,
		preformatted: preformatted,
		mu:           h.mu,
		w:            h.w,
	}
}

func (h *logHandler) WithGroup(group string) slog.Handler {
	return h
}
