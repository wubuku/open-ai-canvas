---
title: 后端架构考古与 Spring Boot 移植评估
status: research
scope: backend-java-migration
updated: 2026-10-11
---

# 后端架构考古与 Spring Boot 移植评估

> **目的**：帮助准备把影策后端重建为 Java Spring Boot 的团队看清当前系统的真实边界、值得保留的约束和迁移时应重画的结构。
> **状态**：研究草稿；事实来自本次 checkout 的源码、配置和测试静态核对，不等于生产环境验收。
> **证据范围**：`backend/cmd/`、`backend/internal/`、部署 Compose、当前后端专题文档和对应测试。用户提供的外部架构考古文档只作为评审问题来源，不作为影策现状的证据。

## 结论

影策后端当前是一个以 Go/Gin/GORM 实现的**模块化单体**：HTTP 入口按路由拆分，应用逻辑主要聚合在 `internal/app`，并包含身份、画布、平台、生成、协议、资源、存储、支付、技能等领域或基础设施包；生产部署将 API 后端、PostgreSQL、Redis 和 `yingce-agent` 分开运行。

迁移最值得保留的是业务不变量，而不是 Go 包名：

- 任务先持久化，再由 worker 领取；租约、定期扫描、取消和恢复共同覆盖进程重启与并发执行。
- Task 可记录逻辑模型/route、`BillingOrder` 和 Provider request ID 等执行事实；这些关联按任务用例和计费方式可选。Java 版应逐用例定义哪些身份必需，不能假定每种 Task 都同时有 managed route 和订单，也不能把上游超时等同于“请求没有提交”。
- 模型目录、渠道、Provider 协议、外部任务 ID、本地任务 ID 和媒体资源身份需要分开。
- 资源身份、素材删除策略和物理对象共享关系必须分开。当前显式 `purge=true` 和回收站到期清理路径都可能在画布、任务、项目或历史仍引用素材时删除 Asset/Resource 记录，并将物理对象清理写入持久化作业；这与仓库规范要求“有引用则保留”和“物理删除失败不得删除素材记录”不一致，不能当成已确认的产品合同。迁移前应解决这一差异；Java 版默认应遵守仓库规范，除非产品明确批准不同语义。自定义上游请求还需要路径约束和出站网络安全。
- Agent 的 Java 控制面应保留与 Node runtime / `yingce-agent` 的显式桥接合同。

当前设计的主要代价也很清楚：

- `internal/app.Service` 是一个包含大量运行时状态与跨域编排的中心对象；文件拆分不等于领域边界已经完全独立。
- `internal/service` 主要以类型、函数和常量别名转发到 `internal/app`，不是一个独立的业务服务层。
- App 直接依赖具体的 GORM Repository；部分模型同时带 JSON 序列化和 ORM 标签，存储模型、任务合同和 API 投影之间有耦合。
- API 与 worker 同进程启动，简化单机部署，但线程池、进程内协调和多实例协调必须明确区分。
- 当前代码没有发现覆盖所有领域事件的统一 Transactional Outbox。任务行本身是持久化工作事实，唤醒 channel 只是快路径；不能把外部参考项目的 Outbox 实现描述成当前项目已有能力。

Java 版宜从**模块化单体 + 明确的用例边界 + 可恢复工作流**起步，按业务域拆 Java package/module，而不是把每个 Go package 机械翻成 Spring package。若保留同一数据库或要求不停机迁移，先把 wire contract 与数据迁移方式定下来，再逐领域切流量。

## 证据等级

| 标记 | 含义 |
| --- | --- |
| 当前源码事实 | 本次 checkout 能从代码路径或配置直接确认；不代表已在生产运行 |
| 当前测试事实 | 有命名明确的测试覆盖指定边界；不代表所有相邻路径均覆盖 |
| 架构判断 | 从已确认实现推导出的得失，需结合团队规模和部署目标调整 |
| 移植建议 | Java 版的设计建议，不应误读为当前已实现 |
| 待验证 | 需要真实数据库、真实上游或部署环境才能确认 |

文中提到数据库 schema 支持版本 `47`，依据是当前源码中的 `CurrentSchemaVersion` 与迁移计划尾项。它不证明任何生产数据库已迁移至 47；本次没有连接生产或测试数据库。

## 当前结构与调用关系

