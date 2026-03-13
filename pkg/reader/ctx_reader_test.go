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
		timeout := 1 * time.Second
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		time.Sleep(timeout)

		cr := &ContextReader{
			ctx:           ctx,
			wrappedReader: bytes.NewReader([]byte("hello world")),
		}
		Eventually(func(g Gomega) bool {
			p := make([]byte, 1)
			n, err := cr.Read(p)
			g.Expect(n).To(BeZero())
			g.Expect(err).To(HaveOccurred())
			g.Expect(err.Error()).To(ContainSubstring(context.DeadlineExceeded.Error()))
			return true
		}, 1*time.Minute, 1*time.Second).Should(BeTrue())
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
