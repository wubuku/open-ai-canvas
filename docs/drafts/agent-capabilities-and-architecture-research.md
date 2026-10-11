---
title: 影策 Agent 能力与技术实现全景调研
status: draft
scope: research
updated: 2026-10-11
---

# 影策 Agent 能力与技术实现全景调研

> **目的**：从产品能力和运行架构两方面回答“当前项目的 Agent 能做什么、通过什么链路做到、哪些边界不能误读”。本文把云端画布 Agent 与画布助手中的创作交互放在同一张地图上，但不把它们当成同一个运行时。
>
> **证据范围**：当前 checkout 的 Web、Go、Node runtime、数据库模型和相关测试静态核对。代码路径和测试能证明实现合同，不等于真实模型、浏览器、多实例或生产链路已经验收。

## 先看结论

当前产品至少有两种不同的 Agent 执行合同：

1. **画布云端 Agent** 是服务端持久化的工具循环。Go 控制面负责准入、策略、工具授权、检查点、任务和事件；Pi Node runtime 负责模型循环；模型提出的工具调用回到 Go 执行。它适合读取画布、迭代理解素材、修改画布、使用 Skill、发起受审批的媒体任务，并在浏览器断开后由服务端继续调度或恢复。
2. **创作交互** 是嵌在普通助手消息里的方案流程，由浏览器 `CreativeAgentController` 编排。模型本轮只得到 `creative_respond` 一个函数工具；可生成问题、方案或局部修改建议。浏览器和 Go 服务分别校验结构、保存方案/审批/报价事实，再由既有生成任务链完成媒体生产。它不是 Pi 工具循环，也不是任意画布操作代理。

首页 `/create` 的“新建画布，与 Agent 创作”入口会先创建并同步画布，再导航到 `/canvas/:id?agent=1`，进入的是第一种画布云端 Agent。它没有直接启动第二种 `CreativeAgentController`。

因此，读代码时应按**实际入口、运行时工具合同和后端执行合同**判断能力。名称、prompt、类型定义、注册表或 API helper 单独存在，都不能证明某项 Agent 能力已经进入主执行链。

## 1. Agent 入口和运行合同

| 用户看到的能力入口 | 实际执行器 | 模型可用的交互方式 | 服务端持久化事实 |
| --- | --- | --- | --- |
| 画布 Agent 面板；首页 `/create` 的 Agent 入口最终进入该面板 | Go 调度器 + Pi Node runtime | 多轮模型请求与服务端动态工具调用 | `Task`、`CloudAgentExecution`、Pi session、追加事件日志、转录和画布变更记录 |
| 普通助手消息中的创作问题、方案、报价和产物卡片 | Web `CreativeAgentController` + 一次一段的受管模型任务 | 单个 `creative_respond` 结构化输出；执行后按界面状态续轮 | `CreationRun`、`CreationSubmission`、已批准方案/画布操作和关联任务 |

运行链可概括为：

```text
画布云端 Agent
CanvasCloudAgentPanel -> /api/agent -> Gin handler -> Go app scheduler
  -> Pi runtime -> /model bridge /tool bridge /event bridge
  -> Go 工具、画布域逻辑或媒体任务 -> event journal -> SSE -> 浏览器应用 patch/校准

创作交互
CanvasCreativeInteraction -> CreativeAgentController
  -> 受管文本任务（唯一工具 creative_respond）-> 后端 CreationRun/报价/审批
  -> 批准方案后校验并提交画布差量 -> 受管媒体任务 -> 资源绑定和结果回写
```

首页入口只负责确保有一个服务端画布，再打开上面的第一条链。代码证据见 `web/src/pages/create/creation-agent-entry.tsx`。

## 2. 画布云端 Agent 的能力

工具合同由 `backend/internal/app/cloud_agent_tools.go` 的 `compileCloudAgentTools` 按本轮上下文和权限生成，执行由 `cloud_agent_runtime_tools.go` 等 Go 代码分派。当前能力按任务归类如下：

