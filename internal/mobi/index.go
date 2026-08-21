package mobi

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// TagX mirrors calibre's TagX namedtuple.
type TagX struct {
	Tag         uint8
	NumOfValues uint8
	Bitmask     uint8
	Eof         uint8
}

// PTagX is internal parsed tag.
type PTagX struct {
	Tag         uint8
	ValueCount  *int
	ValueBytes  *int
	NumOfValues uint8
}

type indxHeader struct {
	Count           int
	Code            int
	Ncncx           int
	Ocnt            int
	Oentries        int
	Ordt1           int
	Ordt2           int
	Tagx            int
	IdxHeaderEndPos int
	OrdtMap         string
	Start           int
}

// parseIndxHeader mirrors calibre's parse_indx_header.
func parseIndxHeader(data []byte) (*indxHeader, error) {
	if len(data) < 4 || string(data[:4]) != "INDX" {
		return nil, fmt.Errorf("not a valid INDX section")
	}
	// INDEX_HEADER_FIELDS has 45 fields = 45*4=180 bytes after 4 byte signature -> total 184
	// But we may have shorter data; calibre unpacks exactly 45 Ls from offset 4.
	// We'll handle truncated data gracefully.
	const numFields = 45
	if len(data) < 4+numFields*4 {
		return nil, fmt.Errorf("INDX header too short: %d < %d", len(data), 4+numFields*4)
	}
	vals := make([]uint32, numFields)
	for i := 0; i < numFields; i++ {
		vals[i] = binary.BigEndian.Uint32(data[4+i*4 : 4+i*4+4])
	}
	// Mapping per INDEX_HEADER_FIELDS
	// ('len','nul1','type','gen','start','count','code','lng','total','ordt','ligt','nligt','ncncx') + 27 unknowns + ('ocnt','oentries','ordt1','ordt2','tagx')
	// Indices: 0=len,1=nul1,2=type,3=gen,4=start,5=count,6=code,7=lng,8=total,9=ordt,10=ligt,11=nligt,12=ncncx, 13-39 = unknown0-26, 40=ocnt,41=oentries,42=ordt1,43=ordt2,44=tagx
	h := &indxHeader{
		Start:           int(vals[4]),
		Count:           int(vals[5]),
		Code:            int(vals[6]),
		Ncncx:           int(vals[12]),
		Ocnt:            int(vals[40]),
		Oentries:        int(vals[41]),
		Ordt1:           int(vals[42]),
		Ordt2:           int(vals[43]),
		Tagx:            int(vals[44]),
		IdxHeaderEndPos: 4 + numFields*4,
		OrdtMap:         "",
	}
	// Handle ORDT maps (rare, calibre handles ebcdic 65002)
	// We only need OrdtMap for decode_string ordt mapping; for most books it's empty.
	// Check ordt1
	if h.Ordt1 > 0 && h.Ordt1+4 <= len(data) && string(data[h.Ordt1:h.Ordt1+4]) == "ORDT" {
		if h.Code == 65002 {
			// calibre builds parsed map from every second byte; for ordt1 we
			// use the fallback '?' * oentries (ordt2 below handles the real map)
			h.OrdtMap = string(bytes.Repeat([]byte{'?'}, h.Oentries))
		}
	}
	if h.Ordt2 > 0 && h.Ordt2+4 <= len(data) && string(data[h.Ordt2:h.Ordt2+4]) == "ORDT" {
		if h.Code == 65002 {
			rawEnd := h.Ordt2 + 4 + 2*h.Oentries
			if rawEnd <= len(data) {
				raw := data[h.Ordt2+4 : rawEnd]
				parsed := make([]byte, h.Oentries)
				for i := 0; i < 2*h.Oentries; i += 2 {
					b := raw[i+1]
					if b > 0x20 && b < 0x7F {
						parsed[i/2] = b
					} else {
						parsed[i/2] = '?'
					}
				}
				h.OrdtMap = string(parsed)
			} else {
				h.OrdtMap = string(bytes.Repeat([]byte{'?'}, h.Oentries))
			}
		} else {
			h.OrdtMap = string(bytes.Repeat([]byte{'?'}, h.Oentries))
		}
	}
	return h, nil
}

