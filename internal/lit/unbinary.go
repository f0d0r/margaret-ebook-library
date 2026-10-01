package lit

import (
	"bytes"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/f0d0r/margaret-ebook-library/book"
	"github.com/f0d0r/margaret-ebook-library/internal/util"
)

// UnBinary flags for tag records.
const (
	flagOpening = 1 << 0
	flagClosing = 1 << 1
	flagBlock   = 1 << 2
	flagHead    = 1 << 3
	flagAtom    = 1 << 4
)

// opfTags maps binary tag ids to OPF element names. Gaps decode no tag.
var opfTags = [...]string{
	1:  "package",
	2:  "dc:Title",
	3:  "dc:Creator",
	16: "manifest",
	17: "item",
	18: "spine",
	19: "itemref",
	20: "metadata",
	21: "dc-metadata",
	22: "dc:Subject",
	23: "dc:Description",
	24: "dc:Publisher",
	25: "dc:Contributor",
	26: "dc:Date",
	27: "dc:Type",
	28: "dc:Format",
	29: "dc:Identifier",
	30: "dc:Source",
	31: "dc:Language",
	32: "dc:Relation",
	33: "dc:Coverage",
	34: "dc:Rights",
	35: "x-metadata",
	36: "meta",
	37: "tours",
	38: "tour",
	39: "site",
	40: "guide",
	41: "reference",
}

// opfAttrs maps binary attribute ids to OPF attribute names. Names starting
// with '%' carry censorship payloads skipped from the output.
var opfAttrs = map[int]string{
	0x0001: "href",
	0x0002: "%never-used",
	0x0003: "%guid",
	0x0004: "%minimum_level",
	0x0005: "%attr5",
	0x0006: "id",
	0x0007: "href",
	0x0008: "media-type",
	0x0009: "fallback",
	0x000A: "idref",
	0x000B: "xmlns:dc",
	0x000C: "xmlns:oebpackage",
	0x000D: "role",
	0x000E: "file-as",
	0x000F: "event",
	0x0010: "scheme",
	0x0011: "title",
	0x0012: "type",
	0x0013: "unique-identifier",
	0x0014: "name",
	0x0015: "content",
	0x0016: "xml:lang",
}

// unbinTables carries one markup vocabulary: tag ids, global attribute
// ids and per-tag attribute maps.
type unbinTables struct {
	tags     []string
	attrs    map[int]string
	tagAttrs []map[int]string
}

var opfTables = unbinTables{tags: opfTags[:], attrs: opfAttrs}

var htmlTables = unbinTables{tags: htmlTags[:], attrs: htmlAttrs, tagAttrs: htmlTagAttrs}

// unbinAtoms carries custom tag/attribute names for content documents.
// Nil maps behave like calibre's empty atom lists (any atom reference is
// corrupt input).
type unbinAtoms struct {
	tags  map[int]string
	attrs map[int]string
}

// unbinary states of the text/tag/attribute machine.
type unbinState int

const (
	stText unbinState = iota
	stGetFlags
	stGetTag
	stGetAttr
	stGetValueLength
	stGetValue
	stGetCustomLength
	stGetCustom
	stGetAttrLength
	stGetCustomAttr
	stGetHrefLength
	stGetHref
	stCloseTag
)

// unbinFrame is one stack entry of the decoder. Only the state that must
// survive a suspension travels here: the tag being closed, the attribute map
// the parent element inherited (a dynamic tag reads it without resetting, for
// reference parity), the nesting depth and the resume point. Everything else
// is consumed within a single inner call.
type unbinFrame struct {
	depth   int
	tagName string
	attrMap map[int]string
	state   unbinState
}

// unbinary decodes MS Reader binary markup into XML text. tables selects
// the vocabulary (OPF or HTML), manifest maps internal href ids to output
// paths (may be empty, in which case hrefs pass through), atoms carries
// custom content tag names (nil behaves like an empty atom list), and doc
// is the decoded document's own OPF href used to relativize output hrefs
// ("" for metadata, which lives at the root).
type unbinary struct {
	tables   *unbinTables
	manifest map[string]string
	atoms    *unbinAtoms
	doc      string
	buf      bytes.Buffer
	cpos     int
	bin      []byte
}

func decodeUnbinary(bin []byte, tables *unbinTables, manifest map[string]string, atoms *unbinAtoms, doc string) ([]byte, error) {
	u := &unbinary{tables: tables, manifest: manifest, atoms: atoms, doc: doc, bin: bin}
	stack := []unbinFrame{{state: stText}}
	for len(stack) > 0 {
		var err error
		stack, err = u.inner(stack)
		if err != nil {
			return nil, err
		}
	}
	raw := bytes.TrimLeft(u.buf.Bytes(), " \t\n\r\v\f")
	return escapeReserved(raw), nil
}

