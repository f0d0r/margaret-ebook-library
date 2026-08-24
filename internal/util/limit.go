package util

import (
	"io"

	"github.com/f0d0r/margaret-ebook-library/book"
)

// LimitReader returns a reader that yields at most max bytes from r and
// reports [ErrLimitExceeded] if r holds more data. Unlike io.LimitReader it
// does not silently truncate: a stream that exceeds the limit surfaces an
// error rather than a clean EOF.
func LimitReader(r io.Reader, max int64) io.Reader {
	return &limitReader{r: r, remaining: max}
}

type limitReader struct {
	r         io.Reader
	remaining int64
}

func (l *limitReader) Read(p []byte) (int, error) {
	if l.remaining <= 0 {
		var probe [1]byte
		if n, _ := l.r.Read(probe[:]); n > 0 {
			return 0, book.ErrLimitExceeded
		}
		return 0, io.EOF
	}
	if int64(len(p)) > l.remaining {
		p = p[:l.remaining]
	}
	n, err := l.r.Read(p)
	l.remaining -= int64(n)
	return n, err
}
