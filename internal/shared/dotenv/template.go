package dotenv

import (
	"fmt"
	"strings"
)

type TemplateEntry struct {
	Key        string
	Default    string
	HasDefault bool
}

type Template struct {
	Entries []TemplateEntry
}

func (t Template) Keys() []string {
	keys := make([]string, len(t.Entries))
	for i, entry := range t.Entries {
		keys[i] = entry.Key
	}
	return keys
}

func (t Template) Lookup(key string) (TemplateEntry, bool) {
	for _, entry := range t.Entries {
		if entry.Key == key {
			return entry, true
		}
	}
	return TemplateEntry{}, false
}

func ParseTemplate(data []byte) (Template, error) {
	var tmpl Template
	seen := map[string]struct{}{}
	for _, l := range splitLines(data) {
		if l.text == "" || strings.HasPrefix(l.text, "#") {
			continue
		}
		key, value, err := parseAssignment(l)
		if err != nil {
			return Template{}, err
		}
		if _, dup := seen[key]; dup {
			return Template{}, &ParseError{Line: l.number, Reason: fmt.Sprintf("chave duplicada %q", key)}
		}
		seen[key] = struct{}{}
		tmpl.Entries = append(tmpl.Entries, TemplateEntry{Key: key, Default: value, HasDefault: value != ""})
	}
	return tmpl, nil
}
