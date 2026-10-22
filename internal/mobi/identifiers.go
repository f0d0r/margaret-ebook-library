package mobi

import (
	"strings"

	"github.com/f0d0r/margaret-ebook-library/book"
	"github.com/f0d0r/margaret-ebook-library/internal/util"
)

// Identifiers returns the identifiers declared by the EXTH records.
// EXTH 112 (source) is only included when it carries an ISBN or calibre
// identifier; arbitrary source descriptions are not identifiers.
func (e *Exth) Identifiers() []book.Identifier {
	if e == nil {
		return nil
	}

	out := make([]book.Identifier, 0, 4)
	out = e.appendRecordIdentifiers(out, ISBN, "ISBN")
	out = e.appendSourceIdentifiers(out)
	beforeASIN := len(out)
	out = e.appendRecordIdentifiers(out, ASIN, "ASIN")
	if len(out) == beforeASIN {
		out = e.appendRecordIdentifiers(out, ASIN_5XX, "ASIN")
	}
	if len(out) == 0 {
		return nil
	}
	return dedupeIdentifiers(out)
}

func (e *Exth) appendRecordIdentifiers(out []book.Identifier, recordType uint32, fallbackScheme string) []book.Identifier {
	for _, raw := range e.Records[recordType] {
		scheme, value := util.SplitIdentifier(decodeString(raw, e.textEncoding), "")
		if value == "" {
			continue
		}
		if scheme == "" {
			scheme = fallbackScheme
		}
		out = append(out, book.Identifier{Scheme: scheme, Value: value})
	}
	return out
}

func (e *Exth) appendSourceIdentifiers(out []book.Identifier) []book.Identifier {
	for _, raw := range e.Records[SOURCE] {
		scheme, value := util.SplitIdentifier(decodeString(raw, e.textEncoding), "")
		if value == "" || (!strings.EqualFold(scheme, "ISBN") && !strings.EqualFold(scheme, "calibre")) {
			continue
		}
		out = append(out, book.Identifier{Scheme: scheme, Value: value})
	}
	return out
}

// Identifiers returns the identifiers from the KF8 EXTH when present,
// falling back to the MOBI6 EXTH otherwise.
func (m *Mobi) Identifiers() []book.Identifier {
	if m == nil {
		return nil
	}
	if m.KF8 != nil {
		if ids := m.KF8.exthIdentifiers(); len(ids) > 0 {
			return ids
		}
	}
	return m.exthIdentifiers()
}

func (m *Mobi) exthIdentifiers() []book.Identifier {
	if m.EXTH == nil {
		return nil
	}
	return m.EXTH.Identifiers()
}

func dedupeIdentifiers(ids []book.Identifier) []book.Identifier {
	if len(ids) < 2 {
		return ids
	}
	seen := make(map[book.Identifier]struct{}, len(ids))
	out := ids[:0]
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}
