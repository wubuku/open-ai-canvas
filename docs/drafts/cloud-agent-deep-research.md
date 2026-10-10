# 云端 Agent 深度调研报告

> 本文是基于当前仓库源码、模型、HTTP handler、前端 Agent 面板、Pi runtime、独立 `yingce-agent`、Skill 服务和已有测试做的深度静态调研。它首先服务于开发者和 AI Agent 的代码导航；未经过真实登录态、真实模型、SSE 断线和独立容器联调的内容不会写成运行验收结论。

> **阅读关系与术语。** 本文是跨模块研究草稿：[架构总览](../content/docs/overview/architecture.mdx)说明稳定的系统边界，[画布开发导航](../content/docs/canvas/canvas-development.mdx)说明画布代码入口，[代码地图](../content/docs/backend/code-map.mdx)负责从功能定位代码，[HTTP API](../content/docs/backend/http-api.mdx)负责协议细节；本文补充它们之间的执行链、证据和待验证边界。下文中的 **Pi runtime** 指运行在 Node.js 进程中的 Pi SDK 会话执行器；**bridge** 指 Go 控制面与 Node runtime 之间的受控请求通道；**SSE**（Server-Sent Events，服务器向浏览器单向推送事件）指 Agent 前端的观察和回放通道；**Skill** 指产品内版本化的工作流参考材料，和项目协作者使用的 `.agents/skills` 不是同一条加载路径。

## 先读结论

当前的 Agent 是一个由浏览器、Go 控制面、任务系统和 Pi Node runtime 组成的持久化执行系统。它不是“前端把聊天消息发给一个模型 API”这么简单，也不是一个拥有服务器文件系统权限的通用 Coding Agent。

最重要的边界如下：

1. **浏览器负责交互和观察。** 画布 Agent 面板负责选择模型、权限模式、Skill、上下文、提交消息、展示事件、收集审批和把画布增量应用到本地编辑器。
2. **Go 后端负责授权和事实。** `backend/internal/handler/agent.go` 接收请求，`backend/internal/app/cloud_agent.go` 完成幂等和运行准入，`backend/internal/app/cloud_agent_*.go` 系列文件负责策略、上下文、工具、审批、任务、持久化和恢复。
3. **Pi runtime 负责模型循环。** Node runtime 负责把服务端提供的工具交给 Pi SDK，执行“模型请求 -> 工具调用 -> 工具结果 -> 下一轮模型请求”的循环；它不决定用户能不能读写画布，也不直接访问数据库。
4. **媒体生成仍然是后端任务。** Agent 只能提出结构化的 `generate_media` 或 `image_layer_split` 工具调用；真正的模型路由、模型能力、价格、额度、资源引用、任务提交、Worker 执行和结果回写仍走现有 Go 任务链。
5. **Skill 不是授权。** Skill 可以提供工作流知识和参考材料，但工具权限、节点能力、审批、预算、资源归属和安全校验永远由服务端代码决定。
6. **运行可以脱离浏览器继续。** 每轮 Agent 是一个持久化运行；根 `cloud_agent` 任务是非计费的控制面载体，正常模型轮次由 `cloud_agent_step` 子任务承载，上下文/记忆压缩使用专用任务，媒体生成则进入媒体任务和既有 Worker/计费链路。浏览器的 SSE 只是观察通道，断开不会自动终止运行。

可以把主链路压缩为：

```text
用户输入
  -> web/src/components/canvas/canvas-cloud-agent-panel.tsx
  -> web/src/services/api/agent.ts
  -> POST /api/agent/runs 或 /api/agent/runs/:id/messages
  -> handler/agent.go
  -> app.CreateCloudAgentRun
  -> Task + CloudAgentExecution
  -> Pi runtime
       -> /model bridge -> Go 文本任务 / Worker / 上游模型
       -> /tool bridge  -> Go 工具调度、审批、画布写入、媒体任务
       -> /event bridge -> Go 事件、会话快照和检查点
  -> append-only event journal + SSE
  -> 前端事件归并、画布 patch 或全量校准
```

如果只想继续开发某一部分，可按下面的入口进入：

| 目标 | 第一入口 | 继续阅读 |
| --- | --- | --- |
| 修改 Agent 面板或提交行为 | `web/src/components/canvas/canvas-cloud-agent-panel.tsx` | `web/src/services/api/agent.ts`、`web/src/services/cloud-agent-conversations.ts` |
| 修改事件流或断线重连 | `web/src/services/api/agent.ts` | `backend/internal/handler/agent.go`、`web/src/components/canvas/canvas-cloud-agent-events.ts` |
| 新增或修改只读工具 | `backend/internal/app/cloud_agent_tools.go` | `backend/internal/app/cloud_agent_tools_read.go`、`backend/internal/app/cloud_agent_runtime_tools.go` |
| 新增画布写操作 | `backend/internal/app/cloud_agent_tools.go` | `backend/internal/app/cloud_agent_tools_canvas.go`、`backend/internal/app/cloud_agent_mutation.go`、`backend/internal/app/cloud_agent_runtime_tools.go` |
| 修改媒体生成 | `backend/internal/app/cloud_agent_media.go` | `backend/internal/app/cloud_agent_runtime_media.go`、`backend/internal/app/task_creation.go`、`backend/internal/app/task_worker.go`、`backend/internal/provider/`、`backend/internal/protocol/` |
| 修改 Pi 与 Go 的桥接 | `backend/internal/app/cloud_agent_pi_coordinator.go` | `backend/internal/app/cloud_agent_pi_model.go`、`backend/internal/app/cloud_agent_pi_bridge.go`、`backend/internal/agent/runtime/` |
| 修改独立 Agent 容器 | `yingce-agent/server.mjs` | `yingce-agent/README.md`、`backend/agent-runtime/pi/agent-runtime.mjs` |
| 修改 Skill 读取和版本快照 | `backend/internal/app/cloud_agent_tools_skills.go` | `backend/internal/skills/`、`backend/internal/model/models_platform.go` |
| 修改偏好、记忆或计划 | `backend/internal/app/cloud_agent_policy.go` | `backend/internal/app/cloud_agent_profile.go`、`backend/internal/app/cloud_agent_lessons.go`、`backend/internal/app/cloud_agent_plan.go` |
| 修改 Agent 行为规则 | `backend/internal/prompts/agent-system-policy.md` | `docs/content/docs/backend/agent-prompt-policy.mdx` |

## 1. 调研范围和证据分级

本文使用四种标记理解代码结论：

- **已实现**：源码中存在完整的注册、调用和持久化路径，或有针对性测试支持。
- **代码意图**：源码或文档明确表达了设计目标，但本轮没有运行真实环境验证。
- **静态推断**：由多个入口和调用关系推导出的架构结论，仍需真实运行补证。
- **待验证**：需要登录态、真实渠道、真实资源、长时间任务、SSE 断线或独立容器才能确认。

重点证据位于：

