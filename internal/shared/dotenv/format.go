package dotenv

import (
	"strings"
)

const EditorInstructions = "#\n" +
	"# Formato: KEY=VALUE, uma por linha. Linhas com # são comentários.\n" +
	"# Salve e feche o editor para aplicar. Deixe o arquivo vazio para cancelar.\n"

var escaper = strings.NewReplacer(
	`\`, `\\`,
	`"`, `\"`,
	"\n", `\n`,
	"\r", `\r`,
	"\t", `\t`,
)

func Format(doc Document) []byte {
	var b strings.Builder
	hasMetadata := doc.Description != "" || len(doc.Tags) > 0
	if doc.Description != "" {
		writeDescription(&b, doc.Description)
	}
	if len(doc.Tags) > 0 {
		writeTags(&b, doc.Tags)
	}
	if hasMetadata && len(doc.Vars) > 0 {
		b.WriteByte('\n')
	}
	writeVars(&b, doc.Vars)
	return []byte(b.String())
}

func FormatEditor(doc Document) []byte {
	var b strings.Builder
	writeDescription(&b, doc.Description)
	writeTags(&b, doc.Tags)
	b.WriteString(EditorInstructions)
	b.WriteByte('\n')
	writeVars(&b, doc.Vars)
	return []byte(b.String())
}

func FormatValue(value string) string {
	if !needsQuotes(value) {
		return value
	}
	return `"` + escaper.Replace(value) + `"`
}

func writeDescription(b *strings.Builder, description string) {
	b.WriteString("# " + descriptionMarker)
	if description != "" {
		b.WriteString(" " + description)
	}
	b.WriteByte('\n')
}

func writeTags(b *strings.Builder, tags []string) {
	b.WriteString("# " + tagsMarker)
	if len(tags) > 0 {
		b.WriteString(" " + strings.Join(tags, ", "))
	}
	b.WriteByte('\n')
}

func writeVars(b *strings.Builder, vars []Var) {
	for _, v := range vars {
		b.WriteString(v.Key)
		b.WriteByte('=')
		b.WriteString(FormatValue(v.Value))
		b.WriteByte('\n')
	}
}

func needsQuotes(value string) bool {
	if value == "" {
		return true
	}
	for _, r := range value {
		if r < 0x20 || r == 0x7f || strings.ContainsRune(" #'\"$=\\`", r) {
			return true
		}
	}
	return false
}
