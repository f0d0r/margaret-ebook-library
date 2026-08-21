package mobi

import (
	"encoding/binary"
	"fmt"
)

// parseFDST parses the FDST record.
// data is the raw FDST record (sections[fdstidx]).
// Returns flow table as slice of [start,end] pairs.
func parseFDST(data []byte) ([][2]int, error) {
	if len(data) < 8 || string(data[:4]) != "FDST" {
		return nil, fmt.Errorf("not a valid FDST record")
	}
	if len(data) < 12 {
		return nil, fmt.Errorf("FDST too short")
	}
	secStart := int(binary.BigEndian.Uint32(data[4:8]))
	numSections := int(binary.BigEndian.Uint32(data[8:12]))
	if numSections <= 0 {
		return nil, nil
	}
	if secStart+numSections*8 > len(data) {
		return nil, fmt.Errorf("FDST secStart %d + %d*8 exceeds data %d", secStart, numSections, len(data))
	}
	secs := make([]uint32, numSections*2)
	for i := 0; i < numSections*2; i++ {
		secs[i] = binary.BigEndian.Uint32(data[secStart+i*4 : secStart+i*4+4])
	}
	flow := make([][2]int, numSections)
	for i := range numSections {
		flow[i][0] = int(secs[i*2])
		flow[i][1] = int(secs[i*2+1])
	}
	return flow, nil
}
