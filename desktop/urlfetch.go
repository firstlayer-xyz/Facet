package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// validateFetchURL parses rawURL and enforces the http/https scheme and a
// non-empty host. It does NOT resolve DNS — the dialer (newFetchClient) checks
// the actual connected IP against isBlockedIP on every connection and redirect,
// which also defeats DNS-rebinding.
func validateFetchURL(rawURL string) (*url.URL, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("unsupported URL scheme %q (only http and https are allowed)", u.Scheme)
	}
	if u.Hostname() == "" {
		return nil, fmt.Errorf("URL has no host")
	}
	return u, nil
}

// isBlockedIP reports whether ip is in a range fetch_url must never reach:
// loopback, private (RFC1918 / IPv6 ULA), link-local (incl. the
// 169.254.169.254 cloud-metadata endpoint), unspecified, or multicast. This is
// the SSRF guard that keeps the assistant away from the local MCP server,
// other localhost services, and instance metadata.
func isBlockedIP(ip net.IP) bool {
	return ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() ||
		ip.IsUnspecified()
}

const (
	// maxImageBytes caps an image download. A 4K-ish PNG fits comfortably;
	// past this we error (a truncated image is useless).
	maxImageBytes = 8 << 20 // 8 MiB
	// maxTextBytes caps returned text. Past this we truncate with a marker so
	// we don't flood the model's context.
	maxTextBytes = 256 << 10 // 256 KiB

	fetchUserAgent = "Facet-Assistant/1.0 (+fetch_url)"
)

// fetchedContent is the routed result of a fetch_url GET. Exactly one of the
// image branch (IsImage + Data + MIME) or the text branch (Text) is populated.
type fetchedContent struct {
	FinalURL    string
	Status      int
	ContentType string
	IsImage     bool
	MIME        string
	Data        []byte
	Text        string
}

// newFetchClient builds the production HTTP client whose dialer rejects any
// connection to a blocked IP (SSRF guard), re-checked on every redirect hop
// because each redirect dials anew. 30s overall timeout.
func newFetchClient() *http.Client {
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
			if err != nil {
				return nil, fmt.Errorf("resolve %s: %w", host, err)
			}
			for _, ip := range ips {
				if isBlockedIP(ip) {
					return nil, fmt.Errorf("refusing to connect to blocked address %s (host %s)", ip, host)
				}
			}
			// Dial the validated IP explicitly so there is no TOCTOU re-resolve.
			return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].String(), port))
		},
	}
	return &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("stopped after 10 redirects")
			}
			if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return fmt.Errorf("redirect to unsupported scheme %q", req.URL.Scheme)
			}
			return nil
		},
	}
}

// isImageMIME / isTextMIME classify the (already lowercased, param-stripped)
// media type for content routing.
func isImageMIME(mt string) bool {
	switch mt {
	case "image/png", "image/jpeg", "image/gif", "image/webp":
		return true
	}
	return false
}

func isTextMIME(mt string) bool {
	if strings.HasPrefix(mt, "text/") {
		return true
	}
	switch mt {
	case "image/svg+xml", "application/json", "application/xml",
		"application/xhtml+xml", "application/javascript", "application/x-ndjson":
		return true
	}
	return false
}

// fetchContent performs a GET, enforces status/size, and routes by content
// type. The client is injected so production passes newFetchClient() (SSRF
// guarded) and tests pass an httptest client.
func fetchContent(ctx context.Context, client *http.Client, rawURL string) (*fetchedContent, error) {
	if _, err := validateFetchURL(rawURL); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", fetchUserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("HTTP %d from %s: %s", resp.StatusCode, resp.Request.URL, strings.TrimSpace(string(snippet)))
	}

	mt := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Type")))
	if i := strings.IndexByte(mt, ';'); i >= 0 {
		mt = strings.TrimSpace(mt[:i])
	}

	out := &fetchedContent{
		FinalURL:    resp.Request.URL.String(),
		Status:      resp.StatusCode,
		ContentType: mt,
	}

	// Sniff when the server gave no usable content type.
	if mt == "" {
		head := make([]byte, 512)
		n, _ := io.ReadFull(resp.Body, head)
		head = head[:n]
		sniffed := strings.ToLower(http.DetectContentType(head))
		if i := strings.IndexByte(sniffed, ';'); i >= 0 {
			sniffed = strings.TrimSpace(sniffed[:i])
		}
		mt = sniffed
		out.ContentType = mt
		// Reconstruct the body = already-read head + remaining stream.
		resp.Body = io.NopCloser(io.MultiReader(strings.NewReader(string(head)), resp.Body))
	}

	switch {
	case isImageMIME(mt):
		data, err := io.ReadAll(io.LimitReader(resp.Body, maxImageBytes+1))
		if err != nil {
			return nil, fmt.Errorf("read image: %w", err)
		}
		if len(data) > maxImageBytes {
			return nil, fmt.Errorf("image too large (over %d MiB cap)", maxImageBytes>>20)
		}
		out.IsImage = true
		out.MIME = mt
		out.Data = data
		return out, nil

	case isTextMIME(mt):
		data, err := io.ReadAll(io.LimitReader(resp.Body, maxTextBytes+1))
		if err != nil {
			return nil, fmt.Errorf("read body: %w", err)
		}
		truncated := false
		if len(data) > maxTextBytes {
			data = data[:maxTextBytes]
			truncated = true
		}
		var b strings.Builder
		fmt.Fprintf(&b, "Fetched %s (HTTP %d, %s, %d bytes)\n\n", out.FinalURL, out.Status, mt, len(data))
		b.Write(data)
		if truncated {
			fmt.Fprintf(&b, "\n\n[truncated: showing first %d bytes]", maxTextBytes)
		}
		out.Text = b.String()
		return out, nil

	default:
		return nil, fmt.Errorf("unsupported content type %q — fetch_url returns images (png/jpeg/gif/webp) and text/JSON/XML/SVG only", mt)
	}
}