func parseTagxSection(data []byte) (int, []TagX, error) {
	if len(data) < 12 || string(data[:4]) != "TAGX" {
		return 0, nil, fmt.Errorf("not a valid TAGX section")
	}
	firstEntryOffset := int(binary.BigEndian.Uint32(data[4:8]))
	controlByteCount := int(binary.BigEndian.Uint32(data[8:12]))
	var tags []TagX
	for i := 12; i < firstEntryOffset; i += 4 {
		if i+4 > len(data) {
			break
		}
		tags = append(tags, TagX{
			Tag:         data[i],
			NumOfValues: data[i+1],
			Bitmask:     data[i+2],
			Eof:         data[i+3],
		})
	}
	return controlByteCount, tags, nil
}

func getTagMap(controlByteCount int, tagx []TagX, data []byte) (map[uint32][]int, []byte, error) {
	// Mirrors calibre's get_tag_map
	ans := make(map[uint32][]int)
	if controlByteCount > len(data) {
		return ans, data, nil
	}
	controlBytes := make([]byte, controlByteCount)
	copy(controlBytes, data[:controlByteCount])
	rest := data[controlByteCount:]

	var ptags []PTagX
	for _, x := range tagx {
		if x.Eof == 0x01 {
			if len(controlBytes) > 0 {
				controlBytes = controlBytes[1:]
			}
			continue
		}
		if len(controlBytes) == 0 {
			continue
		}
		value := controlBytes[0] & x.Bitmask
		if value != 0 {
			var valueCount *int
			var valueBytes *int
			if value == x.Bitmask {
				if countSetBits(int(x.Bitmask)) > 1 {
					vb, consumed := decInt(rest, true)
					rest = rest[consumed:]
					valueBytes = &vb
					_ = consumed
				} else {
					vc := 1
					valueCount = &vc
				}
			} else {
				mask := int(x.Bitmask)
				v := int(value)
				for mask&1 == 0 {
					mask >>= 1
					v >>= 1
				}
				valueCount = &v
			}
			ptags = append(ptags, PTagX{
				Tag:         x.Tag,
				ValueCount:  valueCount,
				ValueBytes:  valueBytes,
				NumOfValues: x.NumOfValues,
			})
		}
	}

	for _, x := range ptags {
		var values []int
		if x.ValueCount != nil {
			for i := 0; i < *x.ValueCount*int(x.NumOfValues); i++ {
				byts, consumed := decInt(rest, true)
				rest = rest[consumed:]
				values = append(values, byts)
			}
		} else if x.ValueBytes != nil {
			totalConsumed := 0
			for totalConsumed < *x.ValueBytes {
				byts, consumed := decInt(rest, true)
				rest = rest[consumed:]
				totalConsumed += consumed
				values = append(values, byts)
			}
		}
		ans[uint32(x.Tag)] = values
	}
	// Ignore leftover bytes warning
	return ans, rest, nil
}

func parseIndexRecord(table map[string]map[uint32][]int, ordered *[]string, data []byte, controlByteCount int, tags []TagX, codec string, ordtMap string) error {
	hdr, err := parseIndxHeader(data)
	if err != nil {
		return err
	}
	idxtPos := hdr.Start
	// calibre prints a warning on missing IDXT but continues regardless
	entryCount := hdr.Count
	idxPositions := make([]int, 0, entryCount+1)
	for j := 0; j < entryCount; j++ {
		off := idxtPos + 4 + 2*j
		if off+2 > len(data) {
			break
		}
		pos := int(binary.BigEndian.Uint16(data[off : off+2]))
		idxPositions = append(idxPositions, pos)
	}
	idxPositions = append(idxPositions, idxtPos)

	for j := 0; j < entryCount && j < len(idxPositions)-1; j++ {
		startPos := idxPositions[j]
		endPos := idxPositions[j+1]
		if startPos < 0 || endPos > len(data) || startPos >= endPos {
			continue
		}
		rec := data[startPos:endPos]
		// decode ident
		ident, consumed, err := decodeIdent(rec, codec, ordtMap)
		if err != nil {
			// try utf-16 fallback?
			continue
		}
		rec = rec[consumed:]
		tagMap, _, err := getTagMap(controlByteCount, tags, rec)
		if err != nil {
			continue
		}
		if ordered != nil {
			if _, exists := table[ident]; !exists {
				*ordered = append(*ordered, ident)
			}
		}
		table[ident] = tagMap
	}
	return nil
}

