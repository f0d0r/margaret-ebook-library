package mobi

import (
	"reflect"
	"testing"

	"github.com/f0d0r/margaret-ebook-library/book"
	"github.com/f0d0r/margaret-ebook-library/internal/config"
)

func TestReadMobiSubjects(t *testing.T) {
	text := []byte("<html><body>content</body></html>")
	opts := defaultMobiHeaderOpts()
	opts.textLength = uint32(len(text))
	opts.textRecordCount = 1
	opts.exthRecords = map[uint32][]byte{
		SUBJECT: []byte("Fantasy; Adventure"),
	}
	raw := makePdbBlob(t, "SubjectTest", []recordSpec{
		makeMobiHeaderRecord(opts),
		{data: text},
	})

	ebook, err := NewMobiReader(config.DefaultConfig()).Read(book.NewBytesBlob(raw))
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}

	want := []string{"Fantasy", "Adventure"}
	if got := ebook.Metadata().Subjects; !reflect.DeepEqual(got, want) {
		t.Errorf("Subjects = %#v, want %#v", got, want)
	}
}

func TestReadKF8Subjects(t *testing.T) {
	blob := makeStandaloneKF8Blob(t, func(records *[]recordSpec, opts *mobiHeaderOpts) {
		opts.exthRecords = map[uint32][]byte{
			SUBJECT: []byte("Science; History"),
		}
	})

	ebook, err := NewMobiReader(config.DefaultConfig()).Read(book.NewBytesBlob(blob))
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}

	want := []string{"Science", "History"}
	if got := ebook.Metadata().Subjects; !reflect.DeepEqual(got, want) {
		t.Errorf("Subjects = %#v, want %#v", got, want)
	}
}
