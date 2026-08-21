package mobi

import (
	"bytes"
	"fmt"
	"regexp"
)

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
	// FDST
	if mobi.FdstIdx != NullIndex {
		if int(mobi.FdstIdx) < len(sections) {
			fdstData := sections[mobi.FdstIdx]
			if len(fdstData) >= 4 && string(fdstData[:4]) == "FDST" {
				ft, e := parseFDST(fdstData)
				if e != nil {
					return nil, nil, nil, fmt.Errorf("FDST parse failed: %w", e)
				}
				flowTable = ft
			}
		}
	}

	// SKEL
	if mobi.SkelIdx != NullIndex && int(mobi.SkelIdx) < len(sections) {
		table, ordered, _, err := ReadIndex(sections, int(mobi.SkelIdx), codec)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("SKEL read failed: %w", err)
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
			if v, ok := tagMap[6]; ok && len(v) >= 2 {
				startPos = v[0]
				length = v[1]
			} else if v, ok := tagMap[6]; ok && len(v) == 1 {
				startPos = v[0]
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
			return nil, nil, nil, fmt.Errorf("DIV read failed: %w", err)
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
			// key is text id (int as string)
			// calibre does int(text)
			// we keep as inserted order; int conversion not needed for building
			elems = append(elems, Elem{
				InsertPos:      atoiOrZero(key), // calibre: int(text)
				TocText:        tocText,
				FileNumber:     fileNumber,
				SequenceNumber: seqNum,
				StartPos:       startPos,
				Length:         length,
			})
		}
		// Also need to handle case where ordered is numeric strings but table keys are those numbers
		_ = cncx
	}
	return flowTable, files, elems, nil
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
		return nil, nil, fmt.Errorf("no flows")
	}
	// first flow is xhtml text
	text := flows[0]
	flows[0] = nil // will be replaced

	// Walk skeleton/div to build parts
	parts = [][]byte{}
	partInfos = []Part{}
	divPtr := 0
	basePtr := 0

	// For aid fallback when divcnt==0
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
				// idtext[12:-2] in calibre – that's the aid without prefix/suffix? For filename we use filenum
				filename = fmt.Sprintf("part%04d.html", e.FileNumber)
				// aidText is e.TocText? Actually calibre: aidtext = idtext[12:-2] where idtext = toc_text
				// toc_text is like '<a id="...">'? Let's store TocText as aid
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

	return parts, partInfos, nil
}

// loadSections loads all PDB records as raw bytes for INDX parsing.
// It returns a slice where index i corresponds to PDB record i.
func loadSections(pdbDb *PdbDb) ([][]byte, error) {
	sections := make([][]byte, len(pdbDb.PdbRecords))
	for i, rec := range pdbDb.PdbRecords {
		data, err := rec.Data()
		if err != nil {
			return nil, fmt.Errorf("failed to load section %d: %w", i, err)
		}
		// copy to avoid aliasing
		cp := make([]byte, len(data))
		copy(cp, data)
		sections[i] = cp
	}
	return sections, nil
}

// extractMobi8Raw extracts the decompressed KF8 rawML.
// It handles joint vs standalone. Returns rawML, the KF8 Mobi header to use, and the offset used.
func extractMobi8Raw(pdbDb *PdbDb, mobi *Mobi, maxSize int64) ([]byte, *Mobi, int, error) {
	var kf8 *Mobi
	var offset int
	if mobi.KF8 != nil {
		kf8 = mobi.KF8
		offset = max(int(mobi.EXTH.KF8HeaderIndex())+1, 1)
	} else if mobi.MobiVersion == 8 {
		kf8 = mobi
		offset = 1
	} else {
		return nil, nil, 0, fmt.Errorf("not a KF8 file")
	}
	raw, err := extractTextWithOffset(pdbDb, kf8, offset, maxSize)
	if err != nil {
		return nil, nil, 0, err
	}
	return raw, kf8, offset, nil
}

// locateBegEndOfTag mirrors calibre's locate_beg_end_of_tag.
func locateBegEndOfTag(ml []byte, aid []byte) (int, int) {
	pattern := `<[^>]*\said\s*=\s*['"]` + regexp.QuoteMeta(string(aid)) + `['"][^>]*>`
	re := regexp.MustCompile(pattern)
	loc := re.FindIndex(ml)
	if loc == nil {
		return 0, 0
	}
	plt := loc[0]
	// find '>' after plt
	rest := ml[plt:]
	idx := bytes.IndexByte(rest, '>')
	if idx < 0 {
		return 0, 0
	}
	pgt := plt + idx
	return plt, pgt
}
