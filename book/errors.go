package book

import (
	"errors"

	"github.com/f0d0r/margaret-ebook-library/internal/util"
)

var (
	// ErrUnsupportedFormat is returned when the application encounters a file
	// format that it does not recognize or know how to parse.
	ErrUnsupportedFormat = errors.New("unsupported format")

	// ErrLimitExceeded is returned when the underlying reader produces more data
	// than the configured safety maximum, protecting against zip-bomb style e-books.
	ErrLimitExceeded = util.ErrLimitExceeded

	// ErrNoTransformer is returned when no Transformer is registered for a
	// requested MIME type conversion.
	ErrNoTransformer = errors.New("no transformer")

	// ErrUnsupportedMediaType is returned when a resource has an empty or
	// otherwise unsupported media type for conversion.
	ErrUnsupportedMediaType = errors.New("unsupported media type")
)