| 能力组 | 当前工具 | 能完成的工作 | 主要边界 |
| --- | --- | --- | --- |
| 画布和任务理解 | `canvas_list_node_types`、`canvas_get_state`、`canvas_read_storyboard`、`canvas_read_batch_table`、`task_get` | 查询节点合同、节点/连接、角色卡、分镜行、批量表和任务状态；用快照哈希及真实行 ID 锚定后续操作 | 内容会分页；模型应先读再写，不能猜节点/行 ID；读结果缓存随版本变化或写入失效 |
| 视觉与图片辅助 | `canvas_inspect_image`、`image_text_detect`、`image_annotation_render` | 查看实际图片；为 OCR 或局部编辑准备图片引用；生成临时标注 PNG 参考图 | 实际看图工具只在模型视觉能力与画布上下文同时启用时暴露；OCR 工具是请求准备，不等于 OCR 已执行或写回；标注不改原图 |
| 画布结构写入 | `canvas_apply_ops`、`canvas_arrange_nodes` | 创建/更新节点、建立连线、按依赖或类型排版 | 通用操作不提供删除节点或任意 metadata/URL/storage key 写入；节点字段和连接受能力注册表约束 |
| 结构化创作资产 | `canvas_create_storyboard`、`canvas_edit_storyboard`、`canvas_create_character`、`canvas_edit_batch_table` | 创建和维护分镜行、把就绪形象组合成角色卡、编辑批量生成计划和参考图列 | 结构化数据必须通过专用合同；批量表编辑只改计划，不提交收费任务；具体写入按当前权限模式审批 |
| 媒体生产 | `model_list`、`generate_media`、`image_layer_split` | 查询模型能力/价格/引用限制；请求图片、视频或音频生成；拆分图片透明图层 | 模式还须同时被生成适配器和节点能力注册表支持；素材、模型、价格、额度和任务在 Go 端重新校验；媒体创建始终有独立用户审批 |
| 预演台 | `previs_scene_read`、`previs_scene_create`、`previs_apply_patch`、`previs_preview` | 读取场景/镜头；创建场景；用语义操作改场景、对象、相机、灯光和动画；请求当前浏览器录制白模预演 | 场景变更是带快照的语义 patch；`previs_preview` 通过浏览器事件调用当前预演台，不等于后端媒体任务或 Worker 作业 |
| 协作、偏好与记忆 | `plan_update`、`ask_user`、`agent_profile_read`、`recall_lessons`、`remember_lesson` | 展示多步进度、对关键歧义提问、读取偏好和已批准经验、提交经验候选 | 计划和偏好不扩大权限；新记忆需用户批准后才成为后续可召回经验 |
| 产品 Skill | `skill_search`、`skill_read_file` | 搜索本轮技能并按需读取文件 | 技能正文是上下文资料，不会安装工具、代码或授权 |

当前云端媒体适配器和节点能力交集包括 `image`、`video`、`audio`。`model_list` 返回的是本轮实际素材和操作筛选后的匹配项；空集合应视为能力缺失，不能用名称相近的模型兜底。

### 工具是否出现取决于本轮合同

- `contextScope` 当前只接受 `canvas`；没有画布上下文时不会注册画布、预演和媒体相关工具。
- `skillIds` 非空才注册 Skill 读取工具；Skill 是否加入、启用、版本固定仍由服务端检查。
- `canvas_inspect_image` 需要本轮模型视觉能力与画布上下文；工具类型存在不代表每个模型都能看图。
- `read_only` 不暴露画布/预演写操作和媒体生成；`request_approval` 下画布/预演语义写入要审批；`auto` 可直接执行已注册的普通画布/预演写操作。媒体生成和图层拆分在任何模式下都经过单独审批。
- `previs_preview` 是面向当前浏览器预演视口的请求，不是节点写入工具，也不是视频生成 Worker。

`CloudAgentSupportedToolNames` 是为了暴露能力全集而用一个最大化请求生成的列表。实际运行必须以本轮 `compileCloudAgentTools` 的结果为准。服务端对模型返回的工具名再做本轮授权检查；发现幻觉工具会拒绝，而不是尝试动态执行任意函数。

## 3. 创作交互 Agent 的能力与执行

### 能力范围

