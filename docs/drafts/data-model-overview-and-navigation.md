---
title: 当前数据模型综述与导航
status: research
scope: data-model
updated: 2026-10-11
---

# 当前数据模型综述与导航

> **目的**：帮助开发者和 AI Agent 从当前源码理解影策的数据落点、领域边界和身份关系，并快速找到下一层的权威定义。
> **状态**：研究草稿；结论按本次 checkout 的源码核对，不等同于一份完整 ER 图或面向外部的稳定数据合同。
> **证据范围**：GORM 模型清单、schema migration 计划及测试、前端画布类型与持久化、画布服务端边界、现有数据库/架构专题。

## 结论先读

截至本次检查，仓库中未发现一份覆盖浏览器状态、API DTO、数据库表、JSON 文档和对象存储的单一数据模型文件。模型定义和约束分布在几层：

1. **数据库行模型**由 Go 的 GORM 模型和迁移定义，涵盖账号、模型目录、任务、计费、资源、素材、短剧项目、画布元数据和 Agent 运行记录等。
2. **嵌入式 JSON 文档**承载部分变化较快或结构化的内容，例如画布完整文档、素材 payload、任务输入/结果、Agent 检查点和平台设置。数据库字段类型为 `text` 并不代表它是任意 JSON；须继续追到写入校验和读取投影。
3. **浏览器状态与缓存**由前端类型、store 和带用户 scope 的 localForage 存储组成。它们服务于编辑和离线降级，不因此成为服务端已保存事实。
4. **媒体字节**位于对象存储或本地存储目标；数据库的 `Resource` 是物理对象及生命周期元数据，`Asset` 是用户素材库的业务对象。二者不能因为都指向媒体而合并成一个身份。

