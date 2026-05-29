package parser

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

var (
	reScript   = regexp.MustCompile(`(?s)<script[^>]*>(.*?)</script>`)
	reNewArray = regexp.MustCompile(`(?s)new Array\((.*?)\)`)
)

// ExtractFirstScript returns the content of the first <script> block in html.
func ExtractFirstScript(html string) string {
	m := reScript.FindStringSubmatch(html)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

// ExtractVar finds `var name = <value>;` in js and returns the raw value string.
// Handles object literals, new Array(...), and scalar values.
func ExtractVar(js, name string) string {
	// Try object: var name = { ... };
	re := regexp.MustCompile(`(?s)var\s+` + regexp.QuoteMeta(name) + `\s*=\s*(\{[\s\S]*?\})\s*;`)
	if m := re.FindStringSubmatch(js); len(m) >= 2 {
		return strings.TrimSpace(m[1])
	}
	// Try new Array(...): var name = new Array(...);
	re2 := regexp.MustCompile(`(?s)var\s+` + regexp.QuoteMeta(name) + `\s*=\s*new Array\(([\s\S]*?)\)\s*;`)
	if m := re2.FindStringSubmatch(js); len(m) >= 2 {
		return "new Array(" + m[1] + ")"
	}
	// Try scalar: var name = value;
	re3 := regexp.MustCompile(`var\s+` + regexp.QuoteMeta(name) + `\s*=\s*([^;{\n]+?)\s*;`)
	if m := re3.FindStringSubmatch(js); len(m) >= 2 {
		return strings.TrimSpace(m[1])
	}
	return ""
}

// JSToJSON converts a JavaScript value string to valid JSON.
// Handles: unquoted object keys, hex literals, new Array(...) syntax.
func JSToJSON(js string) string {
	s := strings.TrimSpace(js)

	// Convert new Array(...) → [...]
	s = reNewArray.ReplaceAllStringFunc(s, func(m string) string {
		inner := reNewArray.FindStringSubmatch(m)
		if len(inner) < 2 {
			return m
		}
		return "[" + inner[1] + "]"
	})

	// Convert hex literals to decimal (only outside string literals)
	s = convertHexOutsideStrings(s)

	// Quote unquoted object keys (only outside string literals)
	s = quoteUnquotedKeys(s)

	return s
}

// convertHexOutsideStrings replaces hex literals (0x...) with their decimal
// equivalents, skipping content inside double-quoted string literals.
func convertHexOutsideStrings(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	i := 0
	for i < len(s) {
		if s[i] == '"' {
			// Copy entire quoted string verbatim
			b.WriteByte(s[i])
			i++
			for i < len(s) {
				c := s[i]
				b.WriteByte(c)
				i++
				if c == '\\' && i < len(s) {
					b.WriteByte(s[i])
					i++
					continue
				}
				if c == '"' {
					break
				}
			}
			continue
		}
		// Check for hex literal 0x...
		if i+1 < len(s) && s[i] == '0' && (s[i+1] == 'x' || s[i+1] == 'X') {
			j := i + 2
			for j < len(s) && isHexDigit(s[j]) {
				j++
			}
			if j > i+2 {
				v, err := strconv.ParseInt(s[i+2:j], 16, 64)
				if err == nil {
					b.WriteString(strconv.FormatInt(v, 10))
					i = j
					continue
				}
			}
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

func isHexDigit(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

// quoteUnquotedKeys quotes bare identifier keys in JS object literals.
// It skips content inside double-quoted strings to avoid corrupting string values.
func quoteUnquotedKeys(s string) string {
	runes := []rune(s)
	var b strings.Builder
	b.Grow(len(s) + 64)
	i := 0
	for i < len(runes) {
		// Skip quoted strings verbatim
		if runes[i] == '"' {
			b.WriteRune(runes[i])
			i++
			for i < len(runes) {
				c := runes[i]
				b.WriteRune(c)
				i++
				if c == '\\' && i < len(runes) {
					b.WriteRune(runes[i])
					i++
					continue
				}
				if c == '"' {
					break
				}
			}
			continue
		}
		// Check for bare identifier followed by ':'
		// A bare identifier starts with letter or underscore
		if isIdentStart(runes[i]) {
			j := i + 1
			for j < len(runes) && isIdentPart(runes[j]) {
				j++
			}
			// Skip whitespace after identifier
			k := j
			for k < len(runes) && (runes[k] == ' ' || runes[k] == '\t') {
				k++
			}
			if k < len(runes) && runes[k] == ':' {
				// Make sure next char after ':' is not ':' (avoid URL schemes)
				if k+1 < len(runes) && runes[k+1] == ':' {
					// Not a key — write char and advance
					b.WriteRune(runes[i])
					i++
					continue
				}
				// Write as a quoted key
				b.WriteRune('"')
				b.WriteString(string(runes[i:j]))
				b.WriteRune('"')
				// Write any whitespace between key and ':'
				b.WriteString(string(runes[j:k]))
				b.WriteRune(':')
				i = k + 1
				continue
			}
		}
		b.WriteRune(runes[i])
		i++
	}
	return b.String()
}

func isIdentStart(r rune) bool {
	return r == '_' || unicode.IsLetter(r)
}

func isIdentPart(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}