- Agent 请求、运行、幂等和续轮：`backend/internal/app/cloud_agent.go`。
- HTTP 合同和 SSE：`backend/internal/handler/agent.go`、`web/src/services/api/agent.ts`。
- Go 调度器和 Pi 生命周期：`backend/internal/app/cloud_agent_runtime_scheduler.go`、`backend/internal/app/cloud_agent_pi_coordinator.go`。
- Pi 请求和隔离运行时：`backend/internal/app/cloud_agent_pi_request.go`、`backend/agent-runtime/pi/agent-runtime.mjs`。
- 工具 schema 和工具编译：`backend/internal/app/cloud_agent_tools.go`、`backend/internal/app/cloud_agent_tools_read.go`、`backend/internal/app/cloud_agent_tools_skills.go`。
- 画布读写和媒体：`backend/internal/app/cloud_agent_runtime_tools.go`、`backend/internal/app/cloud_agent_media.go`、`backend/internal/app/cloud_agent_runtime_media.go`。
- 运行持久化：`backend/internal/model/cloud_agent.go`、`backend/internal/repository/cloud_agent.go`。
- Skill 数据和服务：`backend/internal/model/models_platform.go`、`backend/internal/skills/`、`backend/internal/repository/skill_packages.go`。
- 前端会话、提交和画布同步：`web/src/components/canvas/canvas-cloud-agent-panel.tsx`、`web/src/services/cloud-agent-conversations.ts`、`web/src/services/agent-canvas-sync.ts`。

## 2. Agent 到底能做什么

### 2.1 画布理解

Agent 的画布上下文不是简单把整个 JSON 一次塞给模型。服务端会根据当前画布、用户焦点节点和上下文预算构建增强画布上下文；模型还可以按需调用工具精读。

当前主要能力包括：

- `canvas_list_node_types`：读取服务端注册的节点能力、用途、默认尺寸、可更新字段、输入类型和连接约束。它是 Agent 选择节点类型的事实来源，不能凭 UI 名称猜 `nodeType`。
- `canvas_get_state`：读取画布目录、节点、连线、整画布 `snapshotHash` 和媒体准入使用的 `mediaSnapshotHash`。可以分页读取全部节点，或用 `nodeIds` 精读指定节点，用 `focusNodeIds + depth` 读取邻域，用 `focusNodeIds + includeRelated` 读取连通分量。
- 角色卡节点会返回结构化的设定、形象表示和图片/音频引用；角色卡的 `content` 为空不代表角色卡没有内容。
- 生成字段会区分草稿、任务状态和可用的输出资源引用；模型不能把历史提示词、节点标题或候选素材列表当成真实画面观察。
- `canvas_read_storyboard`：按页读取结构化分镜脚本的镜头行，取得真实 `rowId` 和该分镜节点的 `snapshotHash`。
- `canvas_read_batch_table`：按页读取批量创作表的配置、参考图列、任务行和预览，取得行级 `rowId`、该批量表节点的 `snapshotHash` 与参考图 `mentionToken`。
- `previs_scene_read`：读取预演场景目录或场景/镜头/对象的摘要，并取得预演专用快照哈希。
- `task_get`：查询当前用户、当前画布下的任务状态和脱敏诊断信息。

画布读取有两个保护：只读调用会做参数和次数约束，重复同参数读取可以命中运行内缓存；画布写入会使画布读取缓存失效，避免模型依据旧快照继续提交修改。

### 2.2 视觉理解和图像辅助

在服务端根据本轮模型能力确认 `VisionEnabled` 后，Agent 才会获得 `canvas_inspect_image`。它读取画布图片的真实内容交给模型，用于判断构图、色彩、光线、风格、画面文字或素材内容。

以下边界很关键：

- 图片标题、提示词、标签和资源候选不是视觉事实；需要知道画面内容时必须查看实际图片。
- 同一张图片在一轮中的附图次数受限，重复查看通常只返回文字回执，不能用 `refresh` 绕过限制。
- `image_text_detect` 只准备文字识别所需的安全图片引用和输出格式，不会直接把识别结果写回画布。
- `image_annotation_render` 根据归一化坐标生成临时 PNG 标注参考图，并通过当前用户资源存储和短期 lease 管理；它不修改原图片节点。
- 文字编辑仍要以原图和标注图为参考，进入正常图片生成、审批和任务链。

### 2.3 画布写入和预演操作

当前云端 Agent 主工具合同以 `compileCloudAgentTools` 产生的工具为准。下面列出主要画布写操作，以及不直接写服务端画布的预演请求：

| 工具 | 能做什么 | 关键前置和限制 |
| --- | --- | --- |
| `canvas_apply_ops` | 创建节点、更新节点字段、移动节点、建立连接 | 必须提交最新 `snapshotHash`；每次最多 20 项；不能删除节点、写任意 metadata 或写媒体 URL/storage key |
| `canvas_arrange_nodes` | 自动排版、对齐、等距和分组整理 | 只改坐标；最多 50 个节点；跳过锁定节点、容器和不应被移动的子节点 |
| `canvas_create_storyboard` | 创建结构化分镜脚本节点和真实镜头行 | 适用于多镜头、连续性和逐镜维护；不能用普通 Markdown 伪装结构化分镜 |
| `canvas_edit_storyboard` | 追加、修改或删除单个分镜行 | 必须使用最新 `rowId` 和该分镜节点的 `snapshotHash`；只能改允许的镜头文本和时长字段 |
| `canvas_edit_batch_table` | 编辑批量创作表行、参考图列、模式、并发和全局提示词 | 必须使用最新行 ID 和该批量表节点的 `snapshotHash`；只改计划，不提交收费生成；不能写输出节点、任务状态或资源地址 |
| `canvas_create_character` | 将形象图片及可选声音打包成角色卡并放置节点 | 需要已有就绪图片；已有同名角色应优先复用；设定只能填有依据的内容 |
| `generate_media` | 提交图片、视频或音频生成 | 先读画布的 `mediaSnapshotHash` 和模型目录；服务端做能力、参考素材、价格、额度和任务准入；所有可用模式都走独立审批 |
| `image_layer_split` | 对图片做对象拆分并生成透明图层 | 复用媒体准入；先创建草稿，用户批准后才提交任务 |
| `previs_scene_create` | 创建预演场景并绑定可打开工作台的 video 工作站节点 | 必须提供当前 `canvasSnapshotHash` |
| `previs_apply_patch` | 修改预演场景、镜头、对象、相机、灯光和动画 | 使用预演 `snapshotHash`；最多 32 项；语义 patch 不能提交原始 JSON、URL 或 storage key |
| `previs_preview` | 请求当前浏览器预演视口录制白模预演 | 先读取真实 `sceneId`、`shotId`；后端只校验并返回 `previewRequestId`，前端通过 `previs:preview-requested` 事件交给当前预演工作台录制，不创建后端媒体任务或进入 Worker |

`canvas_apply_ops` 的操作会用能力注册表验证来源节点、目标节点、输入类型、句柄和结构化行引用。分镜行级连接必须使用读取结果里的 `row:<rowId>`，不能把节点 ID 当作行 ID。

### 2.4 计划、追问、偏好和记忆

Agent 还具备一组不直接修改画布的协作能力：

- `plan_update`：维护最多 20 项的结构化计划。计划状态只表示工作组织，不扩大权限，也不能把取消项伪装为完成。
- `ask_user`：在需求歧义会显著影响结果时显示选项或表单。本轮问答会暂停并自动续轮，服务端限制确认轮数。
- `agent_profile_read`：按用户、项目、画布作用域读取已经在本轮快照里列出的长期偏好。偏好属于不可信用户数据，不能改变工具或权限。
- `recall_lessons`：召回用户已经批准的个人经验；系统只提供索引，Agent 按需读取正文。
- `remember_lesson`：把本轮成功经验写入待用户批准的记忆，而不是直接成为未来强制指令。
- `model_list`：查询当前可用的媒体模型能力、价格和参考素材约束。模型选择不能凭模型名猜测。

### 2.5 明确不具备的能力

不能把以下内容写成当前 Agent 已经拥有：

