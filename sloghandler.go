package cfft

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
)

type logHandler struct {
	opts         *slog.HandlerOptions
	preformatted []byte
	mu           *sync.Mutex
	w            io.Writer
}

func NewLogHandler(w io.Writer, opts *slog.HandlerOptions) slog.Handler {
	return &logHandler{
		opts: opts,
		mu:   new(sync.Mutex),
		w:    w,
	}
}

func (h *logHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= h.opts.Level.Level()
}

func (h *logHandler) Handle(ctx context.Context, record slog.Record) error {
	buf := bytes.NewBuffer(nil)
	fmt.Fprint(buf, record.Time.Format("2006-01-02T15:04:05.000Z"))
	fmt.Fprintf(buf, " [%s]", strings.ToLower(record.Level.String()))
	if len(h.preformatted) > 0 {
		buf.Write(h.preformatted)
	}
	record.Attrs(func(a slog.Attr) bool {
		fmt.Fprintf(buf, " [%s:%v]", a.Key, a.Value)
		return true
	})
	fmt.Fprintf(buf, " %s\n", record.Message)
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
