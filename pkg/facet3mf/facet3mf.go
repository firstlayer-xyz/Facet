// Package facet3mf defines the Facet project payload embedded as an OPC part in
// exported 3MF files, so a Facet-produced 3MF can be reopened as an editable,
// re-evaluatable project. The schema is owned here; meshio carries the bytes.
package facet3mf

import (
	"encoding/json"
	"fmt"

	"github.com/firstlayer-xyz/meshio"
)

// PartPath is the package-relative location of the Facet payload inside a 3MF.
const PartPath = "Metadata/Facet/project.json"

// ContentType is the OPC content type registered for the Facet payload.
const ContentType = "application/vnd.facet.project+json"

// Version is the current payload schema version.
const Version = 1

// Project is the embedded Facet payload: the entry-point file's source plus the
// entry name and parameter overrides needed to reproduce the exported geometry.
type Project struct {
	Version   int                    `json:"version"`
	Entry     string                 `json:"entry"`
	Overrides map[string]interface{} `json:"overrides,omitempty"`
	Source    string                 `json:"source"`
}

// Marshal encodes p as the meshio.Attachment to embed in a 3MF.
func Marshal(p Project) (meshio.Attachment, error) {
	data, err := json.Marshal(p)
	if err != nil {
		return meshio.Attachment{}, fmt.Errorf("facet3mf: marshal project: %w", err)
	}
	return meshio.Attachment{Path: PartPath, ContentType: ContentType, Data: data}, nil
}

// Extract returns the Facet project embedded in m, or ok=false if the mesh
// carries no Facet part or the part is malformed.
func Extract(m *meshio.Mesh) (*Project, bool) {
	p, err := ExtractStrict(m)
	if err != nil || p == nil {
		return nil, false
	}
	return p, true
}

// ExtractStrict returns the Facet project embedded in m. It returns (nil, nil)
// when no Facet part is present, and a non-nil error when the part exists but
// cannot be decoded or has an unsupported version.
func ExtractStrict(m *meshio.Mesh) (*Project, error) {
	for _, att := range m.Attachments {
		if att.Path != PartPath {
			continue
		}
		var p Project
		if err := json.Unmarshal(att.Data, &p); err != nil {
			return nil, fmt.Errorf("facet3mf: decode %s: %w", PartPath, err)
		}
		if p.Version != Version {
			return nil, fmt.Errorf("facet3mf: unsupported payload version %d (want %d)", p.Version, Version)
		}
		return &p, nil
	}
	return nil, nil
}
