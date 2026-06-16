package main

import (
	"testing"

	"facet/pkg/facet3mf"

	"github.com/firstlayer-xyz/meshio"
)

func TestFacetProjectAttachment_FromEntryFile(t *testing.T) {
	src := "fn Main() Solid {\n\treturn Cube(size: 10)\n}\n"
	sources := map[string]string{"main.fct": src, "other.fct": "fn Helper() {}\n"}

	att, err := facetProjectAttachment(sources, "main.fct", "Main", map[string]interface{}{"width": 40.0})
	if err != nil {
		t.Fatalf("facetProjectAttachment: %v", err)
	}
	proj, ok := facet3mf.Extract(&meshio.Mesh{Attachments: []meshio.Attachment{att}})
	if !ok {
		t.Fatal("Extract ok=false")
	}
	if proj.Source != src {
		t.Fatalf("source = %q", proj.Source)
	}
	if proj.Entry != "Main" || proj.Overrides["width"] != 40.0 {
		t.Fatalf("payload = %+v", proj)
	}
}

func TestFacetProjectAttachment_MissingKeyErrors(t *testing.T) {
	if _, err := facetProjectAttachment(map[string]string{}, "absent.fct", "Main", nil); err == nil {
		t.Fatal("expected error when entry-point source is absent")
	}
}
