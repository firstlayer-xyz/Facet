package main

import (
	"bytes"
	"compress/flate"
	"encoding/base64"
	"io"
	"math/rand"
	"strings"
	"testing"
)

func TestShareURLRoundtrip(t *testing.T) {
	source := "fn Main() Solid {\n    return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});\n}\n"

	url, err := shareURL(source)
	if err != nil {
		t.Fatalf("shareURL: %v", err)
	}

	const prefix = facetWebPreviewURL + "#code="
	if !strings.HasPrefix(url, prefix) {
		t.Fatalf("url %q lacks prefix %q", url, prefix)
	}

	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(url, prefix))
	if err != nil {
		t.Fatalf("base64url decode: %v", err)
	}
	got, err := io.ReadAll(flate.NewReader(bytes.NewReader(raw)))
	if err != nil {
		t.Fatalf("inflate: %v", err)
	}
	if string(got) != source {
		t.Errorf("roundtrip mismatch:\n got: %q\nwant: %q", got, source)
	}
}

func TestShareURLTooLarge(t *testing.T) {
	// Seeded-random input is incompressible, so the compressed payload alone
	// already exceeds the URL cap before base64 expansion.
	buf := make([]byte, maxShareURLLen)
	rand.New(rand.NewSource(1)).Read(buf)

	_, err := shareURL(string(buf))
	if err == nil {
		t.Fatal("expected an error for an oversized source")
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Errorf("error %q does not mention the size limit", err)
	}
}
