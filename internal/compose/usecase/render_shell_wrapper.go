package usecase

import (
	"context"
	"fmt"
	"slices"
)

const (
	ExportFileVar  = "ENVAULT_EXPORT_FILE"
	ExportShellVar = "ENVAULT_EXPORT_SHELL"
)

const posixWrapper = `envault() {
  case "${1-}" in
    load|'') ;;
    *) command envault "$@"; return ;;
  esac
  local __envault_dir __envault_status
  __envault_dir="$(command mktemp -d "${TMPDIR:-/tmp}/envault.XXXXXX")" || return 1
  %[1]s="$__envault_dir/exports" %[2]s=%[3]s command envault "$@"
  __envault_status=$?
  if [ -s "$__envault_dir/exports" ]; then
    . "$__envault_dir/exports"
  fi
  command rm -f -- "$__envault_dir/exports"
  command rmdir -- "$__envault_dir"
  return "$__envault_status"
}
`

const fishWrapper = `function envault
    if set -q argv[1]; and test "$argv[1]" != load
        command envault $argv
        return $status
    end
    set -l __envault_tmp /tmp
    if set -q TMPDIR; and test -n "$TMPDIR"
        set __envault_tmp $TMPDIR
    end
    set -l __envault_dir (command mktemp -d "$__envault_tmp/envault.XXXXXX"); or return 1
    %[1]s="$__envault_dir/exports" %[2]s=%[3]s command envault $argv
    set -l __envault_status $status
    if test -s "$__envault_dir/exports"
        source "$__envault_dir/exports"
    end
    command rm -f -- "$__envault_dir/exports"
    command rmdir -- "$__envault_dir"
    return $__envault_status
end
`

type RenderShellWrapper struct{}

func NewRenderShellWrapper() *RenderShellWrapper {
	return &RenderShellWrapper{}
}

func (*RenderShellWrapper) Execute(_ context.Context, dialect string) (string, error) {
	if !slices.Contains(ShellDialects(), dialect) {
		return "", fmt.Errorf("%w: %q", ErrUnsupportedShell, dialect)
	}
	script := posixWrapper
	if dialect == ShellFish {
		script = fishWrapper
	}
	return fmt.Sprintf(script, ExportFileVar, ExportShellVar, dialect), nil
}

func ShellInitLine(dialect string) string {
	if dialect == ShellFish {
		return "envault shell-init fish | source"
	}
	if !slices.Contains(ShellDialects(), dialect) {
		dialect = ShellZsh
	}
	return fmt.Sprintf(`eval "$(envault shell-init %s)"`, dialect)
}
