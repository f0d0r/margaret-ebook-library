package mobi

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/f0d0r/margaret-ebook-library/book"
)

// panicError marks an error produced by recovering a panic while parsing a
// malformed file. It lets callers classify such failures as [book.ErrCorrupt]
// instead of a generic parse error.
type panicError struct {
	msg string
}

func (e *panicError) Error() string { return e.msg }

// withRecover wraps a function with panic recovery, converting a panic into an
// error. Parsing untrusted book files must never take the caller down, so every
// function that indexes into raw file data runs through this helper.
func withRecover(name string, fn func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = &panicError{msg: fmt.Sprintf("panic in %s: %v", name, r)}
		}
	}()
	return fn()
}

// kf8ContentError wraps a KF8 content failure for the caller. Recovered panics
// point at malformed structures, so they are reported as [book.ErrCorrupt];
// everything else is a generic parse failure.
func kf8ContentError(err error) error {
	var pe *panicError
	if errors.As(err, &pe) {
		return fmt.Errorf("%w: KF8 parsing failed: %w", book.ErrCorrupt, err)
	}
	return fmt.Errorf("KF8 parsing failed: %w", err)
}

// Part mirrors calibre's Part namedtuple.
type Part struct {
	Num      int
	Type     string
	Filename string
	Start    int
	End      int
	Aid      string
}

// Elem mirrors calibre's Elem.
type Elem struct {
	InsertPos      int
	TocText        string
	FileNumber     int
	SequenceNumber int
	StartPos       int
	Length         int
}

// FileInfo mirrors calibre's File (skel).
type FileInfo struct {
	FileNumber int
	Name       string
	DivCount   int
	StartPos   int
	Length     int
}

// FlowInfo mirrors calibre's FlowInfo (not needed for raw text, kept for completeness).
type FlowInfo struct {
	Type   string
	Format string
	Dir    string
	Fname  string
}

// readMobi8Indices reads FDST, SKEL, DIV tables.
// sections must be the KF8-relative slice (sections[offset-1:] for joint, full for standalone).
// codec is "utf-8" or "cp1252".
// mobi is the KF8 Mobi header with KF8-relative indices.
func readMobi8Indices(sections [][]byte, mobi *Mobi, codec string) (flowTable [][2]int, files []FileInfo, elems []Elem, err error) {
	err = withRecover("readMobi8Indices", func() error {
		// FDST
		if mobi.FdstIdx != NullIndex {
			if int(mobi.FdstIdx) < len(sections) {
				fdstData := sections[mobi.FdstIdx]
				if len(fdstData) >= 4 && string(fdstData[:4]) == "FDST" {
					ft, e := parseFDST(fdstData)
					if e != nil {
						return fmt.Errorf("FDST parse failed: %w", e)
					}
					flowTable = ft
				}
			}
		}

		// SKEL
		if mobi.SkelIdx != NullIndex && int(mobi.SkelIdx) < len(sections) {
			table, ordered, _, err := ReadIndex(sections, int(mobi.SkelIdx), codec)
			if err != nil {
				return fmt.Errorf("SKEL read failed: %w", err)
			}
			// ordered preserves enumeration order; table keys are file names
			for i, name := range ordered {
				tagMap := table[name]
				// Tag 1: div count, Tag 6: start, length
				divCount := 0
				if v, ok := tagMap[1]; ok && len(v) > 0 {
					divCount = v[0]
				}
				startPos, length := 0, 0
				if v, ok := tagMap[6]; ok && len(v) > 0 {
					startPos = v[0]
					if len(v) > 1 {
						length = v[1]
					}
				}
				files = append(files, FileInfo{
					FileNumber: i,
					Name:       name,
					DivCount:   divCount,
					StartPos:   startPos,
					Length:     length,
				})
			}
		}

		// DIV
		if mobi.DivIdx != NullIndex && int(mobi.DivIdx) < len(sections) {
			table, ordered, cncx, err := ReadIndex(sections, int(mobi.DivIdx), codec)
			if err != nil {
				return fmt.Errorf("DIV read failed: %w", err)
			}
			for _, key := range ordered {
				tagMap := table[key]
				tocText := ""
				if v, ok := tagMap[2]; ok && len(v) > 0 {
					if s, ok2 := cncx[v[0]]; ok2 {
						tocText = s
					}
				}
				fileNumber, seqNum, startPos, length := 0, 0, 0, 0
				if v, ok := tagMap[3]; ok && len(v) > 0 {
					fileNumber = v[0]
				}
				if v, ok := tagMap[4]; ok && len(v) > 0 {
					seqNum = v[0]
				}
				if v, ok := tagMap[6]; ok && len(v) >= 2 {
					startPos = v[0]
					length = v[1]
				}
				// key is the text id, calibre does int(text); we keep the
				// enumeration order and only convert for the insert position.
				elems = append(elems, Elem{
					InsertPos:      atoiOrZero(key),
					TocText:        tocText,
					FileNumber:     fileNumber,
					SequenceNumber: seqNum,
					StartPos:       startPos,
					Length:         length,
				})
			}
		}
		return nil
	})
	return
}

