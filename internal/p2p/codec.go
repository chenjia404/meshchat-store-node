package p2p

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

var ErrFrameTooLarge = errors.New("frame too large")

func ReadFrame(r io.Reader, maxSize uint32) ([]byte, error) {
	var header [4]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return nil, err
	}
	size := binary.BigEndian.Uint32(header[:])
	if size == 0 || size > maxSize {
		return nil, fmt.Errorf("%w: %d", ErrFrameTooLarge, size)
	}
	buf := make([]byte, size)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

func WriteFrame(w io.Writer, payload []byte, maxSize uint32) error {
	if len(payload) == 0 || uint32(len(payload)) > maxSize {
		return fmt.Errorf("%w: %d", ErrFrameTooLarge, len(payload))
	}
	var header [4]byte
	binary.BigEndian.PutUint32(header[:], uint32(len(payload)))
	if _, err := w.Write(header[:]); err != nil {
		return err
	}
	_, err := w.Write(payload)
	return err
}

func ReadJSON(r io.Reader, v any, maxSize uint32) error {
	frame, err := ReadFrame(r, maxSize)
	if err != nil {
		return err
	}
	return json.Unmarshal(frame, v)
}

func WriteJSON(w io.Writer, v any, maxSize uint32) error {
	payload, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return WriteFrame(w, payload, maxSize)
}
