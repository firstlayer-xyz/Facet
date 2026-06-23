// Package sharelink encodes and decodes Facet source for the web preview's
// share URL (the #code= hash fragment). The wire format is
// base64url(version ++ brotli(utf8 source)): the desktop app encodes it and the
// wasm web preview decodes it, so the codec lives in one place to stay in
// lockstep across both binaries.
package sharelink

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"

	"github.com/andybalholm/brotli"
)

// FormatBrotli tags the payload's compression. It is the first pre-base64 byte,
// so the decoder knows how to read the rest and the format can evolve without
// ambiguity.
const FormatBrotli byte = 0x01

// MaxInflated caps the decompressed payload. The #code= hash is
// attacker-controlled and brotli can expand enormously, so an uncapped read
// would let a crafted URL exhaust the tab's memory. 4 MiB is far beyond any
// real model.
const MaxInflated = 4 << 20

// Encode compresses source into the #code= payload: base64url of a one-byte
// format tag followed by Brotli-compressed UTF-8 at the maximum quality.
func Encode(source string) (string, error) {
	var buf bytes.Buffer
	buf.WriteByte(FormatBrotli)
	w := brotli.NewWriterLevel(&buf, brotli.BestCompression)
	if _, err := w.Write([]byte(source)); err != nil {
		return "", err
	}
	if err := w.Close(); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf.Bytes()), nil
}

// Decode reverses Encode, returning the original source. It enforces the format
// tag and the MaxInflated size cap.
func Decode(payload string) (string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return "", fmt.Errorf("invalid share payload: %w", err)
	}
	if len(raw) == 0 {
		return "", fmt.Errorf("empty share payload")
	}
	if raw[0] != FormatBrotli {
		return "", fmt.Errorf("unsupported share format version %#x", raw[0])
	}
	r := io.LimitReader(brotli.NewReader(bytes.NewReader(raw[1:])), MaxInflated+1)
	text, err := io.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("decompress shared model: %w", err)
	}
	if len(text) > MaxInflated {
		return "", fmt.Errorf("shared model exceeds the %d-byte limit", MaxInflated)
	}
	return string(text), nil
}