- 任意读取仓库文件、执行 Shell、访问浏览器或访问 Web。代码中确实存在 `routeStandardTool` 和 `read_file`、`bash`、`web_search` 等兼容路由，但当前运行请求不会把它们加入平台工具 schema；即使直接进入这些路由，文件、Shell、Web handler 也返回“未配置/不可用”。Pi runtime 同时关闭了默认 filesystem skills、extensions、prompt templates 和 context files。
- 直接访问数据库。Node runtime 和独立 `yingce-agent` 均通过 Go bridge 工作。
- 直接持有供应商密钥或自行选择未授权上游。模型请求由 Go 的渠道、逻辑模型、能力和计费系统接管。
- 通过 Skill 获得新的权限。Skill 的文本只能影响工作方法和参考知识，不能授权删除节点、提交媒体或越过审批。
- 依靠前端隐藏按钮实现安全。每一次对象读取、写入、任务创建、审批和资源访问仍需服务端校验。

## 3. 前端是如何实现的

### 3.1 面板组成

画布 Agent 的主要入口是 `web/src/components/canvas/canvas-cloud-agent-panel.tsx`。它协调以下职责：

- 加载和保存当前用户作用域下的本地对话列表。
- 读取 Agent capabilities、模型目录、Skill 市场和已安装技能。
- 管理当前对话、父运行、权限模式、已启用 Skill、待审批和待追问状态。
- 组装画布节点、素材和 Skill 的 `@` 引用，以及 Skill 的 `/` 快速选择。
- 首次提交前保存完整请求和幂等键；提交成功后保存服务端运行 ID。
- 运行期间允许继续输入；此时发送入口改为 `interjection`，不与停止按钮混用。
- 处理 SSE 事件、运行快照、审批确认、取消和画布 patch。

输入组件 `web/src/components/canvas/canvas-cloud-agent-composer.tsx` 支持：

- 普通文本输入和附件。
- `@` 选择画布节点、素材、Skill 和附件。
- `/` 搜索当前已安装 Skill。
- 场景胶囊和预设 Skill 组合；缺少的技能可以加入用户技能库。
- 运行中插话、停止、表单回答和审批相关的状态联动。

### 3.2 对话缓存和可靠提交

`web/src/services/cloud-agent-conversations.ts` 使用带用户 scope 的 `localforage` 保存对话 UI 快照。它保存的是面板恢复所需的标题、消息、运行 ID、模型、权限模式和 Skill ID，不是服务端执行的事实源。

提交过程有一条容易被忽略但非常关键的顺序：

1. 生成当前请求、父运行 ID、消息 ID、请求指纹和幂等键。
2. 把完整的待提交正文和身份写入本地 pending 记录。
3. 先保存可以显示这条用户消息的对话快照。
4. 调用 `POST /agent/runs` 或 `POST /agent/runs/:id/messages`。
5. 收到服务端运行后，清除 pending 记录并绑定运行。

如果网络超时或响应丢失，前端使用原来的正文和原来的幂等键重放。pending 记录损坏时会暂停发送，让用户先去任务中心核对运行，不能丢掉身份后生成一个可能重复计费的新请求。

后端以“用户 + 幂等键”派生稳定运行 ID，并保存请求 fingerprint。相同幂等键对应不同正文、父运行或配置时返回冲突，而不是静默接受第二个请求。

### 3.3 SSE 事件流

前端通过 `subscribeAgentEvents` 连接：

```text
GET /api/agent/runs/:id/events?after=<last-seq>
Last-Event-ID: <last-seq>
Accept: text/event-stream
```

事件流有两类消息：

- `agent_event`：有服务端递增 `seq`，表示可重放的事实事件。前端只有收到合法且大于当前 cursor 的事件才推进游标。
- `run_snapshot`：运行状态观察值，没有事件 ID，不推进游标。它用于补充 status、revision、active message、approval、spent credits、step 和 failure message。

连接中断时前端按递增延迟重连；如果运行尚未结束，服务端从 `after` 之后继续回放。15 秒心跳和 45 秒前端 watchdog 用于识别无数据连接。不能把每个快照当成一个新的业务事件，否则会导致重复消息、错误游标和错误的断线恢复。

### 3.4 前端画布同步

`web/src/services/agent-canvas-sync.ts` 维护一个按画布 ID 过滤的协调器：

1. 收到 `canvas_updated` 等事件时优先提取 `canvasPatch`。
2. 在约 40ms 窗口内合并多个 patch，批量应用到当前 store 和编辑器。
3. 需要画布同步的事件缺少 patch、patch 应用失败或事件流出错时，标记 `needsRefresh`；普通文本和状态事件不会因为没有 patch 自动刷新。
4. 全量刷新受约一秒的节流和单请求在途约束，重复通知只保留一次尾随刷新。
5. 画布同步只处理当前画布；其他画布事件直接忽略。

这使 Agent 的节点变更可以快速反映在编辑器中，同时不需要每个工具事件都重新拉整张画布。前端会尽量保留用户正在进行的无关拖动、视口和字段编辑；全量校准是缺口或冲突时的恢复手段。

## 4. Go 后端控制面

### 4.1 HTTP 入口

`backend/internal/handler/agent.go` 注册的主接口包括：

| 方法 | 路径 | 作用 |
| --- | --- | --- |
| `GET` | `/api/agent/capabilities` | 返回权限模式、上下文范围、Skill 开关、工具全集哈希和节点类型 |
| `GET/PATCH` | `/api/agent/profile` | 读取或 CAS 更新用户、项目、画布偏好 |
| `POST` | `/api/agent/runs` | 创建第一轮 Agent 运行 |
| `POST` | `/api/agent/runs/:id/messages` | 以已结束运行作为父轮创建下一轮 |
| `POST` | `/api/agent/runs/:id/interjections` | 向运行中 Agent 投递插话 |
| `GET` | `/api/agent/runs/:id` | 读取运行状态和分页事件窗口 |
| `GET` | `/api/agent/runs/:id/events` | 以 SSE 方式观察和增量回放运行 |
| `POST` | `/api/agent/runs/:id/cancel` | 请求取消当前运行 |
| `POST` | `/api/agent/runs/:id/undo` | 在快照和任务条件满足时撤销 Agent 画布变更 |
| `POST` | `/api/agent/runs/:id/approvals/:approvalId/decision` | 提交审批或拒绝，并可带媒体设置 |
| `GET` | `/api/agent/skills/usage` | 查询当前用户自己的 Skill 使用收据 |

handler 层负责登录、请求体大小、未知字段拒绝、速率限制、参数解析和统一错误返回；业务授权和数据库操作在 `internal/app`，不是靠 handler 的 UI 条件。

### 4.2 运行准入

`CreateCloudAgentRun` 大致按以下顺序处理：

1. 校验画布 ID、提示词、模型选择、权限模式、Skill ID、焦点节点、上下文范围、预算和幂等键。
2. 验证当前用户拥有画布且画布已经同步到服务端。
3. 读取并冻结有效的用户/项目/画布偏好快照。
4. 校验 Skill 是否属于当前用户、已经加入技能库且启用。
5. 根据逻辑模型或受管渠道解析模型，不接受浏览器传入的自定义密钥和任意上游地址。
6. 建立当前请求的可信历史、创作锚点、策略快照、工具 schema 和 Skill 快照。
7. 用事务创建非计费的根 `Task` 控制面载体，并把本轮最大积分作为后续模型/媒体任务的上限；并发重复提交只有一个请求可以成功提交。
8. 确保 `CloudAgentExecution` 存在并启动或恢复 Pi session。

