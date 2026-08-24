package errs

import "errors"

var (
	// ErrUnsupportedFormat is returned when the application encounters a file
	// format that it does not recognize or know how to parse.
	ErrUnsupportedFormat = errors.New("unsupported format")

	// ErrInvalidOffset is returned when the application encounters an invalid
	// offset into the data.
	// Deprecated: internal use only, will be unexported in a future version.
	ErrInvalidOffset = errors.New("invalid offset")

	// ErrLimitExceeded is returned by [LimitReader] when the underlying reader
	// produces more data than the configured maximum, instead of silently
	// truncating the stream.
	ErrLimitExceeded = errors.New("resource exceeds configured size limit")

	// ErrNoTransformer is returned when no Transformer is registered for a
	// requested MIME type conversion.
	ErrNoTransformer = errors.New("no transformer")

	// ErrUnsupportedMediaType is returned when a resource has an empty or
	// otherwise unsupported media type for conversion.
	ErrUnsupportedMediaType = errors.New("unsupported media type")
)
