package compression

import (
	"bytes"
	"context"
	"math/rand"
	"strconv"
	"testing"
)

// compressible builds deterministic input with a ratio in the range a MariaDB
// data directory gives, so the benchmark below is not measuring an unrealistic
// best case. Each MiB is one part incompressible noise to eleven parts drawn
// from a small dictionary.
func compressible(size int) []byte {
	r := rand.New(rand.NewSource(1))
	const ntok = 256
	toks := make([][]byte, ntok)
	for i := range toks {
		tok := make([]byte, 64)
		r.Read(tok)
		toks[i] = tok
	}

	const chunk = 1 << 20
	noise := chunk / 12
	buf := make([]byte, 0, size+chunk)
	block := make([]byte, 0, chunk+64)
	for len(buf) < size {
		block = block[:noise]
		r.Read(block)
		for len(block) < chunk {
			block = append(block, toks[r.Intn(ntok)]...)
		}
		buf = append(buf, block[:chunk]...)
	}
	return buf[:size]
}

func compressZstd(tb testing.TB, concurrency int, src []byte) []byte {
	tb.Helper()
	var out bytes.Buffer
	c := &ZstdCompressor{concurrency: concurrency}
	if err := c.Compress(context.Background(), &out, bytes.NewReader(src)); err != nil {
		tb.Fatalf("concurrency %d: compress: %v", concurrency, err)
	}
	return out.Bytes()
}

// Block-level parallelism changes how the encoder frames its output, so check
// that every concurrency setting still round-trips byte for byte and that the
// ratio does not drift. The input spans several blocks on purpose.
func TestZstdCompressorRoundTripAcrossConcurrency(t *testing.T) {
	src := compressible(8 << 20)
	serial := len(compressZstd(t, 1, src))

	for _, concurrency := range []int{0, 1, 2, 4, 8} {
		compressed := compressZstd(t, concurrency, src)

		var out bytes.Buffer
		c := &ZstdCompressor{concurrency: concurrency}
		if err := c.Decompress(context.Background(), &out, bytes.NewReader(compressed)); err != nil {
			t.Fatalf("concurrency %d: decompress: %v", concurrency, err)
		}
		if !bytes.Equal(out.Bytes(), src) {
			t.Errorf("concurrency %d: round-trip returned %d bytes, want %d unchanged",
				concurrency, out.Len(), len(src))
		}

		// Parallel blocks see a bounded overlap prefix rather than the whole
		// preceding stream, so allow a little drift but not a regression in kind.
		if ratio := float64(len(compressed)) / float64(serial); ratio > 1.05 {
			t.Errorf("concurrency %d: compressed to %d bytes, %.1f%% larger than serial %d",
				concurrency, len(compressed), (ratio-1)*100, serial)
		}
	}
}

// BenchmarkZstdCompressor shows whether concurrency reaches the encoder at all.
// WithEncoderConcurrency on its own leaves the pipeline one block deep, so
// without WithConcurrentBlocks the throughput here stops improving after about
// two threads however high the setting goes. Read the ratio metric alongside
// the rate.
func BenchmarkZstdCompressor(b *testing.B) {
	src := compressible(256 << 20)

	for _, concurrency := range []int{1, 2, 4, 8} {
		b.Run("threads="+strconv.Itoa(concurrency), func(b *testing.B) {
			b.SetBytes(int64(len(src)))
			var compressed int64
			for i := 0; i < b.N; i++ {
				counter := &countWriter{}
				c := &ZstdCompressor{concurrency: concurrency}
				if err := c.Compress(context.Background(), counter, bytes.NewReader(src)); err != nil {
					b.Fatal(err)
				}
				compressed = counter.n
			}
			b.ReportMetric(float64(len(src))/float64(compressed), "ratio")
		})
	}
}

type countWriter struct{ n int64 }

func (c *countWriter) Write(p []byte) (int, error) {
	c.n += int64(len(p))
	return len(p), nil
}