运行请求支持：

- `read_only`：只能读取和分析；不能设置生成预算。
- `request_approval`：写入操作需要用户审批，媒体生成额外需要独立审批。
- `auto`：普通画布写操作可直接执行，但媒体生成和图层拆分仍必须经过审批。

`maxSteps=0` 表示不采用固定的小步数截断，实际运行仍受管理端运行策略、上下文预算、读工具上限、任务预算、超时和模型任务状态约束。

### 4.3 可信历史和提示词编译

服务端把多种来源编译为本轮上下文：

- 版本化系统策略：`backend/internal/prompts/agent-system-policy.md` 和 `backend/internal/prompts/agent-media-policy.md`。
- 当前用户请求与父轮可信历史。
- 画布摘要、焦点节点、结构化内容和资源候选。
- 本轮权限、预算、模型合同和能力集合。
- 已冻结的 Skill 元数据和可读路径。
- 偏好层的版本、哈希和内容。
- 已批准记忆的索引，以及模型自己通过工具召回的经验。
- 计划、待处理插话、工具结果、审批和任务状态等带来源的运行事实。

服务端会把参考素材标题、Skill 正文和用户偏好作为数据而非系统授权。提示词中写着“可以删除节点”不会扩大权限；Skill 文档中写着“必须调用某个工具”也不会跳过服务端的工具白名单。

系统策略、工具 schema、能力集、模型选择和偏好版本参与 prompt cache identity。画布动态内容不应破坏稳定前缀，但画布快照改变会影响工具读取和写入的 `snapshotHash`。

## 5. Pi runtime 和三个 bridge

### 5.1 Go 和 Node 的分工

Go 的 `cloudAgentPiCoordinator` 是生产入口，负责：

- 为每个运行最多启动一个 Pi session。
- 审批恢复时复用或排队重启，不让旧 session 和新 session 同时写检查点。
- 服务退出时取消并等待所有 Pi session 和已批准媒体等待器。
- 启动时扫描仍处于活动状态的运行并恢复。
- 发现 Pi 运行失败时更新运行状态并写入 `run_failed`。

Node 的 `backend/agent-runtime/pi/agent-runtime.mjs` 负责：

- 建立一次性临时 HOME、Agent 目录、工作目录和 session 目录。
- 注册一个由 Go bridge 管理的虚拟模型 provider。
- 把 Go 提供的工具 schema 包装为 Pi custom tools。
- 让 Pi SDK 管理多轮消息、tool call、上下文压缩和 session JSONL。
- 对工具返回的审批暂停信号调用 session abort，把“等待用户”与“运行失败”区分开。

运行时启动时显式关闭 `noExtensions`、`noSkills`、`noPromptTemplates`、`noThemes` 和 `noContextFiles`，并设置 `projectTrusted=false`。因此它不会自动向上查找仓库中的 `.agents/skills`，也不会读取宿主机的 `~/.pi` 配置。平台 Skill 只能通过 Go 的 `skill_search` 和 `skill_read_file` 读取。

### 5.2 `/model` bridge

Node 看到的模型不是直接的外部供应商。Pi runtime 每次要生成模型响应时向 Go 发送 `/model`：

1. runtime 提交当前 system prompt、canonical messages、工具 schema、thinking level 和上下文使用量。
2. Go 校验运行身份、模型任务状态、上下文预算和模型能力。
3. Go 通过受管文本任务和已有 provider 协议调用实际模型。
4. 模型结果被转换为文本和结构化 tool calls。
5. Go 保存模型任务、费用和运行检查点，再把结果交给 runtime 进入下一轮。

这种方式让“模型调用”继续使用已有的任务、Worker、计费、取消、重试分类和审计路径。Node 只看到虚拟 provider `yingce`，不知道真实 API key 和上游地址。

### 5.3 `/tool` bridge

Node 把 Pi 的工具调用提交到 Go 的 `/tool` bridge，Go 会：

1. 重新读取数据库中的运行检查点，而不是相信 Node 的内存状态。
2. 确认 call ID、工具名和参数与本轮模型实际声明完全一致。
3. 如果 bridge 响应丢失而 Node 重试相同 call，先重放已经提交的 tool message，避免重复副作用。
4. 按本轮声明的工具白名单、权限模式、资源归属、快照哈希和参数 schema 执行。
5. 对只读工具做缓存和批量执行；对写入和收费媒体工具保持顺序边界。
6. 把结果写入 canonical 历史、事件日志和运行检查点。
7. 对需要继续模型循环的调用返回文本结果；对审批返回 `pause`，由 Node 中止本轮并等待服务端恢复。

Go 侧 `executeCloudAgentRuntimeTool` 对重复请求使用 `callId` 查找已有 tool message。这个幂等处理是 Agent 可以在网络异常后安全恢复的关键。

### 5.4 `/event` bridge

Pi runtime 的会话事件通过 `/event` 回到 Go，典型内容包括消息开始/结束、文本快照、上下文压力、上下文压缩、审批暂停和 session snapshot。

Go 不把 Node 的事件当作唯一状态，而是把关键事件和控制字段写入 `CloudAgentExecution`、append-only event journal、消息记录和 Pi session snapshot。浏览器随后通过 `/events` SSE 读取服务端事实。

## 6. Skill 的完整生命周期

### 6.1 Skill 数据模型

Skill 相关模型分为几层：

- `Skill`：技能市场和当前技能的摘要、作者、描述、来源、状态、当前版本和内容哈希。
- `SkillVersion`：一个不可变技能版本、入口路径、包键、文件数量、总字节数和来源提交。
- `SkillFile`：版本内的文件路径、类别、MIME、大小和 SHA-256。
- `UserSkillState`：某个用户是否加入、收藏、启用、自动更新以及安装版本。

服务位于 `backend/internal/skills/`，仓储位于 `backend/internal/repository/skill_packages.go` 等文件。版本和文件模型允许运行把技能当作固定输入，而不是在一轮执行期间读取不断变化的工作目录。

### 6.2 云端 Agent 的按需读取路径

云端 Agent 运行的 Skill 过程是：

1. 前端从技能市场或技能库选择 Skill，把 `skillIds` 放入创建运行请求。
2. 后端确认 Skill 属于当前用户，已加入并启用。
3. 后端冻结版本 ID、内容 hash 和可读文件清单；正文不急于注入模型上下文。
4. 初始 runtime request 只包含技能名称、描述、版本、hash、入口路径和可读文件列表。
5. 模型需要工作流知识时调用 `skill_search`，检索已启用技能名称、描述和卡片文件名。
6. 模型调用 `skill_read_file` 读取 `SKILL.md` 或搜索结果中允许的文本卡片。
7. 文件按页返回，每页最多约 12000 字符；空路径用于列出目录。
8. 同一运行内同一个 Skill 的成功读取结果只执行一次并进入缓存；确定性的参数错误也会缓存。瞬时、权限或版本冲突会进入工具历史，但不会冻结重试。
9. 每次读取前后服务端检查版本和 hash；Skill 被更新、禁用或快照不一致时返回冲突，不能混用新旧正文。
10. SSE 只展示读取回执和路径，不把完整 Skill 正文重复写入事件，避免把大段指令扩散到 UI 日志。

因此，Skill 的正确用法是“先选择相关 Skill，再让 Agent 搜索并读取必要文件”，而不是把整个技能包一次性塞入 prompt。卡片文件名是可检索索引的一部分，模型通常可以直接跳到与问题最相关的卡片。

### 6.3 另一条 `skill-runtime` 路径

