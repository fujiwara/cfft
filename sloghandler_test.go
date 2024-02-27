package cfft_test

import (
	"log/slog"
	"os"
	"testing"

	"github.com/fujiwara/cfft"
)

func TestSlogHandler(t *testing.T) {
	opt := &cfft.HandlerOptions{}
	opt.Level = slog.LevelDebug
	opt.Color = true
	h := slog.New(cfft.NewLogHandler(os.Stderr, opt))
	h.Debug("debug")
	h.Info("info")
	h.Warn("warn")
	h.Error("error")
}
