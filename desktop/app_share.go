package main

import (
	"bytes"
	"compress/flate"
	"encoding/base64"
	"fmt"

	qrcode "github.com/skip2/go-qrcode"
)

// facetWebPreviewURL is the hosted browser preview (GitHub Pages, deployed
// from the web/ bundle on every main push).
const facetWebPreviewURL = "https://firstlayer-xyz.github.io/Facet/"

// maxShareURLLen caps share URLs at Windows' ShellExecute command-line limit;
// a longer URL would be truncated or rejected when opening the browser.
const maxShareURLLen = 32000

// maxQRBytes is the byte-mode capacity of a version-40 QR code at
// error-correction level M. URLs longer than this still work by click but
// cannot be rendered as a QR code.
const maxQRBytes = 2331

// ShareLink is the payload behind the Share button: the web-preview URL
// carrying the encoded source, plus a QR rendering of that URL for scanning
// with a phone. QRPNG is a base64-encoded PNG, empty when the URL exceeds
// maxQRBytes.
type ShareLink struct {
	URL   string `json:"url"`
	QRPNG string `json:"qrpng"`
}

// shareURL encodes source into the web preview's share-link form:
// the #code= hash fragment carrying base64url(deflate-raw(utf8 source)).
// The web preview decodes it on boot and renders the model.
func shareURL(source string) (string, error) {
	var buf bytes.Buffer
	w, err := flate.NewWriter(&buf, flate.BestCompression)
	if err != nil {
		return "", err
	}
	if _, err := w.Write([]byte(source)); err != nil {
		return "", err
	}
	if err := w.Close(); err != nil {
		return "", err
	}

	url := facetWebPreviewURL + "#code=" + base64.RawURLEncoding.EncodeToString(buf.Bytes())
	if len(url) > maxShareURLLen {
		return "", fmt.Errorf("source too large to share as a URL: %d characters (limit %d)", len(url), maxShareURLLen)
	}
	return url, nil
}

// BuildShareLink encodes the given source into a share URL and a QR rendering
// of it. The frontend shows the QR in a popover; opening the browser happens
// there via the wails runtime.
func (a *App) BuildShareLink(source string) (*ShareLink, error) {
	url, err := shareURL(source)
	if err != nil {
		return nil, err
	}
	link := &ShareLink{URL: url}
	if len(url) <= maxQRBytes {
		png, err := qrcode.Encode(url, qrcode.Medium, 512)
		if err != nil {
			return nil, fmt.Errorf("QR encode: %w", err)
		}
		link.QRPNG = base64.StdEncoding.EncodeToString(png)
	}
	return link, nil
}