当前主要边界可以概括为：

```text
Gin HTTP routes / middleware
        ↓
internal/handler
        ↓
internal/service (公开别名和稳定导入面)
        ↓
internal/app.Service (组合、业务用例和大量运行时状态)
        ├── internal/auth / canvas / platform / prompts / skills / tools / payment
        ├── internal/generation / protocol / provider / outbound
        ├── internal/assets / storage
        └── internal/repository.Repository (具体 GORM 实现)
                    ↓
              GORM models + database migrations
                    ↓
              SQLite 或 PostgreSQL
```

运行时还包括 Redis 协调、对象存储，以及 Agent Node runtime。生产 Compose 使用 PostgreSQL、Redis 和独立 `yingce-agent`；本地默认路径可以使用 SQLite，并在没有远程 Agent 地址时内嵌启动 Node runtime。这个双模式是当前产品部署选择，不应自动成为 Java 版的长期默认。

可核对入口：

- [服务启动、数据库与优雅关闭](../../backend/cmd/server/main.go)
- [HTTP 路由注册总表](../../backend/internal/handler/api.go)
- [service 别名导出面](../../backend/internal/service/aliases_types.go)
- [app 核心服务与 worker 生命周期](../../backend/internal/app/service.go)
- [GORM Repository](../../backend/internal/repository/repository.go)
- [持久化模型清单](../../backend/internal/database/schema.go)
- [后端 HTTP API 合同](../../docs/content/docs/backend/http-api.mdx)

### 事实、合同和持久化

后端有三种容易在移植时混为一谈的类型：

1. **HTTP DTO / wire contract**：由 handler 请求与响应、嵌入的 OpenAPI 共同构成，前端依赖 JSON 字段、状态码和错误语义。
2. **数据库模型**：由 `internal/model`、GORM tag 和迁移逻辑构成。
3. **任务与领域快照**：例如 task 的 `input_json`、`result_json`、加密媒体恢复 checkpoint、路由选择和计费关系。

它们有部分重叠，但不应强制合为同一个 Java 类。尤其不要把一个 JPA Entity 同时作为 REST 响应、Provider 请求、后台任务载荷和业务聚合根。

[当前数据模型综述与导航](data-model-overview-and-navigation.md) 已按浏览器状态、后端表、JSON 文档和物理资源给出更细的导航。它也是 Java 版盘点数据边界时的入口，不是完整 ER 图。

### 持久化任务与后台执行

`Task` 行记录包括用户和项目归属、类型、输入/结果、状态与阶段、模型/路由信息、可选的 `BillingOrderID`、Provider request ID、下次轮询时间、租约 owner/expiry、取消事实和加密媒体恢复 checkpoint。可回放的文本增量单独保存在以 `TaskID` 关联的 `TaskTextDelta` 行中，最终正文和失败草稿分别归并到 `Task.ResultJSON` 与 `Task.TextDraft`。`task_worker` 在持久化任务上领取工作，短期 channel 用于快速唤醒，定期扫描用于漏通知后的恢复；执行期间续租，任务结果、worker 主流程进度和终态写入受租约 owner 条件约束。Provider 回报的观测性进度走单独同步路径，不应据此推断所有 Task 字段写入都已统一 fencing。

可核对：

- [任务持久化字段](../../backend/internal/model/models_task.go)
- [任务 worker 领取、租约续期和执行](../../backend/internal/app/task_worker.go)
- [过期租约与重新领取 fencing 测试](../../backend/internal/repository/task_lease_fencing_test.go)
- [任务轮询保留原身份的测试](../../backend/internal/repository/task_provider_poll_test.go)
- [任务恢复和人工处理路径](../../backend/internal/app/provider_task_recovery.go)

这不等于所有业务异步事件都有通用 Outbox。迁移时应保留“持久化事实先于唤醒”的性质；如果 Java 版新增需要跨进程可靠投递的通知或副作用，再针对该域设计 Outbox。

### 数据库与运行环境

`database.Models()` 是当前 GORM 表模型清单；迁移由版本化计划驱动，部分步骤调用 GORM `AutoMigrate`，部分执行显式结构或数据变更。源码当前声明 schema 支持版本 47。`CANVAS_AUTO_MIGRATE=false` 的部署路径要求运行已发布的 schema，而不是在每个 API 实例启动时任意改结构。

