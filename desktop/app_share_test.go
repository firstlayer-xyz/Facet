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

func TestBuildShareLinkQR(t *testing.T) {
	link, err := (&App{}).BuildShareLink("fn Main() Solid {\n    return Sphere(d: 10 mm);\n}\n")
	if err != nil {
		t.Fatalf("BuildShareLink: %v", err)
	}
	if !strings.HasPrefix(link.URL, facetWebPreviewURL+"#code=") {
		t.Fatalf("url %q lacks share prefix", link.URL)
	}
	png, err := base64.StdEncoding.DecodeString(link.QRPNG)
	if err != nil {
		t.Fatalf("QRPNG is not valid base64: %v", err)
	}
	magic := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}
	if !bytes.HasPrefix(png, magic) {
		t.Errorf("QRPNG does not start with PNG magic: % x", png[:min(8, len(png))])
	}
}

func TestBuildShareLinkSkipsQRWhenTooLong(t *testing.T) {
	// Incompressible input past QR byte capacity but within the URL cap: the
	// link must still be produced, with the QR explicitly absent.
	buf := make([]byte, 2400)
	rand.New(rand.NewSource(2)).Read(buf)

	link, err := (&App{}).BuildShareLink(string(buf))
	if err != nil {
		t.Fatalf("BuildShareLink: %v", err)
	}
	if len(link.URL) <= maxQRBytes {
		t.Fatalf("test input too small: url %d chars does not exceed maxQRBytes %d", len(link.URL), maxQRBytes)
	}
	if len(link.URL) > maxShareURLLen {
		t.Fatalf("test input too large: url %d chars exceeds the URL cap %d", len(link.URL), maxShareURLLen)
	}
	if link.QRPNG != "" {
		t.Errorf("expected empty QRPNG for a %d-char url", len(link.URL))
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
