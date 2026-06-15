package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// contentBlock is one block in a stream-json user message.
type contentBlock struct {
	Type   string       `json:"type"`
	Text   string       `json:"text,omitempty"`
	Source *imageSource `json:"source,omitempty"`
}

type imageSource struct {
	Type      string `json:"type"`       // "base64"
	MediaType string `json:"media_type"` // e.g. "image/png"
	Data      string `json:"data"`       // base64 (no newlines)
}

type userFrame struct {
	Type    string `json:"type"` // "user"
	Message struct {
		Role    string         `json:"role"` // "user"
		Content []contentBlock `json:"content"`
	} `json:"message"`
}

// buildUserText assembles the user-visible prompt text: the message plus the
// inlined editor code and any errors. Images are not inlined here; they ride
// the frame as separate content blocks.
func buildUserText(userMessage, editorCode, errorsText string) string {
	return buildPrompt(userMessage, editorCode, errorsText, nil)
}

// buildUserFrame marshals one stream-json user turn: a text block plus one
// base64 image block per attached path. Output is single-line NDJSON (no
// embedded newlines) ready to write to the persistent process's stdin.
func buildUserFrame(text string, imagePaths []string) ([]byte, error) {
	var f userFrame
	f.Type = "user"
	f.Message.Role = "user"
	f.Message.Content = append(f.Message.Content, contentBlock{Type: "text", Text: text})
	for _, p := range imagePaths {
		raw, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("read image %s: %w", p, err)
		}
		f.Message.Content = append(f.Message.Content, contentBlock{
			Type: "image",
			Source: &imageSource{
				Type:      "base64",
				MediaType: imageMediaType(p),
				Data:      base64.StdEncoding.EncodeToString(raw),
			},
		})
	}
	return json.Marshal(f)
}

func imageMediaType(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	default:
		return "application/octet-stream"
	}
}