`CanvasCreativeInteraction` 将结构化创作交互渲染在普通助手消息内部，不拥有聊天输入框、会话列表或模型选择器。它的执行状态由 `web/src/services/creative-agent-controller.ts` 驱动，协议定义位于 `web/src/lib/creation/creative-agent-tools.ts`、`creative-agent-contract.ts` 和 `creative-agent-state.ts`。

它可按场景引导用户补充需求，整理 brief 和动态计划，返回完整方案；方案可描述文本、图片、视频、画风入口、故事输入和结构化分镜节点，引用已有图片，配置待生成图片/视频；也能提出针对现有节点的有限字段编辑。图像/视频生成项会基于真实模型能力、时长、比例、数量和素材引用做校验。方案输出本身不会立即建节点或生成媒体。

当前模型工具合同只有 `creative_respond`。每次请求会附带当前需求摘要、最近会话消息、已选 Skill 文本、画布/素材只读快照和可用图片/视频模型目录。模型只能给出协议数据；浏览器生成真实交互 ID、归一化 brief 和问题、校验方案/节点/生成项，并决定接下来的状态转换。最多 20 个方案节点、最多 100 行结构化分镜；每次问题响应最多归一化 3 个问题。单个运行的规划模型调用计数上限为 32。

### 从理解到回写

1. **准备规划**：控制器读取画布和真实资源快照，加载已加入且本轮选中的 Skill，将场景指导、系统提示和最近 20 条消息组成模型输入。当前实现不把任意 `canvas_*` 工具交给该模型。
2. **规划报价**：通过 `prepareBackendToolGenerationTask` 将 `creative_respond` 作为唯一函数工具提交受管文本任务；后端创建 `CreationSubmission` 和报价。创作交互中的规划文本调用也经过任务和报价合同。
3. **呈现和规范化**：收到模型结果后，浏览器只消费第一条 `creative_respond`，对剩余调用设置等待屏障并要求之后重新规划。问题答案与方案结构由浏览器代码验证；拒绝模型生成的可信 ID、审批状态或权限字段。
4. **确认方案后写画布**：方案必须先由用户确认。后端记录批准方案 hash、版本、操作和受影响字段基线；创建或关联真实画布后，客户端把批准操作分别与远端文档和本地 store 对账。服务端通过快照 hash 和批准差量校验拒绝方案外修改，也不允许删除已有节点。
5. **单独确认费用并生成**：每个生成项先创建带稳定 `itemKey` 的报价提交；用户确认后才执行任务。报价过期或模型/价格签名变化时需要刷新并重新确认。批量工作按前置图片依赖分批提交。
6. **等待任务和物化资源**：任务成功后，客户端检查实际输出 `storageKey` 对应的资源，再确认目标节点仍存在，建立业务素材绑定并只更新该目标节点。观察任务失败与生成任务失败分别记录；若仅观察中断，应恢复并继续回写，不应自动再次收费生成。

创作交互目前只允许 `image`、`video` 两类媒体 `generationItems`；云端 Agent 另有音频生成合同。不能用云端 Agent 的工具清单推断创作交互具备同样能力。

## 4. 持久化、并发和安全边界

| 事实 | 画布云端 Agent | 创作交互 Agent |
| --- | --- | --- |
| 运行归属 | 根任务及 `CloudAgentExecution` 绑定用户、画布和对话；每个新消息形成新运行/续轮关系 | `CreationRun` 绑定用户、client key 和可选画布；状态 JSON 和提交记录分开保存 |
| 并发控制 | 调度器以 run revision/CAS 推进一步骤；根任务、运行检查点、Pi session、事件日志和 transcript 分层持久化 | 页面执行器 claim 运行所有权；`executionEpoch` 递增 fencing、45 秒 lease + heartbeat、revision CAS 阻止旧页面写入 |
| 媒体副作用 | 工具先做服务端准入、准备和独立审批，再进入现有 Task/Worker/计费与资源链 | 创建不可变报价提交；审批绑定 submission 和 proposal 版本/hash；执行阶段事务性关联 Task/账单并防止重复扣费 |
| 用户断开后的行为 | 服务端 scheduler 可扫描、唤醒和恢复活动运行；浏览器 SSE 是观察/回放通道 | 浏览器仍是多步工作流执行器；已经创建的媒体 Task 可继续跑，后续审批、规划和回写由重新接管的页面恢复 |
| 可撤销性 | 有独立 Agent undo API 和带 before/after hash 的变更记录；需满足快照与任务约束 | 方案提交按批准差量和基线约束，不是通用撤销栈；模型不能自行删除节点 |

