package stdlib

import (
	"embed"
	"io/fs"
	"sort"
	"strings"
)

//go:embed libraries
var Libraries embed.FS

//go:embed libraries/facet/std/*.fct
var stdFiles embed.FS

// StdlibSource is the concatenation of every .fct under libraries/facet/std/,
// joined in lexical filename order with a separating blank line. The split is
// purely for human maintainability — the compiler still sees the standard
// library as one source.
var StdlibSource = mergeStdlib()

func mergeStdlib() string {
	entries, err := fs.ReadDir(stdFiles, "libraries/facet/std")
	if err != nil {
		panic("stdlib: " + err.Error())
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".fct") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	var b strings.Builder
	for i, name := range names {
		data, err := stdFiles.ReadFile("libraries/facet/std/" + name)
		if err != nil {
			panic("stdlib: " + err.Error())
		}
		if i > 0 {
			b.WriteString("\n")
		}
		b.Write(data)
	}
	return b.String()
}
