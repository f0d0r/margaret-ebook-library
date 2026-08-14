package errs

import "errors"

var (
	// ErrUnsupportedFormat is returned when the application encounters a file
	// format that it does not recognize or know how to parse.
	ErrUnsupportedFormat = errors.New("unsupported format")

	// ErrInvalidOffset is returned when the application encounters an invalid
	// offset into the data.
	ErrInvalidOffset = errors.New("invalid offset")
)
