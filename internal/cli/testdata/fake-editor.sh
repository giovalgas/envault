#!/bin/sh
set -eu

target=""
for arg in "$@"; do
	target="$arg"
done

if [ -z "$target" ]; then
	echo "fake-editor: arquivo ausente" >&2
	exit 98
fi

if [ -n "${FAKE_EDITOR_LOG:-}" ]; then
	cat "$target" >>"$FAKE_EDITOR_LOG"
fi

if [ -n "${FAKE_EDITOR_CONTENT_FILE:-}" ]; then
	cat "$FAKE_EDITOR_CONTENT_FILE" >"$target"
elif [ -n "${FAKE_EDITOR_CONTENT+set}" ]; then
	printf '%s' "$FAKE_EDITOR_CONTENT" >"$target"
fi

exit "${FAKE_EDITOR_EXIT:-0}"
