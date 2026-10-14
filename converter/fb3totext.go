package converter

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"strings"

	stdhtml "html"

	"github.com/f0d0r/margaret-ebook-library/internal/util"
)

// fb3BodyToTextTransformer converts FB3 body parts to plain text
// with a tolerant scanner: local-name matching, transparent unknown
// elements, newlines on block boundaries.
type fb3BodyToTextTransformer struct {
	from string
	to   string
}

func (t *fb3BodyToTextTransformer) From() string { return t.from }
func (t *fb3BodyToTextTransformer) To() string   { return t.to }

func (t *fb3BodyToTextTransformer) Transform(ctx context.Context, r io.Reader) (io.ReadCloser, error) {
	if r == nil {
		return io.NopCloser(bytes.NewReader(nil)), nil
	}
	var closer io.Closer
	if c, ok := r.(io.Closer); ok {
		closer = c
	}
	return &fb3TextReader{
		ctx:         ctx,
		src:         bufio.NewReader(r),
		closer:      closer,
		atLineStart: true,
	}, nil
}

// fb3TextBlocks lists elements whose end starts a new output line.
var fb3TextBlocks = map[string]bool{
	"p": true, "subtitle": true, "title": true, "stanza": true,
	"v": true, "li": true, "th": true, "td": true,
}

type fb3TextReader struct {
	ctx         context.Context
	src         *bufio.Reader
	closer      io.Closer
	out         []byte // ready output
	cur         []byte // current line buffer
	breaks      int    // pending line breaks, materialized by following text
	emitted     bool   // any byte ever queued to out (leading breaks drop)
	atLineStart bool
	done        bool
	err         error // sticky error (io.EOF when done)
}

func (f *fb3TextReader) Read(p []byte) (int, error) {
	if f.err != nil {
		return 0, f.err
	}
	if f.ctx != nil {
		select {
		case <-f.ctx.Done():
			f.err = f.ctx.Err()
			return 0, f.err
		default:
		}
	}
	for len(f.out) == 0 && !f.done {
		if f.ctx != nil {
			select {
			case <-f.ctx.Done():
				f.err = f.ctx.Err()
				return 0, f.err
			default:
			}
		}
		f.scan()
	}
	if len(f.out) == 0 {
		f.err = io.EOF
		return 0, f.err
	}
	n := copy(p, f.out)
	f.out = f.out[n:]
	return n, nil
}

func (f *fb3TextReader) Close() error {
	if f.closer != nil {
		return f.closer.Close()
	}
	return nil
}

func (f *fb3TextReader) appendText(s string) {
	if s == "" {
		return
	}
	if f.atLineStart {
		s = strings.TrimLeft(s, " \t\r\n")
		if s == "" {
			return
		}
	}
	if f.breaks > 0 {
		if f.emitted {
			f.out = append(f.out, strings.Repeat("\n", f.breaks)...)
		}
		f.breaks = 0
	}
	f.cur = append(f.cur, s...)
	f.atLineStart = false
	if len(f.cur) > maxLineBuffer {
		f.out = append(f.out, f.cur...)
		f.emitted = true
		f.cur = f.cur[:0]
	}
}

func (f *fb3TextReader) breakLine() {
	if len(f.cur) > 0 {
		f.out = append(f.out, f.cur...)
		f.emitted = true
		f.cur = f.cur[:0]
	}
	f.breaks++
	f.atLineStart = true
}

func (f *fb3TextReader) finish() {
	if len(f.cur) > 0 {
		f.out = append(f.out, f.cur...)
		f.cur = f.cur[:0]
	}
	f.done = true
}

func (f *fb3TextReader) scan() {
	var text strings.Builder
	for {
		c, err := f.src.ReadByte()
		if err != nil {
			if text.Len() > 0 {
				f.appendText(text.String())
			}
			f.finish()
			return
		}
		switch c {
		case '<':
			if text.Len() > 0 {
				f.appendText(text.String())
				text.Reset()
			}
			// The '<' is already consumed: the tag must be parsed even
			// when output is ready, otherwise its bytes leak as text.
			f.parseTag()
			if len(f.out) > 0 {
				return
			}
		case '&':
			if text.Len() > 0 {
				f.appendText(text.String())
				text.Reset()
			}
			f.parseEntity()
			if len(f.out) > 0 {
				return
			}
		default:
			text.WriteByte(c)
		}
	}
}

