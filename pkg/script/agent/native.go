package agent

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// This file is the framing of a browser's native messaging, by which an
// extension talks to a program it starts (relay.go): each message is
// its length, 32 bits in the machine's own byte order, and then that
// many bytes of JSON.

// maxNativeOut is the longest message a program may send the browser:
// Chrome refuses one over a megabyte.
const maxNativeOut = 1 << 20

// maxNativeIn is the longest message read from the browser: a line of
// the protocol at its longest (maxLine) with room for its envelope.
const maxNativeIn = maxLine + 1<<20

// errNativeTooLong is a message longer than the other side may send.
var errNativeTooLong = errors.New("native message too long")

// readNative reads one message.
func readNative(r io.Reader) ([]byte, error) {
	var n uint32
	if err := binary.Read(r, binary.NativeEndian, &n); err != nil {
		return nil, err
	}
	if n > maxNativeIn {
		return nil, fmt.Errorf("%w: %d bytes", errNativeTooLong, n)
	}
	msg := make([]byte, n)
	if _, err := io.ReadFull(r, msg); err != nil {
		return nil, fmt.Errorf("native message cut short: %w", err)
	}
	return msg, nil
}

// writeNative writes one message.
func writeNative(w io.Writer, msg []byte) error {
	if len(msg) > maxNativeOut {
		return fmt.Errorf("%w: %d bytes", errNativeTooLong, len(msg))
	}
	buf := binary.NativeEndian.AppendUint32(make([]byte, 0, 4+len(msg)), uint32(len(msg)))
	_, err := w.Write(append(buf, msg...))
	return err
}
