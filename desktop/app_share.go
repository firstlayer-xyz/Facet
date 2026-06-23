package main

import (
	"encoding/base64"
	"fmt"

	qrcode "github.com/skip2/go-qrcode"

	"facet/pkg/fctlang/formatter"
	"facet/pkg/fctlang/parser"
	"facet/pkg/sharelink"
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

// encodeShare encodes source into the web preview's share-link form: the
// preview URL with a #code= hash fragment carrying the sharelink payload. The
// web preview decodes it on boot and renders the model.
func encodeShare(source string) (string, error) {
	payload, err := sharelink.Encode(source)
	if err != nil {
		return "", err
	}
	return facetWebPreviewURL + "#code=" + payload, nil
}

// minifySource strips comments, indentation, and blank lines from source while
// preserving its meaning. It is best-effort: source that does not parse (e.g.
// while the user is mid-edit) is returned with ok=false, because reformatting
// unparseable source could corrupt it — the caller then shares the original.
func minifySource(source string) (minified string, ok bool) {
	src, err := parser.Parse(source, "", parser.SourceUser)
	if err != nil {
		return "", false
	}
	return formatter.Minify(src), true
}

// shareURL encodes source into the web-preview share URL. When the source is
// too large for a QR code, it is minified and re-encoded to try to fit; the
// smaller encoding is kept. The result is hard-capped at the URL length limit.
func shareURL(source string) (string, error) {
	url, err := encodeShare(source)
	if err != nil {
		return "", err
	}
	if len(url) > maxQRBytes {
		if minified, ok := minifySource(source); ok {
			if murl, err := encodeShare(minified); err == nil && len(murl) < len(url) {
				url = murl
			}
		}
	}
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
