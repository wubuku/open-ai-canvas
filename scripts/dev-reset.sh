#!/usr/bin/env bash
# 影策本地 dev 数据重置脚本：清空本地开发数据库，回到「全新空库」状态。
#
# 用途：清掉 seed / 手工测试产生的账号、渠道、模型、项目、任务等数据，
#       让下次 dev.sh + seed 从零开始。只删数据库文件，保留插件 / 技能包（重下很慢）
#       与 .settings-key（加密密钥，可复用），不误伤开发者自己的 seed.local.* 配置。
#
# 用法：
#   ./scripts/dev-reset.sh          # 交互确认后执行
#   ./scripts/dev-reset.sh --yes    # 直接执行（脚本化 / 快速重复）
#
# 前置：后端必须已停止。脚本会检测后端是否仍持有 SQLite；仍运行时拒绝删除，避免数据损坏。

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# 与 dev.sh / backend 默认一致；可用环境变量覆盖（一般无需）。
DATA_DIR="${CANVAS_BACKEND_DATA_DIR:-$REPO_ROOT/.local/project-workbench-debug}"
# 数据库文件名固定（见 backend/internal/database/database.go:34），WAL 模式会带 -wal/-shm 姊妹文件。
DB="$DATA_DIR/open_ai_canvas.db"

log()  { printf '\033[36m[reset]\033[0m %s\n' "$*"; }
warn() { printf '\033[33m[reset]\033[0m %s\n' "$*"; }
die()  { printf '\033[31m[reset]\033[0m %s\n' "$*" >&2; exit 1; }

TARGETS=("$DB" "$DB-wal" "$DB-shm")

# 1) 安全检测：后端仍持有 SQLite 时拒绝删除（否则会写进已删除的 inode，造成数据损坏）。
if command -v lsof >/dev/null 2>&1 && [ -f "$DB" ] && lsof "$DB" >/dev/null 2>&1; then
  die "后端仍在运行并持有数据库，拒绝删除。请先在启动 dev.sh 的终端按 Ctrl+C 停止后端，再执行重置。"
fi

# 2) 确认（--yes 跳过）。
if [ "${1:-}" != "--yes" ]; then
  log "将删除以下本地 dev 数据库文件（保留插件 / 技能包与 .settings-key）："
  found=0
  for f in "${TARGETS[@]}"; do
    if [ -f "$f" ]; then printf '    - %s\n' "$f"; found=1; fi
  done
  [ "$found" = 0 ] && { log "    数据库文件不存在，无需清理。"; exit 0; }
  printf '确认清空并回到全新空库？[y/N] '
  read -r ans || true
  case "${ans:-n}" in
    y|Y|yes|YES) ;;
    *) echo "已取消。"; exit 0 ;;
  esac
fi

# 3) 删除数据库主文件与 WAL/SHM 姊妹文件。
removed=0
for f in "${TARGETS[@]}"; do
  if [ -f "$f" ]; then rm -f "$f"; removed=1; fi
done

if [ "$removed" = 1 ]; then
  log "已清空本地 dev 数据库（.local/project-workbench-debug）。"
else
  log "数据库本来就不存在，无需清理。"
fi

echo
log "回到全新环境的后续步骤："
log "  ./scripts/dev.sh                                  # 一键启动前后端（会重建空库）"
log "  bun scripts/dev-seed-models/seed.ts <清单>        # 重新 seed 模型配置"
log "  # seed 详见 scripts/dev-seed-models/README.md"