创作 `CreationRun` 的状态 JSON 不是权限事实源。后端单独保存批准版本/hash、批准操作、基线文档和每项 `CreationSubmission` 的 request hash、报价、价格签名、过期时间、批准时间与 task ID。执行端必须检查 owner/epoch、批准状态、proposal 绑定、报价新鲜度和价格签名。关联 Task 后重复执行返回既有 Task，而不是重新创建。

云端 Agent 与创作交互都把**生成任务执行**交给原有媒体任务系统；Agent 并不等于 Provider/Worker。视频任务的前端等待器当前按类型给出 125 分钟窗口，源码注释说明该值高于后端同类任务 120 分钟的执行超时。它是浏览器等待/观察上限，不是模型供应商生成保证，也不证明断网后一定能恢复回写。

## 5. 不应误读为 Agent 能力的项目部分

- Pi 默认 filesystem skills、`bash`、`read_file`、`web_search` 等兼容入口并未作为当前平台工具暴露；Node runtime 使用临时隔离目录，通过受控 Go bridge 调模型、工具和事件。不能把 Pi SDK 的默认能力当成产品能力。
- Skill 是版本化参考材料。云端 Agent 按需经工具读已冻结版本；创作交互由 Web `skillRuntime` 准备文本上下文。两者均不执行包内脚本，不由 Skill 扩权。
- `web/src/services/api/project-agent-tools.ts` 定义了项目域工具名与 helper，但本轮全仓搜索没有找到其调用方。因此本报告不把这些 helper 列作当前 Agent 主链已提供的工具。它们未来是否被其他运行时装配，应在接入处重新核验。
- 注册表里存在某项工具、某个 schema 有某个字段、或 system prompt 要求模型完成某一步，都不足以证明主执行链有对应 handler、权限和持久化路径。

## 6. 可借鉴的架构经验与当前取舍

### 值得保留的机制

- **工具 schema 和执行授权同源**：按本轮模型、上下文、权限和功能构建 schema；服务端 dispatch 再检查，而非信任模型“只会调用列出的工具”。
- **模型输出是意图，不是写权限**：经过类型归一化、真实 ID/资源确认、快照/hash 冲突检测和权限校验后，才提交业务差量。
- **审批绑定不可变请求**：方案版本/hash、完整媒体请求、报价和价格签名形成服务端提交记录；审批与执行拆开，便于过期、改价、撤销和幂等处理。
- **运行时与副作用分离**：模型循环和画布/资源/计费写入由不同层负责；媒体生成复用任务 Worker，方便限流、重试、状态观察和成本核算。
- **页面恢复要有 fencing**：Web 控制器拥有 lease 时才推进流程；epoch 可拒绝旧标签页/旧页面的迟到写入。
- **事件、检查点与转录分层**：云端 Agent 将可重放事件、模型上下文转录、短期运行检查点和 Pi 原生 session 分开，避免一个 JSON 字段承担所有恢复语义。

### 需要按目标产品重新权衡的部分

- 两种编排器提供不同交互优点，但也造成模型上下文、Skill provenance、计划、审批、运行恢复和诊断分成两套合同。新系统应明确哪些任务需要自主工具循环，哪些只需受限结构化规划，并统一共享领域协议。
- `CreativeAgentController` 把工作流状态机留在 Web，后端主要提供租约、CAS、报价、审批、任务和画布差量合同。它能复用现有 UI 和 store，但服务端不能在没有浏览器执行器时独立完成整个规划到回写过程；若要求无人值守或服务端续跑，应把状态转移迁入 durable workflow/worker。
- 两条链路都依赖结构化 JSON 状态。状态 schema 需要明确版本化、迁移策略、大小限制和恢复语义，避免将 UI 展示快照误当授权记录。
- 当前云端 Agent 有很多细分工具，能力强但 schema、权限、错误修复、测试和模型兼容成本更高。新实现可用少量粗粒度业务命令承载稳定的领域操作，并保留 server-side validation 与可审计 diff。
- 当前创作方案由前端规范化并应用操作，服务端再进行批准差量复核。跨端实现时应把 schema 版本和规范化规则放在可共享的协议模块或服务端领域层，避免前后端各自演化出不同解释。

