package presenter

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
)

type Presenter struct {
	stdout io.Writer
	stderr io.Writer
}

func New(stdout, stderr io.Writer) *Presenter {
	return &Presenter{stdout: stdout, stderr: stderr}
}

func (p *Presenter) Infof(format string, args ...any) error {
	_, err := fmt.Fprintf(p.stderr, format+"\n", args...)
	return err
}

func (p *Presenter) infoLines(lines []string) error {
	for _, line := range lines {
		if err := p.Infof("%s", line); err != nil {
			return err
		}
	}
	return nil
}

func (p *Presenter) Completion(root *cobra.Command, shell string) error {
	switch shell {
	case "bash":
		return root.GenBashCompletionV2(p.stdout, true)
	case "zsh":
		return root.GenZshCompletion(p.stdout)
	case "fish":
		return root.GenFishCompletion(p.stdout, true)
	default:
		return root.GenPowerShellCompletionWithDesc(p.stdout)
	}
}

func (p *Presenter) writeText(text string) error {
	_, err := io.WriteString(p.stdout, text)
	return err
}

func nonNil(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func joined(values []string) string {
	return strings.Join(values, ", ")
}

func FlagsConflict(first, second string) error {
	return UsageError(fmt.Errorf("--%s e --%s não podem ser usados juntos", first, second))
}
