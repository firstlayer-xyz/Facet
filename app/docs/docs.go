package docs

import _ "embed"

//go:embed language-reference.md
var LanguageSpec string

//go:embed ai-prompt.md
var AIPrompt string

//go:embed color-guide.md
var ColorGuide string
