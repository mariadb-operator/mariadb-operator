package compression

import (
	"compress/gzip"
	"fmt"
	"io"

	"github.com/dsnet/compress/bzip2"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
)

type Compressor interface {
	Compress(dst io.Writer, src io.Reader) error
	Decompress(dst io.Writer, src io.Reader) error
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

func (c *NopCompressor) Compress(dst io.Writer, src io.Reader) error {
	return nil
}

func (c *NopCompressor) Decompress(dst io.Writer, src io.Reader) error {
	return nil
}

type GzipCompressor struct{}

func (c *GzipCompressor) Compress(dst io.Writer, src io.Reader) error {
	writer := gzip.NewWriter(dst)
	defer writer.Close()
	_, err := io.Copy(writer, src)
	return err
}

func (c *GzipCompressor) Decompress(dst io.Writer, src io.Reader) error {
	reader, err := gzip.NewReader(src)
	if err != nil {
		return err
	}
	defer reader.Close()
	_, err = io.Copy(dst, reader)
	return err
}

type Bzip2Compressor struct{}

func (c *Bzip2Compressor) Compress(dst io.Writer, src io.Reader) error {
	writer, err := bzip2.NewWriter(dst,
		&bzip2.WriterConfig{Level: bzip2.DefaultCompression})
	if err != nil {
		return err
	}
	defer writer.Close()
	_, err = io.Copy(writer, src)
	return err
}

func (c *Bzip2Compressor) Decompress(dst io.Writer, src io.Reader) error {
	reader, err := bzip2.NewReader(src,
		&bzip2.ReaderConfig{})
	if err != nil {
		return err
	}
	defer reader.Close()
	_, err = io.Copy(dst, reader)
	return err
}
