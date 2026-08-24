package book

import (
	"bytes"
	"io"
	"os"
)

// Blob provides random access to the raw bytes of an e-book file. It
// abstracts over local files, e-book entries inside archives, and remote
// sources, so callers never need to materialize a temporary file.
type Blob interface {
	io.ReaderAt
	Size() (int64, error)
}

// NewFileBlob adapts an already-open file to a Blob. The caller retains
// ownership of the file: the file position is never modified and the file
// must be kept open for the lifetime of the returned Blob.
func NewFileBlob(f *os.File) (Blob, error) {
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	return fileBlob{f: f, size: info.Size()}, nil
}

// NewPathBlob adapts a file path to a Blob that reopens the file on every
// access, so no file handle is retained after the Blob is created.
func NewPathBlob(path string) Blob {
	return pathBlob{path: path}
}

// NewBytesBlob adapts an in-memory byte slice to a Blob.
func NewBytesBlob(data []byte) Blob {
	return bytesBlob{data: data}
}

// NewReaderAtBlob adapts an io.ReaderAt and declared size to a Blob.
func NewReaderAtBlob(r io.ReaderAt, size int64) Blob {
	return readerAtBlob{r: r, size: size}
}

type fileBlob struct {
	f    *os.File
	size int64
}

func (b fileBlob) ReadAt(p []byte, off int64) (int, error) {
	return b.f.ReadAt(p, off)
}

func (b fileBlob) Size() (int64, error) {
	return b.size, nil
}

type pathBlob struct {
	path string
}

func (b pathBlob) ReadAt(p []byte, off int64) (int, error) {
	f, err := os.Open(b.path)
	if err != nil {
		return 0, err
	}
	defer func() { _ = f.Close() }()
	return f.ReadAt(p, off)
}

func (b pathBlob) Size() (int64, error) {
	f, err := os.Open(b.path)
	if err != nil {
		return 0, err
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

type bytesBlob struct {
	data []byte
}

func (b bytesBlob) ReadAt(p []byte, off int64) (int, error) {
	return bytes.NewReader(b.data).ReadAt(p, off)
}

func (b bytesBlob) Size() (int64, error) {
	return int64(len(b.data)), nil
}

type readerAtBlob struct {
	r    io.ReaderAt
	size int64
}

func (b readerAtBlob) ReadAt(p []byte, off int64) (int, error) {
	return b.r.ReadAt(p, off)
}

func (b readerAtBlob) Size() (int64, error) {
	return b.size, nil
}
