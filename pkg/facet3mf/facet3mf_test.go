package facet3mf

import (
	"testing"

	"github.com/firstlayer-xyz/meshio"
)

func TestMarshalExtractRoundTrip(t *testing.T) {
	p := Project{
		Version:   1,
		Entry:     "Main",
		Overrides: map[string]interface{}{"width": 40.0},
		Source:    "fn Main() Solid {\n  return Cube(size: 10)\n}\n",
	}
	att, err := Marshal(p)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if att.Path != PartPath || att.ContentType != ContentType {
		t.Fatalf("attachment meta = %q / %q", att.Path, att.ContentType)
	}

	m := &meshio.Mesh{Attachments: []meshio.Attachment{att}}
	got, ok := Extract(m)
	if !ok {
		t.Fatal("Extract returned ok=false")
	}
	if got.Entry != "Main" || got.Source != p.Source {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
	if got.Overrides["width"] != 40.0 {
		t.Fatalf("overrides lost: %+v", got.Overrides)
	}
}

func TestExtractAbsent(t *testing.T) {
	m := &meshio.Mesh{}
	if _, ok := Extract(m); ok {
		t.Fatal("expected ok=false when no Facet part present")
	}
}

func TestExtractMalformedErrorsViaOk(t *testing.T) {
	m := &meshio.Mesh{Attachments: []meshio.Attachment{{
		Path:        PartPath,
		ContentType: ContentType,
		Data:        []byte("{not json"),
	}}}
	if _, ok := Extract(m); ok {
		t.Fatal("expected ok=false on malformed JSON")
	}
}

func TestExtractStrictDistinguishesAbsentFromCorrupt(t *testing.T) {
	absent := &meshio.Mesh{}
	if p, err := ExtractStrict(absent); p != nil || err != nil {
		t.Fatalf("absent: got (%v, %v), want (nil, nil)", p, err)
	}

	corrupt := &meshio.Mesh{Attachments: []meshio.Attachment{{
		Path: PartPath, ContentType: ContentType, Data: []byte("{bad"),
	}}}
	if _, err := ExtractStrict(corrupt); err == nil {
		t.Fatal("corrupt: expected error, got nil")
	}

	badVer := &meshio.Mesh{Attachments: []meshio.Attachment{{
		Path: PartPath, ContentType: ContentType, Data: []byte(`{"version":99,"source":"x"}`),
	}}}
	if _, err := ExtractStrict(badVer); err == nil {
		t.Fatal("bad version: expected error, got nil")
	}
}