`web/src/services/skill-runtime.ts` 是另一套用于创作、短剧、导演等入口的 linked-context 适配器，不能和云端 Agent 的按需工具读取混为一谈。

它的行为是：

- `canvas`、`creation`、`shortDrama`、`director` 每个 profile 最多选择 4 个 Skill。
- 每个 profile 的上下文总预算约 32000 字符，每个 Skill 最多再选 3 个相关文本文件。
- 主动读取 `SKILL.md`，再按入口中引用的路径、任务词和“必须先阅读”的提示选择少量 `.md`、`.mdx`、`.txt`、`.json`、`.yaml`、`.toml` 或 `.csv` 文件。
- 把文件包在 `<skill-context>` 和 `<skill-file>` 标签中，作为用户任务前的 linked context 交给对应生成入口。
- 记录 Skill ID、版本 ID、版本号、文件路径和文件 hash，形成 provenance metadata。

这条路径是“前端准备后直接链接到生成请求上下文”；云端 Agent 是“后端冻结快照，模型通过工具渐进读取”。新增 Skill 功能时必须先确认调用的是哪条路径。

### 6.4 项目级 `.agents/skills` 与产品 Skill 的边界

仓库中的 `.agents/skills/project-docs/SKILL.md` 是给 Codex/项目协作者执行文档体系建设的项目级 Agent Skill。它不等于产品里的用户 Skill，也不会因为存在于仓库中就自动出现在云端 Agent 的技能清单里。

当前 Pi runtime 明确关闭默认 filesystem skills 和 context files。若需要让产品 Agent 使用某个工作流，正确路径是把它作为技能包纳入 Skill 服务、用户技能库和本轮 `skillIds`，而不是要求 runtime 直接扫描仓库目录。

## 7. 画布读写和媒体生成的深层边界

### 7.1 画布读写是快照协议

Agent 的画布写入不是“传一段 JSON 让后端覆盖整个画布”。它采用读取快照、准备操作、校验快照、提交 mutation 和发出 patch 的流程：

```text
canvas_get_state
  -> 返回 snapshotHash、真实 nodeId/rowId、能力和引用事实
  -> 模型生成 canvas_apply_ops / storyboard / batch 参数
  -> Go 校验用户、节点、能力、快照、句柄和字段
  -> 事务写入画布 + CloudAgentCanvasMutation
  -> append canvas_updated，带 before/after patch
  -> 前端应用 patch；冲突时全量刷新
```

`CloudAgentCanvasMutation` 会保留运行 ID、画布 ID、步骤 ID、前后 snapshot hash、有限的 before JSON、是否已经提交任务和撤销状态。撤销接口还要检查期望快照，避免用户或其他协作者已经继续编辑后把新改动覆盖掉。

快照的粒度由工具决定，不能跨工具混用：普通节点操作使用整画布 `snapshotHash`；分镜和批量表读写使用目标节点的 `snapshotHash`（兼容整画布哈希）；媒体生成使用 `mediaSnapshotHash`；预演修改使用场景 `snapshotHash`。媒体哈希忽略画布位置、尺寸和自动保存字段，但仍包含生成所依赖的节点业务字段与连线。

### 7.2 写入能力的安全模型

服务端对每次写入重新检查：

- 当前用户是否拥有或有权编辑该画布。
- 工具是否在本轮声明和当前权限模式中。
- 当前快照是否仍然匹配。
- 节点类型是否由服务端 capability registry 注册。
- patch 字段是否是该节点类型允许的字段。
- 连线来源是否允许输出、目标是否接受输入、输入类型是否匹配。
- 结构化节点的真实行 ID 是否来自最新读取结果。
- 是否涉及已锁定节点、已有任务或已有产物。
- 是否需要审批、是否已经有相同 call hash 的决定。

### 7.3 媒体工具是“准备、审批、提交、等待、回写”

`generate_media` 的逻辑不是一个普通同步工具调用。`previs_preview` 是另一条轻量路径：它只读取并校验预演场景/镜头，返回录制请求；`web/src/components/canvas/canvas-cloud-agent-events.ts` 把结果转成 `previs:preview-requested` 浏览器事件，由 `web/src/components/canvas/previs/canvas-previs-workbench.tsx` 判断是否属于当前工作台并执行视口录制。它不走下面的媒体 admission、计费、审批和 Worker 流程。

1. 模型先选择操作模式、源文本节点、媒体参考节点和模型选择。
2. `model_list` 根据模式和真实参考节点返回匹配模型、能力和价格；空结果不能退回任意不匹配模型。
3. Go 执行 dry admission，校验 prompt、素材归属、模型能力、画幅、时长、音频、预算、并发、额度和资源引用。
4. 服务端生成包含模型、参考素材、预估费用和目标节点的审批预览，并把审批 call hash 固定下来。
5. 所有可用权限模式下，媒体生成工具都要求用户通过 `/approvals/:approvalId/decision` 明确批准；`auto` 只会自动执行普通画布写入，不能绕过媒体审批。
6. 批准后才创建收费任务；任务由普通 Worker 执行，Agent 等待任务终态。
7. 生成完成后由后端以原始目标节点和任务绑定同时匹配，成功才回写画布。
8. 事件会区分生成错误和回写错误。目标节点不存在、任务绑定变化或结果资源不可用时不能假装媒体已经写回成功。
9. 运行重启时通过持久化 `MediaTaskID` 重建等待器，不能重新提交同一收费任务。

`image_layer_split` 复用媒体准入和结果回写路径，但它是独立的图像分层操作，不能当作一般的画布节点修改。

预演台的 `previs_scene_create` 和 `previs_apply_patch` 属于画布写操作：在 `request_approval` 下进入通用画布审批，在 `auto` 下可按普通写入策略执行。`previs_preview` 不修改服务端画布，也不创建后端 `Task`；浏览器录制完成后，预演工作台才可能通过前端保存/回写路径产生画布节点和资源变化。

### 7.4 现有工具注册表的阅读陷阱

仓库还存在 `backend/internal/app/cloud_agent_pi_canvas_tools_registry.go` 和 `backend/internal/app/cloud_agent_pi_canvas_tools_handlers.go`，其中注册了 `canvas_node_create`、`canvas_node_delete`、`canvas_bulk_operation` 等另一套工具定义，并带有自己的 `RequireApproval`、权限和 handler。

本轮在仓库内没有找到 `NewCanvasToolRegistry` 被云端 Agent 主执行路径调用；当前云端工具 schema 和白名单由 `cloudAgentTools` / `compileCloudAgentTools` 生成，写入主路径使用 `canvas_apply_ops`、`canvas_arrange_nodes`、结构化编辑和媒体工具。尤其是主路径的 `canvas_apply_ops` 明确禁止删除节点。

开发时必须先确认改的是哪套合同：

- 要修改当前 `/model` 交给 Pi 的云端工具，改 `backend/internal/app/cloud_agent_tools.go` 和对应 `backend/internal/app/cloud_agent_*.go` 执行器，并补 `CloudAgentSupportedToolNames` 与运行测试。
- 不能只在 `CanvasToolRegistry` 里添加工具，就认为云端 Agent 已能调用。
- 如果两套注册表都需要保留，应补充唯一事实源或明确它们的产品边界，否则工具文档、capability hash 和实际执行会漂移。

## 8. 权限、审批、预算和计费

### 8.1 权限不是 prompt 约定

Agent request 的 `permissionMode` 只决定服务端采用哪种执行政策，不能由模型自己改变。服务端会从用户、画布和运行状态重建权限判断；runtime request 里传给 Node 的 permission config 是辅助上下文，不是最终授权源。

