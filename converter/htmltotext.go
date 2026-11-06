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

// htmlTextBlocks lists elements whose boundaries separate words.
var htmlTextBlocks = map[string]bool{
	"address": true, "article": true, "aside": true, "blockquote": true,
	"br": true, "dd": true, "div": true, "dl": true, "dt": true,
	"fieldset": true, "figcaption": true, "figure": true, "footer": true,
	"form": true, "h1": true, "h2": true, "h3": true, "h4": true,
	"h5": true, "h6": true, "header": true, "hr": true, "li": true,
	"main": true, "nav": true, "ol": true, "p": true, "pre": true,
	"section": true, "table": true, "tbody": true, "td": true,
	"tfoot": true, "th": true, "thead": true, "tr": true, "ul": true,
}

// htmlTextSkips lists elements whose content is not book text.
var htmlTextSkips = map[string]bool{
	"script": true, "style": true, "noscript": true, "head": true,
}

// htmlHeadElements lists elements allowed inside head. They are ignored while
// skipping the head; any other start tag implies the body has begun.
var htmlHeadElements = map[string]bool{
	"base": true, "basefont": true, "bgsound": true, "link": true,
	"meta": true, "title": true, "style": true, "script": true,
	"noscript": true, "template": true,
}

// htmlTextReader implements io.ReadCloser by pulling TextTokens from an
// html.Tokenizer and copying them into the caller's buffer.
type htmlTextReader struct {
	ctx          context.Context
	tokenizer    *html.Tokenizer
	buf          []byte // unconsumed tail of last text token
	closer       io.Closer
	err          error // sticky error (EOF or tokenizer error)
	pendingBreak bool
	emitted      bool
	skipTag      string
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
fill:
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
		switch tt {
		case html.StartTagToken, html.SelfClosingTagToken:
			h.startTag(h.tagName(), tt == html.SelfClosingTagToken)
			continue
		case html.EndTagToken:
			h.endTag(h.tagName())
			continue
		case html.TextToken:
			if h.skipTag != "" {
				continue
			}
			text := h.tokenizer.Text()
			if len(text) == 0 {
				continue
			}
			// Whitespace between blocks is structural, not content.
			if len(bytes.TrimSpace(text)) == 0 && (h.pendingBreak || !h.emitted) {
				continue
			}
			if h.pendingBreak {
				p[total] = '\n'
				total++
				h.pendingBreak = false
				h.emitted = true
				if total == len(p) {
					h.buf = append(h.buf[:0], text...)
					break fill
				}
			}
			// Copy as much as fits into p[total:].
			n := copy(p[total:], text)
			total += n
			h.emitted = true
			if n < len(text) {
				// Save remainder for next Read.
				h.buf = append(h.buf[:0], text[n:]...)
				break fill
			}
		}
	}
	if total == 0 && h.err != nil {
		return 0, h.err
	}
	return total, nil
}

func (h *htmlTextReader) tagName() string {
	name, _ := h.tokenizer.TagName()
	return string(name)
}

func (h *htmlTextReader) startTag(name string, selfClosing bool) {
	if h.skipTag != "" {
		if h.skipTag != "head" {
			return
		}
		if htmlHeadElements[name] {
			return
		}
		// A non-head element means the body started without </head>.
		h.skipTag = ""
	}
	if htmlTextSkips[name] {
		if !selfClosing {
			h.skipTag = name
		}
		return
	}
	if htmlTextBlocks[name] {
		h.breakLine()
	}
}

func (h *htmlTextReader) endTag(name string) {
	if h.skipTag != "" {
		if name == h.skipTag || (h.skipTag == "head" && (name == "body" || name == "html")) {
			h.skipTag = ""
		}
		return
	}
	if htmlTextBlocks[name] {
		h.breakLine()
	}
}

// breakLine schedules a newline before the next text content. Leading and
// trailing breaks are dropped because the newline is materialized only when
// more text follows.
func (h *htmlTextReader) breakLine() {
	if h.emitted {
		h.pendingBreak = true
	}
}

func (h *htmlTextReader) Close() error {
	if h.closer != nil {
		return h.closer.Close()
	}
	return nil
}
