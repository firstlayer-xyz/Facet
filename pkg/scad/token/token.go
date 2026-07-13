// Package token defines OpenSCAD lexical tokens.
package token

// Kind identifies the category of a lexical token.
type Kind int

const (
	EOF    Kind = iota
	Ident       // foo, cube, $fn
	Number      // 10, 1.5, .5, 1e3
	String      // "text"
	True
	False
	Undef

	// punctuation
	LParen   // (
	RParen   // )
	LBrace   // {
	RBrace   // }
	LBracket // [
	RBracket // ]
	Semi     // ;
	Comma    // ,
	Colon    // :
	Dot      // .
	Question // ?
	Hash     // #   (modifier)
	Bang     // !   (modifier / logical not)
	Percent  // %   (modifier / modulo)
	Star     // *   (modifier / multiply)

	// operators
	Assign // =
	Plus
	Minus
	Slash
	Lt
	Gt
	Le
	Ge
	EqEq
	NeEq
	And // &&
	Or  // ||

	// keywords
	Module
	Function
	For
	If
	Else
	Let
	Each
	Use
	Include

	// Path is the `<...>` file reference that may follow `use`/`include`.
	// Text holds the inner path (the angle brackets are stripped).
	Path
)

// Token is a single OpenSCAD lexical unit with source position.
type Token struct {
	Kind      Kind
	Text      string // literal text (identifier name, number text, string value)
	Line, Col int
}

var keywords = map[string]Kind{
	"module": Module, "function": Function, "for": For, "if": If,
	"else": Else, "let": Let, "each": Each, "use": Use, "include": Include,
	"true": True, "false": False, "undef": Undef,
}

// Lookup returns the keyword kind for an identifier, or Ident.
func Lookup(s string) Kind {
	if k, ok := keywords[s]; ok {
		return k
	}
	return Ident
}
