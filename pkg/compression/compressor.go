package compression

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"

	"github.com/dsnet/compress/bzip2"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/reader"
)

type Compressor interface {
	Compress(ctx context.Context, dst io.Writer, src io.Reader) error
	Decompress(ctx context.Context, dst io.Writer, src io.Reader) error
}

func NewCompressor(calg mariadbv1alpha1.CompressAlgorithm) (Compressor, error) {
	switch calg {
	case mariadbv1alpha1.CompressNone:
		return &NopCompressor{}, nil
	case mariadbv1alpha1.CompressGzip:
		return &GzipCompressor{}, nil
	case mariadbv1alpha1.CompressBzip2:
		return &Bzip2Compressor{}, nil
	default:
		return nil, fmt.Errorf("unsupported compression algorithm: %v", calg)
	}
}

type NopCompressor struct{}

func (c *NopCompressor) Compress(ctx context.Context, dst io.Writer, src io.Reader) error {
	_, err := io.Copy(dst, reader.NewContextReader(ctx, src))
	return err
}

func (c *NopCompressor) Decompress(ctx context.Context, dst io.Writer, src io.Reader) error {
	_, err := io.Copy(dst, reader.NewContextReader(ctx, src))
	return err
}

type GzipCompressor struct{}

func (c *GzipCompressor) Compress(ctx context.Context, dst io.Writer, src io.Reader) error {
	writer := gzip.NewWriter(dst)
	defer writer.Close()
	_, err := io.Copy(writer, reader.NewContextReader(ctx, src))
	return err
}

func (c *GzipCompressor) Decompress(ctx context.Context, dst io.Writer, src io.Reader) error {
	gzipReader, err := gzip.NewReader(src)
	if err != nil {
		return err
	}
	defer gzipReader.Close()
	_, err = io.Copy(dst, reader.NewContextReader(ctx, gzipReader))
	return err
}

type Bzip2Compressor struct{}

func (c *Bzip2Compressor) Compress(ctx context.Context, dst io.Writer, src io.Reader) error {
	writer, err := bzip2.NewWriter(dst,
		&bzip2.WriterConfig{Level: bzip2.DefaultCompression})
	if err != nil {
		return err
	}
	defer writer.Close()
	_, err = io.Copy(writer, reader.NewContextReader(ctx, src))
	return err
}

func (c *Bzip2Compressor) Decompress(ctx context.Context, dst io.Writer, src io.Reader) error {
	bzip2Reader, err := bzip2.NewReader(src,
		&bzip2.ReaderConfig{})
	if err != nil {
		return err
	}
	defer bzip2Reader.Close()
	_, err = io.Copy(dst, reader.NewContextReader(ctx, bzip2Reader))
	return err
}
