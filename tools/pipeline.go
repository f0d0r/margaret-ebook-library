package tools

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"

	"github.com/f0d0r/margaret-ebook-library/book"
	"github.com/f0d0r/margaret-ebook-library/mediatype"
)

// Processor is the base type accepted by ScanReadingOrder. A processor
// implements at least one of the capability interfaces below; ScanReadingOrder
// discovers the capabilities by type assertion and calls only the ones a
// processor provides.
type Processor interface{}

// TextProcessor consumes the whole plain-text reading order in chunks. The
// slice passed to Process is only valid for the duration of the call; copy it
// if the processor needs to retain it. Every byte of the stream is passed to
// Process, including the bytes also passed to HeadProcessor.
type TextProcessor interface {
	Process(p []byte) error
}

// HeadProcessor is implemented by processors that only need the beginning of
// the reading order. HeadLimit is the number of bytes the processor wants; a
// non-positive limit disables head delivery. ProcessHead is called once, as
// soon as HeadLimit bytes are available or when the stream ends, with at most
// HeadLimit bytes.
type HeadProcessor interface {
	HeadLimit() int
	ProcessHead(head []byte) error
}

// FallbackProcessor is implemented by processors that may need resources
// outside the linear reading order after the head has been seen (for example
// an ISBN scanner that looks at non-linear chapters). NeedsFallback is checked
// after the head has been processed; ProcessFallback receives the resource set
// and the set of resources that belong to the linear reading order.
type FallbackProcessor interface {
	NeedsFallback() bool
	ProcessFallback(ctx context.Context, rs *book.ResourceSet, linear map[*book.Resource]struct{}) error
}

// FinishProcessor is called once after the stream and any fallback work are
// complete, so it can finalize results. It is not called when an earlier phase
// returns an error.
type FinishProcessor interface {
	Finish(ctx context.Context) error
}

// headRequest pairs a head processor with the number of bytes it asked for.
type headRequest struct {
	processor HeadProcessor
	limit     int
}

// headCollector buffers the beginning of the reading order and delivers each
// processor the prefix it asked for as soon as that prefix is available.
type headCollector struct {
	pending []headRequest
	buf     []byte
	limit   int
}

func newHeadCollector(requests []headRequest) *headCollector {
	slices.SortStableFunc(requests, func(a, b headRequest) int { return a.limit - b.limit })
	limit := requests[len(requests)-1].limit
	return &headCollector{
		pending: requests,
		buf:     make([]byte, 0, min(limit, 64*1024)),
		limit:   limit,
	}
}

// remaining reports how many bytes are still needed to satisfy every pending
// head processor.
func (h *headCollector) remaining() int {
	return h.limit - len(h.buf)
}

func (h *headCollector) add(chunk []byte) error {
	if len(h.pending) == 0 {
		return nil
	}
	if need := h.remaining(); need > 0 {
		if len(chunk) > need {
			chunk = chunk[:need]
		}
		h.buf = append(h.buf, chunk...)
	}
	if err := h.flushReady(); err != nil {
		return err
	}
	if len(h.pending) == 0 {
		h.buf = nil
	}
	return nil
}

func (h *headCollector) flushReady() error {
	for len(h.pending) > 0 && len(h.buf) >= h.pending[0].limit {
		req := h.pending[0]
		h.pending = h.pending[1:]
		if err := req.processor.ProcessHead(h.buf[:req.limit]); err != nil {
			return fmt.Errorf("processor %T: %w", req.processor, err)
		}
	}
	return nil
}

func (h *headCollector) flushRemaining() error {
	for _, req := range h.pending {
		n := min(len(h.buf), req.limit)
		if err := req.processor.ProcessHead(h.buf[:n]); err != nil {
			return fmt.Errorf("processor %T: %w", req.processor, err)
		}
	}
	h.pending = nil
	h.buf = nil
	return nil
}

