package dotenv

import (
	"errors"
	"fmt"
	"strings"
)

const (
	descriptionMarker = "@description:"
	tagsMarker        = "@tags:"
	exportPrefix      = "export"
	byteOrderMark     = "\ufeff"
)

var ErrSyntax = errors.New("sintaxe inválida")

type ParseError struct {
	Line   int
	Reason string
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("linha %d: %s", e.Line, e.Reason)
}

func (e *ParseError) Unwrap() error {
	return ErrSyntax
}

type line struct {
	number int
	text   string
}

func splitLines(data []byte) []line {
	text := strings.TrimPrefix(string(data), byteOrderMark)
	raw := strings.Split(text, "\n")
	lines := make([]line, 0, len(raw))
	for i, l := range raw {
		l = strings.TrimSuffix(l, "\r")
		lines = append(lines, line{number: i + 1, text: strings.TrimLeft(l, " \t")})
	}
	return lines
}

func IsBlank(data []byte) bool {
	for _, l := range splitLines(data) {
		if l.text != "" && !strings.HasPrefix(l.text, "#") {
			return false
		}
	}
	return true
}

type envParser struct {
	doc         Document
	keys        map[string]struct{}
	description bool
	tags        bool
}

func Parse(data []byte) (Document, error) {
	p := envParser{keys: map[string]struct{}{}}
	for _, l := range splitLines(data) {
		if err := p.consume(l); err != nil {
			return Document{}, err
		}
	}
	return p.doc, nil
}

func (p *envParser) consume(l line) error {
	switch {
	case l.text == "":
		return nil
	case strings.HasPrefix(l.text, "#"):
		return p.metadata(l)
	}
	key, value, err := parseAssignment(l)
	if err != nil {
		return err
	}
	if _, dup := p.keys[key]; dup {
		return &ParseError{Line: l.number, Reason: fmt.Sprintf("chave duplicada %q", key)}
	}
	p.keys[key] = struct{}{}
	p.doc.Vars = append(p.doc.Vars, Var{Key: key, Value: value})
	return nil
}

func (p *envParser) metadata(l line) error {
	body := strings.TrimLeft(l.text[1:], " \t")
	switch {
	case strings.HasPrefix(body, descriptionMarker):
		if p.description {
			return &ParseError{Line: l.number, Reason: "metadado @description repetido"}
		}
		p.description = true
		p.doc.Description = strings.TrimSpace(body[len(descriptionMarker):])
	case strings.HasPrefix(body, tagsMarker):
		if p.tags {
			return &ParseError{Line: l.number, Reason: "metadado @tags repetido"}
		}
		p.tags = true
		tags, err := parseTags(body[len(tagsMarker):])
		if err != nil {
			return &ParseError{Line: l.number, Reason: err.Error()}
		}
		p.doc.Tags = tags
	}
	return nil
}

func parseTags(raw string) ([]string, error) {
	var tags []string
	for _, part := range strings.Split(raw, ",") {
		tag := strings.TrimSpace(part)
		if tag == "" {
			continue
		}
		if err := ValidateTag(tag); err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	return tags, nil
}

func parseAssignment(l line) (key, value string, err error) {
	text := stripExport(l.text)
	eq := strings.IndexByte(text, '=')
	if eq < 0 {
		return "", "", &ParseError{Line: l.number, Reason: "linha sem '=' fora de comentário"}
	}
	key = strings.TrimSpace(text[:eq])
	if ValidateKey(key) != nil {
		return "", "", &ParseError{Line: l.number, Reason: fmt.Sprintf("chave inválida %q", key)}
	}
	value, reason := parseValue(text[eq+1:])
	if reason != "" {
		return "", "", &ParseError{Line: l.number, Reason: fmt.Sprintf("%s na chave %q", reason, key)}
	}
	return key, value, nil
}

func stripExport(text string) string {
	if len(text) > len(exportPrefix) && strings.HasPrefix(text, exportPrefix) {
		if next := text[len(exportPrefix)]; next == ' ' || next == '\t' {
			return strings.TrimLeft(text[len(exportPrefix):], " \t")
		}
	}
	return text
}

func parseValue(raw string) (value, reason string) {
	v := strings.TrimLeft(raw, " \t")
	if v == "" {
		return "", ""
	}
	switch v[0] {
	case '"':
		return parseDoubleQuoted(v[1:])
	case '\'':
		end := strings.IndexByte(v[1:], '\'')
		if end < 0 {
			return "", "aspas não fechadas"
		}
		return v[1 : 1+end], checkTail(v[2+end:])
	case '#':
		if len(v) < len(raw) {
			return "", ""
		}
	}
	return parseUnquoted(v), ""
}

func parseUnquoted(v string) string {
	for i := 1; i < len(v); i++ {
		if v[i] == '#' && (v[i-1] == ' ' || v[i-1] == '\t') {
			v = v[:i]
			break
		}
	}
	return strings.TrimRight(v, " \t")
}

func parseDoubleQuoted(v string) (value, reason string) {
	var b strings.Builder
	for i := 0; i < len(v); i++ {
		switch c := v[i]; c {
		case '"':
			return b.String(), checkTail(v[i+1:])
		case '\\':
			if i+1 >= len(v) {
				return "", "aspas não fechadas"
			}
			i++
			b.WriteString(unescape(v[i]))
		default:
			b.WriteByte(c)
		}
	}
	return "", "aspas não fechadas"
}

func unescape(c byte) string {
	switch c {
	case 'n':
		return "\n"
	case 't':
		return "\t"
	case 'r':
		return "\r"
	case '"':
		return `"`
	case '\\':
		return `\`
	default:
		return string([]byte{'\\', c})
	}
}

func checkTail(tail string) string {
	rest := strings.TrimLeft(tail, " \t")
	if rest == "" || rest[0] == '#' {
		return ""
	}
	return "conteúdo inesperado depois das aspas"
}
