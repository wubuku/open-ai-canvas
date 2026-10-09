---
title: 项目整体认知与文档系统审计
status: draft
scope: research
updated: 2026-10-09
---

# 项目整体认知与文档系统审计

> **目的**：把当前代码库的主要架构结论、调用链、部署边界和文档发现性问题集中到一篇低冲突研究稿中，作为后续文档体系建设的事实入口。
> **状态**：研究草稿；不替代正式 API、数据库或发布文档。
> **证据范围**：当前 Git checkout 的源码、配置、文档、CI、提交历史和实际命令结果。

## 一、结论摘要

影策不是单一的 AI 生图画布，而是面向 AI 影视、短剧和视觉预演的创作工作台。产品主线由自由画布、结构化短剧项目、创作页、素材库、任务中心、模型渠道、插件、计费和云端 Agent 组成。

当前项目的工程重心已经从“页面功能开发”扩展到“可靠的媒体任务平台”：任务准入、模型路由、额度计费、异步 Worker、上游轮询、资源保存、版本冲突、Agent 审批和恢复机制共同组成关键闭环。

文档内容已经能够解释产品定位、前后端边界和部分代码入口，但当前仓库没有完整的 Next.js/Fumadocs 文档应用，文档构建命令不可执行，导航中存在失效页面。因此当前文档系统适合继续做研究和内容沉淀，还不适合作为自洽的发布文档站。

## 二、项目定位和主要用户路径

### 产品定位

- 项目名称：影策（`ddcat-ai/open-ai-canvas`）。
- 目标：让故事从文字进入角色、场景、分镜、图片、视频、音频和时间线产物。
- 当前形态：快速开发中的个人、本地或可信环境部署型 AI 影视创作工作台。
- 主要外部入口：浏览器前端、模型供应商/聚合渠道、对象存储、支付渠道和可选的独立 Agent runtime。

### 用户路径

- `/create`：从文本、灵感、参考素材和提示词开始创作。
- `/canvas`、`/canvas/:id`：以节点、连线和媒体结果组织创作过程。
- `/projects/...`：管理短剧项目、章节、工作流、角色、场景和分镜。
- `/tasks`、`/assets`、`/inspirations`：查看异步任务、素材和灵感资源。
- `/admin/...`：配置用户、模型渠道、逻辑模型、插件、支付、存储、功能开关和运行策略。

## 三、前端架构

### 技术与入口

- 入口：`web/src/application.tsx`、`web/src/router.tsx`。
- 技术栈：Vite、React、TypeScript、React Router、Ant Design、Tailwind、Zustand、TanStack Query。
- 画布/媒体相关：tldraw、Excalidraw、Leafer、Three.js、React Three Fiber、FFmpeg WASM、Vidstack。
- 全局 Providers、主题和应用外壳由 `web/src/components/layout/`、`web/src/lib/app-theme.ts` 和相关 stores 共同组成。

### 页面与职责

- `web/src/pages/` 负责路由页面和页面流程协调，不直接拼装后端协议。
- `web/src/components/` 承担跨页面 UI 和交互能力；画布组件集中在 `web/src/components/canvas/`。
- `web/src/services/api/` 负责业务 API、任务、资源、渠道、模型和 Agent 协议适配。
- `web/src/stores/` 负责跨页面状态；画布状态进一步拆到 `web/src/stores/canvas/`。
- `web/src/lib/` 负责纯函数、画布算法、协议转换、提示词、资源引用和可独立测试的基础能力。

### API 和本地数据

- 业务 JSON 统一经 `web/src/services/api/request.ts` 的 `http` 调用。
- 后端业务响应采用 `{ code, data, msg, reason }` 信封；非零业务 `code` 转为 `ApiError`。
- 二进制、媒体 blob、资源流和 SSE 使用各自的明确边界，不应重新绕过统一业务 API。
- 用户切换需要隔离 React Query、localforage、资源字节缓存和画布状态。
- 画布和素材的大对象主要使用带用户作用域的 localforage；localStorage 只保存小配置、用户 scope 和 UI 偏好。