// ScanReadingOrder reads the book's plain-text reading order once and feeds
// every processor. Capabilities are discovered by type assertion: TextProcessor
// receives every chunk, HeadProcessor receives the requested head prefix,
// FallbackProcessor can inspect the resource set after the head, and
// FinishProcessor runs last. When no TextProcessor is attached, the stream is
// read only far enough to satisfy the head processors.
//
// A processor instance carries its own result and must not be reused for
// another scan. Errors are wrapped with the failing processor's type.
func ScanReadingOrder(ctx context.Context, rs *book.ResourceSet, processors ...Processor) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if rs == nil {
		return fmt.Errorf("resource set is nil")
	}

	var (
		streams  []TextProcessor
		headReqs []headRequest
		falls    []FallbackProcessor
		finishes []FinishProcessor
	)
	for _, processor := range processors {
		if processor == nil {
			continue
		}
		matched := false
		if tp, ok := processor.(TextProcessor); ok {
			streams = append(streams, tp)
			matched = true
		}
		if hp, ok := processor.(HeadProcessor); ok {
			matched = true
			if limit := hp.HeadLimit(); limit > 0 {
				headReqs = append(headReqs, headRequest{processor: hp, limit: limit})
			}
		}
		if fp, ok := processor.(FallbackProcessor); ok {
			falls = append(falls, fp)
			matched = true
		}
		if fp, ok := processor.(FinishProcessor); ok {
			finishes = append(finishes, fp)
			matched = true
		}
		if !matched {
			return fmt.Errorf("processor %T implements no supported capability", processor)
		}
	}

	var heads *headCollector
	if len(headReqs) > 0 {
		heads = newHeadCollector(headReqs)
	}
	if len(streams) > 0 || heads != nil {
		if err := readReadingOrder(ctx, rs, streams, heads); err != nil {
			return err
		}
	}

	if err := ctx.Err(); err != nil {
		return err
	}
	linear := linearResources(rs)
	for _, processor := range falls {
		if !processor.NeedsFallback() {
			continue
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := processor.ProcessFallback(ctx, rs, linear); err != nil {
			return fmt.Errorf("processor %T: %w", processor, err)
		}
	}

	for _, processor := range finishes {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := processor.Finish(ctx); err != nil {
			return fmt.Errorf("processor %T: %w", processor, err)
		}
	}
	return nil
}

// readReadingOrder streams the delimited reading order. When heads is nil no
// head is buffered; when streams is empty it stops as soon as all head
// processors have been served.
func readReadingOrder(ctx context.Context, rs *book.ResourceSet, streams []TextProcessor, heads *headCollector) error {
	rc, err := openDelimitedReadingOrder(ctx, rs, mediatype.PlainText, "\n")
	if err != nil {
		return err
	}
	defer func() { _ = rc.Close() }()

	buf := make([]byte, 32*1024)
	for {
		readBuf := buf
		if len(streams) == 0 && heads != nil {
			if need := heads.remaining(); need > 0 && need < len(readBuf) {
				readBuf = readBuf[:need]
			}
		}
		n, readErr := rc.Read(readBuf)
		if n > 0 {
			chunk := readBuf[:n]
			for _, processor := range streams {
				if err := processor.Process(chunk); err != nil {
					return fmt.Errorf("processor %T: %w", processor, err)
				}
			}
			if heads != nil {
				if err := heads.add(chunk); err != nil {
					return err
				}
				if len(streams) == 0 && len(heads.pending) == 0 {
					return nil
				}
			}
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				break
			}
			return readErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	if heads != nil {
		return heads.flushRemaining()
	}
	return nil
}

// linearResources returns the set of resources that belong to the linear
// reading order.
func linearResources(rs *book.ResourceSet) map[*book.Resource]struct{} {
	out := make(map[*book.Resource]struct{})
	for _, item := range rs.ReadingOrder() {
		if item.Linear && item.Resource != nil {
			out[item.Resource] = struct{}{}
		}
	}
	return out
}
