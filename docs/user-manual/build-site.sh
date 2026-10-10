#!/usr/bin/env bash
set -euo pipefail

manual_dir="$(cd "$(dirname "$0")" && pwd)"
cd "$manual_dir"

if [ "${1:-}" = "--preview" ]; then
  preview_port="4173"
  if [ "${2:-}" = "--port" ] && [ -n "${3:-}" ]; then
    preview_port="$3"
  elif [ -n "${2:-}" ]; then
    preview_port="$2"
  fi
  exec npx vitepress preview . --host 127.0.0.1 --port "$preview_port"
fi

node_version="$(node -p 'process.versions.node.split(".")[0]')"
if [ "$node_version" -lt 18 ]; then
  echo "Node.js 18+ is required" >&2
  exit 1
fi

if [ ! -x "node_modules/.bin/vitepress" ]; then
  npm install --no-audit --no-fund
fi

rm -rf .vitepress/dist .vitepress/cache
npx vitepress build .

test -f .vitepress/dist/index.html
test -f .vitepress/dist/creator/index.html
test -f .vitepress/dist/admin/index.html
printf 'Built user manual at %s\n' "$manual_dir/.vitepress/dist"
