#!/usr/bin/env bash
set -euo pipefail

SUITE_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SUITE_ROOT/../.." && pwd)"
command -v rg >/dev/null || { echo 'Missing prerequisite: rg' >&2; exit 1; }
mkdir -p "$REPO_ROOT/.local/cache"
DETACHED_ROOT="$(mktemp -d "$REPO_ROOT/.local/cache/canvas-contract-detached.XXXXXX")"
cleanup() {
    if [[ -d "$DETACHED_ROOT" ]]; then find "$DETACHED_ROOT" -depth -delete; fi
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

cp -R "$SUITE_ROOT/." "$DETACHED_ROOT/"
(
    cd "$DETACHED_ROOT"
    export GOWORK=off
    go test ./... -run '^$'
    go test -c -o "$DETACHED_ROOT/canvas-contract.test" .
    go test ./contract ./runner ./harness -count=1 -race
    UNFORMATTED="$(gofmt -l .)"
    [[ -z "$UNFORMATTED" ]] || { printf 'Unformatted Go source:\n%s\n' "$UNFORMATTED" >&2; exit 1; }
    DEPENDENCIES="$(go list -deps ./...)"
    if printf '%s\n' "$DEPENDENCIES" | rg '^yingce/backend(/|$)' >/dev/null; then
        echo 'HTTP suite must not import backend implementation packages' >&2
        exit 1
    fi
)
bun test "$SUITE_ROOT/check-catalog.test.ts"
PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s "$SUITE_ROOT/browser" -p '*_test.py'
bun "$SUITE_ROOT/check-catalog.ts" >/dev/null
bash -n "$SUITE_ROOT/run-go-baseline.sh" "$SUITE_ROOT/ci.sh"
bash "$SUITE_ROOT/run-go-baseline.sh" --rounds=2 "$@"