### 画布是复杂度中心

画布不是一个单一组件，而是由页面协调器、状态、节点渲染、连接策略、生成执行器、资源引用、同步/版本历史、任务状态和 Agent 面板组成。主要入口包括：

- `web/src/pages/canvas/project.tsx`：项目画布页面和多种工作流协调。
- `web/src/components/canvas/canvas-workspace-panel.tsx`：工作区面板和工具/资产/任务/历史壳。
- `web/src/stores/canvas/`：画布数据、UI、历史、预演台等状态。
- `web/src/lib/canvas/`：文档模型、布局、连接、引用、生成批次、同步和性能算法。
- `web/src/pages/canvas/use-canvas-*.ts`：生命周期、生成、选择、连接、上传、保存和预演等流程 hook。

## 四、后端架构

### 启动和组合根

后端入口是 `backend/cmd/server/main.go`。启动顺序包括：

1. 读取环境变量和日志配置。
2. 打开 SQLite 或 PostgreSQL 数据库。
3. 自动迁移或校验数据库 schema 版本。
4. 初始化内置模型、提示词、项目工作流、技能和工具。
5. 创建 repository 和 app service。
6. 注册 `/api`、OAuth 回调、健康状态和系统代理。
7. 启动任务、资源删除、技能同步、支付和 Agent 恢复 Worker。

### 调用方向

```text
HTTP
  -> handler
  -> service（稳定导入面，主要是 aliases）
  -> app（组合根与跨域编排）
  -> 业务域包
  -> repository / model / database / resource
  -> protocol / generation / outbound / payment
```

当前 `backend/internal/service/` 主要通过 `aliases_*.go` 暴露 `internal/app` 的类型和函数；主要业务实现集中在 `backend/internal/app/`。已抽出的域包包括 `auth`、`canvas`、`skills`、`prompts`、`platform`、`kernel`、`outbound`、`agentcontext` 和部分 `assets`。

### HTTP API

API 路由集中由 `backend/internal/handler/api.go` 注册，覆盖：

- 认证、OAuth、用户和管理员；
- 画布、项目、章节、工作流和分享；
- 任务、文本事件流、时间线转写和渲染；
- 资源上传、资源访问、素材库和删除引用保护；
- 模型渠道、逻辑模型、模型目录和自定义中转；
- Agent、Agent 记忆、技能、工具和插件；
- 钱包、积分、账单、支付和对账；
- 外观、公告、系统更新和运行策略。

HTTP 参数解析、鉴权上下文和统一响应留在 handler；业务规则和归属校验由 service/app/domain 执行。

## 五、任务和模型生成链路

### 任务准入

`backend/internal/app/task_creation.go` 的 `CreateTask` 是常规生成任务的主要写入口。它负责：

- 标准化 prompt 和任务输入；
- 判断 Agent 任务是否通过正确的 Agent 接口进入；
- 校验功能开关、用户归属、工具 mention 和工作流插件；
- 从服务端模型目录解析逻辑模型、渠道和协议；
- 校验模型能力、媒体类型、参考资源和输入边界；
- 进行预算、额度、价格档、并发和存储准入；
- 拒绝不能安全落盘的内嵌媒体 data URL；
- 持久化任务，并把执行交给 Worker。

因此浏览器提交的模型、渠道或计费信息不是最终事实；执行和计费前仍会由服务端重新解析和校验。

### 执行、轮询和恢复

- `task_worker.go` 负责任务租约、Worker 生命周期和下一项任务调度。
- `task_execution.go` 是按任务类型分派的执行入口。
- `provider_*.go` 处理文本、图片、视频、音频、SSE、轮询、取消和错误归一化。
- `internal/protocol/` 提供供应商协议和声明式 Manifest 适配。
- `internal/generation/` 提供生成域注册和执行边界。
- 任务执行过程会记录日志、上游请求 ID、轮询状态和可恢复信息。
- 上游视频任务可能延期轮询，而不是让 Worker 在请求中长时间睡眠。
- 生成结果需要经过资源保存、业务引用绑定和节点/项目回写，保存失败与模型生成失败要分层表达。

