package xbstream

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"path/filepath"
)

const (
	chunkMagic     = "XBSTCK01"
	chunkHeaderLen = 14
	maxPathLen     = 4096

	chunkTypePayload = 'P'
	chunkTypeRename  = 'R'
	chunkTypeRemove  = 'D'
	chunkTypeSeek    = 'S'
	chunkTypeEOF     = 'E'
)

type MetadataCapture struct {
	targets  []string
	maxBytes int64
	captures map[string]*captureFile
}

type captureFile struct {
	data     []byte
	complete bool
}

func NewMetadataCapture(targets []string, maxBytes int64) *MetadataCapture {
	return &MetadataCapture{
		targets:  targets,
		maxBytes: maxBytes,
		captures: make(map[string]*captureFile),
	}
}

func (c *MetadataCapture) WrapReader(reader io.Reader) io.Reader {
	pipeReader, pipeWriter := io.Pipe()
	done := make(chan error, 1)

	go func() {
		err := c.Parse(pipeReader)
		if err != nil {
			_ = pipeReader.CloseWithError(err)
		}
		done <- err
	}()

	return &captureReader{
		reader: reader,
		writer: pipeWriter,
		done:   done,
	}
}

func (c *MetadataCapture) Parse(reader io.Reader) error {
	header := make([]byte, chunkHeaderLen)
	for {
		if _, err := io.ReadFull(reader, header); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("error reading xbstream chunk header: %w", err)
		}
		if string(header[:len(chunkMagic)]) != chunkMagic {
			return fmt.Errorf("invalid xbstream chunk magic")
		}

		chunkType := header[len(chunkMagic)+1]
		pathLen := binary.LittleEndian.Uint32(header[10:14])
		if pathLen > maxPathLen {
			return fmt.Errorf("xbstream chunk path length too large: %d", pathLen)
		}

		pathBytes := make([]byte, pathLen)
		if _, err := io.ReadFull(reader, pathBytes); err != nil {
			return fmt.Errorf("error reading xbstream chunk path: %w", err)
		}
		path := string(pathBytes)
		target := c.targetName(path)

		switch chunkType {
		case chunkTypeEOF:
			if target != "" {
				c.capture(target).complete = true
			}
		case chunkTypeRemove:
		case chunkTypeSeek:
			if err := discardN(reader, 8); err != nil {
				return fmt.Errorf("error discarding xbstream seek chunk: %w", err)
			}
		case chunkTypeRename:
			if err := discardRenameChunk(reader); err != nil {
				return err
			}
		case chunkTypePayload:
			if err := c.readPayload(reader, target); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported xbstream chunk type: %q", chunkType)
		}
	}
}

func (c *MetadataCapture) Metadata() ([]byte, string, bool) {
	for _, target := range c.targets {
		capture := c.captures[target]
		if capture != nil && capture.complete && len(capture.data) > 0 {
			return bytes.TrimSpace(capture.data), target, true
		}
	}
	return nil, "", false
}

func (c *MetadataCapture) readPayload(reader io.Reader, target string) error {
	payloadHeader := make([]byte, 20)
	if _, err := io.ReadFull(reader, payloadHeader); err != nil {
		return fmt.Errorf("error reading xbstream payload header: %w", err)
	}

	length := int64(binary.LittleEndian.Uint64(payloadHeader[:8]))
	offset := int64(binary.LittleEndian.Uint64(payloadHeader[8:16]))
	if length < 0 || offset < 0 {
		return fmt.Errorf("invalid xbstream payload length or offset")
	}

	if target == "" {
		return discardN(reader, length)
	}
	if offset > c.maxBytes || length > c.maxBytes-offset {
		return fmt.Errorf("xbstream metadata file %q exceeds capture limit of %d bytes", target, c.maxBytes)
	}

	payload := make([]byte, length)
	if _, err := io.ReadFull(reader, payload); err != nil {
		return fmt.Errorf("error reading xbstream metadata payload: %w", err)
	}

	capture := c.capture(target)
	if int64(len(capture.data)) < offset {
		padding := make([]byte, offset-int64(len(capture.data)))
		capture.data = append(capture.data, padding...)
	}
	end := offset + length
	if int64(len(capture.data)) < end {
		capture.data = append(capture.data, make([]byte, end-int64(len(capture.data)))...)
	}
	copy(capture.data[offset:end], payload)
	return nil
}

func (c *MetadataCapture) targetName(path string) string {
	base := filepath.Base(path)
	for _, target := range c.targets {
		if base == target {
			return target
		}
	}
	return ""
}

func (c *MetadataCapture) capture(target string) *captureFile {
	capture := c.captures[target]
	if capture == nil {
		capture = &captureFile{}
		c.captures[target] = capture
	}
	return capture
}

type captureReader struct {
	reader io.Reader
	writer *io.PipeWriter
	done   <-chan error
	closed bool
}

func (r *captureReader) Read(p []byte) (int, error) {
	n, readErr := r.reader.Read(p)
	if n > 0 {
		if _, err := r.writer.Write(p[:n]); err != nil {
			return n, err
		}
	}
	if readErr != nil {
		r.closeWriter(readErr)
		if err := <-r.done; err != nil {
			return n, err
		}
	}
	return n, readErr
}

func (r *captureReader) closeWriter(err error) {
	if r.closed {
		return
	}
	r.closed = true
	if errors.Is(err, io.EOF) {
		_ = r.writer.Close()
		return
	}
	_ = r.writer.CloseWithError(err)
}

func discardRenameChunk(reader io.Reader) error {
	pathLenBytes := make([]byte, 4)
	if _, err := io.ReadFull(reader, pathLenBytes); err != nil {
		return fmt.Errorf("error reading xbstream rename path length: %w", err)
	}
	pathLen := int64(binary.LittleEndian.Uint32(pathLenBytes))
	if pathLen > maxPathLen {
		return fmt.Errorf("xbstream rename path length too large: %d", pathLen)
	}
	if err := discardN(reader, pathLen); err != nil {
		return fmt.Errorf("error discarding xbstream rename path: %w", err)
	}
	return nil
}

func discardN(reader io.Reader, n int64) error {
	if n == 0 {
		return nil
	}
	_, err := io.CopyN(io.Discard, reader, n)
	return err
}
