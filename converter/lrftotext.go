package converter

import (
	"bufio"
	"bytes"
	"context"
	"io"
)

type lrfToTextTransformer struct {
	from string
	to   string
}

func (t *lrfToTextTransformer) From() string { return t.from }
func (t *lrfToTextTransformer) To() string   { return t.to }

// Transform expects UTF-8 input: the book layer decodes the resource's
// utf-16le payload centrally before the transformer chain runs. Plots
// (inline images) arrive as U+FFFC markers and are dropped.
func (t *lrfToTextTransformer) Transform(ctx context.Context, r io.Reader) (io.ReadCloser, error) {
	if r == nil {
		return io.NopCloser(bytes.NewReader(nil)), nil
	}
	var closer io.Closer
	if c, ok := r.(io.Closer); ok {
		closer = c
	}
	return &lrfTextReader{
		ctx:    ctx,
		src:    bufio.NewReader(r),
		closer: closer,
	}, nil
}

type lrfTextReader struct {
	ctx    context.Context
	src    *bufio.Reader
	closer io.Closer
	out    []byte
	state  uint8
	err    error
}

func (l *lrfTextReader) Read(b []byte) (int, error) {
	if l.err != nil {
		return 0, l.err
	}
	for len(l.out) == 0 {
		if l.ctx != nil {
			select {
			case <-l.ctx.Done():
				l.err = l.ctx.Err()
				return 0, l.err
			default:
			}
		}
		if !l.scan() {
			if len(l.out) > 0 {
				break
			}
			l.err = io.EOF
			return 0, l.err
		}
	}
	n := copy(b, l.out)
	l.out = l.out[n:]
	return n, nil
}

func (l *lrfTextReader) Close() error {
	if l.closer != nil {
		return l.closer.Close()
	}
	return nil
}

// scan filters the UTF-8 encoding of U+FFFC (EF BF BC). The states hold a
// partial prefix so a marker split across reads is still recognized; other
// bytes pass through unchanged.
func (l *lrfTextReader) scan() bool {
	c, err := l.src.ReadByte()
	if err != nil {
		switch l.state {
		case 1:
			l.out = append(l.out, 0xEF)
		case 2:
			l.out = append(l.out, 0xEF, 0xBF)
		}
		l.state = 0
		return false
	}
	switch l.state {
	case 1:
		if c == 0xBF {
			l.state = 2
			return true
		}
		l.out = append(l.out, 0xEF)
		l.state = 0
		return l.appendByte(c)
	case 2:
		if c == 0xBC {
			l.state = 0
			return true
		}
		l.out = append(l.out, 0xEF, 0xBF)
		l.state = 0
		return l.appendByte(c)
	default:
		return l.appendByte(c)
	}
}

func (l *lrfTextReader) appendByte(c byte) bool {
	if c == 0xEF {
		l.state = 1
		return true
	}
	l.out = append(l.out, c)
	return true
}
