package log

import (
	"context"
	"errors"
	"io"
	"log/slog"
)

type ContextKey string

// From Adam Woodbeck's Networking Programming with Go
// https://github.com/awoodbeck/gnp/blob/master/ch13/writer.go
type sustainedMultiWriter struct {
	writers []io.Writer
}

func (s *sustainedMultiWriter) Write(p []byte) (n int, err error) {
	for _, w := range s.writers {
		i, wErr := w.Write(p)
		n += i
		err = errors.Join(err, wErr)
	}

	return n, err
}

func SustainedMultiWriter(writers ...io.Writer) io.Writer {
	mw := &sustainedMultiWriter{writers: make([]io.Writer, 0, len(writers))}

	for _, w := range writers {
		if m, ok := w.(*sustainedMultiWriter); ok {
			mw.writers = append(mw.writers, m.writers...)
			continue
		}

		mw.writers = append(mw.writers, w)
	}

	return mw
}

type ContextHandler struct {
	slog.Handler
	Keys []ContextKey
}

func (h ContextHandler) Handle(ctx context.Context, r slog.Record) error {
	for _, k := range h.Keys {
		attr, ok := ctx.Value(k).(slog.Attr)
		if !ok {
			continue
		}
		r.AddAttrs(attr)
	}
	return h.Handler.Handle(ctx, r)
}
