package sharelink

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestEncodeDecodeRoundtrip(t *testing.T) {
	src := "fn Main() Solid {\n    return Cube(size: 10 mm)\n}\n"
	enc, err := Encode(src)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	got, err := Decode(enc)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if got != src {
		t.Errorf("roundtrip mismatch:\n got %q\nwant %q", got, src)
	}
}

func TestDecodeRejectsUnknownVersion(t *testing.T) {
	payload := base64.RawURLEncoding.EncodeToString([]byte{0x02, 0x00})
	_, err := Decode(payload)
	if err == nil || !strings.Contains(err.Error(), "version") {
		t.Fatalf("expected unsupported-version error, got %v", err)
	}
}

func TestDecodeRejectsCorruptBase64(t *testing.T) {
	if _, err := Decode("!!!not base64!!!"); err == nil {
		t.Fatal("expected error for non-base64 payload")
	}
}

func TestDecodeCapsInflatedSize(t *testing.T) {
	// A tiny encoded payload that inflates past the cap (a decompression bomb):
	// brotli crushes the repeated byte, but Decode must refuse to expand it.
	bomb := strings.Repeat("a", MaxInflated+1024)
	enc, err := Encode(bomb)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if len(enc) > 4096 {
		t.Fatalf("expected a small encoded bomb, got %d chars", len(enc))
	}
	_, err = Decode(enc)
	if err == nil || !strings.Contains(err.Error(), "limit") {
		t.Fatalf("expected size-cap error, got %v", err)
	}
}