即使模型发出未声明工具、跨画布 task ID、外部节点 ID、旧 snapshot、未知 patch 字段或伪造审批 call hash，Go 执行器仍会拒绝。

### 8.2 审批是持久化状态机

审批包含：

- 运行 ID 和 approval ID。
- 工具名、参数和稳定 call hash。
- 画布快照、目标节点、参考素材、模型选择和费用信息。
- 独立的媒体设置，例如模型、尺寸、时长或参考图选择。
- `waiting_approval` 状态、用户 decision、reason 和恢复结果。

Node 收到工具执行结果中的 `pause` 后主动中止 Pi 当前 session。用户批准后 Go 校验 approval ID、决定、call hash、快照和资源 lease，再恢复 session；旧 session 尚未退出时会登记 restart，避免双重执行。

### 8.3 预算和费用

运行请求包含最大积分、最多生成任务、最多视频秒数和可选最大步骤。根 `cloud_agent` 任务本身不进入模型 Worker，也不直接计费；正常模型轮次由 `cloud_agent_step` 子任务承载，上下文压缩使用 `cloud_agent_context_compaction`，记忆压缩使用专用 `canvas_text` 任务，媒体生成则创建带审批和额度上限的媒体任务。普通文本 Agent 运行的收费、取消、任务恢复和上游调用日志复用平台任务系统。需要注意，`GET /api/agent/capabilities` 当前仍返回 `billing: "fixed_request"` 这一兼容元数据；判断真实计费应沿 `Task.Operation`、`BillingOrder` 和 `NonBillable` admission 路径核对，不能把该字段解释为根任务已经收费。

“模型返回了工具调用”不等于“已扣费并已生成”：

- dry admission 只做准入和报价，不创建收费任务。
- 审批前的媒体预览不表示任务已提交。
- 审批后任务才进入 Worker。
- 任务失败时保留任务事实，但不宣称存在可用媒体。
- 网络重试只能重放相同的运行或工具 call，不得换幂等键创造新的收费轮次。

## 9. 记忆、偏好、计划和上下文

### 9.1 三层偏好

`AgentProfile` 使用 user、project、canvas 三个作用域。更新使用 revision CAS；运行创建时会读取有效层、合并成快照并保存 revision/hash。偏好会影响表达方式和工作偏好，但不影响权限、工具、预算和安全边界。

运行中的 `agent_profile_read` 只允许读取编译器列出的可用层，同一层在本轮重复读取会被拒绝，防止模型无意义地循环读取。

### 9.2 个人记忆

个人经验模型在 `backend/internal/model/models_agent_lesson.go`，服务和 handler 在 `backend/internal/app/cloud_agent_lessons.go`、`backend/internal/handler/agent_lesson.go` 等文件。记忆按 `author_user_id` 隔离：

- 用户手动添加可以直接批准。
- Agent 通过 `remember_lesson` 写入待审状态。
- 用户可以批准、拒绝、导入、导出和删除。
- `recall_lessons` 默认先读索引，再按 topic/category/keyword 获取正文。
- 压缩通过独立的 `canvas_text` 任务执行，不能在一次普通 Agent 对话里悄悄改写批准记忆。

记忆正文与 Skill、参考素材一样是数据，不是系统规则。

### 9.3 计划和追问

计划保存在运行状态和事件中，前端把 `plan_state`、`pending_plan` 等事件展示成计划卡片。追问通过 `user_question` 事件展示选项或表单；用户回答会以新的续轮进入可信历史，最多经过服务端允许的确认轮次。

计划不应该被用来绕过审批，也不应该要求 Agent 对简单任务强行建立长计划。系统策略明确允许单点读取、单点修改和简单问答直接完成。

### 9.4 上下文压缩

上下文预算来自当前实际模型能力和最大输出配置。运行会在压力达到阈值时把压缩作为独立内部模型任务执行，并通过生命周期事件通知前端。压缩需要同时保留：

- provider 所需的 assistant tool call / tool result 配对。
- 已完成的任务 ID、审批、画布 mutation 和输出事实。
- 最新画布 snapshotHash 和不能重复提交收费任务的依据。
- Skill、偏好和模型合同的版本身份。

旧的同参数画布读取正文可以压缩成 hash 和事实摘要，但不会破坏 provider transcript 的结构。Pi 原生 JSONL 会话和 Go 控制检查点分开保存，前者由 Pi 管理会话/压缩语义，后者由 Go 管理授权和恢复。

## 10. 持久化、恢复和一致性

### 10.1 关键数据对象

| 对象 | 作用 | 事实来源 |
| --- | --- | --- |
| `tasks` | Agent 根任务、模型步骤、媒体生成任务 | 任务状态和取消；模型步骤/媒体任务进入 Worker 与计费，根任务只是非计费控制面载体 |
| `cloud_agent_executions` | Agent 控制检查点 | status、revision、active task、media task、state JSON |
| `cloud_agent_pi_sessions` | Pi 原生 JSONL session | Pi 会话和上下文压缩快照 |
| `cloud_agent_event_records` | append-only 运行事件 | SSE 回放和审计 |
| `cloud_agent_message_records` | 独立消息正文 | 压缩后的 canonical 历史和恢复 |
| `cloud_agent_canvas_mutations` | 画布变更和撤销依据 | before/after hash、有限 before JSON、任务关联 |
| `agent_profiles` | 用户/项目/画布偏好 | revision、hash、内容 |
| `agent_lessons` | 用户私有经验 | 待审/已批准记忆和压缩 |
| `skills`、`skill_versions`、`skill_files`、`user_skill_states` | Skill 市场、不可变版本、文件和用户状态 | 版本/哈希快照 |

控制检查点通过 `revision` 做乐观并发控制。`MutateCloudAgent` 使用 revision 条件更新，发生冲突时重新读取并重试；这既用于多实例恢复，也用于 bridge 重放和审批竞态。

### 10.2 恢复路径

恢复分几种情况：

- 浏览器断线：SSE 使用事件 seq 重新连接；运行继续由 Go 和 Worker 执行。
- Go 进程重启：启动恢复扫描仍处于活动状态的 `CloudAgentExecution`，跳过等待用户审批/追问的运行，恢复其他运行。
- Node session 异常退出：Go 将运行标记失败，或根据已有 Pi session 和持久化恢复提示词继续；已完成的模型轮次不能再次调用模型造成重复扣费。
- 审批后恢复：恢复原 call 的固定参数和 call hash，不重新让模型猜一遍收费任务。
- 媒体任务仍在跑：读取持久化 `MediaTaskID`，重新挂载等待器，完成后再回写。
- bridge 重试：按 `callId` 查找已记录的 tool result 并回放。

服务端还会区分终态可读和可继续执行。运行记录损坏或历史压缩后，终态可以返回最小只读视图，但不能据此伪造活动审批、权限或 Skill 正文。

### 10.3 事件和快照的关系

事件是事实日志，快照是当前状态观察。`CloudAgentRun` 默认只返回事件尾部窗口，并返回 `EventSeqBase`、`EventCount`、`LatestSeq` 和 `EventsTruncated`，让前端知道当前窗口在完整日志中的位置。

这也是为什么：

- 续轮必须读取父轮的完整需要范围，不能只看默认尾部窗口。
- SSE 使用 seq，而不是事件数组下标。
- `run_snapshot` 没有 SSE id。
- 前端不能把“没有收到某类快照”解释成“没有发生过该事实”。

