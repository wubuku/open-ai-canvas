---
title: 本地前后端构建与快速启动指引
status: draft
scope: guide
updated: 2026-10-09
---

# 本地前后端构建与快速启动指引

> **目的**：给出本地起前后端 dev 环境的最小可执行步骤（一键脚本 `scripts/dev.sh` 为首选），并记录两个本机踩过的坑——3000 端口冲突与本机 HTTP 代理干扰，以及用「不常用端口」避开冲突的联动启动方法。
> **状态**：指南草稿；命令均经本机实测，端口/环境变量行为来自源码与 `.env.example`。
> **前置阅读**：`AGENTS.md` 第 7 节「本地开发、部署和数据目录」；`backend/agent-runtime/pi/` 的依赖按需安装。

## 零、一键启动脚本（推荐）

不用记任何命令和坑，直接跑仓库里的 `scripts/dev.sh`：

```bash
./scripts/dev.sh
```

它会自动完成：检查 go/bun → 缺依赖才安装 → 检测端口冲突（3000/8080 被占自动改到 3001/8081 并联动 vite 代理）→ 编译并启动后端 → 启动前端 → 健康检查就绪后打印访问地址。按 `Ctrl+C` 一并停止前后端（进程树递归清理，不留孤儿进程）。

也可指定端口：`CANVAS_BACKEND_PORT=8081 ./scripts/dev.sh`、`CANVAS_FRONTEND_PORT=3001 ./scripts/dev.sh`。

本脚本是 Windows 版 `scripts/start-local.ps1` 的 macOS/Linux 等价实现。下面第二、三、四、五节是手动方式的原理与排障说明，一般无需照做。

## 一、前置要求

| 工具 | 本机实测版本 | 用途 |
| --- | --- | --- |
| Go | 1.25.14 | 后端（`go.mod` 要求 Go 1.25） |
| Bun | 1.3.9 | 前端依赖与 dev server |
| Node | 24.6.0 | Agent runtime（`agent-runtime/pi` 要求 `>=22.19`） |

依赖安装（一次性）：

```bash
cd web && bun install                                       # 走 bun.lock
cd backend/agent-runtime/pi && npm install                  # 走 package-lock.json
```

后端无独立依赖安装步骤，`go build` 时按需拉取模块。

## 二、快速启动（默认端口）

```bash
# 终端 1：后端（默认 :8080）
cd backend
CANVAS_BACKEND_DATA_DIR=../.local/project-workbench-debug go run ./cmd/server

# 终端 2：前端（默认 :3000，代理 /api -> 127.0.0.1:8080）
cd web && bun run dev
```

就绪判定：后端 `curl http://127.0.0.1:8080/api/health` 返回 `ready:true`；前端 `curl http://127.0.0.1:3000/` 返回 200。

## 三、用不常用端口避开冲突（联动启动）

默认端口 `3000`（前端）和 `8080`（后端）容易被其它项目或代理占用。改端口时，**前端和后端必须联动**，否则 vite 的 `/api` 代理仍指向旧端口：

```bash
# 终端 1：后端改到 8081
cd backend
CANVAS_BACKEND_ADDR=:8081 \
CANVAS_BACKEND_DATA_DIR=../.local/project-workbench-debug \
go run ./cmd/server

# 终端 2：前端改到 3001，并让 vite 代理指向新后端端口
cd web
VITE_API_PROXY_TARGET=http://127.0.0.1:8081 \
bun run dev -- --port 3001
```

端口约定与来源：

| 项 | 控制方式 | 来源 |
| --- | --- | --- |
| 后端监听端口 | 环境变量 `CANVAS_BACKEND_ADDR`（默认 `:8080`） | `backend/cmd/server/main.go:77` |
| 后端数据目录 | 环境变量 `CANVAS_BACKEND_DATA_DIR` | `backend/cmd/server/main.go:43`，本地按要求用 `.local/` |
| 前端 dev 端口 | `bun run dev -- --port <n>`（覆盖 `package.json` 里的 `3000`） | `web/package.json` `dev` 脚本 |
| 前端代理目标 | 环境变量 `VITE_API_PROXY_TARGET`（默认 `http://127.0.0.1:8080`） | `web/vite.config.ts` |

排障顺序：先 `lsof -iTCP:8080 -sTCP:LISTEN` 与 `lsof -iTCP:3000 -sTCP:LISTEN` 看端口被谁占用，再决定改哪个端口。改了后端端口，务必将 `VITE_API_PROXY_TARGET` 同步成新后端地址，否则前端页面里所有 `/api` 请求都会打到旧端口而失败。

## 四、坑 1：3000 端口被其它项目占用

本机曾出现 `3000` 被另一个项目（`AICoderTudou/TDCanvas` 的 vite）长期占用。表象：`bun run dev` 正常启动、但实际服务不起来或 vite 提示端口被占。

- **不要**去 kill 无关项目的进程（`AGENTS.md`：不清理非本次产生的变更/进程）。
- **做法**：按第三节改用 `-- --port 3001`（或任意空闲端口），前端无需改代码。

## 五、坑 2：本机 HTTP 代理干扰 localhost 请求

本机 shell 配了 `http_proxy` / `https_proxy` / `all_proxy`（指向 `127.0.0.1:9981`）。`curl` 默认把 `127.0.0.1` 也走代理，导致健康检查返回 **502（Bad Gateway）**，看起来像后端没起，实际后端正常监听：

```bash
# 会得到 502（被代理误导）
curl http://127.0.0.1:8080/api/health/live

# 正确做法一：单次绕过代理
curl --noproxy '*' http://127.0.0.1:8080/api/health/live

# 正确做法二：本次 shell 排除回环地址
export NO_PROXY=127.0.0.1,localhost
```

影响面：只影响走系统代理的 `curl`/`wget` 等命令行回环请求；**浏览器访问 localhost 通常直连、不受影响**（除非浏览器也强制走系统代理且未排除 localhost）。后端其他组件（`go build` 拉模块、`bun install`、前端 vite 内部 `127.0.0.1:8080` 直连）都不走这条代理，无需担心。

## 六、验证记录

本机已实测通过：

- `cd backend && go build ./...` → `BACKEND BUILD OK`。
- `cd web && bun run build` → `✓ built in 3.04s`（`tsc --noEmit` + `vite build`）。
- 后端 `:8080` 启动，`/api/health` 返回 `{"status":"ok","ready":true,"schema":{"current":47,"expected":47}}`。
- 前端 `:3001` 启动，`/` 返回 200，`/api/health/live` 经 vite 代理转发后端返回 200。
- 代理干扰复现：带 `http_proxy` 直接 `curl` localhost 返回 502，加 `--noproxy '*'` 后 200。
- `./scripts/dev.sh` 一键启动：检测到 `3000` 被占自动改用 `3001`（前端 vite 代理联动到后端真实端口），前后端健康检查通过；`Ctrl+C` 后前端、后端进程与端口均干净释放。

未做实：登录、模型 seed、任务生成、SSE 等业务级运行验收——健康检查只能证明入口可用，不能替代业务流程验证（见 `AGENTS.md` 第 8 节）。