### 前端与后端事件

文本任务通过 `GET /api/tasks/:id/text-events` 发送 `progress`、`delta` 和 `terminal` 事件；递增事件 ID 是断线续传游标，不是任务 ID。Agent 使用独立的运行事件流和检查点合同。

## 六、Agent 与插件

### Agent 控制面

Go 后端负责 Agent 的权限、上下文、工具能力、审批、画布变更、记忆、计费、任务准入、事件持久化和恢复。提示词、技能和用户偏好不能自行扩大服务端工具权限。

画布 Agent 相关入口包括：

- 前端：`web/src/components/canvas/canvas-cloud-agent-panel.tsx`、`web/src/services/api/agent.ts`。
- 后端：`backend/internal/handler/agent.go`、`backend/internal/app/cloud_agent*.go`。
- 能力注册：`backend/internal/canvas/capability/`。
- 上下文检查点：`backend/internal/agentcontext/`。
- 原生 Pi 协议：`backend/internal/agent/runtime/`、`backend/agent-runtime/pi/agent-runtime.mjs`。

### 独立 Agent runtime

本地开发不配置 `YINGCE_AGENT_URL` 时，Go 后端可在本进程中启动 Node runtime。生产 Compose 使用独立 `yingce-agent` 容器；该服务不直接连接数据库，也不直接执行计费、审批或画布写入，而是通过 bridge token 与 Go 后端交换模型、工具和事件。具体边界见 `yingce-agent/README.md` 和 `docker-compose.deploy.yml`。

### 插件系统

插件通过 `.yingce-plugin` 包和 Manifest 接入，覆盖：

- 模型协议和 Provider；
- 图片、视频、音频和工作流执行器；
- Eagle 等外部素材连接器；
- 提示词优化和 AI 分析能力；
- 支付渠道运行时。

插件不能任意注册宿主路由、GORM model、后台 Worker，也不能直接读取 Cookie、API Key、数据库或后端数据目录。系统插件、官方应用插件和自定义插件有不同的安装、启用和配置边界。

## 七、数据、资源和部署

### 数据库和持久化

- 本地默认 SQLite，数据目录由 `CANVAS_BACKEND_DATA_DIR` 控制。
- 生产部署使用 PostgreSQL；`backend/internal/database/schema.go` 维护模型清单和迁移。
- Redis 用于生产环境的协调、租约、缓存或运行时策略。
- 关键表覆盖用户认证、模型渠道、逻辑模型、计费支付、资源素材、项目分镜、画布快照、任务、Agent 会话和技能插件状态。

### 资源存储

资源可以存放在后端数据目录或对象存储。当前存储实现覆盖本地、阿里云 OSS、腾讯云 COS、七牛和 S3 兼容服务。资源访问 URL、供应商可访问 URL、CDN 和上传/删除引用保护由后端统一控制。

### 部署拓扑

```text
浏览器
  -> Nginx / web:3000
  -> backend:8080（Compose 内网）
       -> PostgreSQL
       -> Redis
       -> 本地/对象存储
       -> 模型供应商和协议渠道
       -> yingce-agent:8081（生产独立 runtime）
```

本地开发主要使用 `backend/` 的 Go server、`web/` 的 Vite dev server 和 SQLite。生产编排由 `docker-compose.deploy.yml` 提供 PostgreSQL、Redis、独立迁移、backend、web 和 Agent 服务。

### 安全边界

- 默认拒绝本机、私网和链路本地模型上游；开发时仅按主机精确放行。
- API Key 不应出现在 URL、日志、错误上报或长期明文存储中。
- 后端对象读取、更新、删除必须校验用户和资源归属。
- 素材物理删除前需要检查项目、画布、任务和其他引用。
- SSE 仅对明确的事件流路径关闭 Nginx 缓冲，不能对所有 `/api/` 请求套用流式策略。

