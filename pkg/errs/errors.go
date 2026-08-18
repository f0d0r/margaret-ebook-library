package errs

import "errors"

var (
	// ErrUnsupportedFormat is returned when the application encounters a file
	// format that it does not recognize or know how to parse.
	ErrUnsupportedFormat = errors.New("unsupported format")

	// ErrInvalidOffset is returned when the application encounters an invalid
	// offset into the data.
	ErrInvalidOffset = errors.New("invalid offset")

	// ErrLimitExceeded is returned by [LimitReader] when the underlying reader
	// produces more data than the configured maximum, instead of silently
	// truncating the stream.
	ErrLimitExceeded = errors.New("resource exceeds configured size limit")
)
