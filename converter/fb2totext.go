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

// fb2BodyToTextTransformer converts FictionBook body fragments
// (application/x-fictionbook-body+xml) to plain text.
//
// The input is a raw <body> content slice: not necessarily well-formed XML
// (multiple roots) and possibly using undeclared namespace prefixes, so
// encoding/xml cannot parse it. Instead a small tolerant scanner walks the
// markup: element names match by local name, unknown elements are
// transparent, and <binary>/<stylesheet> subtrees are skipped. Text inside
// blocks passes through verbatim; block boundaries emit newlines.
type fb2BodyToTextTransformer struct {
	from string
	to   string
}

func (t *fb2BodyToTextTransformer) From() string { return t.from }
func (t *fb2BodyToTextTransformer) To() string   { return t.to }

func (t *fb2BodyToTextTransformer) Transform(ctx context.Context, r io.Reader) (io.ReadCloser, error) {
	if r == nil {
		return io.NopCloser(bytes.NewReader(nil)), nil
	}
	var closer io.Closer
	if c, ok := r.(io.Closer); ok {
		closer = c
	}
	return &fb2TextReader{
		ctx:         ctx,
		src:         bufio.NewReader(r),
		closer:      closer,
		atLineStart: true,
	}, nil
}

// fb2TextBlocks lists elements whose end starts a new output line.
var fb2TextBlocks = map[string]bool{
	"p": true, "v": true, "subtitle": true, "text-author": true,
	"th": true, "td": true, "title": true, "stanza": true,
}

// fb2TextSkips lists subtrees excluded from the output (binary payloads,
// stylesheets).
var fb2TextSkips = map[string]bool{
	"binary": true, "stylesheet": true,
}

// fb2TextReader implements io.ReadCloser by scanning FB2 markup and emitting
// text lines. It never fails on malformed input: unknown constructs degrade
// to literal text instead of errors.
type fb2TextReader struct {
	ctx         context.Context
	src         *bufio.Reader
	closer      io.Closer
	out         []byte // ready output
	cur         []byte // current line buffer
	breaks      int    // pending line breaks, materialized by following text
	emitted     bool   // any byte ever queued to out (leading breaks drop)
	atLineStart bool
	skipDepth   int
	done        bool
	err         error // sticky error (io.EOF when done)
}

func (f *fb2TextReader) Read(p []byte) (int, error) {
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

func (f *fb2TextReader) Close() error {
	if f.closer != nil {
		return f.closer.Close()
	}
	return nil
}

// maxLineBuffer caps a single output line: pathological single-paragraph
// inputs flush through instead of accumulating unboundedly.
const maxLineBuffer = 1 << 20

// appendText adds a text run to the current line. Formatting whitespace at
// line starts is dropped; everything else passes through verbatim. Pending
// breaks materialize only before real text, so leading and trailing breaks
// never appear in the output.
func (f *fb2TextReader) appendText(s string) {
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

// breakLine ends the current line; the newline is emitted when (and if)
// further text follows.
func (f *fb2TextReader) breakLine() {
	if len(f.cur) > 0 {
		f.out = append(f.out, f.cur...)
		f.emitted = true
		f.cur = f.cur[:0]
	}
	f.breaks++
	f.atLineStart = true
}

// finish flushes the trailing line at EOF; pending breaks are dropped.
func (f *fb2TextReader) finish() {
	if len(f.cur) > 0 {
		f.out = append(f.out, f.cur...)
		f.cur = f.cur[:0]
	}
	f.done = true
}

// scan consumes input until output is produced or EOF is reached.
func (f *fb2TextReader) scan() {
	var text strings.Builder
	for {
		if f.skipDepth > 0 {
			if text.Len() > 0 {
				f.appendText(text.String())
				text.Reset()
			}
			f.scanSkipped()
			if len(f.out) > 0 || f.done {
				return
			}
			continue
		}
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

// parseEntity consumes an entity reference after '&'. Unknown or unterminated
// entities are emitted literally instead of failing.
func (f *fb2TextReader) parseEntity() {
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

// parseTag consumes a markup construct after '<'.
func (f *fb2TextReader) parseTag() {
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
		name, selfClosing := f.parseStartTag()
		local := localName(name)
		switch {
		case local == "empty-line":
			f.breakLine()
			// Exactly one blank line: cap pending breaks so consecutive
			// breaks (e.g. after </p>) don't accumulate.
			if f.breaks < 2 {
				f.breaks = 2
			}
		case fb2TextSkips[local] && !selfClosing:
			f.skipDepth = 1
		}
	default:
		_ = f.src.UnreadByte()
		f.appendText("<")
	}
}

// parseEndTag consumes an end tag after '</'. Block ends start a new line.
func (f *fb2TextReader) parseEndTag() {
	name := f.readTagName()
	f.skipToTagEnd()
	if fb2TextBlocks[localName(name)] {
		f.breakLine()
	}
}

// parseStartTag consumes a start tag (decoder positioned after its first
// name byte) and reports the raw name and whether it is self-closing.
func (f *fb2TextReader) parseStartTag() (string, bool) {
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

// scanSkipped discards tokens until the skip subtree ends.
func (f *fb2TextReader) scanSkipped() {
	for {
		c, err := f.src.ReadByte()
		if err != nil {
			f.finish()
			return
		}
		if c != '<' {
			continue
		}
		d, err := f.src.ReadByte()
		if err != nil {
			f.finish()
			return
		}
		switch {
		case d == '/':
			f.skipToTagEnd()
			f.skipDepth--
			if f.skipDepth <= 0 {
				f.skipDepth = 0
				return
			}
		case d == '!' || d == '?':
			f.skipSpecial(d)
		case util.IsXMLNameChar(d):
			_ = f.src.UnreadByte()
			_, selfClosing := f.parseStartTag()
			if !selfClosing {
				f.skipDepth++
			}
		}
	}
}

// skipSpecial consumes comments, CDATA (as literal text, entities
// unresolved per XML rules), PIs and other declarations after '<!' or '<?'.
func (f *fb2TextReader) skipSpecial(c byte) {
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
			if f.skipDepth > 0 {
				f.skipCdata()
			} else {
				f.copyCdata()
			}
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

// copyCdata copies bytes until "]]>" as literal text.
func (f *fb2TextReader) copyCdata() {
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

// skipCdata discards bytes until "]]>".
func (f *fb2TextReader) skipCdata() {
	var win [3]byte
	n := 0
	for {
		c, err := f.src.ReadByte()
		if err != nil {
			return
		}
		if n < 3 {
			win[n] = c
			n++
		} else {
			win[0], win[1], win[2] = win[1], win[2], c
		}
		if n == 3 && string(win[:]) == "]]>" {
			return
		}
	}
}

// skipUntil discards bytes until the marker is seen.
func (f *fb2TextReader) skipUntil(marker string) {
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

// skipToTagEnd discards bytes until '>' outside quotes.
func (f *fb2TextReader) skipToTagEnd() {
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

// skipQuoted discards bytes until the matching quote.
func (f *fb2TextReader) skipQuoted(quote byte) {
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

// readTagName reads a tag name starting at the current position.
func (f *fb2TextReader) readTagName() string {
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

// localName strips an optional namespace prefix.
func localName(name string) string {
	if i := strings.LastIndexByte(name, ':'); i >= 0 {
		return name[i+1:]
	}
	return name
}