func (f *fb3TextReader) parseEntity() {
	var name strings.Builder
	for i := 0; i < 64; i++ {
		c, err := f.src.ReadByte()
		if err != nil {
			f.appendText("&" + name.String())
			f.finish()
			return
		}
		if c == ';' {
			f.appendText(stdhtml.UnescapeString("&" + name.String() + ";"))
			return
		}
		name.WriteByte(c)
	}
	f.appendText("&" + name.String())
}

func (f *fb3TextReader) parseTag() {
	c, err := f.src.ReadByte()
	if err != nil {
		f.appendText("<")
		f.finish()
		return
	}
	switch {
	case c == '/':
		f.parseEndTag()
	case c == '!' || c == '?':
		f.skipSpecial(c)
	case util.IsXMLNameChar(c):
		_ = f.src.UnreadByte()
		name, _ := f.parseStartTag()
		if localName(name) == "br" {
			f.breakLine()
		}
	default:
		_ = f.src.UnreadByte()
		f.appendText("<")
	}
}

func (f *fb3TextReader) parseEndTag() {
	name := f.readTagName()
	f.skipToTagEnd()
	if fb3TextBlocks[localName(name)] {
		f.breakLine()
	}
}

func (f *fb3TextReader) parseStartTag() (string, bool) {
	name := f.readTagName()
	for {
		c, err := f.src.ReadByte()
		if err != nil {
			return name, true
		}
		switch c {
		case '>':
			return name, false
		case '"', '\'':
			f.skipQuoted(c)
		case '/':
			d, err := f.src.ReadByte()
			if err != nil {
				return name, true
			}
			if d == '>' {
				return name, true
			}
			_ = f.src.UnreadByte()
		}
	}
}

func (f *fb3TextReader) skipSpecial(c byte) {
	if c == '?' {
		f.skipUntil("?>")
		return
	}
	d, err := f.src.ReadByte()
	if err != nil {
		return
	}
	if d == '[' {
		var opener [6]byte
		if _, err := io.ReadFull(f.src, opener[:]); err == nil && string(opener[:]) == "CDATA[" {
			f.copyCdata()
			return
		}
		f.skipToTagEnd()
		return
	}
	if d != '-' {
		f.skipToTagEnd()
		return
	}
	e, err := f.src.ReadByte()
	if err != nil {
		return
	}
	if e != '-' {
		f.skipToTagEnd()
		return
	}
	// Comment: skip until '-->'.
	f.skipUntil("-->")
}

func (f *fb3TextReader) copyCdata() {
	var text strings.Builder
	var win [3]byte
	n := 0
	for {
		c, err := f.src.ReadByte()
		if err != nil {
			text.Write(win[:n])
			f.appendText(text.String())
			f.finish()
			return
		}
		if n < 3 {
			win[n] = c
			n++
		} else {
			text.WriteByte(win[0])
			win[0], win[1], win[2] = win[1], win[2], c
		}
		if n == 3 && string(win[:]) == "]]>" {
			f.appendText(text.String())
			return
		}
	}
}

func (f *fb3TextReader) skipUntil(marker string) {
	var tail strings.Builder
	for {
		c, err := f.src.ReadByte()
		if err != nil {
			return
		}
		tail.WriteByte(c)
		s := tail.String()
		if len(s) > len(marker) {
			s = s[len(s)-len(marker):]
			tail.Reset()
			tail.WriteString(s)
		}
		if tail.String() == marker {
			return
		}
	}
}

func (f *fb3TextReader) skipToTagEnd() {
	for {
		c, err := f.src.ReadByte()
		if err != nil {
			return
		}
		switch c {
		case '>':
			return
		case '"', '\'':
			f.skipQuoted(c)
		}
	}
}

func (f *fb3TextReader) skipQuoted(quote byte) {
	for {
		c, err := f.src.ReadByte()
		if err != nil {
			return
		}
		if c == quote {
			return
		}
	}
}

func (f *fb3TextReader) readTagName() string {
	var name strings.Builder
	for {
		c, err := f.src.ReadByte()
		if err != nil {
			break
		}
		if !util.IsXMLNameChar(c) {
			_ = f.src.UnreadByte()
			break
		}
		name.WriteByte(c)
	}
	return name.String()
}