func atoiOrZero(s string) int {
	n := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		} else {
			// non-numeric: calibre would fail, but we return 0
			break
		}
	}
	return n
}

// buildMobi8Parts mirrors calibre's build_parts flow split + skeleton/div reconstruction.
// rawML is the decompressed KF8 HTML (mobi_html).
// flowTable is from FDST or nil (fallback to single flow).
// files and elems from SKEL/DIV.
func buildMobi8Parts(rawML []byte, flowTable [][2]int, files []FileInfo, elems []Elem) (parts [][]byte, partInfos []Part, err error) {
	err = withRecover("buildMobi8Parts", func() error {
		flows := [][]byte{}
		ft := flowTable
		if len(ft) == 0 {
			ft = [][2]int{{0, len(rawML)}}
		}
		for _, se := range ft {
			start, end := se[0], se[1]
			if start < 0 {
				start = 0
			}
			if end > len(rawML) {
				end = len(rawML)
			}
			if start > end {
				start = end
			}
			flows = append(flows, rawML[start:end])
		}
		if len(flows) == 0 {
			return fmt.Errorf("no flows")
		}
		// first flow is xhtml text
		text := flows[0]
		flows[0] = nil // will be replaced

		// Walk skeleton/div to build parts
		parts = [][]byte{}
		partInfos = []Part{}
		divPtr := 0
		basePtr := 0

		for skelNum, f := range files {
			skelPos, skelLen := f.StartPos, f.Length
			basePtr = skelPos + skelLen
			if skelPos < 0 || basePtr > len(text) {
				// clamp
				if skelPos < 0 {
					skelPos = 0
				}
				if basePtr > len(text) {
					basePtr = len(text)
				}
				if skelPos > len(text) {
					skelPos = len(text)
				}
			}
			skeleton := make([]byte, basePtr-skelPos)
			copy(skeleton, text[skelPos:basePtr])

			divCnt := f.DivCount
			aidText := ""
			filename := ""
			insposWarned := false
			for i := range divCnt {
				if divPtr >= len(elems) {
					break
				}
				e := elems[divPtr]
				if i == 0 {
					// calibre: aidtext = idtext[12:-2], filename from the file number
					filename = fmt.Sprintf("part%04d.html", e.FileNumber)
					if len(e.TocText) > 14 {
						aidText = e.TocText[12 : len(e.TocText)-2]
					} else {
						aidText = e.TocText
					}
				}
				insertPos := e.InsertPos - skelPos
				// skeleton is mutable, need to adjust for already inserted parts earlier in this file
				// calibre does skeleton = skeleton[0:insertpos] + part + skeleton[insertpos:] and basePtr += length
				part := []byte{}
				partStart := basePtr
				partEnd := basePtr + e.Length
				if partStart < 0 {
					partStart = 0
				}
				if partEnd > len(text) {
					partEnd = len(text)
				}
				if partStart < partEnd {
					part = text[partStart:partEnd]
				}
				if insertPos < 0 {
					insertPos = 0
				}
				if insertPos > len(skeleton) {
					insertPos = len(skeleton)
				}
				head := skeleton[:insertPos]
				tail := skeleton[insertPos:]
				// Check incomplete tag
				if (bytes.Index(tail, []byte(">")) < bytes.Index(tail, []byte("<")) && bytes.Contains(tail, []byte("<"))) || (bytes.LastIndex(head, []byte(">")) < bytes.LastIndex(head, []byte("<"))) {
					if !insposWarned {
						insposWarned = true
					}
					// locate_beg_end_of_tag fallback
					if aidText != "" {
						bp, ep := locateBegEndOfTag(skeleton, []byte(aidText))
						if bp != 0 || ep != 0 {
							insertPos = ep + 1 + e.StartPos
							if insertPos < 0 {
								insertPos = 0
							}
							if insertPos > len(skeleton) {
								insertPos = len(skeleton)
							}
							head = skeleton[:insertPos]
							tail = skeleton[insertPos:]
						}
					}
				}
				// rebuild skeleton
				newSkeleton := make([]byte, 0, len(skeleton)+len(part))
				newSkeleton = append(newSkeleton, head...)
				newSkeleton = append(newSkeleton, part...)
				newSkeleton = append(newSkeleton, tail...)
				skeleton = newSkeleton
				basePtr = basePtr + e.Length
				divPtr++
			}
			parts = append(parts, skeleton)
			if divCnt < 1 {
				// Empty file
				aidText = fmt.Sprintf("empty-%d", skelNum)
				filename = aidText + ".html"
			}
			partInfos = append(partInfos, Part{
				Num:      skelNum,
				Type:     "text",
				Filename: filename,
				Start:    f.StartPos,
				End:      basePtr,
				Aid:      aidText,
			})
		}

		// If no files (SKEL missing), fallback to single part
		if len(parts) == 0 {
			parts = [][]byte{text}
			partInfos = []Part{{Num: 0, Type: "text", Filename: "part0000.html", Start: 0, End: len(text), Aid: ""}}
		}

		return nil
	})
	return
}

