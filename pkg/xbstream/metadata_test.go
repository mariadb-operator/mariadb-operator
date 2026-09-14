package xbstream

import (
	"bytes"
	"encoding/binary"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetadataCapture(t *testing.T) {
	var stream bytes.Buffer
	writePayloadChunk(t, &stream, "ibdata1", 0, []byte("large data"))
	writePayloadChunk(t, &stream, "mariadb_backup_binlog_info", 0, []byte("mariadb-repl-bin.000001 335 0-10-9\n"))
	writeEOFChunk(t, &stream, "mariadb_backup_binlog_info")

	capture := NewMetadataCapture([]string{"mariadb_backup_binlog_info", "xtrabackup_binlog_info"}, 1024)
	err := capture.Parse(&stream)
	require.NoError(t, err)

	meta, name, ok := capture.Metadata()
	require.True(t, ok)
	assert.Equal(t, "mariadb_backup_binlog_info", name)
	assert.Equal(t, []byte("mariadb-repl-bin.000001 335 0-10-9"), meta)
}

func TestMetadataCaptureWrapReader(t *testing.T) {
	var stream bytes.Buffer
	writePayloadChunk(t, &stream, "xtrabackup_binlog_info", 0, []byte("mariadb-repl-bin.000002 42 0-10-10\n"))
	writeEOFChunk(t, &stream, "xtrabackup_binlog_info")

	capture := NewMetadataCapture([]string{"mariadb_backup_binlog_info", "xtrabackup_binlog_info"}, 1024)
	wrapped := capture.WrapReader(bytes.NewReader(stream.Bytes()))

	copied, err := io.ReadAll(wrapped)
	require.NoError(t, err)
	assert.Equal(t, stream.Bytes(), copied)

	meta, name, ok := capture.Metadata()
	require.True(t, ok)
	assert.Equal(t, "xtrabackup_binlog_info", name)
	assert.Equal(t, []byte("mariadb-repl-bin.000002 42 0-10-10"), meta)
}

func writePayloadChunk(t *testing.T, buf *bytes.Buffer, path string, offset uint64, payload []byte) {
	t.Helper()

	writeChunkHeader(buf, chunkTypePayload, path)

	payloadHeader := make([]byte, 20)
	binary.LittleEndian.PutUint64(payloadHeader[:8], uint64(len(payload)))
	binary.LittleEndian.PutUint64(payloadHeader[8:16], offset)
	buf.Write(payloadHeader)
	buf.Write(payload)
}

func writeEOFChunk(t *testing.T, buf *bytes.Buffer, path string) {
	t.Helper()
	writeChunkHeader(buf, chunkTypeEOF, path)
}

func writeChunkHeader(buf *bytes.Buffer, chunkType byte, path string) {
	buf.WriteString(chunkMagic)
	buf.WriteByte(0)
	buf.WriteByte(chunkType)

	pathBytes := []byte(path)
	pathLen := make([]byte, 4)
	binary.LittleEndian.PutUint32(pathLen, uint32(len(pathBytes)))
	buf.Write(pathLen)
	buf.Write(pathBytes)
}
