#!/usr/bin/env bash
# 影策本地 dev 环境一键启动（macOS / Linux），等价于 Windows 的 scripts/start-local.ps1。
#
# 用法：
#   ./scripts/dev.sh                 # 默认端口 8080 / 3000，冲突时自动改到 8081 / 3001 …
#   CANVAS_BACKEND_PORT=8081 ./scripts/dev.sh   # 指定后端端口
#   CANVAS_FRONTEND_PORT=3001 ./scripts/dev.sh  # 指定前端端口
#
# 已内建处理两个坑，无需手工记忆：
#   1. 端口冲突：默认 8080（后端）/ 3000（前端）被占时自动递增找空闲端口，
#      前端 vite 的 /api 代理（VITE_API_PROXY_TARGET）联动指向实际后端端口。
#   2. 本机 HTTP 代理：NO_PROXY 追加 127.0.0.1/localhost/::1，健康检查不会再被代理误导返回 502。
#
# 依赖、数据目录、Go 缓存都落在仓库内 .local/（已被 .gitignore 忽略），不污染家目录。

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BACKEND_DIR="$REPO_ROOT/backend"
WEB_DIR="$REPO_ROOT/web"
DATA_DIR="$REPO_ROOT/.local/project-workbench-debug"
CACHE_DIR="$REPO_ROOT/.local/cache"

BACKEND_PORT="${CANVAS_BACKEND_PORT:-8080}"
FRONTEND_PORT="${CANVAS_FRONTEND_PORT:-3000}"

BACKEND_LOG="/tmp/yingce-backend.log"
WEB_LOG="/tmp/yingce-web.log"

log()    { printf '\033[36m[dev]\033[0m %s\n' "$*"; }
warn()   { printf '\033[33m[dev]\033[0m %s\n' "$*"; }
die()    { printf '\033[31m[dev]\033[0m %s\n' "$*" >&2; exit 1; }

# 端口是否已在监听（macOS 用 lsof，Linux 优先 ss，兜底 /dev/tcp）。
is_port_used() {
    if command -v lsof >/dev/null 2>&1; then
        lsof -nP -iTCP:"$1" -sTCP:LISTEN >/dev/null 2>&1
    elif command -v ss >/dev/null 2>&1; then
        ss -ltn "sport = :$1" >/dev/null 2>&1
    else
        (echo >"/dev/tcp/127.0.0.1/$1") >/dev/null 2>&1
    fi
}

# 从给定端口开始，找到第一个空闲端口（静默，仅返回端口号；
# 「端口被占」提示由调用方在命令替换之外打印，避免污染端口变量）。
pick_free_port() {
    local port="$1"
    while is_port_used "$port"; do
        port=$((port + 1))
    done
    printf '%s' "$port"
}

# ---- 运行时检查 ----
command -v go  >/dev/null 2>&1 || die "未找到 go（要求 Go 1.25+），请先安装。"
command -v bun >/dev/null 2>&1 || die "未找到 bun，请先安装。"
command -v npm >/dev/null 2>&1 || die "未找到 npm（Agent runtime 依赖需要），请先安装。"

# ---- 依赖安装（缺才装） ----
if [ ! -d "$WEB_DIR/node_modules" ]; then
    log "安装前端依赖（bun install）..."
    (cd "$WEB_DIR" && bun install --frozen-lockfile)
fi
if [ ! -d "$BACKEND_DIR/agent-runtime/pi/node_modules" ]; then
    log "安装 Agent runtime 依赖（npm install）..."
    (cd "$BACKEND_DIR/agent-runtime/pi" && npm install --no-fund --no-audit)
fi

# ---- 数据目录与缓存 ----
mkdir -p "$DATA_DIR" "$REPO_ROOT/.local/bin" "$CACHE_DIR/go-build" "$CACHE_DIR/go-mod"

# ---- 端口与代理：规避两个坑 ----
orig_backend="$BACKEND_PORT"
orig_frontend="$FRONTEND_PORT"
BACKEND_PORT="$(pick_free_port "$BACKEND_PORT")"
FRONTEND_PORT="$(pick_free_port "$FRONTEND_PORT")"
[ "$BACKEND_PORT" != "$orig_backend" ]  && warn "端口 ${orig_backend} 已被占用，后端改用 ${BACKEND_PORT}"
[ "$FRONTEND_PORT" != "$orig_frontend" ] && warn "端口 ${orig_frontend} 已被占用，前端改用 ${FRONTEND_PORT}"
# 前端 /api 代理联动到实际后端端口。
export VITE_API_PROXY_TARGET="http://127.0.0.1:${BACKEND_PORT}"
# 回环地址不走系统代理，避免 curl 健康检查被代理误导。
export NO_PROXY="${NO_PROXY:+${NO_PROXY},}127.0.0.1,localhost,::1"
export no_proxy="$NO_PROXY"

