---
title: 画布开发导航审计
status: research
scope: documentation
updated: 2026-10-12
---

# 画布开发导航审计

## 结论

在新增正式页面前，画布相关内容不能被快速导航到。已有信息分散在以下位置：

- `docs/content/docs/backend/code-map.mdx`：包含云端 Agent 和部分页面入口，但不是画布开发指南。
- `docs/design/canvas-consistency-repair.mdx`：深入记录批量生成、持久化和节点可见性的一致性边界，但属于问题治理设计记录。
- `docs/design/canvas-final-performance-architecture.md`、`canvas-floating-controls.mdx`、`canvas-node-visual-contrast.mdx`：分别关注性能、浮动控件和视觉设计，不能作为代码入口总览。
- `docs/content/docs/plugins/plugin-system.mdx`、`plugin-surfaces.mdx`：覆盖插件和画布 surface，但不解释宿主画布 store、任务回写和远端同步。
- 源码本身已经形成较清晰的模块边界：`web/src/pages/canvas/`、`web/src/components/canvas/`、`web/src/stores/canvas/`、`web/src/lib/canvas/`、`web/src/services/api/user-data.ts`、`backend/internal/canvas/`。

因此，开发者若只阅读现有 `docs/index.md` 或代码地图，能找到部分画布入口，但仍需要自行推导以下关系：

```text
CanvasProject / CanvasNodeData / CanvasConnection
  -> 页面与 hook 编排
  -> Zustand + localforage + revision merge
  -> remote canvas API + history
  -> task admission + generation consumer
  -> Agent capability / plugin surface
```

## HTTP 合同覆盖审计

当前不能把 `GET/PUT /api/canvas-projects/:id` 的绿色结果称为画布 API
整体通过。按真实 handler、Web API 调用点和画布深度调研重新盘点，至少有以下
边界：

| 覆盖面 | 当前 method/path 数量 | 关键执行路径 | 当前独立 runner 状态 |
| --- | ---: | --- | --- |
| 文档与版本 | 7 个唯一 method/path；另有 `repairMissingResources` 保存分支 | 未登录、owner、foreign、CAS/stale、历史恢复、删除、304/gzip、并发 | `core` 已执行；修复检查有效 preimage/失败/权限/CAS，断链 preimage 由资源 profile 联验 |
| 分享与公开读取 | 5 个 | 分享创建/旋转/撤销、过期/错误 token、公开 JSON、分享资源文件、缓存与安全 headers | `share-public` 已通过隔离 PostgreSQL fixture 实测过期、owner 状态、foreign 隔离、源画布/历史不变和无过期恢复；分享资源另已通过本地 CDN/object-store fixture 验证 redirect、成功读取、GET 故障与恢复；生产远端对象存储/CDN、TTL/条件缓存仍未验，Go foreign 写返回 `500/internal` 待决策 |
| 外部导入 | 2 个 | LibTV/TapNow 有效、坏输入、上游错误/超时、重复提交、媒体/拓扑/数量边界和 owner scope | 固定 HTTPS 真 PNG/WAV/MP4 替身、选定拓扑/参数/数量边界和后续资源/素材登记/保存/CAS/跨用户链已专项两轮通过；导入端点本身仍仅返回转换结果，不等于自动保存或浏览器验收。响应丢失/重启和远端存储故障仍为依赖门禁 |
| 项目画布关联 | 4 个 | 功能开关、项目/单元归属、重复关联、解除关联和 foreign user | 118 场景新版已全量双轮，补同 key/已有 link/不同 unit/跨项目竞争；D-26 返回未持久化 ID，D-27 角色超长和 D-28 首次关联竞争的 500 均部分推进画布。完整正文/历史、投影、解除和显式恢复已验收；受控交错/任意失败回滚仍缺 |
| 资源完整性依赖 | 11 个 catalog operation；另有旧 assets/folder 和独立公开资源交付 | 有效/缺失/跨用户资源引用、access、文件、上传、素材替换保护和 purge 后修复 | `resource-integrity` 已执行 URL 导入、配额/限流、分片并发、SSRF 和本地 TTL 等选定路径；持久化 reservation 过期后的新进程重新准入已由 `restart-recovery` companion 验证，后台 reconciler、远端对象存储故障和更深的受控交错仍缺；`legacy-assets` 另验证本地物理删除失败保留对象，不把 purge 描述为引用删除保护 |
| 旧素材/文件夹兼容 | 12 个 | 批量读删、文件夹创建/更新/移动/删除、素材更新、用户数据快照和归属隔离 | `legacy-assets` 已执行；另有六次画布引用/素材 purge 竞态与本地 object-store DELETE 故障路径，生产 outbox 重试时序和任务/项目引用源仍未覆盖 |
| 独立公开资源交付 | 2 个 | 签名、过期、文件名、Range、缓存和对象存储失败 | `public-resource` 已在隔离 PostgreSQL + 本地 HTTP 文件 fixture 执行签名交付；对象存储/CDN 故障、真实 TTL 和条件缓存仍 deferred |
| 项目工作台及角色库 | 50 个 | 项目/单元/分镜/工作流/素材/角色与声音归属 | `project-workbench-core` 已执行 15 个项目/单元 operation；其余 35 个工作台/角色库 operation deferred |
| 跨进程恢复 | 12 个状态流 operation，不新增 endpoint | 崩溃/优雅重启后的 session、文档/历史/CAS/ETag、分享、资源字节、项目单元/关联及删除状态 | `restart-recovery` 已实测；新增 durable upload reservation 过期后经进程重启重新准入；Go 内存上传会话失效、多实例 session、写入中崩溃仍待评审 |
| 任务/Agent 回写依赖 | 由 tasks、creation-runs、Agent run/tool 组成的独立合同族 | admission、异步完成、最小增量回写、取消、重试、恢复和权限 | 当前 `generation-boundary` 已执行未认证、已有任务 owner 详情/日志、foreign 任务列表/详情/日志、missing-ID/foreign creation-run read；Agent run owner/foreign durable fixture、Provider 生成、SSE 断线重连、Agent event/approval/undo、creation-run 和 durable writeback 仍需独立 suite 或集成 profile；`generation-writeback` 另执行 provider-free text replay + SSE 游标重放的 5 个 operation/4 个场景 |