func decodeIdent(rec []byte, codec string, ordtMap string) (string, int, error) {
	if len(rec) == 0 {
		return "", 0, fmt.Errorf("empty rec")
	}
	// calibre's decode_string: first byte via decint? Actually for INDX ident it's utils.decode_string which uses '>B' length.
	// But we have ordtMap handling: if ordtMap != "" then mapping.
	// For simplicity, we implement both: try decInt length if ordt? No, calibre uses decode_string with '>B'.
	// Let's use byte length as in varint.go decodeStringWithByteLength, but with ordt support.
	// For ordtMap case, we need to map bytes via ordtMap.
	if ordtMap != "" {
		// ordtMap is string of length oentries, mapping byte values
		length, consumed := decInt(rec, true) // Actually calibre uses decint? Check: decode_string uses '>B' but with ordt it uses mapped bytes
		// In calibre's parse_index_record they call decode_string(rec, codec, ordt_map)
		// decode_string does: length = unpack('>B', raw[0:1]); raw = raw[1:1+length]; if ordt_map: return ''.join(ordt_map[x] for x in bytearray(raw)), consumed
		// So it's byte length, not varint.
		// We'll do byte length.
		lengthB := int(rec[0])
		if 1+lengthB > len(rec) {
			return "", 0, fmt.Errorf("ident length exceeds rec")
		}
		raw := rec[1 : 1+lengthB]
		consumedB := 1 + lengthB
		// map via ordt
		out := make([]byte, len(raw))
		for i, b := range raw {
			if int(b) < len(ordtMap) {
				out[i] = ordtMap[b]
			} else {
				out[i] = '?'
			}
		}
		_ = length
		_ = consumed
		return string(out), consumedB, nil
	}
	// Normal path: codec decode with byte length
	s, consumed, err := decodeStringWithByteLength(rec, codec)
	if err != nil {
		return "", 0, err
	}
	// calibre also handles '\x00' in ident -> try utf-16
	if bytes.Contains([]byte(s), []byte{0}) {
		// Try utf-16 decode? Simplify: replace
		s = string(bytes.ReplaceAll([]byte(s), []byte{0}, []byte{}))
	}
	return s, consumed, nil
}

func getTagSectionStart(data []byte, hdr *indxHeader) int {
	tagSectionStart := hdr.Tagx
	if tagSectionStart+4 <= len(data) && string(data[tagSectionStart:tagSectionStart+4]) == "TAGX" {
		return tagSectionStart
	}
	// search from idxHeaderEndPos
	searchStart := hdr.IdxHeaderEndPos
	if searchStart < 0 {
		searchStart = 0
	}
	if searchStart >= len(data) {
		return tagSectionStart
	}
	idx := bytes.Index(data[searchStart:], []byte("TAGX"))
	if idx >= 0 {
		return searchStart + idx
	}
	return tagSectionStart
}

// ReadIndex mirrors calibre's read_index(sections, idx, codec)
// sections is slice of raw PDB record data (ordered), idx is 0-based PDB record index (KF8-relative).
// codec is "utf-8" or "cp1252".
// Returns ordered table (map + ordered keys) and CNCX.
func ReadIndex(sections [][]byte, idx int, codec string) (map[string]map[uint32][]int, []string, CNCX, error) {
	table := make(map[string]map[uint32][]int)
	var ordered []string
	cncx := make(CNCX)

	if idx < 0 || idx >= len(sections) {
		return table, ordered, cncx, fmt.Errorf("index %d out of range %d", idx, len(sections))
	}
	data := sections[idx]
	hdr, err := parseIndxHeader(data)
	if err != nil {
		return table, ordered, cncx, err
	}
	indxCount := hdr.Count

	if hdr.Ncncx > 0 {
		off := idx + indxCount + 1
		var cncxRecords [][]byte
		for i := off; i < off+hdr.Ncncx && i < len(sections); i++ {
			cncxRecords = append(cncxRecords, sections[i])
		}
		c, err := parseCNCX(cncxRecords, codec)
		if err == nil {
			cncx = c
		}
	}

	tagSectionStart := getTagSectionStart(data, hdr)
	if tagSectionStart+4 > len(data) {
		return table, ordered, cncx, fmt.Errorf("TAGX not found")
	}
	controlByteCount, tags, err := parseTagxSection(data[tagSectionStart:])
	if err != nil {
		return table, ordered, cncx, err
	}

	for i := idx + 1; i < idx+1+indxCount && i < len(sections); i++ {
		recData := sections[i]
		_ = parseIndexRecord(table, &ordered, recData, controlByteCount, tags, codec, hdr.OrdtMap)
	}

	return table, ordered, cncx, nil
}
