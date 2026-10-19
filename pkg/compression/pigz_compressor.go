package compression

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type PigzCompressor struct {
	concurrency int // 0 = all available CPUs (pigz default)
}

func (c *PigzCompressor) Compress(ctx context.Context, dst io.Writer, src io.Reader) error {
	return c.run(ctx, dst, src, "--stdout")
}

func (c *PigzCompressor) Decompress(ctx context.Context, dst io.Writer, src io.Reader) error {
	return c.run(ctx, dst, src, "--stdout", "--decompress")
}

func (c *PigzCompressor) run(ctx context.Context, dst io.Writer, src io.Reader, args ...string) error {
	if c.concurrency < 0 {
		return fmt.Errorf("pigz compression threads must not be negative: %d", c.concurrency)
	}
	if c.concurrency > 0 {
		args = append(args, "--processes", strconv.Itoa(c.concurrency))
	}
	cmd := exec.CommandContext(ctx, "pigz", args...)
	// Prevent inherited options from changing the output format or thread count.
	cmd.Env = append(os.Environ(), "GZIP=", "PIGZ=")
	cmd.Stdin = src
	cmd.Stdout = dst
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error running pigz: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}