本次 checkout 中 `backend/internal/database.CurrentSchemaVersion` 为 **47**，迁移测试校验它与 migration plan 最后一项一致。正式数据库专题仍写着“当前结构版本为 42”，同时包含 Schema 45 变更描述，已经落后于源码。迁移文件里“44 和 45 保留给独立迁移”的注释也与同一 migration plan 中显式登记的版本 45 `upload_reservations` 冲突。阅读当前状态时，以可执行的 migration plan、版本常量和相应测试为准；这不代表所有部署数据库都已升级到 47。见[版本差异说明](#当前版本事实与文档差异)。

## 总体形状

下面是领域关系和数据落点的导航图，不表示所有连线都是数据库外键，也不表示每条关系都有相同基数。

```text
浏览器
  ├─ 页面临时状态 / React Query / Zustand
  ├─ localForage：用户作用域下的画布、素材和其他大型本地数据
  └─ HTTP API / 媒体资源请求
       ↓
Go 后端业务域
  ├─ 账号与身份 ─┬─ 项目 / 章节 / 分镜
  ├─ 模型目录 / 渠道 / 路由 ── 生成任务
  ├─ 积分账户 / 账单 / 账本 ────┘
  ├─ CanvasProject：整张画布的 JSON 文档 + 服务端 revision
  ├─ Asset：素材库业务对象 ── AssetVersion / Representation ── Resource
  └─ Agent execution / event journal / message / Pi session
       ↓                                  ↓
GORM 持久化表与 JSON 文档         对象存储或本地物理媒体
```

同一概念可能在浏览器类型、API 请求/响应和后端模型中各有表示。检查写路径时沿“前端类型 → API 模块 → handler/service → model/repository → migration/测试”追踪，不要因为字段同名就认定三层合同相同。

## 领域模型速览

| 领域 | 主要实体或文档 | 主要边界 | 事实入口 |
|---|---|---|---|
| 身份与账号 | `User`、`AuthSession`、`UserIdentity`、OAuth/验证码记录 | 用户归属是大多数个人数据隔离的基础；敏感凭据通常不通过 JSON 暴露 | [身份模型](../../backend/internal/model/models_identity.go)、[数据库专题：认证](../content/docs/backend/backend-database.mdx#账号认证与密码找回) |
| 模型和渠道 | `ModelChannel`、`ChannelModel`、价格档、逻辑模型及 revision/route | 渠道配置、前台可选逻辑模型、任务实际命中的 route 是不同层次 | [渠道模型](../../backend/internal/model/models_channel.go)、[逻辑模型](../../backend/internal/model/models_logical_model.go)、[模型渠道存储研究](model-channel-configuration-storage-research.md) |
| 异步生成 | `Task`、请求日志、任务执行诊断和媒体恢复记录 | Task 记录运行状态和输入/结果；任务成功、资源落盘、素材登记和积分结算并非同一个状态 | [Task 模型](../../backend/internal/model/models_task.go)、[任务稳定性专题](../content/docs/backend/task-runtime-stability.mdx) |
| 计费和支付 | `CreditAccount`、`CreditLedgerEntry`、`BillingOrder`、支付订单、兑换码 | 账户余额是当前投影，账本记录余额变动，BillingOrder 固化一次任务的报价/授权/结算事实；支付订单是外部支付生命周期 | [计费模型](../../backend/internal/model/models_finance.go)、[支付模型](../../backend/internal/model/models_payment.go)、[数据库专题：支付与账单](../content/docs/backend/backend-database.mdx#支付与插件版本) |
| 资源与素材 | `Resource`、删除任务、`Asset`、素材版本/表现、项目关联 | `Resource` 记录物理存储对象；`Asset` 是可分类、版本化、可关联的业务素材；URL 不能替代物理资源身份 | [资源与项目模型](../../backend/internal/model/models_project.go)、[资源完整性设计](../design/resource-delivery-architecture.md)、[数据库专题：资源存储](../content/docs/backend/backend-database.mdx#资源存储) |
| 自由画布 | `CanvasProject` 行 + `payload_json` 文档、`CanvasSnapshot`、资源引用快照 | 节点、连线、会话、视口等主要在文档 JSON 内；行上的 `revision` 是服务端版本，不等于内容哈希或本地 journal 序号 | [前端画布合同](../../web/src/stores/canvas/use-canvas-store.ts)、[画布类型](../../web/src/types/canvas.ts)、[画布开发导航](../content/docs/canvas/canvas-development.mdx)、[画布深度调研](canvas-deep-research.md) |
| 结构化短剧项目 | `Project`、`ProjectUnit`、`Shot`、`ShotRevision`、`ShotArtifact`、工作流记录 | 这是结构化领域聚合；它与可独立编辑的 `CanvasProject` 不同，可用链接关联，但不能把两种 Project 身份混为一谈 | [项目模型](../../backend/internal/model/models_project.go)、[项目 API 类型](../../web/src/services/api/projects.ts)、[架构总览](../content/docs/overview/architecture.mdx) |
| Agent 运行和偏好 | `CloudAgentExecution`、事件/消息记录、Pi JSONL session、canvas mutation、Agent profile/lesson | Go 控制检查点、事件凭证、对话消息和 Pi 原生会话快照承担不同用途；偏好和记忆不授予额外能力 | [Agent 模型](../../backend/internal/model/cloud_agent.go)、[Agent 深度调研](cloud-agent-deep-research.md)、[代码功能地图](../content/docs/backend/code-map.mdx) |
| 平台配置和扩展 | `SystemSetting` JSON、插件状态、技能与版本、策展配置、公告 | 单表 JSON 配置、版本化技能和平台/用户两级插件状态有各自的 schema 与生命周期 | [平台模型](../../backend/internal/model/models_platform.go)、[插件状态模型](../../backend/internal/model/models_plugin.go)、[数据库专题](../content/docs/backend/backend-database.mdx) |

## 最容易混淆的身份

| 身份 | 表达什么 | 不应当作什么 |
|---|---|---|
| `UserID` / 用户 scope | 服务端数据归属和浏览器本地隔离范围 | 不能省略后退化为全局缓存键或默认用户 |
| `Project.ID` | 短剧/结构化创作项目身份 | 不等同于画布 ID |
| `CanvasProject.ID` | 自由画布文档身份 | 不等同于 `ProjectID`；后者是可选的项目关联 |
| `Task.ID` | 一次异步任务的身份 | 不等同于上游 request ID、渠道 route、生成批次或账单 ID |
| `BillingOrder.ID` | 一次授权/结算的账单身份 | 不等同于支付订单；金额应以账单快照和账本事实核对 |
| `Asset.ID` | 素材库的业务对象身份 | 不等同于底层文件或对象存储 key |
| `Resource.ID` | 后端跟踪一个物理资源的业务记录 | 不等同于 `ObjectKey`、公共 URL或素材 `AssetID` |
| `revision` | 所属聚合或文档的并发版本 | 不等同于数据库 schema 版本、内容 hash、任务 ID或时间戳 |
| schema migration version | 数据库结构升级位置 | 不等同于 JSON 配置的 `schemaVersion`、业务对象 revision 或应用版本 |

画布还有自己的同步身份：ETag、`remoteContentHash`、本地持久化 revision 和 journal 序号不能互相替代。详见[画布深度调研第 5 节](canvas-deep-research.md#5-云端同步revisionetag历史与内容身份)。

## 从问题找到正确的模型来源

### 想确认数据库里有哪些 GORM 模型

先看 [`Models()` 持久化清单](../../backend/internal/database/schema.go)，它用于数据库初始化和迁移相关流程；再看各模型定义及 GORM tags。`Models()` 是代码清单，不是业务约束全集：权限、归属和跨对象一致性还需阅读 app/service、repository 和测试。

### 想知道字段或索引何时加入

查看 [`migrations.go`](../../backend/internal/database/migrations.go) 的迁移注册表、具体迁移函数和 [`migrations_test.go`](../../backend/internal/database/migrations_test.go)。不要只看某一份运行数据库的快照：迁移代码还记录约束演化、回填和拒绝条件。

### 想理解 API 合同和前后端映射

阅读对应的 `web/src/services/api/` 模块、`backend/internal/handler/` 和应用/域层。数据库 `json:"-"` 字段、服务端投影、加密密文和客户端 DTO 可能故意不同。HTTP envelope 和接口边界见 [HTTP API 专题](../content/docs/backend/http-api.mdx)。

### 想理解一张画布保存了什么

从 `CanvasProject` 前端类型开始，再读画布开发导航的持久化模型、同步和生成回写章节。服务端将画布内容保存为 `payload_json`，并单独管理所有者、标题、项目关联和 revision；历史快照保存文档结构，媒体由 Resource 体系承载。

### 想追踪媒体为何能被删除或保留

从 `Asset`/`AssetVersion`/`AssetRepresentation` 与 `Resource` 同时向下追踪引用解析、删除守卫和异步物理清理。只看素材卡片 URL 或某个 `resource_id` 字段可能漏掉 metadata 中的嵌套引用。

### 想理解一次生成和扣费

把 Task 生命周期、模型 route、BillingOrder 不可变快照、CreditLedgerEntry 余额变动和结果 Resource 分开核对。当前实现不会因为前端显示某种状态就自动证明上游任务、资源物化和积分结算全部完成。

## 当前版本事实与文档差异

截至 2026-10-11 本次 checkout：

- [`backend/internal/database/migrations.go`](../../backend/internal/database/migrations.go) 声明 `CurrentSchemaVersion = 47`；当前 migration plan 最后一项为版本 47 `skill_curation_roots`。
- [`TestCurrentSchemaVersionMatchesMigrationPlan`](../../backend/internal/database/migrations_test.go) 断言当前版本与 migration plan 最后一项一致；另一个迁移测试 `TestMigrateSchemaRecordsAndValidatesVersion` 检查迁移后的数据库包含 `upload_reservations` 表。
- [后端数据库专题](../content/docs/backend/backend-database.mdx) 顶部记录 Schema 45 `upload_reservations`，但“结构迁移与版本”段落仍称当前结构版本为 42，并以 42 为旧程序兼容边界。这两处不应作为当前 schema 最新版本引用。
- `migrations.go` 注册表附近的注释称 44 和 45 保留给独立迁移；列表又显式登记了 45 `upload_reservations`。这段注释与可执行列表不一致，应由维护者进一步核对是否只是陈旧注释。
- `CurrentSchemaVersion = 47` 说明当前代码支持的 schema 上限，不证明每个开发、测试或生产数据库已经执行到该版本。应通过目标数据库的迁移状态/ready 检查确认。

本草稿仅记录差异，不在本轮改写正式数据库专题，避免把宽泛的迁移历史重排成未经维护者确认的新 schema 字典。

## 证据和验证记录

本次检查过的主要来源：

- [`database.Models()`](../../backend/internal/database/schema.go) 与 `AutoMigrate` 调用。
- [`CurrentSchemaVersion` 和 migration plan](../../backend/internal/database/migrations.go)。
- [`TestCurrentSchemaVersionMatchesMigrationPlan`](../../backend/internal/database/migrations_test.go)。
- `backend/internal/model/` 中 identity、task、finance、channel、project、platform、plugin 和 Agent 模型定义。
- `web/src/stores/canvas/use-canvas-store.ts`、`web/src/types/canvas.ts`、`backend/internal/canvas/user_data.go` 和 `backend/internal/model/models_canvas_history.go`。
- 正式文档 `docs/content/docs/backend/backend-database.mdx`、`docs/content/docs/canvas/canvas-development.mdx`、`docs/content/docs/overview/architecture.mdx`、`docs/content/docs/backend/code-map.mdx`，以及画布和 Agent 研究草稿。

已执行：源码和文档静态阅读、引用路径核对，以及 `cd backend && go test ./internal/database -run '^TestCurrentSchemaVersionMatchesMigrationPlan$' -count=1`，通过。未在本轮连接任一真实数据库、导出生产 schema、检查部署实例的实际迁移状态，也未运行完整后端测试；因此本文描述的是当前 checkout 可见的模型和约束，不是生产数据审计。

## 后续维护与推广

当模型、JSON 合同或迁移变化时，按对应实体更新本文入口和版本事实，并同步所属正式专题。正式数据库专题的结构版本应与当前 migration plan 对齐；若要发布一份完整数据字典，还需决定是否为高频聚合补齐稳定 ER 图、字段表、删除策略、所有权边界和 JSON 子 schema，不能仅把 GORM struct 列表复制成“完整模型”。
