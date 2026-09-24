package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"strconv"
	"unicode/utf8"
)

const (
	defaultSource  = "skill/envault/SKILL.md"
	defaultOutput  = "internal/skill/domain/content.go"
	packageName    = "domain"
	constantName   = "content"
	outputFilePerm = 0o644
)

func main() {
	source := flag.String("src", defaultSource, "arquivo SKILL.md de origem")
	output := flag.String("out", defaultOutput, "arquivo Go gerado")
	flag.Parse()
	if err := generate(*source, *output); err != nil {
		fmt.Fprintf(os.Stderr, "skillgen: %s\n", err)
		os.Exit(1)
	}
}

func generate(source, output string) error {
	raw, err := os.ReadFile(filepath.Clean(source))
	if err != nil {
		return fmt.Errorf("ler %s: %w", source, err)
	}
	if !utf8.Valid(raw) {
		return errors.New(source + " não é UTF-8 válido")
	}
	code, err := render(raw)
	if err != nil {
		return err
	}
	current, err := os.ReadFile(filepath.Clean(output))
	if err == nil && bytes.Equal(current, code) {
		return nil
	}
	if err := os.WriteFile(filepath.Clean(output), code, outputFilePerm); err != nil {
		return fmt.Errorf("gravar %s: %w", output, err)
	}
	return nil
}

func render(raw []byte) ([]byte, error) {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "package %s\n\nconst %s = %s\n", packageName, constantName, strconv.Quote(string(raw)))
	code, err := format.Source(buf.Bytes())
	if err != nil {
		return nil, fmt.Errorf("formatar código gerado: %w", err)
	}
	return code, nil
}
