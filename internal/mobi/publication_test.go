package mobi

import (
	"testing"

	"github.com/f0d0r/margaret-ebook-library/book"
	"github.com/f0d0r/margaret-ebook-library/internal/config"
)

func TestReadKF8PublisherAndPublicationDate(t *testing.T) {
	reader := NewMobiReader(config.DefaultConfig())
	blob := makeStandaloneKF8Blob(t, func(records *[]recordSpec, opts *mobiHeaderOpts) {
		opts.exthRecords = map[uint32][]byte{
			PUBLISHER:        []byte("KF8 Press"),
			PUBLICATION_DATE: []byte("2001-02-03"),
		}
	})

	ebook, err := reader.Read(book.NewBytesBlob(blob))
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}

	if got := ebook.Metadata().Publisher; got != "KF8 Press" {
		t.Errorf("Publisher = %q, want %q", got, "KF8 Press")
	}
	if got := ebook.Metadata().PublicationDate; got != "2001-02-03" {
		t.Errorf("PublicationDate = %q, want %q", got, "2001-02-03")
	}
}

func TestPalmDocPublisherAndPublicationDate(t *testing.T) {
	payload := []byte(`<HTML><HEAD><metadata><dc-metadata>` +
		`<dc:Title>Test Palm Book</dc:Title>` +
		`<dc:Creator>Jane Author</dc:Creator>` +
		`<dc:Publisher>Palm Press</dc:Publisher>` +
		`<dc:Date>1997-06-01</dc:Date>` +
		`</dc-metadata></metadata></HEAD><BODY><P>Hello PalmDOC</P></BODY></HTML>`)
	raw := buildPalmDocBookFile("PalmPub", payload, palmDocBookOpt{compression: 2})

	ebook, err := NewMobiReader(config.DefaultConfig()).Read(book.NewBytesBlob(raw))
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}

	if got := ebook.Metadata().Publisher; got != "Palm Press" {
		t.Errorf("Publisher = %q, want %q", got, "Palm Press")
	}
	if got := ebook.Metadata().PublicationDate; got != "1997-06-01" {
		t.Errorf("PublicationDate = %q, want %q", got, "1997-06-01")
	}
}

func TestReadPublisherAndPublicationDate(t *testing.T) {
	reader := NewMobiReader(config.DefaultConfig())
	text := []byte("<html><body>content</body></html>")

	tests := []struct {
		name            string
		exthRecords     map[uint32][]byte
		wantPublisher   string
		wantPublication string
	}{
		{
			name: "EXTH publisher and date",
			exthRecords: map[uint32][]byte{
				PUBLISHER:        []byte("Acme Press"),
				PUBLICATION_DATE: []byte("2000-01-01T00:00:00Z"),
			},
			wantPublisher:   "Acme Press",
			wantPublication: "2000-01-01T00:00:00Z",
		},
		{
			name: "raw date preserved",
			exthRecords: map[uint32][]byte{
				PUBLISHER:        []byte("Acme Press"),
				PUBLICATION_DATE: []byte("circa 1999"),
			},
			wantPublisher:   "Acme Press",
			wantPublication: "circa 1999",
		},
		{
			name: "missing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := defaultMobiHeaderOpts()
			opts.textLength = uint32(len(text))
			opts.textRecordCount = 1
			if tt.exthRecords != nil {
				opts.exthRecords = tt.exthRecords
			}

			raw := makePdbBlob(t, "PubTest", []recordSpec{
				makeMobiHeaderRecord(opts),
				{data: text},
			})
			ebook, err := reader.Read(book.NewBytesBlob(raw))
			if err != nil {
				t.Fatalf("Read() error: %v", err)
			}

			if got := ebook.Metadata().Publisher; got != tt.wantPublisher {
				t.Errorf("Publisher = %q, want %q", got, tt.wantPublisher)
			}
			if got := ebook.Metadata().PublicationDate; got != tt.wantPublication {
				t.Errorf("PublicationDate = %q, want %q", got, tt.wantPublication)
			}
		})
	}
}