## 11. 开发、排障和扩展路径

### 11.1 新增一个只读工具

建议步骤：

1. 在 `backend/internal/app/cloud_agent_tools.go` 中定义工具名、描述、参数 schema 和暴露条件。
2. 在 `backend/internal/app/cloud_agent_tools_read.go` 的读取分支中解析严格参数。
3. 在服务端按当前用户和当前画布校验归属；不要接受默认用户、空 ID 或客户端的权限结论。
4. 判断是否可缓存、是否可批量执行、是否需要读次数预算。
5. 把结果作为可序列化数据写入 canonical tool message，避免返回无法重放的临时对象。
6. 若结果会出现在 SSE，输出脱敏的 receipt；大正文应按需或分页返回。
7. 增加参数错误、跨用户/跨画布、缓存和恢复测试。

### 11.2 新增一个画布写操作

不要直接在 handler 中写 GORM。应沿以下路径：

```text
cloudAgentTools.go schema
  -> cloudAgentToolAllowed / permission policy
  -> cloudAgentRuntimeTools.go
  -> 准备计划和 snapshot 校验
  -> repository transaction / domain helper
  -> CloudAgentCanvasMutation
  -> canvas_updated + canvasPatch
  -> frontend agent-canvas-sync
```

写工具要明确：

- 是否允许在 `auto` 直接执行。
- 是否一定进入审批。
- 是否会创建收费任务。
- 需要哪个快照和哪些真实 ID。
- 变更是否可以撤销。
- 事件如何给前端增量应用。
- 任务、资源、连线和节点字段如何保持归属一致。

### 11.3 新增媒体能力

至少需要同时修改和核对：

- `cloudAgentTools` 的参数合同和模型选择提示。
- `cloudAgentMedia` 的参数解析、引用绑定和 admission。
- `cloudAgentRuntimeMedia` 的审批、任务等待和结果回写。
- `backend/internal/app/task_creation.go` 的能力、价格、额度和资源校验。
- 对应 provider、Worker、结果资源和任务状态。
- `canvas_updated` 的 draft、submit、complete 或失败事件。
- 前端审批卡、任务状态和回写失败展示。
- `backend/internal/app/cloud_agent_media_*_test.go`、任务事实和恢复测试。

只在 Skill 或 prompt 中新增“支持某种媒体”不算完成，因为真正的能力由服务端注册表、模型目录和任务适配器决定。

### 11.4 修改 Pi bridge

修改 bridge 时要保持三个合同同步：

- Go 的 `cloudAgentPiProcessRequest` 和 payload 解析。
- Node runtime 的请求字段、事件序列化和 abort 语义。
- Pi session JSONL 到 Go canonical message 的转换。

重点测试：

- assistant tool call 与 tool result 一一配对。
- 别名字段冲突时 fail closed。
- tool call 参数不是 JSON object 时拒绝。
- bridge 响应丢失后同一 call ID 重放。
- 审批 pause 不被误判为失败。
- 上下文压缩前后仍保留可供 provider 接受的消息顺序。

### 11.5 修改 Skill

如果是产品 Skill：

1. 用 Skill 包结构加入 `SKILL.md` 和必要的文本卡片。
2. 确认文件被 Skill package 服务纳入，并具有版本、hash、路径和二进制标记。
3. 更新市场/用户加入状态和前端选择入口。
4. 按云端 Agent 或 `skill-runtime` 的目标路径写对应测试。
5. 不在 Skill 正文中声明新的权限；若需要新能力，修改 Go tool schema 和服务端执行器。

如果是项目协作 Skill：

- 放在 `.agents/skills/<name>/`。
- 先读项目级 `project-docs` Skill 的自包含和研究优先规则。
- 不要假设产品云端 Agent 会自动读取它。

### 11.6 常见排障顺序

| 现象 | 先查哪里 | 典型根因 |
| --- | --- | --- |
| 消息没有创建运行 | `web/src/components/canvas/canvas-cloud-agent-panel.tsx` pending、`web/src/services/api/agent.ts`、`backend/internal/handler/agent.go` | 对话 scope、请求校验、模型选择、幂等记录损坏 |
| 运行创建但不动 | `backend/internal/app/cloud_agent_pi_coordinator.go`、`backend/internal/app/cloud_agent_runtime_scheduler.go`、Task | runtime 未启动、活动任务卡住、恢复状态不一致 |
| Agent 反复读取画布 | `backend/internal/app/cloud_agent_tools_read.go`、runtime read cache | 读预算、缓存 key、上下文压缩后正文不在历史 |
| 写入被拒绝 | `cloudAgentToolAllowed`、快照校验、capability registry | 只读权限、旧 snapshot、未知字段或节点能力不匹配 |
| 媒体一直等审批 | `backend/internal/app/cloud_agent_runtime_media.go`、`backend/internal/handler/agent.go` approval | approval ID、call hash、恢复竞争或媒体设置不一致 |
| 生成成功但画布没结果 | `settleCloudAgentMedia`、writeback 事件 | 目标节点缺失、任务绑定变化、资源不可用或画布冲突 |
| 画布 UI 没同步 | `web/src/services/agent-canvas-sync.ts`、`canvas_updated` payload | patch 缺失、事件游标缺口、应用冲突或刷新节流 |
| Skill 读取失败 | `backend/internal/app/cloud_agent_tools_skills.go`、Skill version/hash | Skill 未加入/启用、路径不在快照、版本已变化或重复读取 |
| 审批后重复收费 | `callId`、`MediaTaskID`、Pi session | 没有重放已有回执、重新 dry/admit 或未复用持久化任务 |

## 12. 测试和验证地图

本轮没有启动前后端、真实模型或独立 `yingce-agent`，因此以下是源码中可找到的测试地图，不把它们写成已运行结果：

- Agent 基础合同和能力：`backend/internal/app/cloud_agent_test.go`、`backend/internal/app/cloud_agent_contract_test.go`。
- 工具分派和参数修复：`backend/internal/app/cloud_agent_tool_dispatch_test.go`、`backend/internal/app/cloud_agent_invalid_arguments_test.go`、`backend/internal/app/cloud_agent_tool_repair_test.go`。
- 上下文压缩和 Skill 按需读取：`backend/internal/app/cloud_agent_context_test.go`、`backend/internal/app/cloud_agent_skill_search_test.go`、`backend/internal/app/cloud_agent_skill_usage_test.go`。
- 画布 patch、布局、快照和撤销：`backend/internal/app/cloud_agent_canvas_events_test.go`、`backend/internal/app/cloud_agent_layout_test.go`、`backend/internal/app/cloud_agent_step_hash_test.go`、`backend/internal/app/cloud_agent_undo_test.go`。
- 分镜、批量表和角色卡：`backend/internal/app/cloud_agent_storyboard_test.go`、`backend/internal/app/cloud_agent_batch_table_test.go`、`backend/internal/app/cloud_agent_character_test.go`。
- 媒体模型、审批、回写和失败分类：`backend/internal/app/cloud_agent_model_selection_test.go`、`backend/internal/app/cloud_agent_media_test.go`、`backend/internal/app/cloud_agent_approval_settings_test.go`、`backend/internal/app/cloud_agent_media_writeback_test.go`、`backend/internal/app/cloud_agent_tool_error_class_test.go`。
- Pi session、bridge、恢复和完整运行：`backend/internal/app/cloud_agent_pi_contract_test.go`、`backend/internal/app/cloud_agent_pi_runner_test.go`、`backend/internal/app/cloud_agent_pi_recovery_test.go`、`backend/internal/app/cloud_agent_runtime_e2e_test.go`。
- SSE 和 HTTP：`backend/internal/handler/text_events_test.go`、`backend/internal/handler/agent_run_pagination_test.go`、`web/src/services/api/agent.ts` 的流解析实现。
- Skill 包、版本和文件服务：`backend/internal/skills/skill_packages_test.go`、`backend/internal/skills/skills_json_test.go`、`backend/internal/repository/cloud_agent_pi_session_test.go`。

