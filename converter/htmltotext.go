package converter

import (
	"bytes"
	"context"
	"io"

	"golang.org/x/net/html"
)

// htmlToTextTransformer converts HTML/XHTML/MobiHTML to plain text by
// streaming text tokens. Each instance represents a single From->To edge.
type htmlToTextTransformer struct {
	from string
	to   string
}

func (h *htmlToTextTransformer) From() string { return h.from }
func (h *htmlToTextTransformer) To() string   { return h.to }

func (h *htmlToTextTransformer) Transform(ctx context.Context, r io.Reader) (io.ReadCloser, error) {
	if r == nil {
		return io.NopCloser(bytes.NewReader(nil)), nil
	}
	var closer io.Closer
	if c, ok := r.(io.Closer); ok {
		closer = c
	}
	return &htmlTextReader{
		ctx:       ctx,
		tokenizer: html.NewTokenizer(r),
		closer:    closer,
	}, nil
}

// htmlTextReader implements io.ReadCloser by pulling TextTokens from an
// html.Tokenizer and copying them into the caller's buffer.
type htmlTextReader struct {
	ctx       context.Context
	tokenizer *html.Tokenizer
	buf       []byte // unconsumed tail of last text token
	closer    io.Closer
	err       error // sticky error (EOF or tokenizer error)
}

func (h *htmlTextReader) Read(p []byte) (int, error) {
	if h.err != nil {
		return 0, h.err
	}
	if h.ctx != nil {
		select {
		case <-h.ctx.Done():
			h.err = h.ctx.Err()
			return 0, h.err
		default:
		}
	}

	// Drain buffered tail first.
	if len(h.buf) > 0 {
		n := copy(p, h.buf)
		h.buf = h.buf[n:]
		if len(h.buf) == 0 {
			h.buf = nil
		}
		if n > 0 {
			return n, nil
		}
	}

	// Fill p by consuming text tokens until p is full or EOF.
	total := 0
	for total < len(p) {
		if h.ctx != nil {
			select {
			case <-h.ctx.Done():
				h.err = h.ctx.Err()
				if total == 0 {
					return 0, h.err
				}
				return total, nil
			default:
			}
		}
		tt := h.tokenizer.Next()
		if tt == html.ErrorToken {
			err := h.tokenizer.Err()
			if err == io.EOF {
				h.err = io.EOF
			} else {
				h.err = err
			}
			if total == 0 {
				return 0, h.err
			}
			return total, nil
		}
		if tt != html.TextToken {
			continue
		}
		text := h.tokenizer.Text()
		if len(text) == 0 {
			continue
		}
		// Copy as much as fits into p[total:].
		n := copy(p[total:], text)
		total += n
		if n < len(text) {
			// Save remainder for next Read.
			h.buf = append(h.buf[:0], text[n:]...)
			break
		}
	}
	if total == 0 && h.err != nil {
		return 0, h.err
	}
	return total, nil
}

func (h *htmlTextReader) Close() error {
	if h.closer != nil {
		return h.closer.Close()
	}
	return nil
}
