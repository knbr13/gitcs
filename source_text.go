package main

import (
	"strconv"
	"strings"
)

// Import and module declarations are found with patterns rather than a full
// parser, which means a commented-out import would otherwise look exactly like
// a real one. Blanking comments first is what keeps the graph honest, and doing
// it with a scanner rather than a regex is what keeps string literals -- which
// may contain "//" -- from being mistaken for the start of a comment.

type commentSyntax struct {
	// Rust allows /* /* nested */ */; C-family languages do not.
	nestedBlocks bool
	// Quote characters that open a string literal. Rust is deliberately given
	// only `"`, because a lone `'` there is far more likely to be a lifetime
	// than a character literal.
	stringDelimiters string
}

var (
	ecmaScriptComments = commentSyntax{stringDelimiters: "\"'`"}
	rustComments       = commentSyntax{nestedBlocks: true, stringDelimiters: "\""}
)

// stripComments replaces every comment with a space, leaving all other bytes
// where they are so offsets and line structure survive.
func stripComments(source string, syntax commentSyntax) string {
	var out strings.Builder
	out.Grow(len(source))

	blockDepth := 0
	inLineComment := false
	var quote byte

	for index := 0; index < len(source); index++ {
		character := source[index]

		switch {
		case blockDepth > 0:
			if syntax.nestedBlocks && character == '/' && index+1 < len(source) && source[index+1] == '*' {
				blockDepth++
				index++
				out.WriteString("  ")
				continue
			}
			if character == '*' && index+1 < len(source) && source[index+1] == '/' {
				blockDepth--
				index++
				out.WriteString("  ")
				continue
			}
			out.WriteByte(blankOutside(character))

		case inLineComment:
			if character == '\n' {
				inLineComment = false
				out.WriteByte('\n')
				continue
			}
			out.WriteByte(' ')

		case quote != 0:
			out.WriteByte(character)
			if character == '\\' && index+1 < len(source) {
				index++
				out.WriteByte(source[index])
				continue
			}
			if character == quote {
				quote = 0
			}

		case character == '/' && index+1 < len(source) && source[index+1] == '/':
			inLineComment = true
			index++
			out.WriteString("  ")

		case character == '/' && index+1 < len(source) && source[index+1] == '*':
			blockDepth = 1
			index++
			out.WriteString("  ")

		default:
			if strings.IndexByte(syntax.stringDelimiters, character) >= 0 {
				quote = character
			}
			out.WriteByte(character)
		}
	}

	return out.String()
}

// Newlines are kept inside block comments so that line-anchored patterns
// elsewhere still see the same line boundaries as the original file.
func blankOutside(character byte) byte {
	if character == '\n' {
		return '\n'
	}
	return ' '
}

// --- String masking -------------------------------------------------------

// A string literal is a value, not code, so a file that merely *talks about* an
// import -- a code generator, a test fixture, a docs snippet -- must not be read
// as performing one. Masking replaces each literal's contents with an opaque
// token, leaving the quotes in place so patterns still match the shape of a
// statement, and only a token that came from a real literal can be read back.

type maskedLiterals []string

// lookup returns the original text behind a masked token. A capture that is not
// a token was never a string literal, and callers should discard it.
func (literals maskedLiterals) lookup(token string) (string, bool) {
	if len(token) < 2 || token[0] != maskSentinel || token[len(token)-1] != maskSentinel {
		return "", false
	}
	index := 0
	for _, character := range []byte(token[1 : len(token)-1]) {
		if character < '0' || character > '9' {
			return "", false
		}
		index = index*10 + int(character-'0')
	}
	if index >= len(literals) {
		return "", false
	}
	return literals[index], true
}

// Chosen because it cannot appear in source: patterns that scan "anything but a
// quote" will happily match it, and nothing else will produce it.
const maskSentinel = '\x01'

func maskStringContents(code string) (string, maskedLiterals) {
	var out strings.Builder
	out.Grow(len(code))

	var literals maskedLiterals
	var quote byte
	var literal strings.Builder

	for index := 0; index < len(code); index++ {
		character := code[index]

		if quote == 0 {
			if character == '"' || character == '\'' || character == '`' {
				quote = character
				literal.Reset()
			}
			out.WriteByte(character)
			continue
		}

		if character == '\\' && index+1 < len(code) {
			literal.WriteByte(code[index+1])
			index++
			continue
		}

		if character != quote {
			literal.WriteByte(character)
			continue
		}

		out.WriteByte(maskSentinel)
		out.WriteString(strconv.Itoa(len(literals)))
		out.WriteByte(maskSentinel)
		out.WriteByte(character)
		literals = append(literals, literal.String())
		quote = 0
	}

	// An unterminated literal is broken source; keep what was read so the rest
	// of the file still yields whatever it legitimately imports.
	if quote != 0 {
		out.WriteString(literal.String())
	}

	return out.String(), literals
}
