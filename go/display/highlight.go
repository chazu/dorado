package display

import "unicode"

// TokenKind identifies a syntax element for highlighting.
type TokenKind int

const (
	TokPlain TokenKind = iota
	TokKeyword          // method:, classMethod:, subclass:, instanceVars:, ^
	TokString           // 'single-quoted strings'
	TokComment          // "double-quoted comments"
	TokSymbol           // #symbol, #(array literal)
	TokNumber           // 42, 3.14
	TokSelector         // message selectors in sends
	TokSelf             // self, super, true, false, nil, thisContext
	TokBlock            // [ ] | |
	TokAssign           // :=
)

// Token represents a colored span of text.
type Token struct {
	Start int       // byte offset in the line
	End   int       // byte offset end (exclusive)
	Kind  TokenKind
}

// DefaultTheme returns the color for each token kind.
func DefaultTheme(kind TokenKind) uint32 {
	switch kind {
	case TokKeyword:
		return ColorRGB(140, 40, 140)  // purple
	case TokString:
		return ColorRGB(0, 120, 60)    // green
	case TokComment:
		return ColorRGB(120, 120, 120) // gray
	case TokSymbol:
		return ColorRGB(0, 100, 180)   // blue
	case TokNumber:
		return ColorRGB(180, 80, 0)    // orange
	case TokSelf:
		return ColorRGB(140, 40, 140)  // purple (same as keyword)
	case TokBlock:
		return ColorRGB(80, 80, 80)    // dark gray
	case TokAssign:
		return ColorRGB(180, 0, 0)     // red
	default:
		return ColorRGB(0, 0, 0)       // black
	}
}

// TokenizeLine tokenizes a single line of Maggie source code.
// Returns tokens with rune-based start/end offsets.
func TokenizeLine(line string) []Token {
	runes := []rune(line)
	var tokens []Token
	i := 0

	for i < len(runes) {
		ch := runes[i]

		// Comment: "..."
		if ch == '"' {
			start := i
			i++
			for i < len(runes) && runes[i] != '"' {
				i++
			}
			if i < len(runes) {
				i++ // closing quote
			}
			tokens = append(tokens, Token{start, i, TokComment})
			continue
		}

		// String: '...'
		if ch == '\'' {
			start := i
			i++
			for i < len(runes) {
				if runes[i] == '\'' {
					i++
					// Double quote escape
					if i < len(runes) && runes[i] == '\'' {
						i++
						continue
					}
					break
				}
				i++
			}
			tokens = append(tokens, Token{start, i, TokString})
			continue
		}

		// Symbol: #word or #( or #'...'
		if ch == '#' {
			start := i
			i++
			if i < len(runes) {
				if runes[i] == '\'' {
					// #'symbol with spaces'
					i++
					for i < len(runes) && runes[i] != '\'' {
						i++
					}
					if i < len(runes) {
						i++
					}
				} else if runes[i] == '(' {
					// #(array literal) — just highlight the #(
					i++
				} else if isIdentStart(runes[i]) {
					for i < len(runes) && isIdentChar(runes[i]) {
						i++
					}
					// Include trailing colons for keyword symbols
					if i < len(runes) && runes[i] == ':' {
						i++
					}
				}
			}
			tokens = append(tokens, Token{start, i, TokSymbol})
			continue
		}

		// Number
		if unicode.IsDigit(ch) || (ch == '-' && i+1 < len(runes) && unicode.IsDigit(runes[i+1])) {
			start := i
			if ch == '-' {
				i++
			}
			for i < len(runes) && unicode.IsDigit(runes[i]) {
				i++
			}
			if i < len(runes) && runes[i] == '.' && i+1 < len(runes) && unicode.IsDigit(runes[i+1]) {
				i++
				for i < len(runes) && unicode.IsDigit(runes[i]) {
					i++
				}
			}
			tokens = append(tokens, Token{start, i, TokNumber})
			continue
		}

		// Assignment :=
		if ch == ':' && i+1 < len(runes) && runes[i+1] == '=' {
			tokens = append(tokens, Token{i, i + 2, TokAssign})
			i += 2
			continue
		}

		// Return ^
		if ch == '^' {
			tokens = append(tokens, Token{i, i + 1, TokKeyword})
			i++
			continue
		}

		// Blocks [ ] and temporaries | |
		if ch == '[' || ch == ']' {
			tokens = append(tokens, Token{i, i + 1, TokBlock})
			i++
			continue
		}

		// Identifier / keyword
		if isIdentStart(ch) {
			start := i
			for i < len(runes) && isIdentChar(runes[i]) {
				i++
			}
			// Check for trailing colon (keyword)
			if i < len(runes) && runes[i] == ':' {
				i++
			}
			word := string(runes[start:i])
			kind := classifyWord(word)
			tokens = append(tokens, Token{start, i, kind})
			continue
		}

		// Skip other characters
		i++
	}

	return tokens
}

func isIdentStart(r rune) bool {
	return unicode.IsLetter(r) || r == '_'
}

func isIdentChar(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

func classifyWord(word string) TokenKind {
	switch word {
	case "self", "super", "true", "false", "nil", "thisContext":
		return TokSelf
	case "method:", "classMethod:", "subclass:", "instanceVars:", "import:", "namespace:", "trait:", "include:":
		return TokKeyword
	}
	return TokPlain
}

// DrawStringHighlighted renders a line with syntax highlighting.
func DrawStringHighlighted(dst *Form, x, y int, line string, font *Font) int {
	tokens := TokenizeLine(line)
	runes := []rune(line)

	if len(tokens) == 0 {
		return DrawStringFont(dst, x, y, line, ColorRGB(0, 0, 0), font)
	}

	cx := x
	lastEnd := 0

	for _, tok := range tokens {
		// Draw any plain text before this token
		if tok.Start > lastEnd {
			plain := string(runes[lastEnd:tok.Start])
			cx = DrawStringFont(dst, cx, y, plain, ColorRGB(0, 0, 0), font)
		}
		// Draw the token
		text := string(runes[tok.Start:tok.End])
		color := DefaultTheme(tok.Kind)
		cx = DrawStringFont(dst, cx, y, text, color, font)
		lastEnd = tok.End
	}

	// Draw any trailing plain text
	if lastEnd < len(runes) {
		plain := string(runes[lastEnd:])
		cx = DrawStringFont(dst, cx, y, plain, ColorRGB(0, 0, 0), font)
	}

	return cx
}