SQLite 与 PostgreSQL 都是代码支持的数据库，但锁、并发、类型和 SQL 能力不同。当前启动装配在 PostgreSQL driver 下无条件要求 `REDIS_URL`；协调器的错误信息将它描述为 PostgreSQL 多实例模式的限流、并发和熔断协调依赖，但代码没有按后端实例数区分。SQLite 未配置 Redis 时则使用进程内协调状态。Java 版如果只以 PostgreSQL 为生产目标，应明确把 SQLite 定位为本地开发能力，并决定是否继续要求 Redis；如果还要支持 SQLite，就要为每项迁移、抢占查询、JSON 运算和并发语义分别建测试。

入口：

- [表模型清单](../../backend/internal/database/schema.go)
- [版本化 schema 迁移](../../backend/internal/database/migrations.go)
- [数据库连接与连接池](../../backend/internal/database/database.go)
- [Redis / 本地协调器](../../backend/internal/platform/coordination.go)
- [生产部署 Compose](../../docker-compose.deploy.yml)

## 值得保留的设计

### 1. 先持久化任务事实，再执行慢副作用

长耗时生成不是一个同步 HTTP handler 的延伸。当前任务表能表达排队、运行、轮询、终态、取消、租约和部分恢复证据；API 可以先返回稳定 task ID，客户端再查询或订阅变化。Java 版应让业务任务记录成为可恢复事实源，HTTP 返回与 worker 生命周期解耦。

**保留原则**：先持久化任务；对需要计费的任务，在 admission 阶段一并建立适用的额度授权关系。领取者使用租约与 fencing 条件；Provider HTTP、媒体下载、转码和对象存储调用放在短数据库事务之外；完成后用短事务记录结果、结算或进入待恢复状态。

### 2. 业务任务身份与 Provider 任务身份分开

本地任务 ID 用于用户权限、计费、取消和状态展示；Provider request ID 用于上游查询/取消。它们生命周期不同。一次 Provider 提交遇到读取超时，不能直接推导为“上游没收到请求”，否则自动重试可能重复创建任务、重复扣费。

**保留原则**：在发起外部 create 前冻结请求和路由；记录提交阶段和上游 receipt；无法判定是否被接受时进入可查询、可人工恢复的未知状态，不盲目切换模型或重新 create。

### 3. 将模型选择和 Provider 协议分层

当前仓库有模型目录、逻辑模型/路由、generation 合同、协议注册和 Provider 接入点。它们减少业务代码直接依赖某个厂商 payload 的需要。Java 版应继续拆开：

- 面向产品的公开模型与能力；
- 被选中的渠道、Provider 和上游模型；
- 价格、配额和任务所冻结的计费快照；
- Provider 特有的请求编码、异步轮询、结果解析和错误分类。

不要以一个 `GenericProvider` 填入越来越多厂商 `if/else`，也不要允许高成本任务在提交后静默换路由。

### 4. 资源身份、字节存储和访问 URL 分离

项目包含资源、素材、项目/画布引用、对象存储位置和短时访问 URL 等不同概念。文件字节是否已物化、业务资源记录是否存在、素材/项目是否引用它，是不同事实。删除或恢复不能只看 URL 是否非空。

当前代码存在素材删除行为与仓库规范不一致的情况：显式 `purge=true` 会允许删除仍被画布、任务、项目或画布历史引用的素材记录；回收站到期自动清理也会通过 `DeleteUserAsset()` 进入 archived 素材的引用豁免分支。`TestExpiredArchivedAssetCleanupDeletesReferencedAssetAndUsesDeletionOutbox` 明确验证画布仍引用时 Asset/Resource 记录被删除并创建删除作业。删除事务会移除部分历史资源索引，物理对象随后异步清理；画布后续保存可通过 `repairMissingResources` 修复部分断链。对象清理失败时作业可重试，但 Asset/Resource 记录已在前置事务中删除；这不符合仓库 [素材删除规范](../../AGENTS.md) 对引用保护和保留记录的要求。源码及测试见[素材删除实现](../../backend/internal/app/resource_delete.go)、[自动过期清理](../../backend/internal/app/resource_deletion_worker.go)、[删除事务与引用处理](../../backend/internal/repository/resource_reference.go)、[删除作业重试](../../backend/internal/repository/resource_deletion.go)、[画布修复入口](../../backend/internal/handler/user_data.go)和[删除回归测试](../../backend/internal/app/resource_delete_test.go)。这是待解决的实现/规范差异，不应直接当成产品已确认的 Java 兼容合同；Java 版默认应在有引用时保留并返回来源，并在物理删除成功前保留可查询的资源记录，除非产品明确批准另一种策略。