## 7. 测试证据和未验证范围

源码中已有针对性后端测试覆盖租约/epoch 和 stale revision、报价过期/变更、并发 execute 恰好一个 Task 与账务结果、方案画布差量、结果资源绑定、跨用户资源隔离以及云端 Agent 工具往返和审批往返。代表性测试位于：

- `backend/internal/app/creation_test.go`：`TestCreationRunOwnershipCASAndFence`、`TestCreationSubmissionApprovalExpiryAndChangedQuote`、`TestCreationConcurrentExecuteChargesOnceAndRollback`、`TestCreationApprovedUpdateDoesNotOverwriteLaterManualEdit`。
- `backend/internal/app/creation_agent_references_test.go`：Agent 输入图片和敏感配置持久化边界。
- `backend/internal/app/cloud_agent_runtime_e2e_test.go`、`cloud_agent_test.go`、`cloud_agent_previs_mutation_test.go`：工具往返、审批、准入、恢复与预演语义操作。
- Web 已有 `web/test/creative-agent-controller.test.ts`、`creative-agent-state.test.ts` 和 `creative-agent-redo.test.ts`，覆盖单工具合同、恢复与租约交接、报价过期和重复执行保护、交互等待屏障、方案校验、画布操作、生成结果观察/回写以及局部重做规格。它们是确定性单测，不等于真实浏览器交互或多标签页并发验收。

本次研究使用源码和已有测试作为证据，没有把它们写成真实运行验收。以下仍待真实环境验证：

- 登录态 Web 中两个入口的完整 UI 流程及路由状态；
- 真实模型对结构化工具输出、图像输入和长上下文的兼容性；
- 真实计费、模型配置变化、媒体队列长时间运行和视频等待边界；
- 多标签页真实竞争 lease、断网后的浏览器恢复和页面关闭后的创作交互接续；
- Pi 独立容器桥接、SSE 断线重放、多后端实例恢复；
- 图片/视频/音频真实生成、资源物化和失败后回写恢复。

## 8. 代码和文档导航

| 想了解的内容 | 入口 |
| --- | --- |
| 云端 Agent 全部执行链、工具、事件、恢复和详细测试地图 | [云端 Agent 深度调研报告](cloud-agent-deep-research.md) |
| 产品 Skill 目录、加入/使用和两条加载路径 | [技能广场与 Skill 运行机制调研](skills-usage-and-runtime-research.md) |
| 画布节点、同步、资源、并发和 Agent 写入约束 | [画布系统深度调研报告](canvas-deep-research.md) |
| 预演台数据模型和 Agent 语义 patch | [预演台系统深度调研报告](previs-workbench-deep-research.md) |
| 画布云端 Agent UI/API | `web/src/components/canvas/canvas-cloud-agent-panel.tsx`、`web/src/services/api/agent.ts`、`backend/internal/handler/agent.go` |
| 云端 Agent schema/dispatch/runtime | `backend/internal/app/cloud_agent_tools.go`、`cloud_agent_runtime_tools.go`、`cloud_agent_runtime_scheduler.go`、`cloud_agent_pi_coordinator.go`、`backend/agent-runtime/pi/agent-runtime.mjs` |
| 创作交互协议、编排和 UI | `web/src/lib/creation/creative-agent-tools.ts`、`web/src/lib/creation/creative-agent-contract.ts`、`web/src/services/creative-agent-controller.ts`、`web/src/components/canvas/canvas-creative-interaction.tsx` |
| 创作交互后端事实与路由 | `backend/internal/model/models_creation.go`、`backend/internal/handler/creation.go`、`backend/internal/app/creation.go`、`backend/internal/app/creation_canvas.go` |
