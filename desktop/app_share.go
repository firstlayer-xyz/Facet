package main

import (
	"bytes"
	"compress/flate"
	"encoding/base64"
	"fmt"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// facetWebPreviewURL is the hosted browser preview (GitHub Pages, deployed
// from the web/ bundle on every main push).
const facetWebPreviewURL = "https://firstlayer-xyz.github.io/Facet/"

// maxShareURLLen caps share URLs at Windows' ShellExecute command-line limit;
// a longer URL would be truncated or rejected when opening the browser.
const maxShareURLLen = 32000

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

// ShareToWeb opens the default browser on the hosted web preview with the
// given source rendered. The source travels in the URL hash — see shareURL.
func (a *App) ShareToWeb(source string) error {
	url, err := shareURL(source)
	if err != nil {
		return err
	}
	wailsRuntime.BrowserOpenURL(a.ctx, url)
	return nil
}