**面向新实现的默认建议**：分开建模资源记录 ID、对象存储 object key、画布引用键（如 `resource:<ResourceId>`）和业务 `AssetId`；访问时生成授权 URL；删除前查询所有权和业务引用，有引用时保留并返回来源；无引用时可用持久化作业异步清理，但物理对象删除成功前保留可查询的业务记录，失败时继续重试；共享对象不能随任一业务引用一并删掉。

### 5. 外部网络调用应有集中安全边界

当前有 `internal/outbound` 和上游代理路径校验。Java 版应为自定义渠道、模型 Provider、媒体导入和回调下载统一定义 SSRF 规则、重定向策略、DNS/IP 验证、超时、body 上限与可审计日志。把密钥放在数据库加密字段或 secret manager 中，不得写入 URL、普通业务日志或任务对外响应。

## 当前实现暴露的改进空间

| 观察 | 证据 | 移植时的处理 |
| --- | --- | --- |
| `app.Service` 是运行时中心对象，包含 worker、缓存、取消、Agent runner、路由缓存和多个业务协作者 | `internal/app/service.go` | 保留组合根作用，缩小每个 use case 的依赖面；按有界上下文划分服务和端口，避免新的全能 Spring `@Service` |
| `internal/service` 是 `internal/app` 类型/函数的别名稳定入口 | `internal/service/aliases_types.go`、`aliases_funcs.go` | 不把它误译成 Java 业务 service 层；Java API module 应依赖明确的应用用例接口 |
| App 依赖具体 `*repository.Repository`，Repository 接收 GORM 数据库且包含多领域方法 | `app/service.go`、`repository/repository.go` | 将 port 放在拥有用例的模块内；JPA/JdbcTemplate adapter 实现它，避免 domain 依赖 Spring Data |
| Go model 部分同时表达数据库列和 JSON 序列化行为；Task JSON 标记也兼顾输入/输出 | `model/models_task.go` | Java 分开 persistence entity、API DTO、workflow snapshot；用显式 mapper，禁止把 Hibernate proxy 返回给 API |
| Worker 与 HTTP server 由同一启动流程创建 | `cmd/server/main.go`、`app/service.go` | 开发默认 all-in-one；生产可用同一 artifact 的 `api`、`worker` 角色，隔离线程池和副作用故障域 |
| 部分并发能力使用 Redis，单机路径使用本地 map/mutex | `platform/coordination.go` | 把协调能力封装为端口，并为每种部署拓扑写清一致性等级；单机锁不能冒充集群锁 |
| 启动时执行 schema 校验/迁移、默认数据 ensure、技能/工具 seed 和旧存储迁移 | `cmd/server/main.go` | Spring 启动校验与数据变更分开；Flyway 负责 schema，版本化 seed 或显式运维任务负责业务数据，避免每实例启动都竞态修复 |
| 手工 SQL 与 SQLite/PostgreSQL 方言适配存在于 Repository | `repository/repository.go` 等 | Java 版先选生产数据库；复杂 claim/CAS 用明确 SQL，不依赖 JPA 自动生成跨方言等价语义 |
| 当前代码和测试覆盖许多局部恢复场景，但静态测试不等于真实 Provider 或多实例验收 | 任务/资源测试；本轮未运行后端 | 用可控故障注入与 PostgreSQL 集成测试验证，并把生产运行证据和单元测试分开记录 |

这里的“改进空间”不是对当前设计的简单否定。当前方案需要支持嵌入式单机、PostgreSQL 多实例、内嵌/独立 Agent 和快速功能演进；Java 版若只服务一种部署模式，可以减少分支，但要把被放弃的能力写成产品决策。

## 对外部架构参考的取舍

外部参考文档强调了 Transactional Outbox、`AFTER_COMMIT` 快速唤醒、Reconciler、租户上下文和失败语义。可以借鉴其**评审问题**，但不能把其实现记到影策账上：