func (u *unbinary) inner(stack []unbinFrame) ([]unbinFrame, error) {
	fr := stack[len(stack)-1]
	stack = stack[:len(stack)-1]
	depth, tagName, currentMap := fr.depth, fr.tagName, fr.attrMap
	// inCensor, goingDown and flags never cross a suspension: both push sites
	// are reached with them already reset, so the zero value resumes exactly.
	var inCensor, goingDown bool
	var flags int
	state := fr.state
	var count int
	var href strings.Builder
	var customName []byte

	if state == stCloseTag {
		if tagName == "" {
			return stack, fmt.Errorf("lit: tag ends before it begins: %w", book.ErrCorrupt)
		}
		u.buf.WriteString("</")
		u.buf.WriteString(tagName)
		u.buf.WriteString(">")
		tagName = ""
		state = stText
	}

	for u.cpos < len(u.bin) {
		c, next, err := decodeRune(u.bin, u.cpos)
		if err != nil {
			return stack, err
		}
		oc := int(c)
		u.cpos = next

		switch state {
		case stText:
			switch {
			case oc == 0:
				state = stGetFlags
			case c == '\v':
				u.buf.WriteByte('\n')
			case c == '>':
				u.buf.WriteString(">>")
			case c == '<':
				u.buf.WriteString("<<")
			default:
				writeXMLEscaped(&u.buf, c)
			}

		case stGetFlags:
			if oc == 0 {
				state = stText
				continue
			}
			flags = oc
			state = stGetTag

		case stGetTag:
			if oc == 0 {
				state = stText
			} else {
				state = stGetAttr
			}
			if flags&flagOpening != 0 {
				tag := oc
				u.buf.WriteByte('<')
				if flags&flagClosing == 0 {
					goingDown = true
				}
				if tag == 0x8000 {
					state = stGetCustomLength
					continue
				}
				if flags&flagAtom != 0 {
					var name string
					var ok bool
					if u.atoms != nil {
						name, ok = u.atoms.tags[tag]
					}
					if !ok || name == "" {
						return stack, fmt.Errorf("lit: atom tag %d not in atom list: %w", tag, book.ErrCorrupt)
					}
					tagName = name
					currentMap = u.atoms.attrs
				} else if tag < len(u.tables.tags) && u.tables.tags[tag] != "" {
					tagName = u.tables.tags[tag]
					currentMap = nil
					if tag < len(u.tables.tagAttrs) {
						currentMap = u.tables.tagAttrs[tag]
					}
				} else {
					// Unknown tags (past the table or gaps) are corrupt
					// input; the reference crashes here.
					return stack, fmt.Errorf("lit: unknown tag %d: %w", tag, book.ErrCorrupt)
				}
				writeXMLEscapedStr(&u.buf, tagName)
			} else if flags&flagClosing != 0 {
				if depth == 0 {
					// Trailing closes with nothing open: MS-era files
					// carry them (their mid-stream stray already
					// consumed the matching opens). Tolerate instead of
					// failing; both references reject such files.
					continue
				}
				// Drop this frame; the outer loop continues with the
				// close-tag frame pushed when the tag opened.
				return stack, nil
			}

		case stGetAttr:
			inCensor = false
			if oc == 0 {
				state = stText
				if !goingDown {
					tagName = ""
					u.buf.WriteString(" />")
				} else {
					u.buf.WriteByte('>')
					stack = append(stack,
						unbinFrame{depth: depth, tagName: tagName, attrMap: currentMap, state: stCloseTag},
						unbinFrame{depth: depth + 1, state: stText},
					)
					return stack, nil
				}
			} else {
				if oc == 0x8000 {
					state = stGetAttrLength
					continue
				}
				var attr string
				if currentMap != nil {
					attr = currentMap[oc]
				}
				if attr == "" {
					attr = u.tables.attrs[oc]
				}
				if attr == "" {
					return stack, fmt.Errorf("lit: unknown attribute %d in tag %s: %w", oc, tagName, book.ErrCorrupt)
				}
				if strings.HasPrefix(attr, "%") {
					inCensor = true
					state = stGetValueLength
					continue
				}
				u.buf.WriteByte(' ')
				writeXMLEscapedStr(&u.buf, attr)
				u.buf.WriteByte('=')
				if attr == "href" || attr == "src" {
					state = stGetHrefLength
				} else {
					state = stGetValueLength
				}
			}

		case stGetValueLength:
			if !inCensor {
				u.buf.WriteByte('"')
			}
			count = oc - 1
			if count == 0 {
				if !inCensor {
					u.buf.WriteByte('"')
				}
				inCensor = false
				state = stGetAttr
				continue
			}
			state = stGetValue
			if oc == 0xFFFF {
				continue
			}
			if count < 0 || count > len(u.bin)-u.cpos {
				return stack, fmt.Errorf("lit: invalid character count %d: %w", count, book.ErrCorrupt)
			}

		case stGetValue:
			if count == 0xFFFE {
				if !inCensor {
					// Faithful to the reference decoder, which formats
					// the value numerically here.
					fmt.Fprintf(&u.buf, "%d\"", oc-1)
				}
				inCensor = false
				state = stGetAttr
			} else if count > 0 {
				if !inCensor {
					switch c {
					case '"':
						u.buf.WriteString("&quot;")
					case '<':
						u.buf.WriteString("&lt;")
					default:
						writeXMLEscaped(&u.buf, c)
					}
				}
				count--
			}
			if count == 0 {
				if !inCensor {
					u.buf.WriteByte('"')
				}
				inCensor = false
				state = stGetAttr
			}

		case stGetCustomLength:
			count = oc - 1
			if count <= 0 || count > len(u.bin)-u.cpos {
				return stack, fmt.Errorf("lit: invalid character count %d: %w", count, book.ErrCorrupt)
			}
			state = stGetCustom
			tagName = ""
			customName = customName[:0]

		case stGetCustom:
			customName = append(customName, string(c)...)
			count--
			if count == 0 {
				tagName = string(customName)
				writeAttrEscaped(&u.buf, tagName)
				state = stGetAttr
			}

		case stGetAttrLength:
			count = oc - 1
			if count <= 0 || count > len(u.bin)-u.cpos {
				return stack, fmt.Errorf("lit: invalid character count %d: %w", count, book.ErrCorrupt)
			}
			u.buf.WriteByte(' ')
			state = stGetCustomAttr

		case stGetCustomAttr:
			switch c {
			case '"':
				u.buf.WriteString("&quot;")
			case '<':
				u.buf.WriteString("&lt;")
			default:
				writeXMLEscaped(&u.buf, c)
			}
			count--
			if count == 0 {
				u.buf.WriteByte('=')
				state = stGetValueLength
			}

		case stGetHrefLength:
			count = oc - 1
			if count <= 0 || count > len(u.bin)-u.cpos {
				return stack, fmt.Errorf("lit: invalid character count %d: %w", count, book.ErrCorrupt)
			}
			href.Reset()
			state = stGetHref

		case stGetHref:
			href.WriteRune(c)
			count--
			if count == 0 {
				path := resolveHref(href.String(), u.manifest, docDir(u.doc))
				u.buf.WriteByte('"')
				writeAttrEscaped(&u.buf, path)
				u.buf.WriteByte('"')
				state = stGetAttr
			}
		}
	}
	// Input exhausted: drop the suspended frame, the outer loop continues
	// with the next stacked frame.
	return stack, nil
}

