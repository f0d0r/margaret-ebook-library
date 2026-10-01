package compress

import (
	"bytes"
	"io"
	"testing"
)

// FuzzLZXDecode fuzzes the streaming decoder for panics and hangs. Output
// is bounded by maxOut and zero-block runs are capped, so arbitrary input
// terminates quickly with either bytes or an error.
func FuzzLZXDecode(f *testing.F) {
	f.Add([]byte{
		0x00, 0x30, 0x10, 0x00,
		0x01, 0x00, 0x00, 0x00,
		0x01, 0x00, 0x00, 0x00,
		0x01, 0x00, 0x00, 0x00,
		0x41,
	}, 15, int64(1))
	f.Add([]byte{0x00}, 15, int64(0))
	f.Add([]byte{0x00, 0x00, 0x00, 0x00}, 21, int64(100))
	f.Fuzz(func(t *testing.T, data []byte, windowBits int, outLen int64) {
		if windowBits < 15 || windowBits > 21 {
			t.Skip("window out of range")
		}
		if outLen < -1 || outLen > 1<<20 {
			t.Skip("length out of range")
		}
		if len(data) > 1<<16 {
			t.Skip("input too large")
		}
		rc, err := NewLZXReader(bytes.NewReader(data), windowBits, outLen, 1<<20)
		if err != nil {
			return
		}
		defer func() { _ = rc.Close() }()
		_, _ = io.ReadAll(io.LimitReader(rc, 2<<20))
	})
}
