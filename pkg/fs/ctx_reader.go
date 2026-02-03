package fs

import (
	"context"
	"fmt"
	"io"
)

// ContextReader is a cancellable `io.Reader` wrapper
// Use this when dealing with big files that you may need to interrupt reading
type ContextReader struct {
	Ctx context.Context
	R   io.Reader
}

func (cr *ContextReader) Read(p []byte) (int, error) {
	select {
	case <-cr.Ctx.Done():
		return 0, fmt.Errorf("error reading from file, context is closed, err is: %w", cr.Ctx.Err())
	default:
		return cr.R.Read(p)
	}
}