// loadSections loads all PDB records as raw bytes for INDX parsing.
// It returns a slice where index i corresponds to PDB record i.
func loadSections(pdbDb *PdbDb) (sections [][]byte, err error) {
	err = withRecover("loadSections", func() error {
		sections = make([][]byte, len(pdbDb.PdbRecords))
		for i, rec := range pdbDb.PdbRecords {
			data, err := rec.Data()
			if err != nil {
				return fmt.Errorf("failed to load section %d: %w", i, err)
			}
			// copy to avoid aliasing
			cp := make([]byte, len(data))
			copy(cp, data)
			sections[i] = cp
		}
		return nil
	})
	return
}

// extractMobi8Raw extracts the decompressed KF8 rawML.
// It handles joint vs standalone. Returns rawML, the KF8 Mobi header to use, and the offset used.
func extractMobi8Raw(pdbDb *PdbDb, mobi *Mobi, maxSize int64) (rawML []byte, kf8Mobi *Mobi, offset int, err error) {
	err = withRecover("extractMobi8Raw", func() error {
		var offsetVal int
		if mobi.KF8 != nil {
			kf8Mobi = mobi.KF8
			offsetVal = max(int(mobi.EXTH.KF8HeaderIndex())+1, 1)
		} else if mobi.MobiVersion == 8 {
			kf8Mobi = mobi
			offsetVal = 1
		} else {
			return fmt.Errorf("not a KF8 file")
		}
		raw, err := extractTextWithOffset(pdbDb, kf8Mobi, offsetVal, maxSize)
		if err != nil {
			return err
		}
		rawML = raw
		offset = offsetVal
		return nil
	})
	return
}

// aidTagRe matches an HTML/XML tag carrying an aid attribute and captures the
// attribute value. calibre compiles the same pattern per call with the aid
// interpolated into it; we compile it once and compare the captured value,
// which is equivalent (and keeps calibre's case-insensitivity).
var aidTagRe = regexp.MustCompile(`(?i)<[^>]*\said\s*=\s*['"]([^'"]*)['"][^>]*>`)

// locateBegEndOfTag mirrors calibre's locate_beg_end_of_tag: it returns the
// start and end positions of the first tag whose aid attribute equals aid, or
// (0, 0) when no such tag exists.
func locateBegEndOfTag(ml []byte, aid []byte) (int, int) {
	want := string(aid)
	for _, m := range aidTagRe.FindAllSubmatchIndex(ml, -1) {
		if !strings.EqualFold(string(ml[m[2]:m[3]]), want) {
			continue
		}
		plt := m[0]
		rest := ml[plt:]
		idx := bytes.IndexByte(rest, '>')
		if idx < 0 {
			return 0, 0
		}
		return plt, plt + idx
	}
	return 0, 0
}
