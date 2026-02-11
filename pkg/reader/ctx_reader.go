package reader

import (
	"context"
	"fmt"
	"io"
)

// ContextReader is a cancellable `io.Reader` wrapper
// Use this when dealing with big files that you may need to interrupt reading
type ContextReader struct {
	ctx           context.Context
	wrappedReader io.Reader
}

func NewContextReader(ctx context.Context, wrappedReader io.Reader) *ContextReader {
	return &ContextReader{
		ctx:           ctx,
		wrappedReader: wrappedReader,
	}
}

func (cr *ContextReader) Read(p []byte) (int, error) {
	select {
	case <-cr.ctx.Done():
		return 0, fmt.Errorf("error reading from file, context is closed, err is: %w", cr.ctx.Err())
	default:
		return cr.wrappedReader.Read(p)
	}
}
