package main

import (
	"net"
	"testing"
)

func TestIsBlockedIP(t *testing.T) {
	cases := []struct {
		ip      string
		blocked bool
	}{
		{"8.8.8.8", false},
		{"1.1.1.1", false},
		{"127.0.0.1", true},             // loopback
		{"10.0.0.5", true},              // RFC1918
		{"192.168.1.1", true},           // RFC1918
		{"172.16.0.1", true},            // RFC1918
		{"169.254.169.254", true},       // link-local / cloud metadata
		{"0.0.0.0", true},               // unspecified
		{"224.0.0.1", true},             // multicast
		{"::1", true},                   // IPv6 loopback
		{"fc00::1", true},               // IPv6 unique-local
		{"fe80::1", true},               // IPv6 link-local
		{"2606:4700:4700::1111", false}, // public IPv6
	}
	for _, c := range cases {
		ip := net.ParseIP(c.ip)
		if ip == nil {
			t.Fatalf("bad test IP %q", c.ip)
		}
		if got := isBlockedIP(ip); got != c.blocked {
			t.Errorf("isBlockedIP(%s) = %v, want %v", c.ip, got, c.blocked)
		}
	}
}

func TestValidateFetchURL(t *testing.T) {
	if _, err := validateFetchURL("https://example.com/x.png"); err != nil {
		t.Errorf("valid https rejected: %v", err)
	}
	if _, err := validateFetchURL("http://example.com"); err != nil {
		t.Errorf("valid http rejected: %v", err)
	}
	for _, bad := range []string{"file:///etc/passwd", "data:text/plain,hi", "ftp://x/y", "://nohost", "https://"} {
		if _, err := validateFetchURL(bad); err == nil {
			t.Errorf("expected error for %q, got nil", bad)
		}
	}
}
