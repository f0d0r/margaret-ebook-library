package mobi

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/f0d0r/margaret-ebook-library/book"
	"github.com/f0d0r/margaret-ebook-library/internal/pdb"
)

// panicError marks failures recovered from panics on malformed input.
type panicError struct {
	msg string
}

func (e *panicError) Error() string { return e.msg }

// withRecover converts panics on untrusted input into errors.
func withRecover(name string, fn func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = &panicError{msg: fmt.Sprintf("panic in %s: %v", name, r)}
		}
	}()
	return fn()
}

// kf8ContentError reports recovered panics as corruption, other failures as parse errors.
func kf8ContentError(err error) error {
	var pe *panicError
	if errors.As(err, &pe) {
		return fmt.Errorf("%w: KF8 parsing failed: %w", book.ErrCorrupt, err)
	}
	return fmt.Errorf("KF8 parsing failed: %w", err)
}

// Part describes one reconstructed document part.
type Part struct {
	Num      int
	Type     string
	Filename string
	Start    int
	End      int
	Aid      string
}

// Elem describes a skeleton/div insertion.
type Elem struct {
	InsertPos      int
	TocText        string
	FileNumber     int
	SequenceNumber int
	StartPos       int
	Length         int
}

// FileInfo describes a skeleton file.
type FileInfo struct {
	FileNumber int
	Name       string
	DivCount   int
	StartPos   int
	Length     int
}

// FlowInfo describes a flow split.
type FlowInfo struct {
	Type   string
	Format string
	Dir    string
	Fname  string
}

// readMobi8Indices reads the FDST, SKEL and DIV tables from KF8-relative sections.
func readMobi8Indices(sections [][]byte, mobi *Mobi, codec string) (flowTable [][2]int, files []FileInfo, elems []Elem, err error) {
	err = withRecover("readMobi8Indices", func() error {
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

		if mobi.SkelIdx != NullIndex && int(mobi.SkelIdx) < len(sections) {
			table, ordered, _, err := ReadIndex(sections, int(mobi.SkelIdx), codec)
			if err != nil {
				return fmt.Errorf("SKEL read failed: %w", err)
			}
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
				// key is the text id; only the insert position needs its integer value.
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
			// non-numeric: return 0
			break
		}
	}
	return n
}

// buildMobi8Parts splits flows and reconstructs skeletons and divs.
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
		flows[0] = nil

		parts = [][]byte{}
		partInfos = []Part{}
		divPtr := 0
		basePtr := 0

		for skelNum, f := range files {
			skelPos, skelLen := f.StartPos, f.Length
			basePtr = skelPos + skelLen
			if skelPos < 0 || basePtr > len(text) {
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
					// aidtext = idtext[12:-2], filename from the file number
					filename = fmt.Sprintf("part%04d.html", e.FileNumber)
					if len(e.TocText) > 14 {
						aidText = e.TocText[12 : len(e.TocText)-2]
					} else {
						aidText = e.TocText
					}
				}
				insertPos := e.InsertPos - skelPos
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
				if (bytes.Index(tail, []byte(">")) < bytes.Index(tail, []byte("<")) && bytes.Contains(tail, []byte("<"))) || (bytes.LastIndex(head, []byte(">")) < bytes.LastIndex(head, []byte("<"))) {
					if !insposWarned {
						insposWarned = true
					}
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
func loadSections(pdbDb *pdb.PdbDb) (sections [][]byte, err error) {
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

// extractMobi8Raw extracts the decompressed KF8 rawML, handling joint vs standalone.
func extractMobi8Raw(pdbDb *pdb.PdbDb, mobi *Mobi, maxSize int64) (rawML []byte, kf8Mobi *Mobi, offset int, err error) {
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

// aidTagRe matches a tag carrying an aid attribute and captures its value (case-insensitive).
var aidTagRe = regexp.MustCompile(`(?i)<[^>]*\said\s*=\s*['"]([^'"]*)['"][^>]*>`)

// locateBegEndOfTag returns the bounds of the first tag with the given aid, or (0, 0).
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