因此，迁移计划的最低可审计口径是：**直接画布 14 个唯一 method/path（文档 7 + 分享 5 + 导入 2）全部进入 catalog；创建与更新共用 PUT，不重复计 endpoint。全清单是 163 个 operation / 143 个唯一 method/path，包含项目、资源、旧素材、任务、Agent、Ark 同步与重启恢复分支；只有实际执行的 scenario 才计入运行证据。**

`tests/canvas-contract` 已建立独立 HTTP runner、真实路由/场景审计、提交后响应丢失/重启 fixture 和隔离 Go 基线；另有[真实浏览器伴随套件](../../tests/canvas-contract/README.md#real-browser-companion)，需要已运行的真实前端、同源目标和隔离 fixture，不要求 Java 验收时启动 Go。当前完整双轮 `20261013-122339-53249` 已完成 91/118 版本的 11 个 profile；第一/第二轮分别为 `14197/14196` 与 `14193/14192` 个清单内 observed/wire 请求，均为 71 条映射并执行路由、72 条实际触达路由、71 条非错误路由、6 条受控故障、4 条响应丢失、4 条声明中的 incomplete 和 0 条未分类中断；浏览器两轮均为 6 个场景，页面响应头分别为 171/170，阻断装饰均为 9/9。历史 115 场景双轮 `20261013-073513-51126` 仍保留为历史证据。当前 manifest 为 163 个 operation / 143 个唯一 method/path、91 个 Required operation / 118 个 HTTP 场景；`generation-boundary` 已纳入已有任务 owner/foreign task read；分享资源 object-store fixture 已验证 CDN redirect、成功读取、GET 故障和恢复，不等同生产远端 CDN/TTL/条件缓存。HTTP 完整响应、受控丢失和页面响应头分开统计，不能用请求数冒充端点或代码覆盖率。完整单元正文逐一 GET，文件路由显式区分字节、typed 描述和重定向；排序、成员写和关联竞争按完整 HTTP 结局检查最终状态，不把 start barrier 当数据库交错证明。`generation-writeback` 已覆盖 provider-free text replay + SSE 游标重放，但不覆盖 Provider、SSE 断线重连、creation-run 或 Agent 写回。独立签名撤权、解码预算和并发错误均待负责人裁决，不要求 Java 复制 Go 内部实现。Required 剩余路径、deferred/out-of-scope 和 Java/远端门禁持续可见；多标签并发、任务回写和完整 UI 旅程仍未闭合。版本化运行证据、最新计数和 clean-review 状态唯一维护于[合同测试计划](canvas-java-http-contract-test-plan.md#当前进度)。

当前 Go 权限事实要按资源类型区分：foreign 用户删除 `DELETE /api/canvas-projects/{id}` 返回 `200` 但不删除 owner 数据；foreign `DELETE /api/projects/{id}` 返回 `404`；foreign `GET /api/projects/{id}/units` 和项目 canvas list 返回 `500/internal`。这些是待合同评审的现状证据，不是 Java 目标规则。

## 需要稳定推广的内容

以下内容已经有当前代码依据，适合提升到正式开发导航：

1. 节点、连线、项目和 metadata 的持久化模型。
2. 页面、组件、store、纯函数和 service 的职责边界。
3. 本地 scope、storage revision、journal、tombstone 和远端 ETag/revision 的协作。
4. 生成任务提交、任务消费、最小增量回写、资源物化和幂等效果键。
5. Agent capability registry、结构化画布操作和插件节点的发现路径。
6. 画布性能、批量生成一致性和常见问题的最短排查路径。

这些内容已整合到 `docs/content/docs/canvas/canvas-development.mdx`，并通过 `docs/index.md`、`README.md`、`AGENTS.md` 和 `docs/content/docs/canvas/meta.json` 导航。

## 仍需验证或不应过度承诺的内容

- 当前 checkout 没有完整的文档站构建应用，不能把 MDX 页面当作文档站已经成功构建的证据。
- 画布真实登录、跨设备同步、SSE 断线、对象存储、真实模型上游和独立 `yingce-agent` 运行仍需要环境验收。
- 画布文档只描述当前实现入口，不替代具体 API、数据库、插件 Manifest 或设计记录。

## 维护规则

- 新增画布功能先在本文“按问题找入口”表中补入口，再决定是否增加专题页。
- 新的现场调查、性能数据和未确认根因优先放在 `docs/drafts/`，确认后只把稳定结论增量推广到正式页。
- 修改节点字段、生成合同、远端同步或 Agent capability 时，同时检查本文中的数据模型、调用链和验证清单，避免只更新 UI 文案。