## 八、当前文档系统审计

### 已有优点

- `README.md` 已有产品能力、启动方式、安全边界和架构概览。
- `docs/content/docs/backend/code-map.mdx` 已经给出后端调用链和域包职责。
- `http-api.mdx` 对 Agent、错误信封、SSE、资源和计费合同记录较详细。
- `docs/content/docs/plugins/plugin-system.mdx` 能解释插件 Manifest、权限和宿主边界。
- `docs/content/docs/progress/pending-test.mdx` 能区分“代码已实现”和“真实环境未验收”。

### 构建缺口

当前 checkout 没有：

- `docs/package.json`；
- `docs/source.config.ts`；
- Next.js/Fumadocs app/page/layout；
- 文档站专用 CI 构建 job。

因此 `cd docs && bun run types:check` 和 `cd docs && bun run build` 当前都失败，并不是项目文档站的有效验证命令。文档站究竟是外部工程、生成目录还是尚未迁回仓库，需要项目维护者确认。

### 导航缺口

- README 目前引用了不存在的 `docs/content/docs/overview/quick-start.mdx`。
- README 目前引用了不存在的 `docs/content/docs/canvas/canvas-node-manual.mdx`。
- README 和 AGENTS 目前引用了不存在的 `docs/content/docs/progress/todo.mdx`。
- `docs/content/docs/backend/meta.json` 引用了不存在的 `protocol-plugins` 和 `canvas-data-structure`。
- `meta.json` 没有覆盖全部实际存在的发布 MDX 页面。
- `docs/index.md`、README、MDX frontmatter 和导航元数据没有共享可验证的页面清单。

### 可发现性判断

- 产品定位和大架构：通过 README 可以较快发现。
- 后端分层：通过 `code-map.mdx` 可以发现，但需要继续跳转源代码。
- 任务准入、Worker、协议和计费：分散在代码地图、HTTP API 和 `backend/internal/app/`，缺少统一时序图。
- Agent Go 控制面、Pi runtime 和独立 `yingce-agent`：分散在代码地图、部署文件和 runtime README 中，发现成本较高。
- 文档构建和发布边界：当前无法从仓库自洽确认。

## 九、后续建议

1. 先决定文档站应用是否属于本仓库；不要在缺少事实依据时添加新的 Fumadocs scaffold。
2. 以本目录草稿为低冲突事实区，补一篇“页面 -> API -> 任务 -> provider -> 资源/账单”的时序文档。
3. 修复或删除 README 和 `meta.json` 中的失效页面引用，优先做小范围增量修改。
4. 增加文档链接完整性检查，并在文档站归属明确后增加真实构建检查。
5. 对稳定的架构选择再写入 `docs/adr/`；不要把未验收的推断直接提升为正式合同。

## 十、验证记录

### 已执行

- 检查 `git status --short --branch`、remote、分支和当前提交。
- 检查 `README.md`、`AGENTS.md`、`docs/index.md`、`docs/content/docs/`、`docs/design/`、`docs/adr/`、`docs/plans/` 和 `docs/superpowers/`。
- 检查前端入口、路由、统一 API client、画布状态和服务目录。
- 检查后端启动、API 注册、Service 组合根、任务准入、数据库、存储、Provider、协议和部署 Compose。
- 执行 `cd docs && bun run types:check`：失败，`Script not found "types:check"`。
- 执行 `cd docs && bun run build`：失败，`Script not found "build"`。

### 未执行

- 未启动前端或后端服务。
- 未运行 `cd web && bun run build`、前端测试或浏览器 E2E。
- 未运行 `cd backend && go test ./...`。
- 未对真实登录、模型上游、对象存储、支付、SSE 或 Agent 容器做运行验收。

以上未执行项不能被本研究稿描述为已验证能力。
