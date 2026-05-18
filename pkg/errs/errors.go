package errs

import "errors"

var (
	ErrUnsupportedFormat = errors.New("unsupported format")
)