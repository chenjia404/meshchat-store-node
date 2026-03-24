package p2p

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"testing"
)

func TestReadWriteFrame(t *testing.T) {
	var buf bytes.Buffer
	payload := []byte(`{"ok":true}`)
	if err := WriteFrame(&buf, payload, 1024); err != nil {
		t.Fatalf("WriteFrame() error = %v", err)
	}
	got, err := ReadFrame(&buf, 1024)
	if err != nil {
		t.Fatalf("ReadFrame() error = %v", err)
	}
	if string(got) != string(payload) {
		t.Fatalf("ReadFrame() = %s, want %s", got, payload)
	}
}

func TestReadFrameInvalidLength(t *testing.T) {
	var buf bytes.Buffer
	_ = binary.Write(&buf, binary.BigEndian, uint32(2048))
	buf.WriteString("x")
	_, err := ReadFrame(&buf, 1024)
	if !errors.Is(err, ErrFrameTooLarge) {
		t.Fatalf("ReadFrame() error = %v, want ErrFrameTooLarge", err)
	}
}

func TestReadFramePartialPayload(t *testing.T) {
	var buf bytes.Buffer
	_ = binary.Write(&buf, binary.BigEndian, uint32(4))
	buf.WriteString("ab")
	_, err := ReadFrame(&buf, 1024)
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("ReadFrame() error = %v, want io.ErrUnexpectedEOF", err)
	}
}

func TestWriteFrameTooLarge(t *testing.T) {
	var buf bytes.Buffer
	err := WriteFrame(&buf, bytes.Repeat([]byte("a"), 2048), 1024)
	if !errors.Is(err, ErrFrameTooLarge) {
		t.Fatalf("WriteFrame() error = %v, want ErrFrameTooLarge", err)
	}
}
