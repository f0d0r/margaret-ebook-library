package converter

import (
	"context"
	"io"
)

// Transformer converts a byte stream from one MIME type to another.
type Transformer interface {
	From() string // MIME, e.g. "application/xhtml+xml"
	To() string   // MIME, e.g. "text/plain"
	Transform(ctx context.Context, r io.Reader) (io.ReadCloser, error)
}
