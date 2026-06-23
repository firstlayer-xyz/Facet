package main

import (
	"bytes"
	"encoding/base64"
	"math/rand"
	"strings"
	"testing"

	"facet/pkg/sharelink"
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

	got, err := sharelink.Decode(strings.TrimPrefix(url, prefix))
	if err != nil {
		t.Fatalf("sharelink.Decode: %v", err)
	}
	// This source has no comments and fits a QR, so it is not minified — the
	// roundtrip must reproduce it byte-for-byte.
	if got != source {
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

// A small function buried under a large, high-entropy comment block: the
// un-minified encoding exceeds QR capacity, but minify-on-overflow strips the
// comments so a QR can still be produced.
func TestMinifyShrinksSourceToFitQR(t *testing.T) {
	var b strings.Builder
	b.WriteString("fn Main() Solid {\n")
	r := rand.New(rand.NewSource(7))
	tok := make([]byte, 48)
	for i := 0; i < 80; i++ {
		r.Read(tok)
		b.WriteString("    // ")
		b.WriteString(base64.RawURLEncoding.EncodeToString(tok))
		b.WriteString("\n")
	}
	b.WriteString("    return Cube(size: 10 mm)\n}\n")
	source := b.String()

	// Setup sanity: the un-minified encoding really is too big for a QR.
	plain, err := encodeShare(source)
	if err != nil {
		t.Fatalf("encodeShare: %v", err)
	}
	if len(plain) <= maxQRBytes {
		t.Fatalf("test setup: un-minified url is %d chars, need > %d to exercise minify", len(plain), maxQRBytes)
	}

	link, err := (&App{}).BuildShareLink(source)
	if err != nil {
		t.Fatalf("BuildShareLink: %v", err)
	}
	if link.QRPNG == "" {
		t.Errorf("minify-on-overflow failed to fit the QR cap; url is %d chars", len(link.URL))
	}
	// The QR'd URL must be the minified one — strictly smaller than the plain one.
	if len(link.URL) >= len(plain) {
		t.Errorf("expected minified url (%d) shorter than plain url (%d)", len(link.URL), len(plain))
	}
}

func TestBuildShareLinkSkipsQRWhenTooLong(t *testing.T) {
	// Incompressible input past QR byte capacity but within the URL cap: the
	// link must still be produced, with the QR explicitly absent. It does not
	// parse, so minify-on-overflow cannot shrink it.
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
