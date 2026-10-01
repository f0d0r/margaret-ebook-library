package lit

import (
	"testing"
)

// FuzzUnbinary fuzzes the binary markup decoder for panics and hangs.
// Every state consumes input or errors, so arbitrary input terminates.
func FuzzUnbinary(f *testing.F) {
	f.Add([]byte{0x00, 0x01, 0x01, 0x00, 0x00, 0x02, 0x00})
	f.Add([]byte("plain text"))
	f.Add([]byte{0x00})
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 1<<16 {
			t.Skip("input too large")
		}
		_, _ = decodeUnbinary(data, &htmlTables, nil, nil, "")
		_, _ = decodeUnbinary(data, &opfTables, nil, nil, "")
	})
}

// FuzzManifest fuzzes the /manifest parser for panics and hangs.
// Every entry consumes input or errors, so arbitrary input terminates.
func FuzzManifest(f *testing.F) {
	f.Add([]byte{0x01, 'r', 0, 0, 0, 0, 0})
	f.Add([]byte{0x00})
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 1<<16 {
			t.Skip("input too large")
		}
		_, _ = readManifest(data)
	})
}
