package reader

import (
	"bytes"
	"context"
	"io"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type testErrorReader struct {
	err error
}

func (r *testErrorReader) Read(p []byte) (int, error) {
	return 0, r.err
}

var _ = Describe("ContextReader", func() {
	It("should read all content", func() {
		data := []byte("hello world")
		reader := bytes.NewReader(data)
		cr := &ContextReader{
			ctx:           context.Background(),
			wrappedReader: reader,
		}
		p := make([]byte, len(data))
		n, err := cr.Read(p)

		Expect(err).ToNot(HaveOccurred())
		Expect(n).To(Equal(len(data)))
		Expect(p).To(Equal(data))
	})

	It("should return a context canceled error", func() {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		cr := &ContextReader{
			ctx:           ctx,
			wrappedReader: bytes.NewReader([]byte("hello world")),
		}
		p := make([]byte, 1)
		n, err := cr.Read(p)

		Expect(n).To(BeZero())
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring(context.Canceled.Error()))
	})

	It("should return a context deadline exceeded error", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()
		time.Sleep(5 * time.Millisecond)
		cr := &ContextReader{
			ctx:           ctx,
			wrappedReader: bytes.NewReader([]byte("hello world")),
		}
		p := make([]byte, 1)
		n, err := cr.Read(p)

		Expect(n).To(BeZero())
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring(context.DeadlineExceeded.Error()))
	})

	It("should propagate the error", func() {
		expectedErr := io.ErrUnexpectedEOF
		cr := &ContextReader{
			ctx:           context.Background(),
			wrappedReader: &testErrorReader{err: expectedErr},
		}
		p := make([]byte, 1)
		n, err := cr.Read(p)

		Expect(n).To(BeZero())
		Expect(err).To(MatchError(expectedErr))
	})
})