# ---- 启动后端 ----
# 先增量编译成仓库内二进制再 exec 运行，避免 go run 再包一层子进程，
# 让 $! 直接指向最终 server 进程，Ctrl+C 时无需沿进程树递归清理。
log "编译后端（首次较慢，之后增量）..."
(
    cd "$BACKEND_DIR"
    GOCACHE="$CACHE_DIR/go-build" GOMODCACHE="$CACHE_DIR/go-mod" \
    go build -o "$REPO_ROOT/.local/bin/yingce-server" ./cmd/server
)

log "启动后端 :${BACKEND_PORT}（数据目录 ${DATA_DIR}）..."
(
    cd "$BACKEND_DIR"
    CANVAS_BACKEND_ADDR=":${BACKEND_PORT}" \
    CANVAS_BACKEND_DATA_DIR="$DATA_DIR" \
    GOCACHE="$CACHE_DIR/go-build" \
    GOMODCACHE="$CACHE_DIR/go-mod" \
    exec "$REPO_ROOT/.local/bin/yingce-server"
) >"$BACKEND_LOG" 2>&1 &
BACKEND_PID=$!

# ---- 启动前端 ----
log "启动前端 :${FRONTEND_PORT}（/api 代理 -> 127.0.0.1:${BACKEND_PORT}）..."
(
    cd "$WEB_DIR"
    # 直接 exec vite，避免 bun run 再包一层子进程；--port 覆盖 package.json 的默认 3000。
    exec "$WEB_DIR/node_modules/.bin/vite" --host 0.0.0.0 --port "$FRONTEND_PORT"
) >"$WEB_LOG" 2>&1 &
WEB_PID=$!

# 递归终止一个进程及其全部后代。主进程虽已通过 exec 直启（避免 go run / bun run
# 的中间层），仍保留递归作为兜底，清理 vite 可能 spawn 的 esbuild worker 等孙进程。
kill_tree() {
    local pid="$1" child
    for child in $(pgrep -P "$pid" 2>/dev/null); do
        kill_tree "$child"
    done
    kill "$pid" 2>/dev/null || true
}

cleanup() {
    log "停止前后端进程..."
    kill_tree "$BACKEND_PID"
    kill_tree "$WEB_PID"
    wait "$BACKEND_PID" "$WEB_PID" 2>/dev/null || true
}
trap 'exit 130' INT
trap 'exit 143' TERM
trap cleanup EXIT

# ---- 健康检查 ----
wait_http() {
    local url="$1" label="$2" logfile="$3"
    local deadline=$(( $(date +%s) + 180 ))
    log "等待 ${label} 就绪（${url}）..."
    while [ "$(date +%s)" -lt "$deadline" ]; do
        if curl -fsS --noproxy '*' --max-time 2 "$url" >/dev/null 2>&1; then
            log "${label} 就绪 ✅"
            return 0
        fi
        # 服务已退出则立刻报错，不再空等。
        kill -0 "$BACKEND_PID" 2>/dev/null || kill -0 "$WEB_PID" 2>/dev/null || {
            warn "${label} 启动失败，日志见 ${logfile}："
            tail -n 20 "$logfile" >&2 || true
            return 1
        }
        sleep 1
    done
    warn "${label} 未在 3 分钟内就绪，日志见 ${logfile}："
    tail -n 20 "$logfile" >&2 || true
    return 1
}

wait_http "http://127.0.0.1:${BACKEND_PORT}/api/health" "后端" "$BACKEND_LOG" || exit 1
wait_http "http://127.0.0.1:${FRONTEND_PORT}/"             "前端" "$WEB_LOG"    || exit 1

echo
log "影策本地 dev 环境已就绪："
log "  前端  http://localhost:${FRONTEND_PORT}"
log "  后端  http://127.0.0.1:${BACKEND_PORT}"
log "按 Ctrl+C 同时停止前后端。"

# 保持前台运行，任一服务退出则收尾退出。
while kill -0 "$BACKEND_PID" 2>/dev/null; do
    kill -0 "$WEB_PID" 2>/dev/null || break
    sleep 1
done