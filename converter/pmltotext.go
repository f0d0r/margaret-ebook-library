package converter

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"strings"
	"unicode/utf8"
)

type pmlToTextTransformer struct {
	from string
	to   string
}

func (t *pmlToTextTransformer) From() string { return t.from }
func (t *pmlToTextTransformer) To() string   { return t.to }

// Transform expects UTF-8 input: the book layer decodes the resource's
// original encoding (windows-1252 for eReader PML) centrally before the
// transformer chain runs, so every converter sees UTF-8.
func (t *pmlToTextTransformer) Transform(ctx context.Context, r io.Reader) (io.ReadCloser, error) {
	if r == nil {
		return io.NopCloser(bytes.NewReader(nil)), nil
	}
	var closer io.Closer
	if c, ok := r.(io.Closer); ok {
		closer = c
	}
	return &pmlTextReader{
		ctx:    ctx,
		src:    bufio.NewReader(r),
		closer: closer,
	}, nil
}

type pmlTextReader struct {
	ctx     context.Context
	src     *bufio.Reader
	closer  io.Closer
	out     []byte
	breaks  int
	emitted bool
	err     error
}

func (p *pmlTextReader) Read(b []byte) (int, error) {
	if p.err != nil {
		return 0, p.err
	}
	for len(p.out) == 0 {
		if p.ctx != nil {
			select {
			case <-p.ctx.Done():
				p.err = p.ctx.Err()
				return 0, p.err
			default:
			}
		}
		if !p.scan() {
			p.err = io.EOF
			return 0, p.err
		}
	}
	n := copy(b, p.out)
	p.out = p.out[n:]
	return n, nil
}

func (p *pmlTextReader) Close() error {
	if p.closer != nil {
		return p.closer.Close()
	}
	return nil
}

func (p *pmlTextReader) scan() bool {
	c, err := p.src.ReadByte()
	if err != nil {
		return false
	}
	if c == '\\' {
		p.parseTag()
	} else {
		p.writeByte(c)
	}
	return true
}

func (p *pmlTextReader) parseTag() {
	d, err := p.src.ReadByte()
	if err != nil {
		p.emitRaw(`\`)
		return
	}
	switch d {
	case '\\':
		p.emitRaw(`\`)
	case 'v':
		p.skipComment()
	case 'a':
		p.parseCode(3, 10)
	case 'U':
		p.parseCode(4, 16)
	case 'x', 'X':
		if d == 'X' {
			p.skipDigits(1)
		}
		p.breakLine()
		p.breakLine()
	case 'C':
		if p.skipDigits(1) {
			if s, ok := p.readValue(); ok {
				p.breakLine()
				p.breakLine()
				p.emitRaw(s)
			}
			p.breakLine()
			p.breakLine()
		}
	case 'p', 'w':
		p.readValue()
		p.breakLine()
		p.breakLine()
	case 'F':
		if c, ok := p.peek(); ok && c == 'n' {
			_, _ = p.src.Discard(1)
			p.readValue()
		}
	case 'S':
		if c, ok := p.peek(); ok && (c == 'p' || c == 'b' || c == 'd') {
			_, _ = p.src.Discard(1)
			p.readValue()
		}
	default:
		p.readValue()
	}
}

func (p *pmlTextReader) parseCode(n int, base int) {
	lead := byte('a')
	if base == 16 {
		lead = 'U'
	}
	raw := []byte{'\\', lead}
	for i := 0; i < n; i++ {
		c, err := p.src.ReadByte()
		if err != nil {
			p.emitRaw(string(raw))
			return
		}
		raw = append(raw, c)
	}
	v, ok := parseCodeValue(raw[2:], base)
	if !ok {
		p.emitRaw(string(raw))
		return
	}
	var buf [utf8.UTFMax]byte
	p.emitRaw(string(buf[:utf8.EncodeRune(buf[:], rune(v))]))
}

func parseCodeValue(digits []byte, base int) (int, bool) {
	v := 0
	for _, c := range digits {
		d := digitValue(c, base)
		if d < 0 {
			return 0, false
		}
		v = v*base + d
	}
	if v > utf8.MaxRune || (v >= 0xD800 && v <= 0xDFFF) {
		return 0, false
	}
	return v, true
}

func digitValue(c byte, base int) int {
	var d int
	switch {
	case '0' <= c && c <= '9':
		d = int(c - '0')
	case 'a' <= c && c <= 'f':
		d = int(c-'a') + 10
	case 'A' <= c && c <= 'F':
		d = int(c-'A') + 10
	default:
		return -1
	}
	if d >= base {
		return -1
	}
	return d
}

func (p *pmlTextReader) skipComment() {
	for {
		c, err := p.src.ReadByte()
		if err != nil {
			return
		}
		if c == '\\' {
			if d, ok := p.peek(); ok && d == 'v' {
				_, _ = p.src.Discard(1)
				return
			}
		}
	}
}

func (p *pmlTextReader) skipDigits(max int) bool {
	skipped := false
	for i := 0; i < max; i++ {
		c, ok := p.peek()
		if !ok || c < '0' || c > '9' {
			break
		}
		_, _ = p.src.Discard(1)
		skipped = true
	}
	return skipped
}

func (p *pmlTextReader) peek() (byte, bool) {
	c, err := p.src.Peek(1)
	if err != nil || len(c) == 0 {
		return 0, false
	}
	return c[0], true
}

func (p *pmlTextReader) readValue() (string, bool) {
	c, ok := p.peek()
	if !ok || c != '=' {
		return "", false
	}
	_, _ = p.src.Discard(1)
	q, err := p.src.ReadByte()
	if err != nil || q != '"' {
		return "", false
	}
	var sb strings.Builder
	for {
		c, err := p.src.ReadByte()
		if err != nil || c == '"' {
			return sb.String(), true
		}
		sb.WriteByte(c)
	}
}

func (p *pmlTextReader) writeByte(c byte) {
	if c == '\r' {
		p.breakLine()
		if d, ok := p.peek(); ok && d == '\n' {
			_, _ = p.src.Discard(1)
		}
		return
	}
	if c == '\n' {
		p.breakLine()
		return
	}
	p.flushBreaks()
	p.out = append(p.out, c)
	p.emitted = true
}

func (p *pmlTextReader) emitRaw(s string) {
	for i := 0; i < len(s); i++ {
		p.writeByte(s[i])
	}
}

func (p *pmlTextReader) breakLine() {
	if p.breaks < 2 {
		p.breaks++
	}
}

func (p *pmlTextReader) flushBreaks() {
	if p.breaks > 0 {
		if p.emitted {
			p.out = append(p.out, bytes.Repeat([]byte{'\n'}, p.breaks)...)
		}
		p.breaks = 0
	}
}