建议实现改动后从小到大验证：

```bash
cd backend
go test ./internal/app -run 'TestCloudAgent|TestAgent'
go test ./internal/agent/...
```

涉及前端 Agent API、事件流或画布同步时，再执行针对性的前端测试或构建；涉及真实登录、模型、媒体、SSE 断线和独立容器时，需要补真实环境冒烟，不能用静态阅读代替。

## 13. 当前架构观察和后续建议

### 已经形成的优点

- Agent 执行从浏览器生命周期中脱离，具备任务、检查点、事件、session 和恢复模型。
- Go 控制面与 Node runtime 分工清晰；独立 `yingce-agent` 不接触数据库和业务授权。
- 工具 schema、执行器、审批、错误分类和事件回执大体沿同一条服务端合同演进。
- 画布写入采用 snapshot hash、能力注册表、mutation 和 patch，远比整图覆盖安全。
- 媒体生成复用统一任务和计费链路，避免 Skill 或模型输出绕过 admission。
- Skill 采用版本/hash 快照和按需读取，降低上下文膨胀，也能发现运行中版本漂移。
- 前端提交幂等、SSE 游标和 patch 优先同步解决了几个最危险的重复执行和断线问题。

### 需要持续关注的风险

1. **两套画布工具注册表并存。** `CanvasToolRegistry` 和当前 `cloudAgentTools` 的能力描述可能漂移，新增工具必须明确唯一事实源。
2. **静态合同和实际运行仍有环境差距。** runtime 的虚拟模型、真实上游渠道、独立容器、SSE 代理和媒体 Worker 需要真实联调才能确认端到端行为。
3. **Agent 上下文很复杂。** canonical 历史、Pi JSONL、消息记录、事件记录和压缩摘要各自有职责，修改其中任何一层都要验证 provider 配对、续轮和恢复。
4. **结构化画布内容依赖真实 ID。** 分镜 row、批量表 row、参考图 mention 和媒体任务绑定不能由模型猜测；文档和工具错误提示必须继续强调“先读、再写”。
5. **前端 UI 状态不是服务端事实。** localforage 对话缓存、运行快照、事件日志和画布本地 store 的失配仍可能产生“界面看起来完成、服务端尚未完成”的误解，需要保持状态文案和诊断工具清晰。

### 最值得补的验证

- 用真实登录态验证：创建 Agent、刷新页面、关闭页面后运行是否继续，以及任务中心是否能恢复。
- 让模型进行一次画布读取、节点写入、分镜行编辑、媒体审批和结果回写，核对事件、patch 和数据库 mutation。
- 主动断开 SSE，验证按 `after` 和 `Last-Event-ID` 恢复不会重复消息或漏掉 canvas patch。
- 在审批前后重启 backend 或独立 runtime，验证不重复调用模型、不重复提交收费媒体任务。
- 用 Skill 更新/禁用制造版本冲突，验证运行拒绝混用新旧文件。
- 对 `CanvasToolRegistry` 做生产调用审计，决定删除、合并或明确它与当前云端工具合同的边界。

## 14. 给开发者和 Agent 的使用提示词

下面这段可以作为刚接触项目、准备研究或修改 Agent 功能时的导读：

```text
你正在研究影策的云端画布 Agent。先把它理解为“浏览器观察层 + Go 控制面 + 持久化任务/运行检查点 + Pi Node runtime”，不要把它当成可以直接访问仓库和数据库的普通 Coding Agent。

先读：
1. AGENTS.md 的 Agent、画布和文档约定；
2. docs/content/docs/overview/architecture.mdx；
3. docs/content/docs/backend/code-map.mdx；
4. docs/content/docs/backend/http-api.mdx；
5. 本报告 docs/drafts/cloud-agent-deep-research.md。

然后按问题导航：
- 前端面板：web/src/components/canvas/canvas-cloud-agent-panel.tsx
- API、幂等和 SSE：web/src/services/api/agent.ts、web/src/services/cloud-agent-conversations.ts
- 画布增量同步：web/src/services/agent-canvas-sync.ts
- Go 运行准入：backend/internal/app/cloud_agent.go
- Pi 生命周期和恢复：backend/internal/app/cloud_agent_pi_coordinator.go、backend/internal/app/cloud_agent_runtime_scheduler.go
- 模型/工具/事件 bridge：backend/internal/app/cloud_agent_pi_model.go、backend/internal/app/cloud_agent_pi_bridge.go、backend/internal/agent/runtime/
- 工具 schema：backend/internal/app/cloud_agent_tools.go
- 画布读取：backend/internal/app/cloud_agent_tools_read.go、backend/internal/app/cloud_agent_canvas_state.go
- 画布写入：backend/internal/app/cloud_agent_runtime_tools.go、backend/internal/app/cloud_agent_tools_canvas.go、backend/internal/app/cloud_agent_mutation.go
- 媒体任务：backend/internal/app/cloud_agent_media.go、backend/internal/app/cloud_agent_runtime_media.go、backend/internal/app/task_creation.go
- Skill：backend/internal/app/cloud_agent_tools_skills.go、backend/internal/skills/、web/src/services/skill-runtime.ts
- Node runtime：backend/agent-runtime/pi/agent-runtime.mjs；独立服务：yingce-agent/server.mjs

研究或修改时遵守：
- 先区分当前 cloudAgentTools 合同和未必接入主链路的 CanvasToolRegistry；
- 先读 snapshotHash、真实 nodeId/rowId 和 capability，再提交写入；
- Skill 是参考知识，不是授权；产品 Skill 和项目 .agents/skills 不是同一条加载路径；
- 媒体生成必须经过 model_list、admission、独立审批、Worker 和结果回写；
- 事件 seq 用于 SSE 重放，run_snapshot 只是状态观察；
- 不把前端缓存、静态文档或 prompt 中的权限描述当成服务端事实；
- 不声称完成真实模型、登录、SSE 断线或独立容器验证，除非实际运行过对应流程；
- 修改前检查 git status，研究结论优先写入 docs/drafts，并在报告末尾注明证据和待验证边界。
```

## 15. 结论状态

本报告目前属于 `docs/drafts/` 的深度研究草稿。代码路径、工具名、数据模型和调用关系已经按当前 checkout 交叉核对；以下事项仍不应被描述为端到端验收：

- 真实供应商模型和计费结果。
- 真实登录态下的前端 Agent 面板完整操作。
- SSE 断线、代理缓冲、浏览器刷新和多实例恢复。
- 独立 `yingce-agent` 容器的生产网络和并发行为。
- 实际媒体生成、资源存储和画布回写。
- Skill 在版本更新、禁用和跨进程恢复期间的完整用户体验。

稳定且已经在正式文档中出现的内容，继续以 `docs/content/docs/overview/architecture.mdx`、`docs/content/docs/backend/http-api.mdx`、`docs/content/docs/backend/code-map.mdx` 和 `docs/content/docs/backend/agent-prompt-policy.mdx` 为专题入口；本草稿保留跨模块证据链、设计观察和待验证清单，避免重复维护一套互相竞争的正式事实源。