// resolveHref maps an internal href to an output path relativized against
// dir (the decoded document's own directory, "" at the root). The binary
// form carries a one-character prefix that is stripped; manifest lookups
// may be absent (metadata-only decoding), in which case the id passes
// through unresolved but still relativized.
func resolveHref(href string, manifest map[string]string, dir string) string {
	doc := href
	if _, size := utf8.DecodeRuneInString(doc); size > 0 {
		doc = doc[size:]
	}
	frag := ""
	if i := strings.IndexByte(doc, '#'); i >= 0 {
		frag, doc = doc[i+1:], doc[:i]
	}
	if target, ok := manifest[doc]; ok {
		doc = target
	}
	doc = relativize(doc, dir)
	if frag != "" {
		doc += "#" + frag
	}
	return doc
}

// docDir returns the directory of an OPF href ("" at the root).
func docDir(doc string) string {
	if i := strings.LastIndexByte(doc, '/'); i >= 0 {
		return doc[:i]
	}
	return ""
}

// relativize rewrites target relative to base, splitting on "/".
func relativize(target, base string) string {
	if base == "" {
		return target
	}
	tb := strings.Split(target, "/")
	bb := strings.Split(base, "/")
	index := 0
	for index < len(bb) && index < len(tb) && bb[index] == tb[index] {
		index++
	}
	var rel []string
	for i := 0; i < len(bb)-index; i++ {
		rel = append(rel, "..")
	}
	return strings.Join(append(rel, tb[index:]...), "/")
}

// writeAttrEscaped writes attribute/tag text with XML-significant ASCII
// (" and <) and non-ASCII code points escaped. Bare & is left for the
// escapeReserved post-pass, which preserves valid entities.
func writeAttrEscaped(buf *bytes.Buffer, s string) {
	for _, c := range s {
		switch c {
		case '"':
			buf.WriteString("&quot;")
		case '<':
			buf.WriteString("&lt;")
		default:
			writeXMLEscaped(buf, c)
		}
	}
}