- 当前影策明确存在持久化 task、任务扫描/领取、续租和恢复。
- 当前影策有 `ResourceDeletionJob` 等面向特定流程的持久化作业。
- 本次检查没有找到统一覆盖计费、资源、通知等所有领域的 Outbox 事实表与 relay。
- Spring Event 是未来 Java 实现可用的进程内唤醒机制；它不能替代数据库事实、租约或 reconciler。
- Outbox 的适用范围应按交付合同决定。单纯更新同一数据库状态并由 worker 扫描时，任务行本身可能足够；要可靠发布事务外事件时，再把事件与业务事实同事务持久化。

建议把所有异步流程画成以下状态转移，标出数据库提交点与不可逆外部动作：

```text
validate / authorize
        -> persist task + applicable billing authorization
        -> claim with lease / fencing token
        -> Provider create/query/cancel outside transaction
        -> persist receipt or explicit UNKNOWN
        -> persist result + billing outcome
        -> reconcile stale, missing, or ambiguous work
```

## 面向 Spring Boot 的架构决策

1. **采用模块化单体**。按身份、模型目录、生成任务、计费、资源、项目/画布、Agent、支付、管理后台等业务域分模块；先共享数据库，暂不拆微服务。
2. **用例拥有事务边界**。事务标在应用用例入口或显式 transaction runner 上；不让 controller、Entity callback 或 Provider adapter 自行开跨域事务。
3. **外部副作用不占数据库事务**。prepare / persist intent / call remote / apply result 分段；不确定结果有独立状态与恢复路线。
4. **持久化任务是事实，事件只是唤醒**。`@TransactionalEventListener(AFTER_COMMIT)` 可减少延迟，但要保留扫描、租约和重启恢复；需保证可靠外部投递的事件再加 Outbox。
5. **框架边界不可泄漏**。Controller 只做 HTTP DTO 解析和错误投影；JPA Entity、Spring Security principal、`HttpServletRequest`、GORM/JPA 类型不能流入 domain。
6. **先保护合同，再重构合同**。如果前端继续运行在本仓库上，第一阶段保持路径、JSON、状态码、Cookie、SSE 游标、上传协议与 OpenAPI 兼容。
7. **强校验默认拒绝**。对象归属、管理员权限、模型能力、配额、计费额度、资源引用、Agent 工具调用都在服务端再次校验。
8. **把运行方式当成显式配置**。API、worker、内嵌开发 Agent 和远程 Agent 各自有启动/健康/关闭合同；远程 Agent 失败不可静默回退为另一条会重复执行的路径。

## 评审入口

- [当前数据模型综述与导航](data-model-overview-and-navigation.md)：浏览器、本地/云端、数据库行、JSON 和对象资源的事实边界。
- [HTTP API 合同](../../docs/content/docs/backend/http-api.mdx)：现有前端兼容面。
- [后端代码地图](../../docs/content/docs/backend/code-map.mdx)：按业务能力进入 Go 代码。
- [云端 Agent 深度调研](cloud-agent-deep-research.md)：Agent 控制面与 Node runtime 边界。
- [Go → Spring Boot 后端移植指引](go-to-spring-boot-migration-guide.md)：将本报告的评估转为目标设计和分阶段实施。

## 验证记录与限制

本报告的结论来自静态源码、配置、文档和测试名称核对；没有在本轮运行后端服务或真实 Provider。

- 源码 schema 常量与迁移计划：`backend/internal/database/migrations.go`；测试包含 `TestCurrentSchemaVersionMatchesMigrationPlan`。
- 持久化任务并发：`backend/internal/repository/task_lease_fencing_test.go`、`backend/internal/repository/task_provider_poll_test.go`。
- Agent 运行路径：`backend/internal/agent/runtime/runtime.go`、`backend/internal/agent/yingceagent/client.go`；没有完成完整 Agent 运行链路验收。
- 未连接生产数据库，也未核对其实际 schema 水位。
- 未执行跨实例 Redis 锁竞争、真实模型生成、真实对象存储或生产压测。

本次审计不是完整逐 endpoint、逐表、逐 Provider 的代码审计。实施迁移前仍需按[移植指引](go-to-spring-boot-migration-guide.md)建立完整合同清单和数据迁移演练。
