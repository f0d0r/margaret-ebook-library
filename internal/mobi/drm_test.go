package mobi

import (
	"encoding/binary"
	"errors"
	"testing"

	"github.com/f0d0r/margaret-ebook-library/book"
	"github.com/f0d0r/margaret-ebook-library/internal/config"
)

func TestIsDRMProtected(t *testing.T) {
	tests := []struct {
		name string
		mobi *Mobi
		want bool
	}{
		{"nil", nil, false},
		{"clean", &Mobi{DRMOffset: 0xFFFFFFFF, Encryption: EncryptionNone}, false},
		{"drm offset", &Mobi{DRMOffset: 0, Encryption: EncryptionNone}, true},
		{"drm offset max-1", &Mobi{DRMOffset: 0xFFFFFFFE, Encryption: EncryptionNone}, true},
		{"old encryption", &Mobi{DRMOffset: 0xFFFFFFFF, Encryption: EncryptionOldMobi}, true},
		{"mobi encryption", &Mobi{DRMOffset: 0xFFFFFFFF, Encryption: EncryptionMobi}, true},
		{"both", &Mobi{DRMOffset: 1000, Encryption: EncryptionMobi}, true},
		{
			"clean with clean kf8",
			&Mobi{DRMOffset: 0xFFFFFFFF, Encryption: EncryptionNone,
				KF8: &Mobi{DRMOffset: 0xFFFFFFFF, Encryption: EncryptionNone}},
			false,
		},
		{
			"kf8 drm offset",
			&Mobi{DRMOffset: 0xFFFFFFFF, Encryption: EncryptionNone,
				KF8: &Mobi{DRMOffset: 0, Encryption: EncryptionNone}},
			true,
		},
		{
			"kf8 encryption",
			&Mobi{DRMOffset: 0xFFFFFFFF, Encryption: EncryptionNone,
				KF8: &Mobi{DRMOffset: 0xFFFFFFFF, Encryption: EncryptionMobi}},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isDRMProtected(tt.mobi); got != tt.want {
				t.Errorf("isDRMProtected() = %v, want %v", got, tt.want)
			}
		})
	}
}

// buildDRMTestMOBI returns a minimal parseable PDB+MOBI blob. When drmOffset
// is non-nil the record is extended to 184 bytes and the DRM offset field
// is set, mirroring a DRMed file.
func buildDRMTestMOBI(encryption uint16, drmOffset *uint32) []byte {
	const pdbHeaderSize = 78

	header := make([]byte, pdbHeaderSize)
	copy(header[0:], "TestBook")
	copy(header[60:64], "BOOK")
	copy(header[64:68], "MOBI")
	binary.BigEndian.PutUint32(header[68:72], 1)
	binary.BigEndian.PutUint16(header[76:78], 1)

	recordInfo := make([]byte, 8)
	binary.BigEndian.PutUint32(recordInfo[0:4], uint32(pdbHeaderSize+8))
	copy(recordInfo[5:8], []byte{0x00, 0x00, 0x01})

	size := 100
	if drmOffset != nil {
		size = 184
	}
	mobiData := make([]byte, size)
	copy(mobiData[16:20], "MOBI")
	binary.BigEndian.PutUint32(mobiData[20:24], 232)
	binary.BigEndian.PutUint16(mobiData[0:2], 1)
	binary.BigEndian.PutUint32(mobiData[4:8], 1000)
	binary.BigEndian.PutUint16(mobiData[8:10], 1)
	binary.BigEndian.PutUint16(mobiData[10:12], 4096)
	binary.BigEndian.PutUint32(mobiData[24:28], 2)
	binary.BigEndian.PutUint32(mobiData[28:32], 65001)
	binary.BigEndian.PutUint16(mobiData[12:14], encryption)
	if drmOffset != nil {
		binary.BigEndian.PutUint32(mobiData[168:172], *drmOffset)
	}

	var buf []byte
	buf = append(buf, header...)
	buf = append(buf, recordInfo...)
	buf = append(buf, mobiData...)
	return buf
}

func TestMobiReaderRead_RejectsEncryption(t *testing.T) {
	for _, enc := range []uint16{uint16(EncryptionOldMobi), uint16(EncryptionMobi)} {
		data := buildDRMTestMOBI(enc, nil)
		_, err := NewMobiReader(config.DefaultConfig()).Read(book.NewBytesBlob(data))
		if !errors.Is(err, book.ErrDRM) {
			t.Errorf("encryption=%d: Read() error = %v, want ErrDRM", enc, err)
		}
	}
}

func TestMobiReaderRead_RejectsDRMOffset(t *testing.T) {
	zero := uint32(0)
	for _, off := range []uint32{0, 1000, 0xFFFFFFFE} {
		o := off
		_ = zero
		data := buildDRMTestMOBI(uint16(EncryptionNone), &o)
		_, err := NewMobiReader(config.DefaultConfig()).Read(book.NewBytesBlob(data))
		if !errors.Is(err, book.ErrDRM) {
			t.Errorf("drmOffset=%d: Read() error = %v, want ErrDRM", o, err)
		}
	}
}

func TestMobiReaderRead_AcceptsClean(t *testing.T) {
	data := buildDRMTestMOBI(uint16(EncryptionNone), nil)
	ebook, err := NewMobiReader(config.DefaultConfig()).Read(book.NewBytesBlob(data))
	if err != nil {
		t.Fatalf("Read() error = %v, want nil", err)
	}
	if ebook.FileType() != book.MOBI {
		t.Errorf("FileType = %q, want MOBI", ebook.FileType())
	}
}

// Supports stays DRM-agnostic: the BOOKMOBI magic is present even in
// locked files; only Read rejects them.
func TestMobiReaderSupports_DRMFile(t *testing.T) {
	data := buildDRMTestMOBI(uint16(EncryptionMobi), nil)
	if !NewMobiReader(config.DefaultConfig()).Supports(book.NewBytesBlob(data)) {
		t.Error("Supports() = false for DRM MOBI, want true (Read must reject, not Supports)")
	}
}