// decodeRune decodes one strict UTF-8 character. A genuine U+FFFD
// (three-byte encoding) is accepted; only undecodable bytes fail.
func decodeRune(data []byte, pos int) (rune, int, error) {
	if pos >= len(data) {
		return 0, pos, fmt.Errorf("lit: truncated text: %w", book.ErrCorrupt)
	}
	c, size := utf8.DecodeRune(data[pos:])
	if c == utf8.RuneError && size <= 1 {
		return 0, pos, fmt.Errorf("lit: invalid UTF-8: %w", book.ErrCorrupt)
	}
	return c, pos + size, nil
}

// writeXMLEscaped writes a rune with non-ASCII code points as numeric
// character references.
func writeXMLEscaped(buf *bytes.Buffer, c rune) {
	if c < 128 {
		buf.WriteByte(byte(c))
		return
	}
	fmt.Fprintf(buf, "&#%d;", int(c))
}

// writeXMLEscapedStr writes a string with non-ASCII code points as numeric
// character references.
func writeXMLEscapedStr(buf *bytes.Buffer, s string) {
	for _, c := range s {
		writeXMLEscaped(buf, c)
	}
}

// escapeReserved fixes reserved characters per the reference escaper:
// bare & becomes &amp;, << (unless <<!--) becomes &lt;, qualifying >>
// becomes &gt;, then remaining doublings collapse.
func escapeReserved(raw []byte) []byte {
	raw = escapeAmpersand(raw)
	raw = escapeOpenAngle(raw)
	raw = escapeCloseAngle(raw)
	return collapseDoubleAngle(raw)
}

func isEntityStart(s []byte) bool {
	i := 0
	if i < len(s) && s[i] == '#' {
		i++
		if i < len(s) && (s[i] == 'x' || s[i] == 'X') {
			i++
			start := i
			for i < len(s) && isHex(s[i]) {
				i++
			}
			return i > start && i < len(s) && s[i] == ';'
		}
		start := i
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
		}
		return i > start && i < len(s) && s[i] == ';'
	}
	if i < len(s) && (isNameStart(s[i])) {
		i++
		for i < len(s) && util.IsXMLNameChar(s[i]) {
			i++
		}
		return i < len(s) && s[i] == ';'
	}
	return false
}

func isHex(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}

func isNameStart(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '_' || c == ':'
}

func escapeAmpersand(raw []byte) []byte {
	var out bytes.Buffer
	for i := 0; i < len(raw); {
		if raw[i] == '&' && !isEntityStart(raw[i+1:]) {
			out.WriteString("&amp;")
			i++
			continue
		}
		out.WriteByte(raw[i])
		i++
	}
	return out.Bytes()
}

func escapeOpenAngle(raw []byte) []byte {
	var out bytes.Buffer
	for i := 0; i < len(raw); {
		if raw[i] == '<' && i+1 < len(raw) && raw[i+1] == '<' && !hasCommentOpen(raw[i+2:]) {
			out.WriteString("&lt;")
			i += 2
			continue
		}
		out.WriteByte(raw[i])
		i++
	}
	return out.Bytes()
}

// hasCommentOpen reports whether s starts with "!--" (an HTML comment open
// following "<<").
func hasCommentOpen(s []byte) bool {
	return len(s) >= 3 && s[0] == '!' && s[1] == '-' && s[2] == '-'
}

func escapeCloseAngle(raw []byte) []byte {
	var out bytes.Buffer
	for i := 0; i < len(raw); {
		if raw[i] == '>' && i+1 < len(raw) && raw[i+1] == '>' &&
			!hasCommentCloseBefore(raw, i) && i+2 < len(raw) {
			out.WriteString("&gt;")
			i += 2
			continue
		}
		out.WriteByte(raw[i])
		i++
	}
	return out.Bytes()
}

// hasCommentCloseBefore reports whether raw[:i] ends with "<!--".
func hasCommentCloseBefore(raw []byte, i int) bool {
	return i >= 4 && string(raw[i-4:i]) == "<!--"
}

func collapseDoubleAngle(raw []byte) []byte {
	var out bytes.Buffer
	for i := 0; i < len(raw); {
		if i+1 < len(raw) && ((raw[i] == '<' && raw[i+1] == '<') || (raw[i] == '>' && raw[i+1] == '>')) {
			out.WriteByte(raw[i])
			i += 2
			continue
		}
		out.WriteByte(raw[i])
		i++
	}
	return out.Bytes()
}
