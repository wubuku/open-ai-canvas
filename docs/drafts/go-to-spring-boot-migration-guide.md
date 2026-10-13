---
title: Go 后端迁移到 Java Spring Boot 指引
status: draft
scope: backend-java-migration
updated: 2026-10-12
audience: 准备以 Java Spring Boot 重建影策后端的架构师与开发团队
---

# Go 后端迁移到 Java Spring Boot 指引

> **目标读者**：正在评估用 Java Spring Boot 重建当前 AI 视频创作工作台后端的开发者。
> **文档定位**：这是迁移和架构改进指引，不是 Go 文件逐个对应 Java 文件的翻译表。
> **事实边界**：当前项目事实以本仓库源码、配置和测试为准；Java 目标结构是建议。本轮已在隔离 Go/PostgreSQL 环境完成画布 HTTP 合同与真实前端 companion 基线；没有运行 Java、生产数据库或模型调用。
> **相关研究**：[后端架构考古与 Spring Boot 移植评估](backend-architecture-review.md)记录当前实现的得失；[当前数据模型综述与导航](data-model-overview-and-navigation.md)用于逐领域盘点数据落点；[画布 Java HTTP 合同测试计划](canvas-java-http-contract-test-plan.md)将本文的 API 合同、owner、切流和验收原则落到画布专项。

本文按 `spring-boot-principal-architect` 的 **B：重写/大型重构**模式组织：先建立证据账本、事实 owner、operation 合同和失败恢复协议，再选择模块边界与基础设施；默认采用 PostgreSQL durable facts、短事务、transactional outbox、reconciler、单 writer 和可回滚的逐 operation 切换。该架构指引约束 Java 的目标设计，但不把 Go 当前的包结构、锁顺序、错误文本或缺陷自动提升为兼容合同。

## 阅读方法与证据等级

本文将内容分成三类，迁移实施时不要混用：

- **当前事实**：能由当前 checkout 的源码、配置、测试或正式文档直接确认；引用路径时以源码和测试为优先。
- **目标建议**：Java 版应采用的模块、端口、状态机或验收约束；不表示 Go 当前已经这样实现。
- **待决策/待验收**：需要产品、运维或真实环境确认的兼容性、SLA、数据切换、回滚和 Provider 行为。

证据等级单独表示结论强度：`CORROBORATED` 表示由多个相互独立的源码、测试、配置或正式文档交叉支持；`FACT` 表示有直接证据但尚未完成交叉验证；`UNKNOWN` 表示本轮没有足够证据，不能作为实现前提。

如果源码、测试和正式文档出现冲突，先把冲突记录为 M0 事项，不要用 Java 设计“顺手修正”而不留下迁移决策。本文的目标不是证明迁移可无停机，而是把必须证明的兼容合同、数据不变量和恢复路径列出来。

### 迁移验收运行边界与阅读入口

保留现有 Web 是外部合同选择，不是要求 Java 复制 Go 的内部架构。当前前端能独立构建/启动，但仍依赖后端的 HTTP/业务合同；配置可以指向 Java 不等于兼容性已经验证。启动移植或联调前，按目标选择入口：

- **只验收后端合同**：运行独立 Go module 的 HTTP 主套件，准备目标 Java/网关和隔离 fixture；无需 Web、浏览器或 Go 产品进程。从源码运行仍需 Go 工具链。见[套件用途](../../tests/canvas-contract/README.md#成果用途与证据职责)和[独立运行矩阵](../../tests/canvas-contract/README.md#独立运行边界)。
- **验收真实前端配合**：另运行 Python/Playwright/Chromium companion，准备真实 Web、同源 `/api` 目标及 fixture；可用本地 Vite，不必生产部署。UI 操作与直接 API setup/audit 分开统计，浏览器场景不计入 HTTP 覆盖。见[前端配合条件及连接示例](../../tests/canvas-contract/README.md#web-前端与-java-的配合条件)。
- **只迁移部分 API**：为登录/session、Resource/Asset、页面辅助 API 和测试 fixture 明确提供者；采用混合网关时坚持逐事实单 writer。只有画布 CRUD 不足以运行现有浏览器场景；当前 setup 还依赖管理员创建测试用户等 API。依赖审计和最新缺口见[专项运行边界](canvas-java-http-contract-test-plan.md#http-与浏览器的独立运行边界)。
- **有意改变合同**：先批准差异，再选择兼容 adapter 或前端/测试同步改造；不要为保留 UI 无条件复制 Go 缺陷，也不要因后端换语言而默认重写前端。架构实施门槛见 [M1](#132-m1java-基础壳和合同测试)，成果和后续变更的文档落点见[维护责任](canvas-java-http-contract-test-plan.md#文档落点与维护责任)。

## 0. 迁移进度账本

| 项目 | 当前值 |
| --- | --- |
| 目标 | 在不重复外部副作用、不破坏历史数据和不扩大权限的前提下，以 Java Spring Boot 模块化单体逐 operation 接管 Go 控制面 |
| 模式 | B：重写 / 大型重构 |
| 范围 | API/认证兼容、PostgreSQL/Flyway、任务/Provider/计费、媒体、画布、Agent 控制面、切换与回滚 |
| 非目标 | 第一阶段不重写 Node/Pi runtime，不拆微服务，不引入 Broker/workflow engine，不承诺无停机 |
| 基线 | `research` 分支的当前 checkout；具体源码、suite input hash 和运行证据以画布合同计划登记的 evidence 目录为准 |
| 当前唯一进行中步骤 | M0：补齐逐 endpoint/逐表/逐 Provider/逐环境清单，并清零或明确阻断关键未知项；画布专项同步推进 M1 前置合同缺口 |
| 下一步验证 | 画布分享资源本地 CDN/object-store 成功/GET 故障/恢复和 90/115 版本完整双轮已加入基线；下一步补 `core` updated-at 并列排序、项目关联 persistence/revision failure injection、资源预留到期/协调恢复；全局迁移再用真实 PostgreSQL 副本补逐表映射，并用 wire/故障注入测试证明一个低成本 Provider 的 prepare → UNKNOWN → reconcile → settle 闭环 |
| 阻塞项 | tenant/RLS 是否启用、archived/purge 删除例外、Provider callback inventory、真实数据水位和回滚窗口 |

完成一项 M0 决策后，先更新本节的当前步骤、证据和阻塞项，再进入下一阶段；不要另建第二份迁移状态表。

## 1. Executive decision

以保持现有 Web UI 可工作的 Java 替代后端为目标，推荐路线是：

1. 先冻结并测试前端依赖的 HTTP、Cookie、SSE、上传和资源访问合同。
2. 先定 PostgreSQL 数据兼容策略和写入切换方式，再建 Java 模块骨架。
3. 采用按业务域组织的模块化单体。一个代码库、一个主要事务数据库；API 与 worker 可使用同一构建产物的不同运行角色。
4. 把任务、计费、上游提交、媒体落盘和 Agent 运行建成可恢复工作流；事务外调用 Provider。
5. 按读路径、低副作用写路径、资源/身份、任务生成与计费、画布/Agent/后台分阶段迁移。
6. 先用影子读取、合同测试和可回滚流量路由验证；避免 Go、Java 同时写同一业务记录的非原子双写。

这是 **B 模式（重写 / 大型重构）**，不是“换语言但保持原包结构”的项目。需要兼容的是已承诺的外部合同、历史数据和在途副作用；Go 当前的包划分、Service 形态和隐含状态不是 Java 目标架构。本文是架构决策与实施授权前的约束，不等于已经批准开始写代码。

重写动因仍要单独验证。当前能确认的是“产品希望启动 Java/Spring Boot 移植”，以及现有 Go 代码存在中心组合对象、GORM 领域耦合、长任务/计费/资源/Agent 多类副作用等结构事实；本轮没有性能基线、事故统计、团队交付成本或合规证据证明“整体重写”已经优于继续演进 Go。因此 M0 的出口条件不是“Spring Boot 项目能启动”，而是完成以下基线决策：

| 方案 | 当前判断 | M0 要补的证据 |
| --- | --- | --- |
| 继续 Go，按 owner/operation 重构 | 可行的基线方案 | 需要估算边界治理、长期维护和目标能力交付成本 |
| Java 增量替换 Go 控制面 | 本文推荐的工作假设 | 需要证明合同冻结、数据映射、在途接管和单 writer 切换可行 |
| 一次性整体重写 | 暂不推荐 | 必须有停机预算、数据/在途迁移方案、回滚或 forward-only 批准，以及“不增大风险”的证据 |

在上述证据完成前，本文支持的是 Java 目标架构评审和增量试点，不把语言迁移本身当成业务收益。

目标架构的硬选择：

- Spring WebMVC + JPA/Hibernate + PostgreSQL 作为默认基线；JPA 管理聚合，CAS、claim、fencing、批量查询使用明确的 JdbcTemplate/native SQL。
- PostgreSQL durable fact + transactional outbox + reconciler 作为任务、事件和恢复基座；Redis 只能做协调、限流或唤醒，不能成为 task、receipt、账本或权限事实源。
- 每类高成本或非幂等外部操作都进入版本化 operation catalog；默认一次用户意图只允许一个可证明的远端副作用。结果无法证明时进入 `UNKNOWN`，恢复 worker 只查询、对账或转人工；只有 Provider 的书面合同和 wire test 证明同一稳定幂等 key 会落到同一远端操作时，才允许使用原 key 做受控 query/replay，这不是普通自动重试，也不能创建新 attempt 或切换 route。
- 每类 durable fact 只有一个写入 owner；Controller、回调、admin job、投影和跨模块代码不能直接写另一个模块的表或实体。
- `spring.jpa.open-in-view=false`；事务只覆盖短数据库阶段，Provider、对象存储、支付和 Node runtime 调用一律在事务外。
- 第一阶段不引入微服务、Broker、workflow engine 或新的 Agent runtime。只有故障域、吞吐、合规隔离或组织边界的可重复证据满足触发器后，才提交 ADR 引入。

不建议照搬的内容：

- 不要把每个 Go package 机械映射为一个 Spring @Service。
- 不要让 Controller 直接访问 Repository 或把 JPA Entity 返回给 Web。
- 不要把所有业务放进一个 CanvasService / BackendService。
- 不要把 Provider 调用包在数据库事务中。
- 不要用 Redis、Spring Event 或消息 Broker 代替数据库中的任务事实和恢复协议。
- 不要仅凭任务 HTTP 请求超时、一次轮询失败或上游响应慢，就把视频任务标成失败并重新提交。

## 2. Capability profile、假设与证据账本

### 2.1 推荐的默认假设

本文按以下默认前提写：

- Web 前端继续使用当前仓库的 API 调用层，Java 服务先兼容现有路径和主要 JSON 结构。
- 生产数据库以 PostgreSQL 为目标，Redis 保留给跨实例协调或高争用并发控制。
- 本地开发可以选择只支持 PostgreSQL，或额外保留 SQLite；后者是需要单独付出测试成本的产品能力。
- Agent 的 Pi/Node runtime 和独立 yingce-agent 继续作为独立运行边界；本任务是移植 Go 控制面，不是重写 Node runtime。
- 先完成模块化单体，不在迁移第一阶段拆微服务或全面引入消息平台。

这些都是工作假设，需要由实施方在启动阶段确认。当前生产编排使用 PostgreSQL、Redis 和远程 `yingce-agent`（见 [`docker-compose.deploy.yml`](../../docker-compose.deploy.yml)）；`cmd/server` 的本地默认仍允许 SQLite 和内嵌 Agent runtime（见 [`main.go`](../../backend/cmd/server/main.go)）。当前协调器在 PostgreSQL 方言下即使只有一个后端实例也要求配置可用的 `REDIS_URL`（见 [`coordination.go`](../../backend/internal/platform/coordination.go)）；Java 实施应明确决定是否保留这一部署约束，不能把它误认为 Redis 对单实例 PostgreSQL 天然必需。部署能力需由产品决定是否全部保留。

### 2.2 把“迁移完成”定义成可验收的合同

按 endpoint/用例建立清单，至少记录：

| 维度 | 迁移合同 |
| --- | --- |
| 请求 | method/path、认证方式、请求字段、空值和缺省语义、大小上限 |
| 响应 | HTTP 状态码、JSON 字段、分页和排序、空集合语义 |
| 错误 | HTTP status、code、reason、安全展示文案与可重试分类 |
| 身份 | 当前用户如何从 session 得到；Cookie、CORS 与 CSRF 语义；对象归属和管理员权限如何核实 |
| 副作用 | 是否创建 task、扣减额度、调用 Provider、写资源、发 SSE |
| 幂等 | 同请求重放会返回同一 task、拒绝重复还是创建新任务 |
| 并发 | CAS 字段、租约、取消竞争、重复回调与陈旧 worker 写入 |
| 资源 | 业务 ID、object key、访问 URL、引用和删除行为 |
| 观测 | request/trace ID、task ID、provider request ID、审计字段和脱敏要求 |
| 测试 | Go 现状合同/行为基线证据、Java 合同测试、数据库集成测试和真实环境验收责任人 |

OpenAPI 是清单起点，不是完整语义证明。需要补上 SSE、上传分片、回调、资源 URL 和有副作用的后台管理操作。

### 2.3 Capability profile

能力先标记为 `enabled`、`deferred` 或 `disabled`；未启用的能力不因为 Spring 生态“有现成组件”就预装进目标系统。

| 能力 | 当前判断 | Java 目标约束 | 证据/触发器 |
| --- | --- | --- | --- |
| 外部副作用与长任务 | `enabled` | operation catalog、fence/receipt、`UNKNOWN`、worker、reconciler | 生成任务、Provider 查询和 worker 源码/测试 |
| 计费与支付 | `enabled` | prepare 授权、terminal settle、不可变动账、resolution、对账 | 额度、充值、模型报价和后台调账路径 |
| 对象存储与媒体 | `enabled` | managed object 身份、上传 reservation、checksum、孤儿恢复 | Resource/Asset、上传和删除实现 |
| SSE / 实时投影 | `enabled` | 订阅鉴权、递增游标、断线恢复；SSE 只是 projection/wake-up | 前端任务事件合同和 `/text-events` 实现 |
| 支付 webhook | `enabled` | 签名/来源认证、时间窗、replay 防护、订单权威与用户读取分离 | [`payment.go`](../../backend/internal/handler/payment.go)、[支付 API](../../docs/content/docs/backend/http-api.mdx) |
| Provider callback | `deferred` | 逐 Provider 登记签名、来源、关联 receipt、乱序和 replay 合同；未盘点前只允许轮询或人工路径 | 当前主要依据 Provider polling；M0 补齐 callback inventory |
| 多租户 | `deferred` | 在确认 tenant 语义前只做 user/owner scope，不伪造 `tenant_id` | 当前审阅到的是 `user_id` 归属；需产品与数据盘点确认 |
| PostgreSQL RLS | `deferred` | 仅在多租户启用后评估；若启用必须 fail-closed、`FORCE RLS`、真实 PG 测试 | 多租户决策与合规要求 |
| SQLite 生产兼容 | `deferred` | Java 目标以 PostgreSQL 为准；保留 SQLite 必须独立 profile 和测试 | 当前 Go 支持 SQLite，但生产编排使用 PostgreSQL |
| Broker / workflow engine | `disabled（M0-M6）` | 数据库状态机、outbox、reconciler 先满足需求 | outbox 积压、DB 锁等待、Provider 排队和故障域证据 |
| Node / Agent runtime | `enabled（外部边界）` | Java 只做控制面和 bridge adapter，不重写 runtime；`/model`、`/tool`、`/event` 是带 run 绑定的内部 bridge，不是公共 webhook | 当前内嵌/远程 yingce-agent 合同 |

### 2.4 证据账本

以下是本轮审阅已确认的最小证据账本；`DEFERRED` 不得在实现中被当作已决定能力。

| 结论 | 分类 | 证据等级 | 证据入口 | 迁移含义 |
| --- | --- | --- | --- | --- |
| Web 依赖统一响应信封、`code/reason`、Cookie、SSE 和资源 API | `CURRENT FACT` | `CORROBORATED` | [HTTP API](../../docs/content/docs/backend/http-api.mdx)、[OpenAPI](../../backend/internal/handler/openapi.yaml)、[前端 http](../../web/src/services/api/request.ts) | 第一阶段保持兼容；新合同要版本化 |
| Task 是持久化事实，worker 通过数据库 claim/lease 工作；过期租约的部分旧 writer 会被拒绝 | `CURRENT FACT` | `CORROBORATED` | [`models_task.go`](../../backend/internal/model/models_task.go)、[`task_worker.go`](../../backend/internal/app/task_worker.go)、[lease test](../../backend/internal/repository/task_lease_fencing_test.go) | 不能把局部 lease fencing 泛化为所有 Task 字段写入都已 fencing；Java 应为每类状态转移和结果写入定义统一 fence |
| 视频执行期限与 Provider polling 默认约一小时 | `CURRENT FACT` | `FACT` | [`runtime_policy_test.go`](../../backend/internal/app/runtime_policy_test.go) | 只证明当前默认值，不证明所有 Provider SLA |
| 生产部署需要 PostgreSQL、Redis 和远程 Agent | `CURRENT FACT` | `CORROBORATED` | [`docker-compose.deploy.yml`](../../docker-compose.deploy.yml)、[`coordination.go`](../../backend/internal/platform/coordination.go)、[`runtime.go`](../../backend/internal/agent/runtime/runtime.go) | Redis/Agent 继续作为 adapter；不把它们升格为事实源 |
| 当前没有覆盖所有领域的统一 Transactional Outbox | `CURRENT FACT` | `CORROBORATED` | [`backend-architecture-review.md`](backend-architecture-review.md)、[资源删除作业](../../backend/internal/repository/repository_storage_resources.go) | Java 目标按副作用逐 operation 引入 outbox，不把当前 Go 机制描述成已存在的统一能力 |
| archived/purge 删除路径与仓库删除规范冲突 | `CURRENT FACT` | `CORROBORATED` | [删除实现](../../backend/internal/app/resource_delete.go)、[回归测试](../../backend/internal/app/resource_delete_test.go)、[独立 HTTP 基线](../../tests/canvas-contract/README.md)、[AGENTS.md](../../AGENTS.md) | HTTP DELETE/批量删除的 purge 可制造引用断链；已验证普通保存拒绝、显式修复保留损坏 preimage、旧历史恢复拒绝。M0 仍须选择 KEEP 例外或 OVERTURN，不自动批准当前行为 |
| 当前系统是否有可直接迁移的 tenant 合同 | `UNKNOWN` | `UNKNOWN` | 本轮未找到独立 tenant/membership/RLS 合同 | 安全默认只按已验证的 user/owner scope；不把 userId 改名为 tenantId |

## 3. Architecture principles

Java 版采用以下原则；每条原则都必须有架构测试、集成测试或故障注入的检查方式。它们是对 Go 现状的重新取舍，不是把现有包名翻译成 Java 包名。

1. **证据先于偏好**：重要结论标为 `CURRENT FACT`、`TARGET INVARIANT`、`DECISION`、`DEFERRED` 或 `UNKNOWN`；代码里存在某个机制不等于目标环境已启用。
2. **一个事实，一个写入 owner**：Task/Attempt、receipt、资金授权、动账、对象身份、画布 revision、Agent checkpoint、outbox 和人工 resolution 分别登记 owner；其他模块只能通过 port/API 请求状态转移。
3. **模块边界必须可测试**：模块间只交换 DTO、ID、port 和事件；禁止跨模块直接访问 JPA Entity、Repository 或表。用 ArchUnit 或 Spring Modulith 阻止依赖倒灌。
4. **先分类 operation，再决定重试**：纯查询、幂等 mutation、非幂等 create、本地副作用分别定义 timeout、retry、cost、权限和审计语义；异常类名不能替代 failure facet。
5. **外部副作用必须在事务外**：事务只提交 prepare/apply/settle 等短阶段事实；Provider、支付、对象存储和 Node runtime 不得持有数据库锁、连接或持久化上下文等待。
6. **非幂等 create 必须可判定、可停机**：写出之后的超时、连接重置、5xx 或空响应默认都是 `UNKNOWN`；reconciler 默认不重发 create。只有 Provider 明确书面承诺稳定幂等 key 的 query/replay 语义，并经 wire test 证明不会产生第二个远端副作用时，才可按原 key 有界恢复；否则无法按 receipt 查询就停在 `UNKNOWN`/人工队列。
7. **事件、回调和 SSE 不能替代事实**：事件和 after-commit hook 只做 wake-up；必须送达的后续动作写 transactional outbox；SSE 从 durable 状态和递增游标重建。
8. **身份、归属和平台访问分层**：认证主体、Actor、user/tenant、资源 ACL、system principal 和 operator principal 不混用；当前用户 ID、task ID、URL 或回调正文不能成为授权依据。
9. **计费是事实协议**：授权、结算、动账、余额/usage projection、resolution 和审计分开建模；动账和 resolution 默认 append-only，可从账本重算并对账。
10. **JPA、CAS 和 Flyway 有交互合同**：`open-in-view=false`；JPA 管聚合，native SQL 管 fenced 状态；bulk/CAS 后 reload，生产只使用追加式 Flyway，禁止 `ddl-auto=update`。

违反原则时要记录后果、证据、替代方案和回滚成本，不以“Go 现在也这样”作为豁免理由。

## 4. Durable facts、source of truth 与 ownership

目标系统先登记事实所有权，再画模块。下表是第一版 owner map；M0 必须把它扩展到逐表、逐字段和逐操作。`projection`、`cache` 和 `wake-up hint` 不得被误当成 durable fact。

| Durable fact | 目标唯一写入 owner | 其他模块允许做什么 | 禁止做什么 |
| --- | --- | --- | --- |
| Task、worker lease、task 状态转移 | `tasks` | 通过 typed transition API 申请创建、claim、cancel、apply | Controller、回调、admin job 直接更新任务状态 |
| Generation operation、Attempt、fence、Provider receipt、查询证据、ambiguity policy | `generation` | 通过 task port 绑定本地任务，通过 receipt/query port 查询和提交证据 | 用 task ID、URL 或异常文本猜测远端状态；绕过 fence 发 create |
| Payment order、外部支付通知与支付证据 | `payments` | 通过 payment port 请求创建/查询/验签；向 billing 提交已验证的结算意图 | 直接修改额度、绕过 webhook 验签或把前端 return URL 当权威 |
| 额度授权、generation settlement、ledger、refund、resolution | `billing` | 读取报价快照和账务结果；提交带幂等键的业务意图 | 修改余额字段、重复结算、把 task success 当扣费事实 |
| Resource/Asset 身份、object key、删除 job | `media` | 申请上传、引用、交付 URL 或删除检查 | 把签名 URL 当身份；无引用检查直接删记录 |
| Project、剧本、分镜、镜头及其 revision | `projects` | 通过 project/shot port 读取和提交版本 | 直接修改项目聚合或把画布 JSON 当项目事实 |
| Canvas 文档、revision、history、share | `canvas` | 通过 patch/use case 读写文档 | 用旧整份快照覆盖用户新编辑；跨模块写画布表 |
| Agent run、step、approval、checkpoint、event record | `agent` | 通过 run/tool/approval port 读取和申请动作 | 重复执行已 checkpoint 的高成本 step |
| Outbox message（随业务事实分片） | 产生事件的事实 owner | relay 发布、claim、重放和消费；通过 owner port 申请状态推进 | 由通用模块跨 owner 直接写入，或把发布成功当成业务事实 |
| Reconciler checkpoint（按 operation 分片） | 对应 operation owner | scheduler 触发扫描并请求恢复 | scheduler 直接修改任务、账务或资源事实 |
| Manual resolution（按 operation 分片） | 对应 operation owner | operator 提交带原因、证据和权限的 resolution command | operator 直接编辑 task、receipt 或 ledger 行 |
| Audit event（按业务域分片） | 产生动作的业务 owner | 通过 append-only audit port 记录和查询 | 把普通日志或内存事件当作必达审计事实 |

`shared` 不拥有上述事实；它只提供稳定的错误、Actor、trace、分页、时钟、幂等键规范化和 outbox 基础接口。SSE、列表和管理统计是对应 owner 的查询投影，不属于事实源；它们可以缓存或持久化，但不能反向修改 owner。模块之间用 ID 引用，不建立跨模块 JPA association。

## 5. Operation catalog 与 identity contracts

以下是第一批必须注册的目标 operation。它是 catalog 起点，不代替 M0 的逐 endpoint 清单；在 catalog 未补齐前，不能把 Provider、支付或批量删除切到 Java。

| `operationId` | 类型 | 关键身份 | owner | `UNKNOWN` / 重试政策 |
| --- | --- | --- | --- | --- |
| `generation.submit.v1` | 非幂等/高成本 create | public idempotency key、request fingerprint、local task/attempt、provider receipt | `generation` | prepare 后至结果可判定前都不删除 fence；写出后的超时/5xx 进入 `UNKNOWN`，只能按 receipt 查询，或在 Provider 书面合同和 wire test 证明同一稳定幂等 key 的 query/replay 安全时按原 key 有界恢复，否则人工对账；不得创建新 attempt |
| `generation.query.v1` | 纯查询 | local attempt + provider receipt | `generation` | 可按 provider policy 有界重试；绝不创建新 attempt |
| `generation.cancel.v1` | Provider mutation | local attempt、receipt、cancel request ID | `generation` | 只有 Provider 权威确认才推进取消终态；未知时保留原任务并进入恢复队列 |
| `resource.upload.finalize.v1` | 幂等 mutation | reservation ID、checksum、fingerprint | `media` | 同一 reservation 只能物化一次；对象成功而 DB 失败由 finalize/reconciler 找回 |
| `resource.delete.v1` | 破坏性本地 mutation | actor、asset/resource ID、引用检查结果、request fingerprint | `media` | 先检查引用和共享对象；物理删除由 durable job 执行；`purge` 例外必须有单独授权和审计 |
| `canvas.patch.v1` | 带版本的本地 mutation | actor、canvas ID、expected revision/ETag、patch fingerprint | `canvas` | 重放同一 revision 可幂等返回；陈旧 revision 拒绝或进入明确合并路径，不用整份旧文档覆盖 |
| `billing.settle.v1` | 本地资金事实 | task/attempt、settlement key、terminal evidence | `billing` | 唯一键/CAS 防止重复动账；失败进入可查询 resolution，不依赖前端重试 |
| `payment.webhook.v1` | 外部回调 | provider event ID、签名时间窗、order ID | `payments` | 先验签和 replay 去重；未知事件只记录/唤醒，不直接改用户余额或结果 |
| `agent.run.v1` | 高成本本地/远端 workflow | actor、run ID、public idempotency key、input fingerprint | `agent` | 同一 run 只创建一次；远端 runtime 失败不 fallback 造成第二次执行 |
| `agent.tool.invoke.v1` | 可能有副作用 | run ID、step ID、tool call ID、checkpoint | `agent` | 同一 step 只应用一次；远端失败不得 fallback 到另一 runtime 重复执行 |

五种身份不可互相冒充：

- `operationId` 是语义合同和版本，不是一次请求 ID。
- public idempotency key 是客户端业务意图的 opaque key，必须绑定 Actor/作用域、fingerprint 和保留期。
- local task/attempt ID 由 Java owner 生成，不能由回调正文或 URL 任意伪造。
- receipt 是 Provider 返回的后续查询身份；没有 receipt 不等于确定未受理。
- request fingerprint 是不含 secret 的规范化输入摘要；同一幂等键下指纹不同必须拒绝。

每个 operation 还要登记确定未发出、远端受理、terminal result、本地 applied、`UNKNOWN` 的判定证据，以及 timeout、cancel、retry budget、限流键、并发上限、资金后果、回调权威和人工动作。高成本 operation 未有这些字段时，安全默认是拒绝自动重试。

## 6. Logical modules and runtime resources

### 6.1 模块化单体和依赖方向

建议以业务域命名模块，以用例划分应用入口：

    com.yingce.canvas
    ├── bootstrap              Spring Boot 启动、配置、运行角色
    ├── shared
    │   ├── api                 响应信封、错误码、分页、request/trace context
    │   ├── security            Actor、鉴权适配器、权限检查原语
    │   ├── tx                  显式事务边界和 outbox/task 调度接口
    │   └── observability       日志脱敏、指标、trace
    ├── identity                用户、session、OAuth、注册/验证策略
    ├── modelcatalog            渠道、模型、能力、逻辑路由、报价
    ├── generation              创建合同、准备、Provider adapter、回调/轮询
    ├── tasks                   持久化任务、claim/lease、取消、恢复、SSE 投影
    ├── billing                 账户、授权、结算、退款、订单/账本
    ├── media                   Resource、Asset、对象存储、上传/交付/删除
    ├── projects                项目、剧本/分镜/镜头等创作结构
    ├── canvas                  画布文档、版本、历史、分享、资源引用
    ├── agent                   控制面、审批、工具和 Node bridge
    ├── payments                充值渠道、异步通知、对账
    └── admin                   用户与系统管理查询、强权限操作

每个业务模块内部可采用轻量的四层分工。箭头分别标出编译期依赖和运行期调用，避免把 port 误画成 domain 的依赖：

    编译期依赖：
    api -> application -> domain
    infrastructure -> application port

    运行期调用：
    api -> application -> (domain rules + output ports)
                         -> infrastructure adapters (JPA, SQL, HTTP, Redis, S3)

application 负责通过自己定义的 port 加载和提交事实，并调用纯业务规则；domain 不依赖 repository/provider port、框架或基础设施。

依赖规则比目录名字重要：

- api 可以依赖应用用例和 API DTO；不能依赖 JPA Entity。
- application 拥有用例编排和事务边界；不依赖 Gin、Servlet、GORM 或 Spring Data repository 具体类型。
- domain 不依赖 Spring、ORM、HTTP 和模型 SDK。
- infrastructure 实现模块拥有的 port；不能反过来规定领域状态机。
- 跨模块调用对方公开的用例/查询 API；不能直接读写另一个模块的 JPA Entity。
- shared 只放确实横切且稳定的协议，不得成长为新的业务大包。

初期不必为每个模块拆 Maven/Gradle 子工程。可以先单体工程按 package 隔离，用 ArchUnit 检查依赖；边界稳定且需要独立发布或编译时再拆 build module。

### 6.2 Spring Boot 组件选择

建议使用 Spring Boot 的成熟基础能力，但把组件选择与领域合同分开：

- HTTP：Spring MVC。SSE 端点使用明确的事件 ID 和断线恢复合同。
- 校验：Jakarta Bean Validation 做入参格式校验；领域规则仍由 use case/domain 校验。
- 安全：Spring Security；当前用户身份统一解析为不可变 Actor，应用代码不读 request/session 静态上下文。
- 数据库：PostgreSQL + Flyway；JPA 用于常规聚合读写，JdbcTemplate/SQL 用于 CAS、任务 claim、复杂统计和需要明确锁语义的路径。
- JSON：Jackson 显式配置日期、枚举、未知字段、精度和 null 策略；不要依赖默认值悄悄变化。
- Redis：用于需要跨进程协调的限流和并发槽位等短期状态，不保存业务任务事实；worker 对 task 的 claim、续租和 fencing 仍以数据库任务记录为准。
- 外部 HTTP：每类 Provider 有单独超时、连接池、重试和日志脱敏策略；网络目标经过统一 SSRF 规则。
- 作业：先使用数据库持久化 task + transactional outbox + worker claim + reconciler。只有吞吐或隔离需求明确后再加 Broker。
- 可观测性：Micrometer 指标、结构化日志、OpenTelemetry trace；request ID 与业务 task ID 分开。

选择 JDK 和 Spring Boot 版本时，以迁移实施时支持的 LTS、供应商依赖和部署镜像为准；本草稿不锁定版本。

### 6.3 运行时资源边界

运行时资源也要有 owner 和上限，不能把线程数、连接池或 Redis 当成隐式背压：

| 资源 | Java 目标职责 | 必须显式配置/观测 |
| --- | --- | --- |
| HTTP request threads | 只做校验、prepare、查询和短 apply，不等待 Provider 长任务 | 最大并发、拒绝策略、P95/P99、请求超时 |
| Worker executor | 执行已 claim 的短阶段和事务外 adapter 调用 | task/Provider/tenant admission、并发槽位、lease、取消 |
| Scheduler/reconciler | 只扫描、唤醒、claim 和恢复 | 扫描窗口、单次批量、最老 `UNKNOWN`、重试预算 |
| PostgreSQL pool | 短事务和必要查询 | active/idle、等待、锁等待、事务时长、失败率 |
| Provider client pools | 每类 Provider 的连接与并发隔离 | connect/read/total timeout、重试、限流、熔断、成本 |
| Object storage workers | 上传、下载、checksum、删除和孤儿恢复 | 文件大小、带宽、并发、残留对象和删除作业年龄 |
| SSE subscriptions | 从 durable task/event 状态读取并按 cursor 投影 | 每实例订阅数、断线、重连、跨实例扇出策略 |
| Redis（若保留） | 协调、限流、短期 slot 或 wake-up | 不可用时的 fail-closed/降级；不存事实 |

多实例前必须清点进程内 registry、SSE 订阅、本地缓存、临时文件、定时任务和限流预算。没有观测和替换测试，不因为“已经用了 Redis”就声称系统具备安全的水平扩展能力。

## 7. Transaction, external side-effect and recovery protocol

所有非幂等或高成本外部调用统一走四段协议：

```text
[短事务 prepare]
  校验 Actor / scope / ACL / quota；冻结 model/provider/capability/price/request snapshot
  计算 fingerprint；创建唯一 fence / attempt；冻结 ambiguity policy；提交 PREPARED

[事务外 external I/O]
  调用 Provider / payment / object storage / Node runtime
  不持有数据库锁、连接或 JPA persistence context；只保留最小 response evidence

[短事务 apply]
  重新读取 owner 事实；校验 attempt / fingerprint / fence；绑定 receipt 和证据
  提交 SUBMITTED / ACCEPTED / RESULT_SAVED 等本地事实

[短事务 settle]
  仅在权威 terminal result 或验签回调成立后，原子提交终态、资金方向、审计、outbox 和 APPLIED checkpoint
```

Spring 实现必须额外满足：

- 关闭 `spring.jpa.open-in-view`；编排层不持有全流程事务，短事务放在经 Spring proxy 调用的独立 bean 或 `TransactionTemplate` 中。
- 不依赖同一 Bean 的 self-invocation 或 private `@Transactional`；checked exception 的回滚策略显式声明。
- `REQUIRES_NEW` 只用于有书面证据必须独立提交的纯数据库事实；它会占用第二条连接，不能用于外部 I/O、等待、成功业务状态、receipt 或动账拆分。
- `SELECT ... FOR UPDATE SKIP LOCKED`、CAS 或 `@Version` 的锁序、等待上限、持锁时间和受影响行数都要有真实 PostgreSQL 测试；CAS/native SQL 后必须 reload，禁止把 stale managed entity 再 `save()`。
- 必须可靠交付的后续动作在 prepare/apply/settle 事务内写 transactional outbox；after-commit listener、Spring Event 和 SSE 只负责唤醒或投影。

后台角色分为首次执行者和恢复者：首次执行者在 durable intent、claim、fence 和 operation policy 通过后才可 create；恢复者默认只能 query、reconcile、推进已证明结果或转人工，只有明确登记且已验证的同 key replay policy 才能按原 key 有界恢复。scheduler 只唤醒和认领，不在调度线程执行长任务；reconciler 负责进程崩溃、事件丢失、重复投递、租约过期和 `UNKNOWN` 收敛。

Provider callback 先验签、校验时间窗/nonce/replay，再按已绑定的 Provider receipt 找 `generation` owner；未知关联只能进入有界对账或唤醒。支付 webhook 按签名验证过的 provider event ID/order ID 找 `payments` owner，再向 `billing` 提交已验证的结算意图，不能用前端 return URL 作为权威。Agent bridge 使用 bearer token 与 run 绑定校验，是 Node runtime 的内部协议，不走公共 webhook 的 receipt 关联路径。无法验签或无法证明绑定的外部内容最多作为有界、可去重的 wake-up hint；未经验证的正文不得改状态、资金、租户或结果地址。回调路由与用户结果读取路由分离，未知关联统一响应且不形成 task-existence oracle。

## 8. Identity, tenancy, authorization and platform access

### 8.1 API 兼容约束

当前 Web 通过 http 调用后端，业务错误依赖 HTTP status 以及 code/reason，成功响应也有统一结构。Java 第一阶段应实现一个集中 ApiEnvelope 和错误映射器：

- 成功与失败都保持前端已依赖的字段。
- 保持错误 status 与业务 code 一致表达失败；HTTP 200 不能包装权限、校验或服务端错误。
- 机器分支依据 reason/稳定 code，不解析人类文案。
- 内部异常只写脱敏日志；不能把 Provider 响应体、密钥或 SQL 错误原文返回给用户。
- requestId、traceId 放在日志和指定 response header，不让任务 ID 代替链路 ID。

先从 [HTTP API 合同](../../docs/content/docs/backend/http-api.mdx) 和 [嵌入 OpenAPI](../../backend/internal/handler/openapi.yaml)生成 endpoint matrix，再逐项补目前规范没有覆盖的真实行为。

Java 新增 API 使用显式 DTO、唯一 `operationId`、OpenAPI schema、权限、幂等和 wire test，不暴露 Entity 或动态 Map。长操作的 Java 原生新接口优先返回 `202 + status resource`；为保持现有 Web UI 兼容的路径可以暂时保留当前响应信封和状态码，但必须把差异写入 operation catalog，不得让“兼容”变成永久的隐式合同。若未来要引入 RFC 9457 Problem Details，应通过版本化 API 或前端改造切换，不能在同一 endpoint 混用两种错误格式。

[黑盒套件的 `v1-observed` 响应 schema](../../tests/canvas-contract/README.md#versioned-response-shapes) 已绑定 66 条 Required 路由，并在原始 HTTP 响应解包前校验；这是协议观测，不是 Java Entity 或 Spring 模块生成模板，也不是已批准合同。实测发现 Go 项目画布分页漏选 revision，返回 0（D-11），而文档 revision 为正；Java 应采用经过裁决的显式 DTO，不复制该查询缺陷或用分页值做 CAS。单元创建/导入重复 POST 会产生新 ID（D-12），自动重试前必须先明确稳定请求身份。二进制/304/416 与 JSON 的 wire 证据分开统计；请求 schema、业务不变量和行为审批仍需独立门禁，不能由响应 schema 绿色代替。

### 8.2 用户身份、会话和对象归属

先审计当前 AuthSession、Cookie 属性、密码摘要、OAuth state、注册/找回流程和认证 middleware，再确定 Spring Security 的 session 或 token 实现。不要只因为 Java 有 JWT starter 就把有状态登录改成无状态 token；这会影响退出登录、封禁、会话回收和前端凭据发送。

当前源码可见的 session Cookie 使用 `Path=/`、`HttpOnly`、`SameSite=Lax`，`Secure` 由 TLS 或 `X-Forwarded-Proto` 决定；CORS middleware 对非空 `Origin` 做策略校验，并允许凭据请求（见 [Cookie 设置](../../backend/internal/handler/auth.go) 和 [CORS middleware](../../backend/cmd/server/main.go)）。静态搜索未发现独立 CSRF token 机制。尤其不要把当前对 `X-Forwarded-Proto` 的处理直接当成 Java 合同：Java 版必须明确可信代理边界，不能接受任意客户端伪造的 forwarded header。Cookie 会话下的跨站请求威胁模型也要写成明确合同，并为有副作用的请求选定 CSRF token 或严格的 Origin/Referer 校验等防护；不能仅凭 CORS 策略或 SameSite 属性就假定所有部署形态都已覆盖。合同测试应区分同源浏览器请求、非允许 Origin、缺失 Origin 的非浏览器客户端，以及反向代理转发协议的可信边界。

每个读写用例使用解析后的 Actor，Repository 查询尽量直接带 user_id/tenant scope：

- 不从客户端请求体接受可信 userId。
- 所有 get/update/delete 在服务端校验对象归属和状态。
- 管理员权限在应用用例再次检查，不能依赖前端隐藏入口。
- 子线程、@Async 和 worker 显式携带最小必要的 actor/tenant/job context；不要靠 ThreadLocal 穿越异步边界。
- 管理员批量操作写可查询审计事实，并对审计失败策略分级；不能把“日志已配置”当成必达审计。

### 8.3 错误与平台安全

把输入错误、鉴权失败、归属冲突、配额不足、Provider 可重试失败、Provider 结果未知、永久失败和内部错误分开建类型。HTTP 映射集中配置；不要在各 Controller 复制 status/code 判断。

出站请求在 Java 版统一经过 host policy：阻止回环、私网、链路本地、云 metadata 地址和 DNS 解析到禁用 IP 的目标；校验重定向后的最终地址；限制 scheme、端口、body 大小和耗时。自定义渠道应校验 method、path、query 和 content type 白名单，避免将 API key 通过 URL 传递。

### 8.4 Tenant / owner scope 的决策边界

当前迁移不能把 `user_id` 机械改名为 `tenant_id`。在 tenant capability 仍为 `deferred` 时：

- 认证主体、Actor、用户、项目/画布 ACL 和管理员 operator 分开建模；每个对象读写仍必须带已认证 owner scope。
- 异步任务持久化最小必要的 owner scope，并由唯一 dispatcher 恢复；禁止从线程池、URL、回调正文或客户端 header 推断 scope。
- 如果 M0 证明产品需要多租户，目标默认为 shared schema + `tenant_id` + 应用授权 + PostgreSQL RLS 纵深防御；RLS 不替代资源 ACL。
- 启用 RLS 前必须确定应用角色没有 `BYPASSRLS`、表 `ENABLE/FORCE ROW LEVEL SECURITY`、租户变量使用事务级 `SET LOCAL`/`set_config(..., true)`，并在隔离 PostgreSQL 中验证连接池借还和 native SQL。
- 在 tenant/RLS 决策明确前，安全默认是 fail-closed 的 user/owner scope，而不是允许任意 tenant 或把管理员查询暴露给普通用户。

## 9. 持久化、Provider 与计费合同

### 9.1 先选择“共用现有库”还是“转换到新 schema”

两条路线不能混成一个模糊方案：

| 路线 | 适用情况 | 主要工作 |
| --- | --- | --- |
| 保留现有表结构，切换写服务 | 需要快速替换后端、用户数据量大、前后端合同不变 | 逐表理解 GORM tags/自定义 SQL，创建 Flyway baseline，验证 Java 映射后单写入切换 |
| 新建 Java schema 并转换数据 | 可以接受明确停机/导入窗口，或计划修正模型边界 | 定义目标模型与版本化 ETL，迁移 ID/关联/JSON/密钥/资源引用，进行数据校验与回滚演练 |

不要让 GORM 和 Hibernate 同时对同一 schema 执行自动 DDL。Java 生产环境使用 Flyway append-only 迁移，关闭 `ddl-auto=update/create`。当前 checkout 的 `CurrentSchemaVersion` 是 47（见 [`migrations.go`](../../backend/internal/database/migrations.go)）；这只表示代码支持的迁移水位，不代表生产库水位，也不能简单把 `schema_migrations=47` 填成 Flyway 已成功版本。必须对真实数据库结构和现有迁移记录做盘点，再选择经过验证的 baseline。

### 9.2 建立逐表和逐字段映射

至少从 [模型清单](../../backend/internal/database/schema.go)、[迁移历史](../../backend/internal/database/migrations.go)、[数据模型导航](data-model-overview-and-navigation.md)补出以下清单：

- 表名、主键、唯一键、外键、索引、删除策略和 ownership scope。
- Go 零值、空串、SQL NULL、JSON null 与“字段缺省”的差异。
- JSON 列的版本、写入方、读取方、最大体积和查询需求。
- 金额/额度单位；以明确约定的最小计价单位整数（如 microcredits）或 decimal 表示，禁止浮点金额。
- 时间列精度、UTC 约定、轮询时间和过期时间。
- 密文、签名值、密码摘要、OAuth state 和对象存储凭证的密钥来源与轮换策略。
- 预置数据、用户配置与代码内默认值的覆盖顺序。
- 迁移脚本是否可重跑，是否需要锁、备份、断点或分批提交。

数据库 Entity 只映射持久化事实。JSON API 和异步任务 snapshot 使用独立 DTO/record；历史 payload 需要显式 version 与升级策略，不能靠当前 Java 类字段推测旧数据语义。Jackson 对 JSON 数字精度、未知字段和 null 的策略应有测试。

### 9.3 数据迁移演练

在数据副本上至少完成一次全量导入和一次增量/冻结切换演练：

1. 记录 Go 服务 schema 版本、行数和关键聚合统计。
2. 备份 PostgreSQL、对象目录/桶元数据、应用加密密钥和 Agent 持久化数据。
3. 将源库复制到隔离环境，不在生产库上第一次试跑迁移。
4. 按表依赖顺序迁移；保留稳定 ID，禁止重建 task、asset、billing order 和 canvas ID。
5. 对 JSON 做 parse-and-validate，不能静默丢弃未知字段；记录隔离行和修复原因。
6. 校验每用户任务/资源/画布计数、账本余额、任务关联、对象 key 可达性、重要 checksum 和 orphan 行。
7. 做只读双读对比；任何差异要归因到映射规则，不能用“整体数量差不多”通过。
8. 冻结写入或使用可靠 CDC/事件日志追平增量；确定唯一 writer 时间点。
9. 切换后按预先设计的回滚窗口恢复路由；若 Java 已产生新格式写入，Go 是否能读回必须提前证明。

严禁“Go 写一次、Java 再写一次”作为临时双写方案，除非两边操作有可证明的幂等键、失败补偿和一致性监控。两库双写不存在天然原子性。

## 10. Domain model and key workflows

### 10.1 身份、模型目录与报价

把公开模型身份、渠道、Provider 上游模型、能力和价格分成不同类型：

- PublicModelId：Web/API 选择与展示使用。
- ChannelId / ChannelModelId：当前部署允许的渠道和模型实例。
- ProviderModelKey：上游协议使用的模型标识。
- CapabilityProfile：该模型在图像、视频、音频、文本等操作的能力约束。
- PriceSnapshot：需要计费时，任务创建时使用的计费版本和参数。
- RoutePolicySnapshot：任务准入时可用的候选路由、选择顺序、回退条件和计费/数据驻留约束。
- RouteAttempt：一次具体 Provider 提交尝试所使用的渠道、Provider、上游模型和结果；追加新尝试不能覆盖既有尝试。

对需要计费的高成本请求，在任务创建时显式解析能力与报价并保存适用的价格快照；所有会调用 Provider 的任务都应记录准入时的路由策略和每次实际 route attempt。默认 Provider 回退不能悄悄改变授权、价格或数据驻留语义。新增 Provider 时通过 contract test 验证 create/query/cancel、轮询状态、错误分类、回执和媒体下载。

### 10.2 视频生成：提交、等待和用户感知超时

视频长任务等待是本迁移评估的高优先级风险。当前文档没有重现线上等待问题，也没有据此推断某个 Go 文件是根因；Java 版应把它作为验收约束提前设计，而不是把“延长 timeout”当成唯一修复。

至少分开以下时间概念：

| 时间 | 含义 | 超时后的处理 |
| --- | --- | --- |
| 客户端到服务端的任务提交超时 | 客户端没有收到服务端响应；服务端可能已创建任务并继续执行 | 以同一个服务端幂等身份查询或重试，重复提交必须返回同一 task；缺少幂等合同且无法确认提交结果时先对账，不要直接创建第二个 task |
| 服务端到 Provider 的 create 超时 | 上游可能已接受请求，但服务端没有收到可持久化的 receipt | 将本地任务记为提交结果未知；若已有 Provider task ID 可查询同一任务；只有 Provider 书面合同和 wire test 同时证明稳定 client idempotency key 的 query/replay 安全时，才可按原 key 有界恢复；否则进入人工对账。仅有本地 Task ID 时不推断上游状态，也不自动创建新 attempt |
| Provider 单次查询超时 | 轮询一次上游任务允许等待多久 | 保留原 Provider task identity，安排下一次查询；不要结束整体生成任务 |
| 任务执行 deadline | 本地 worker 持有执行上下文的最长阶段时间 | 保存可恢复状态。已有 Provider task ID 时释放或允许 worker lease 到期，由后续 worker 查询原任务；提交结果未知时保持 UNKNOWN，走按 receipt 查询，或按已验证的稳定 key 合同有界恢复，否则人工对账，不自动创建新 attempt |
| 用户可等待/产品 deadline | 产品展示“仍在生成”多久后需要介入 | 提示可继续查看/人工排查；不等价于上游失败或自动重提 |
| Provider 模型最大处理时间 | 厂商/模型合同允许的异步处理窗口 | 按 Provider 能力、历史分位数和配置单独定义 |

建议实现：

- 对每个 Provider/能力配置提交超时、单次轮询超时、轮询退避、最大观察期限和可恢复策略；避免一个全局短 timeout 覆盖所有视频模型。
- 在配置的最大观察期内，Provider 返回“处理中”或遇到暂时查询失败时，task 保持非终态并安排回查；超过观察期后转入可恢复/人工处理状态，而不是直接判定上游失败或自动重新 create。网络超时与 Provider 明确失败使用不同 error code / retry class。
- 任务保存 nextPollAt、原始 providerRequestId、submittedAt、最近查询错误、累计查询次数与模型/路由快照。
- worker 运行时 lease 可以短于 Provider 任务寿命；worker 退出后由另一个实例恢复轮询同一 Provider task。
- create 请求可能已送达而响应丢失时进入 UNKNOWN；只有 Provider 书面合同同时允许按稳定 client idempotency key 查询/重放，并由 wire test 证明它会定位同一远端操作而不会产生第二个副作用，才允许按原 key 自动找回；幂等 key 不能单独证明首次请求未被受理，也不能把“幂等重放”当成默认的第二次 create。否则进入人工对账，不创建新 attempt 或切换 route。
- 单任务历史总耗时、每次 Provider 响应时间和轮询失败率进入指标与可诊断日志，安全脱敏后用于按模型调参。
- 页面状态区分“排队”“已提交，等待上游”“上游处理中”“正在保存结果”“需要恢复/人工检查”和“最终失败”。

当前测试 [`runtime_policy_test.go`](../../backend/internal/app/runtime_policy_test.go) 分别覆盖 `TestDefaultVideoTaskTimeoutIsOneHour`、`TestVideoTaskTimeoutHasFiveMinuteSafetyFloor` 和 `TestProviderPollingDeadlineDefaultsToOneHour`，表明默认视频执行期限和 Provider polling deadline 都按约一小时测试，并对视频执行期限设有五分钟下限。它们不证明这些期限适合所有真实模型，也不验证上游 SLA；Java 验收要加入“生成耗时超过普通 HTTP timeout 仍可完成”“进程重启后继续 poll”“观察期内的短暂 query timeout 不变终态”“超过观察期进入可恢复状态”“UNKNOWN 不产生未经证明安全的第二个远端副作用”的场景。

### 10.3 任务表、租约和并发 fencing

当前 Task 是持久化任务记录，worker 通过数据库 claim/lease 工作，并已有针对过期租约旧 writer 拒写的回归测试；Provider 状态和进度路径并非所有写入都统一带租约条件。Java 版应把任务事实保留在数据库，不要改成只存在 JVM 内存的 `@Async` 工作。入口见 [`models_task.go`](../../backend/internal/model/models_task.go) 和 [`task_lease_fencing_test.go`](../../backend/internal/repository/task_lease_fencing_test.go)。

建议任务字段按职责分组：

- 身份：业务 task ID、用户/项目 scope、幂等键、任务类型。
- 输入冻结：经服务端校验和脱敏的 generation snapshot、schema version。
- 执行选择：逻辑模型、route/model snapshot、Provider request ID、attempt number。
- 计费：授权订单/额度引用、预估和最终结算证据。
- 调度：状态、阶段、nextPollAt、attempt、lease token/owner/expiry、重试分类。
- 结果：物化资源 ID、输出快照、safe error、execution diagnostic。
- 生命周期：创建、开始、取消请求、终态和最后更新时间。

Java 目标在领取 worker 时通过单条条件更新或事务内锁竞争保证一个当前 owner；每次会改变任务状态或结果的写入都携带 lease token/version，状态变化和 token/version 不匹配时拒绝旧 worker。仅保存 leaseOwner 字符串而不 fencing 旧执行者，会让过期 worker 在新 worker 之后覆盖结果。Provider 观测性进度若不参与状态转移，也必须明确其乱序和陈旧写入策略，不能把它默认为任务状态已 fencing。

SELECT FOR UPDATE SKIP LOCKED 适合 PostgreSQL worker claim，但对 SQLite 和其它数据库并不自动等价。选定生产数据库后，用真实目标数据库测试并发竞争、lease 到期和重启恢复。Redis slot 用于限制全局容量，不取代数据库 task 状态和 fencing。

### 10.4 事务和外部副作用

每个生成用例采用短阶段：

1. 短事务 A：校验 actor、项目、model 和 quota；冻结 route 与适用的 price snapshot；建立 task、适用的 billing authorization 和必要幂等记录；提交。
2. 事务外：获取并发槽位、调用 Provider create、轮询、下载、转码和对象存储。
3. 短事务 B：记录 Provider receipt 或 UNKNOWN；只 apply 远端受理/结果证据和资源物化事实，不在未证明 terminal result 时动账；提交。
4. 终态短事务：在权威 terminal result 成立后 settle 资金方向、task 终态、审计、outbox 和 checkpoint；失败则留下可重放的 settlement key。
5. Reconciler：找回过期 lease、待轮询任务、未知提交和未完成结算；恢复者默认不得自动重发原始 create，已登记并验证的同 key replay 例外必须由对应 adapter policy 控制。

Spring 的 @Transactional 适合清晰的短数据库事务。不要让它跨越 Provider HTTP 或大文件操作；不要把 Provider 调用放入 @TransactionalEventListener 后就当作持久工作队列。AFTER_COMMIT / Spring Event 用来唤醒 worker 即可，数据库任务表或适用的 Outbox 仍需保存事实。

若一个用例需要“业务状态和一条要可靠交付的事件同生共死”，在同一事务写 Outbox；consumer 使用幂等键、claim/lease 和重放策略。不是所有低延迟通知都需要 outbox：可丢失的唤醒由 reconciler 兜底，但任务状态、资金、资源删除、审计和必须送达的事实变更不能只靠 Spring Event 或内存通知。

### 10.5 计费与支付

计费与生成结果不是两个互不相关的状态字段。保持这些不变量：

- 生成 task 要绑定已解析的模型和路由；需要计费时，还要冻结对应价格快照。
- 对需要计费的用例，额度授权、账本记录和 task 创建有可重放的业务幂等身份；不计费任务不能被强行套入账本流程。
- Provider create 前失败、create 后 UNKNOWN、上游明确失败、媒体已保存但本地结算失败，必须走不同的终态/待处理路径。
- 结算或退款操作通过数据库唯一键/状态 CAS 保证重试不会重复记账。
- 金额用明确约定的最小计价单位整数或严格 decimal；模型 token、时长、分辨率等量纲要写入计算合同。
- 支付 webhook 要验签、去重、校验订单状态，回调完成不能只靠前端 return URL。
- 后台手工调账和人工恢复应有管理员身份、原因、关联 task/order、审计记录。
- 资金事实优先使用带唯一幂等键的 append-only ledger entry；余额和 usage 是可重算 projection。ledger、settlement evidence 和人工 resolution 默认禁止 UPDATE/DELETE，差错通过反向动账或新的 resolution 修正。
- `quote`/报价不等于授权；只有显式 prepare 事务冻结的价格快照和资金 ambiguity policy 才能放行高成本 create。

不要在 Java 版中用一个 success 字段覆盖 task、Provider、资源物化和账务的独立事实。相关对账要有可操作的查询与恢复界面。

### 10.6 素材、资源和对象存储

保留并强化以下区分：

- ResourceId：后端 `Resource` 记录身份。
- `Resource.ObjectKey`：对象存储中的物理定位符；不要与资源记录 ID 混用。
- 画布 `storageKey`：文档和浏览器缓存使用的资源引用键，当前远端资源形如 `resource:<ResourceId>`；它不是对象存储的物理 key。
- AssetId：素材库业务身份，可关联资源版本和不同项目引用。
- 访问 URL：带权限/时效的交付形式，可重建，不是字节存在的证据。
- TaskId：生成任务身份，关联结果和账务，不等于资源身份。
- 画布节点中的 preview 字段：展示投影，不是可靠物理存储凭据。

写入路径建议使用上传 reservation/配额预留、流式大小限制、content type 检查、checksum 和幂等 finalize；不要把大媒体放入 JPA Entity 或普通 JSON request。

删除策略必须把“业务记录删除”和“物理对象删除”分开决定：

- **Java 默认合同**：删除前检查项目、画布、任务、素材版本、公告/外观等业务引用及对象共享情况；有引用时保留记录并返回来源。无引用时可以创建持久化删除作业，但在物理删除成功前保留可查询的 Asset/Resource 记录；物理删除失败时保留记录并继续重试。
- **当前 Go 的已知例外**：显式 `purge=true` 允许即使仍有引用也删除素材记录；回收站到期自动清理也会通过 `DeleteUserAsset()` 进入 archived 素材的引用豁免分支。`TestExpiredArchivedAssetCleanupDeletesReferencedAssetAndUsesDeletionOutbox` 明确覆盖了画布仍引用时删除 Asset/Resource 并创建删除作业的行为。
- **M0 必须决策**：不能仅凭当前 Go 路径把上述例外当成 Java 兼容合同。迁移默认遵守仓库 [素材删除规范](../../AGENTS.md)；若要保留现状，必须明确批准例外、补充权限和审计要求，并更新规范与验收矩阵。

当前实现的证据是：[素材删除实现](../../backend/internal/app/resource_delete.go)、[自动过期清理](../../backend/internal/app/resource_deletion_worker.go)、[删除事务](../../backend/internal/repository/resource_reference.go)、[删除作业重试](../../backend/internal/repository/resource_deletion.go)、[画布保存入口](../../backend/internal/handler/user_data.go)和[删除回归测试](../../backend/internal/app/resource_delete_test.go)。其中 Asset/Resource 删除与 `ResourceDeletionJob` 在同一数据库事务提交，随后异步删除物理对象；对象存储失败时删除作业可重试，但在上述 archived/purge 例外路径中业务记录已经删除。这正是需要在 M0 明确“兼容现状”还是“遵守规范”的地方。

对对象存储测试分别覆盖：上传成功、DB commit 失败、对象写入失败、访问授权过期、同一物理对象多引用、删除重试和资源恢复。

### 10.7 项目与画布

项目/剧本/分镜/镜头和无限画布是相关但不同的聚合：

- 先通过 [数据模型综述](data-model-overview-and-navigation.md)列出当前表与 JSON 文档字段。
- 画布内容如继续使用 JSON 文档，定义 document schema version、最大体积、字段 owner、校验器和升级器。
- 画布 revision、ETag、内容哈希、客户端 journal、任务 ID 与媒体资源 ID 各有独立语义；不要共用一个 version 字段。
- 节点生成结果只 patch 生成拥有的字段，不能用任务开始前保存的整份旧文档覆盖用户等待期间的移动、尺寸或标题更改。
- 历史快照如果引用媒体，定义资源保留策略；回滚后的素材引用要重新校验。
- 多人协作若属于目标能力，先选操作日志/版本冲突/OT/CRDT 方向；不能把单用户 CAS 误说成实时协作。

画布细节见[画布系统深度调研报告](canvas-deep-research.md)。

### 10.8 Agent 与 Node runtime

Java 控制面要分清 Agent 的持久化会话、审批、工具授权、任务计费和 Node runtime 执行进程。当前 Go runtime 通过 bearer token 和 JSON bridge 提供 `/model`、`/tool`、`/event` 回调；这些是 Node runtime 的内部 bridge endpoint，不是面向浏览器的公共 `/api` 路由。它同时支持内嵌 Node 与远程 `yingce-agent` 两种路径，bridge 实现见 [`runtime.go`](../../backend/internal/agent/runtime/runtime.go)。

移植要求：

- 将 Go 的 /model、/tool、/event bridge request/response 和认证方式列为内部协议，建立版本字段、超时和 body 上限。
- 工具授权由 Java 服务端逐次校验；类型定义、registry 中存在和当前 run 实例可调用是不同事实。
- run/session/event/checkpoint 使用稳定身份，审批恢复不重复执行已完成的高成本 step。
- 远程 Agent 配置生效时只走远程执行；远程请求失败不要悄悄启动内嵌执行造成重复副作用。
- 同一 run 的工具调用限流、取消、线程/进程关闭和客户端断线都要有确定语义。
- 保持 secret 与工作目录隔离；传给 Node process 的环境变量使用白名单。

当前 Agent 的能力、前端配合和未验收边界见[云端 Agent 深度调研](cloud-agent-deep-research.md)。

## 11. Failure matrix、scale verdict 与 replacement tests

每个高风险 operation 的设计评审至少覆盖以下失败矩阵；表中的“禁止动作”比“自动重试”更重要。

| 场景 | 本地事实 | 远端/资金事实 | 自动动作 | 人工动作 | 明确禁止 |
| --- | --- | --- | --- | --- | --- |
| prepare 前失败 | 无 durable intent | 未证明发出 | 返回可重试错误，不创建 attempt | 无 | 不能扣费或调用 Provider |
| prepare 已提交、create 前进程崩溃 | `PREPARED` + fence | 未知是否发出 | 普通进程日志不能证明请求未发出；默认转 `UNKNOWN`，仅由已登记的 adapter 协议决定是否可恢复 | 必要时对账 | 不能绕过原 fence 新建 attempt |
| create 写出后超时/5xx/连接重置 | `SUBMITTING`/`UNKNOWN` | 可能已受理 | 只按 receipt 查询；仅在 Provider 书面合同和 wire test 证明同一稳定幂等 key 的 query/replay 安全时按原 key 有界恢复，否则进入人工 | 对账、resolution | 不能凭 HTTP 状态自动重发、创建新 attempt 或退款 |
| Provider 明确处理中 | `SUBMITTED`/`PROCESSING` | 非终态 | 按 `nextPollAt` 回查 | 超过观察窗后介入 | 不能标记终态或创建第二个 task |
| Provider 权威成功但媒体写入失败 | `RESULT_VERIFIED`，物化未完成 | 远端成功 | 重试物化/对象 finalize | 处理孤儿和补偿 | 不能重复扣费或重新 create |
| 本地 settle 提交失败 | 结果证据已保存 | 结算未应用 | 通过 settlement key 重放短事务 | 对账 resolution | 不能依赖前端再次支付/再次生成 |
| 回调伪造、重放、乱序或未知 ID | 原状态不变 | 不可信 | 记录安全事件/有界唤醒 | 检查来源与密钥 | 不能改 task、资金、租户或结果地址 |
| lease 过期、旧 worker 回写 | 新 owner/fence 已存在 | 远端任务继续 | 条件更新拒绝旧写入 | 观察最老 lease | 不能让 stale entity 覆盖新状态 |
| 租户/owner context 缺失 | scope 不可证明 | 不适用 | fail-closed | operator 排查 | 不能默认用户、tenant 或权限 |
| 备份/恢复失败 | 事实源不可确认 | 外部状态仍可能存在 | 停止切换并保护写入 | 恢复演练和人工裁决 | 不能继续灰度扩大 |

### 11.1 Scale verdict

当前推荐保持一个可部署的模块化单体，API、worker、scheduler/reconciler 可以是同一构建产物的不同运行角色。没有以下证据前，不引入 Broker、workflow engine 或微服务：

- PostgreSQL 状态机、outbox 和 reconciler 在代表性数据规模下已出现无法接受的锁等待、积压或恢复窗口。
- 通过隔离测试证明故障域、吞吐、合规租户隔离或组织发布边界必须拆分。
- 有数据/合同迁移、replacement test、观测触发器和回滚成本评估。

必须持续观测：各状态数量与年龄、最老 `UNKNOWN`、outbox 积压、reconciler 延迟、lease 过期、Provider 并发、连接池等待、对象删除作业年龄、每用户/未来每租户队列年龄、备份恢复 RPO/RTO。没有基线前只写趋势和实验，不写未经测量的吞吐数字。

### 11.2 Replacement tests

每个高成本基础设施决定都要能回答“替换它会改哪些模块”：

- **Provider adapter**：替换 Provider 只影响 adapter、operation profile、contract test 和 route policy，不应改 Task、Billing 或 Canvas owner。
- **Redis**：替换或移除 Redis 只影响协调 adapter；任务事实、receipt、账本和恢复仍由 PostgreSQL 接管。
- **Object storage**：替换 S3/OSS 只影响 managed object adapter；ResourceId/object key/引用合同不变。
- **Agent runtime**：替换内嵌或远程 runtime 只影响 `AgentExecutionPort` 和 bridge contract；审批、checkpoint 和账务不随进程内存消失。
- **Outbox publisher**：替换发布器不会改变 outbox 行或 owner transition；重复投递必须由 consumer 幂等处理。

替换测试若溢出 adapter，说明抽象泄漏或 owner 边界还没有稳定。

## 12. Key decisions、deferred decisions 与存量资产分类

### 12.1 初步 ADR

| ADR | 结论 | 依据与取舍 | 失效触发器 |
| --- | --- | --- | --- |
| AD-001 | 目标先采用 Spring MVC + JPA/Hibernate + PostgreSQL 模块化单体 | 兼容现有部署并降低分布式故障面；用 ArchUnit/Spring Modulith 强制边界 | 真实工作负载或合规要求证明必须拆分 |
| AD-002 | 高成本 create 采用 fence + receipt + `UNKNOWN`，恢复者默认不自动重提 | 防止 Provider 重复生成、重复扣费和不可逆副作用；只有 Provider 书面承诺稳定幂等 key 的 query/replay 语义并经 wire test 证明同一远端操作时，才可按原 key 有界恢复，这不是普通重试 | Provider 合同或 wire test 证明可启用受控同 key replay 例外，或真实故障证据要求调整 ambiguity policy |
| AD-003 | JPA 管聚合，CAS/claim/fencing/native bulk 用 SQL；`open-in-view=false` | 兼顾领域读写与并发事实安全 | 真实 PostgreSQL 测试发现目标路径不能满足性能/一致性 |
| AD-004 | 可靠后续动作写 transactional outbox；after-commit/SSE 仅唤醒 | 进程崩溃不能丢任务；不把事件系统升格为事实源 | outbox/reconciler 在基线负载下出现可证据化瓶颈 |
| AD-005 | Agent Node/yingce-agent 保持外部运行边界，Java 只迁移控制面 | 降低一次性重写范围，保留远程失败不 fallback 的安全合同 | 明确产品决策、运行时替换实验和回滚方案 |

### 12.2 Deferred decisions

| 决策 | 缺什么证据 | 最迟决策点 | 答案前安全默认 |
| --- | --- | --- | --- |
| 是否把 user scope 升级为多租户 | 产品组织模型、数据隔离/合规要求、真实租户边界 | M0 完成表与权限盘点前 | 只按已验证 user/owner scope，fail-closed |
| 是否启用 PostgreSQL RLS | tenant 决策、角色/连接池设计、隔离测试 | 第一条跨租户写路径前 | 不伪造 tenant；应用授权和对象归属必测 |
| 是否保留 SQLite | 本地用户、部署矩阵、锁/JSON 兼容成本 | M1 Flyway/集成测试前 | Java 只承诺 PostgreSQL |
| 是否引入 Broker/workflow engine | outbox 积压、锁等待、故障域和恢复窗口证据 | M4 主链灰度前 | PostgreSQL task + outbox + reconciler |
| 是否保留 archived/purge 强制删除例外 | 产品批准、审计、恢复和断链 UX | M3 资源写路径切换前 | 遵守有引用保留和物理删除失败保留记录的规范 |

### 12.3 Existing-asset classification

当前代码位置不是目标模块边界；存量资产按以下五类处理：`KEEP`、`CHANGE`、`OVERTURN`、`HISTORICAL-ONLY`、`INSUFFICIENT`。

| 资产/行为 | 分类 | 目标去向 | 证据/特征测试 |
| --- | --- | --- | --- |
| Web API 响应信封、Cookie、SSE 和主要路径 | `KEEP`（兼容窗口） | Java adapter + DTO + contract tests | HTTP API、OpenAPI、前端 `http` |
| Go package / `Service` 组织方式 | `HISTORICAL-ONLY` | 只作为定位线索，不复制为 Java Service | 本文架构原则 |
| Task、Provider receipt、lease/fencing 行为 | `CHANGE` | `tasks/generation` owner + operation catalog | task model/worker/lease tests |
| 当前可变余额/额度写路径 | `CHANGE` | billing authorization + append-only ledger + settlement | 计费路径与对账测试 |
| archived/purge 强制删除路径 | `OVERTURN`，除非明确批准 | 遵守引用检查；例外单独授权、审计和恢复 | resource delete test 与 AGENTS 规范 |
| GORM schema/migrations | `HISTORICAL-ONLY` | 生成字段/约束清单，转换为 Flyway expand/contract | migrations.go、真实库盘点 |
| Redis 协调 | `KEEP`（adapter） | 仅协调/限流/唤醒，不存 durable fact | coordination.go、部署配置 |
| Pi/Node/yingce-agent bridge | `KEEP`（外部边界） | Java Agent port + bridge contract | runtime.go、Agent E2E |
| SQLite 兼容 | `INSUFFICIENT` | M0/M1 决定；默认不作为 Java 生产合同 | 当前本地能力 vs 生产 PostgreSQL |

### 12.4 In-flight state mapping

切换前必须为旧系统每种在途状态指定“排空”或“接管”方式；没有 receipt、fence 或资金证据的记录不能靠当前默认值补全。左列只列 Go 当前可观察的状态或事实，右列是 Java 目标状态；`PREPARED`、`UNKNOWN`、`TERMINAL_VERIFIED` 和 `APPLIED` 是迁移目标语义，不表示 Go 当前已经有同名状态。

| Go 当前状态/事实 | Java 映射 | 接管策略 | 资金/幂等处理 | 无法映射时 |
| --- | --- | --- | --- | --- |
| `queued`，尚未 claim | 先保留为 Java `QUEUED`；只有 admission、scope 和适用资金事实完整时才转 `PREPARED` | Java 以唯一 owner claim | 保留原 task ID；若存在可验证的 public key 则沿用，否则标记幂等身份缺失，不新建重试 task | 保留队列，operator 处理 |
| `running`，已有 `ProviderRequestID` | `SUBMITTED` / `PROCESSING` | Java 复用源记录中的 receipt 查询 | 源记录存在 route/price/authorization snapshot 才迁移；缺失时进入对账，不重新 create | `UNKNOWN` + 人工 |
| `running`，没有 receipt 或提交结果不明 | `UNKNOWN` | 只做可证明的 query/reconcile | 不把 Go 的 task lease/owner 当作 Provider fence；不补造 receipt；有资金证据才按原引用接管，否则进入 resolution | 停止自动 create，进入 resolution |
| `succeeded` | 经证据校验后进入 `TERMINAL_VERIFIED`，再按物化/结算结果进入 `APPLIED` | 校验结果资源、账务和引用后接管读 | 不重复结算；缺证据的成功不直接补账 | 对账后人工确认 |
| `failed` / `cancelled` | 对应终态或 `RECONCILE` | 读取 failure facet、取消证据和资金状态 | 退款/释放只走 settlement key | 保留原记录，禁止猜测 |
| 资源 archived/purge 例外 | `MEDIA_REVIEW_REQUIRED` | 先做引用扫描和物理对象核对 | 不因删除作业成功就假定业务记录可安全删除 | M0 决策后再切换 |

### 12.5 单写者切换表

| 步骤 | 切换范围 | 单写者切换点 | 退出标准 | 回滚动作 | 兼容窗口 / forward-only |
| --- | --- | --- | --- | --- | --- |
| S-1 | 无副作用读 | 路由开关 | 字段级差异可归因，错误/延迟在预算内 | 切回 Go 读 | 否 |
| S-2 | 单个幂等低风险写 | owner marker + 旧 writer 停止 | 重放、权限、审计和对账通过 | 回切仍能读新数据的版本 | 若 Go 不读新字段，标 forward-only |
| S-3 | generation/媒体/账务 operation | 每个 operation 独立 fence/owner marker | UNKNOWN、重启、重复回调、settle 对账通过 | 只回滚路由，不回滚已证明外部事实 | 旧幂等键保留到清理窗口 |
| S-4 | 全部高权限与 Agent | 控制面和运行时的明确 owner | 在途映射、备份恢复、人工 resolution 通过 | 停止扩大，不重新 create | 逐 operation 收口 |

Java 写出的新数据如果 Go 不能读，切换就是 `forward-only`；不得把“反向代理切回”宣传成完整回滚。回滚优先向前修复数据或继续由 Java 接管事实，只有合同证明旧版本可读时才允许退回 Go。

## 13. Phased migration, cutover and acceptance

建议按风险和依赖推进。每一阶段需有可验证交付和退出条件。

### 13.1 M0：决策、合同与数据盘点

交付：

- 本文的 capability profile、证据账本、目标架构原则和架构决策记录；每个 `deferred` 项都有安全默认、实验、触发器和最迟决策点。
- 当前端点、OpenAPI、Cookie、SSE、上传、资源访问和错误语义清单。
- 业务域/表/索引/JSON payload/对象 key/密钥映射表。
- durable fact / source of truth / owner map；每个高风险 operation 的 operation catalog、identity、ambiguity policy、资金后果和人工路径。
- 存量资产五分类、Go 在途状态到 Java 状态的映射表、单 writer 切换表和 forward-only 标记。
- 目标数据库、SQLite 兼容、部署角色、停机预算和回滚窗口决策。
- Go 现状合同/行为基线用例、代表性性能基线、Java 合同测试与真实环境验收矩阵。
- video task timeout 的业务目标和每 Provider SLA 初始参数来源。

退出条件：关键未知项（写入 owner、表/字段归属、加密字段、在途映射、高成本重试与资金后果、回滚窗口）必须已经得到证据支持并作出决策；无法在 M0 关闭的事项必须显式标为阻断并禁止相关 operation 进入 Java。其余未知项至少有负责人、截止时间、验收证据和安全默认，不能只以“有人跟进”作为 M0 通过条件。

### 13.2 M1：Java 基础壳和合同测试

交付：

- Spring Boot 模块结构、统一错误映射、request/trace ID、认证上下文、运行角色和健康探针；启用 ArchUnit/Spring Modulith 依赖规则。
- 只冻结合同、不改变 schema 的兼容层：DTO、operationId、错误、权限、幂等、SSE cursor 和 wire tests。
- 先执行[画布 Java HTTP 合同测试计划](canvas-java-http-contract-test-plan.md)中 manifest 登记的 11 个 Required profile：`core`、`share-public`、`project-integration`、`project-workbench-core`、`resource-integrity`、`import`、`legacy-assets`、`public-resource`、`restart-recovery`、`generation-boundary`、`generation-writeback`；使用独立的[`tests/canvas-contract`](../../tests/canvas-contract/README.md) runner 固定 Go 基线，再把同一套场景用于 Java 和混合路由。当前已登记 90 个 Required operation、115 个 HTTP 场景、31 项行为决策；最新完整双轮 `20261013-073513-51126` 已执行 90/115 版本，两轮均为 `14163/14162` 个清单内 observed/wire 请求，每轮 71 条映射并执行路由、72 条实际触达路由、71 条非错误路由、6 条受控故障、4 条响应丢失、4 条 incomplete 和 0 条未分类中断；真实浏览器 companion 另完成 6 个场景两轮，页面响应头分别为 170/171，阻断装饰均为 9 条。`generation-boundary` 当前已验证任务未认证、已有任务 owner 详情/日志、foreign 任务列表/详情/日志、missing-ID 和 foreign creation-run read；`generation-writeback` 已加入 provider-free text replay + SSE 游标重放，但 Provider、SSE 断线重连、Agent run fixture、任务恢复和 Agent durable writeback 仍由独立 suite 阻断。响应丢失专项覆盖文档/资源/上传完成提交后的丢 ACK、重启、CAS 和幂等重放，但不覆盖在途事务 crash；分享资源场景现已验证本地 CDN redirect 与 object-store 成功/GET 故障/恢复，但生产远端存储/CDN、TTL/条件缓存、完整 UI 缓存/冲突和 Java 切流仍未验收。Resource–Asset–Canvas 的当前 HTTP 顺序是持久化前置条件证据，不要求 Java 复制 Go 的表结构、service 别名或锁；Java 必须显式选择事实 owner、短事务与外部下载/存储的 outbox/恢复协议。工作台非核心及任务/Agent 写回按 manifest 保持 deferred/separate-suite。远端对象存储 Range/交付、生产 CDN、TTL/条件缓存不能由本地 fixture 绿灯隐式覆盖。
- 真实前端 companion 的 6 个场景已与 HTTP 基线一起双轮通过；账号切换后的 owner 文档不变性由独立重新认证的 owner audit API 会话验证。该结果仍不代表完整浏览器用户旅程、多标签并发写入、任务/Agent durable writeback、远端存储或 Java/混合网关已通过。最新版本和剩余矩阵以专项计划为准，不能因测试构造而要求 Java 复制 Go 内部进程、表结构或内存会话。
- 两类执行器及保留前端的条件见[运行边界与阅读入口](#迁移验收运行边界与阅读入口)。M1 交付必须列明目标 API/网关、身份和资源提供者、隔离 fixture；启用浏览器验收时再列明真实 Web 与同源代理。当前 browser setup 的 admin/user 合同也须满足，或先显式改造 fixture；`--implementation=java` 只是报告标签，不补齐依赖、不启动服务。连接配置、源码证据和合同变更处理以[前端配合条件](../../tests/canvas-contract/README.md#web-前端与-java-的配合条件)为准；Go 基线启动器不是 Java 验收入口。
- 关联合同登记 D-26（返回未持久化 ID）、D-27（角色超长失败部分推进）和 D-28（首次同 key 竞争的 internal 也推进 canvas revision），并扩充 D-03 的跨项目残留链接证据。旧版 114 场景已全量双轮验证角色、三种同项目竞争、跨项目争用、完整正文/历史、投影和解除/显式恢复及分享到期恢复；新增 SSE 后不把旧版结果冒充 90/115 版本证据，起跑屏障也不控制数据库交错。Java 应通过公开 owner port 编排关联，明确权威身份、冲突/重放语义和原子提交边界，不越过 Canvas owner 写表，不复制 Go 非事务顺序、生成响应 ID 或数据库报错文案。目标预算、revision 与错误合同仍须批准。
- 共存期登录态桥接：沿用现有数据库时，Java 能校验 Go 签发的 Cookie/session；改用新 schema 时，受保护 Java 路由上线前先完成显式 session 迁移，或保留经过验证的 Go 认证入口，不接受客户端自报 userId。
- 读取 OpenAPI/固定 fixture 的兼容测试。
- `spring.jpa.open-in-view=false`、事务外 I/O 检查、统一 dispatcher 和日志脱敏基线。
- api 与 worker 两种启动角色；本地开发可组合运行。

退出条件：无需前端改代码即可用现有登录态访问受保护的基础读 API；缺失、过期或无权访问的身份按合同拒绝，并通过错误信封和 request ID 合同测试。本阶段验证现有登录态的消费，不代表注册、OAuth、密码和 session 签发流程已迁移。

### 13.3 M2：前向 schema 与低副作用读取

在隔离 PostgreSQL 上建立 Flyway baseline/expand 迁移和 schema compatibility tests；不要让 GORM 与 Hibernate 并行自动 DDL。若保留 SQLite，明确其单独 profile 和测试矩阵。随后迁移公告、外观、渠道/模型目录只读、技能/工具元数据等低风险读路径。通过同一数据快照对 Go/Java 结果做字段级比较，覆盖分页、排序、null 和权限边界。

退出条件：影子读取结果差异有归因；Java 读流量可单独回切。

### 13.4 M3：身份、上传、资源与普通写路径

迁移账号生命周期、OAuth、密码和 session 签发/撤销等身份写流程前，先完成 cookie/hash 兼容与既有 session 处置策略；M1 的登录态桥接需继续支持共存期间的受保护路由。迁移上传、素材和对象访问前完成资源引用检查。对幂等写和批量删除实现并发测试。

退出条件：账号切换、越权 ID、上传中断、对象删除失败和多业务引用均通过故障测试。

### 13.5 M4：任务/生成/计费主链

建议先接一个文本或低成本图片 Provider，再接视频，再扩充其他能力。实现 task admission、适用的 billing authorization、lease fencing、Provider create/query/cancel、UNKNOWN、结果物化、transactional outbox、reconciler、结算、人工恢复和长轮询时间策略。

退出条件：能在 Java worker 重启、Provider 超时、重复回调、租约过期和媒体写失败后恢复，且不会产生第二个未经证明安全的远端副作用或重复扣费。视频专项验收需覆盖真实模型的长耗时。

### 13.6 M5：项目、画布、Agent、支付和后台高权限操作

逐聚合迁移项目/画布和资源引用；逐能力迁移 Agent 工具与审批；支付 webhook、调账、配额修改、密钥操作等最后切流，因为失败影响面大。

退出条件：每个高权限用例具备权限矩阵、审计检查、并发与补偿测试；Agent 有实际可运行链路验收。

### 13.7 M6：单 writer 切换与 Go 退场

可采用反向代理按 endpoint 或 route family 切流。推荐顺序：

1. Java shadow read，不产生副作用。
2. 对兼容读路径切少量流量；逐步扩大并比较错误率、延迟与业务统计。
3. 每个写路径指定唯一 owner；切换前停止旧服务对应写入或通过明确 fencing 阻止旧 writer。
4. 观察一个业务周期，核对任务、账务、资源、队列积压和管理审计。
5. 回滚只切回仍能读懂当前数据的版本；Java 写出的新格式若 Go 不兼容，不能承诺简单回滚。

不要同时双写 task/billing/resource 作为“保险”。影子比对只对读路径或无副作用计算安全；写路径双写需要独立的幂等、事务和对账设计。

## 14. 测试、验收与 Coding Agent guardrails

### 14.1 必须自动化的测试

| 层级 | 验收内容 |
| --- | --- |
| Domain 单测 | 状态转换、route/能力、配额、模型报价、终态互斥、金额精度 |
| Architecture test | ArchUnit/Spring Modulith 依赖方向、模块间不访问 Entity/Repository、shared 不吞业务 |
| API 合同测试 | 路径、Cookie、JSON field/null、错误 status/code/reason、分页和 SSE 游标；画布专项见[HTTP 黑盒合同测试计划](canvas-java-http-contract-test-plan.md) |
| Operation catalog test | 每个高成本 operation 的 `operationId`、幂等键、fingerprint、fence、receipt、ambiguity policy 和 retry budget 完整 |
| Repository 集成测试 | PostgreSQL 真实 dialect 下唯一约束、claim CAS、租约 fencing、事务回滚 |
| Transaction boundary test | `open-in-view=false`、外部 client 无活跃事务、短事务提交点、CAS/bulk 后 reload |
| SQLite 测试（若保留） | 迁移、JSON 列、抢占并发、锁等待与降级语义；不可只跑 H2 代替 |
| Provider contract | 请求编码、认证、长任务状态、错误分类、超时、重试、取消和响应脱敏 |
| UNKNOWN/recovery test | create 后超时/5xx、receipt 丢失、重启、reconciler、人工 resolution；断言不产生未经证明安全的第二个远端副作用，并覆盖已登记同 key replay 例外 |
| Outbox test | 业务事实与 outbox 同事务、重复投递、poison row、replay、唤醒丢失后 reconciler 收敛 |
| Billing immutability test | ledger/settlement/resolution 禁止 UPDATE/DELETE、唯一动账键、对账与反向修正 |
| Callback security test | 签名、时间窗、nonce/replay、未知关联、乱序、回调与用户读取路由分离 |
| 资源故障测试 | DB/对象写入交错失败、重复删除、共享对象、URL 过期、文件过大 |
| 权限测试 | 每个对象型 endpoint 的 owner/admin/foreign user；隐藏 UI 按钮不能代替测试 |
| Worker 测试 | 重复唤醒、并行 claim、进程崩溃、租约过期、旧 worker 回写拒绝、reconciler |
| Agent 测试 | bridge 认证、审批恢复、工具权限、远端失败不 fallback、取消和进程退出 |
| 数据迁移测试 | 真实样本导入、行数/余额/引用核对、JSON 兼容、密钥解密、增量追平 |
| Cutover test | 单 writer marker、旧幂等键、在途状态接管、forward-only、回切/向前修复和备份恢复 |

关键并发测试应使用 PostgreSQL Testcontainers 或专用集成数据库。H2 的锁与 SQL 行为不能证明 PostgreSQL 正确。

画布专项已建立完整 operation catalog、profile/场景映射、独立响应 schema 与隔离 Go HTTP 基线；创建/更新、故障/恢复和重复请求不重复计为新端点。完整单元正文逐一 GET，资源描述/派生授权、请求体预算、排序及成员变化按多请求状态流验收，不用摘要或起跑同步代替业务证据。旧版 114 场景双轮证据已合并分享到期 fixture，并补入本地 CDN/object-store 的分享资源 redirect、成功读取、GET 故障与恢复；90/115 版本最新双轮证据为 `20261013-073513-51126`，不与旧版请求数拼接。`core` updated-at 并列排序、受控 persistence/revision failure injection、资源预留到期/协调恢复、生产远端对象存储/CDN、TTL/条件缓存等仍是明确缺口。独立签名撤权、解码器未消费尾部、分页零 revision、非幂等 POST、并发内部错误等现状均进入待裁决账本；deferred/out-of-scope 和 Required 缺口不能被绿色覆盖。Java 继续遵循本 SKILL 的事实 owner、模块依赖、副作用恢复和撤权时限设计，不复制 Go Entity、聚合、错误、超时、签名、解码器或锁序。最新计数、源码版本/运行证据和剩余路径唯一维护于[画布合同计划](canvas-java-http-contract-test-plan.md#当前进度)；合同批准及远端门禁未闭合前，Gate 0 不通过。

### 14.2 必须进行的真实环境验收

自动化通过后，还需在隔离环境完成：

- 登录、退出、账号切换和管理员权限。
- 真实图片、视频、音频生成及模型路由；视频至少覆盖实际常见长耗时。
- Provider 处理中、短暂查询失败、长时间查询和任务超时边界。
- 跨 worker 实例竞争、Redis 不可用时的启动策略和恢复。
- 真实对象存储上传、预览、签名地址刷新和删除。
- SSE 断线重连与游标恢复。
- Agent bridge 从开始到工具调用、审批、恢复、结果写回的完整链路。
- 备份恢复、灰度切流、数据核对和回滚演练。
- 代表性生产数据规模的性能基线，不以本地小库速度代替。

## 15. 设计记录与实施护栏

每个迁移切片记录：

- 记录生成时的 Go checkout/配置版本、数据快照时间和证据日期；源码持续变化时重新核对，不要把本草稿中的数字当成永久合同。
- 对应的 Go 入口、用例、Repository、表和测试。
- Java 用例和 port/adapter 名称。
- 保持的外部合同，以及有意变化的合同和前端配套。
- 状态机图、事务提交点、外部副作用、幂等身份和补偿策略。
- 数据迁移脚本版本、输入输出水位和校验报告。
- 自动测试命令、实际环境验收证据和未验收项。
- 监控指标、告警阈值、灰度 owner 与回滚命令。

不要把一次代码审查或单元测试通过写成真实上游、跨标签页、跨实例或生产验收。

### 15.1 Coding Agent guardrails

这些规则应进入 Java 仓库的 ArchUnit/Spring Modulith、静态检查、集成测试或 review checklist；不靠口头约定：

- 依赖方向为 `api -> application -> domain`，adapter 只实现 owner port；存量违规冻结为基线，只允许减少。
- `spring.jpa.open-in-view=false`；出站 client 在测试中拦截“事务仍活跃”的调用，证明 Provider、支付、对象存储和 Agent 不在事务内。
- 每个高成本/非幂等 operation 必须有 catalog 条目、public idempotency key、fingerprint、durable fence、receipt owner 和 ambiguity policy。
- `UNKNOWN` 默认只允许 query、reconcile、人工 resolution 和有限的结果应用；只有对应 Provider adapter 具备书面稳定幂等 key query/replay 合同、相同 fingerprint 校验和 wire test 时，才可按原 key 有界恢复。该恢复不能创建新 attempt、切换 route 或被实现成通用原始 create 重试。
- 回调先验签、防 replay、校验关联 receipt；未验签内容只能唤醒，不能改 task、资金、scope 或结果地址。
- 异步工作经统一 dispatcher 捕获并清理 owner/tenant/job context；禁止裸 executor、未清理 ThreadLocal 和默认用户/权限。
- prepare、apply、settle、projection 各自在短事务内提交本阶段事实；新增 `REQUIRES_NEW` 必须写 ADR 说明连接池和回滚影响。
- native/CAS/bulk 更新后 reload；动账、resolution 和必要的证据表禁止 UPDATE/DELETE；金额禁止浮点。
- 新 API 有 DTO、唯一 `operationId`、OpenAPI schema、wire test、权限、错误和幂等语义；不返回 JPA Entity。
- 日志使用 allowlist，不记录 secret、Authorization、原始 payload、用户提示词、签名 URL、媒体地址或可跨 scope 关联的敏感字段。
- Provider 默认使用 stub；真实付费调用必须显式开关和次数上限。RLS、并发、迁移、备份恢复和资金测试使用隔离的真实 PostgreSQL，不以 H2 或共享开发库作证据。

## 16. 主要源码证据入口

| 主题 | 当前入口 |
| --- | --- |
| 启动、配置、数据库和 worker 关闭 | [cmd/server/main.go](../../backend/cmd/server/main.go) |
| API 路由和 OpenAPI | [handler/api.go](../../backend/internal/handler/api.go)、[openapi.yaml](../../backend/internal/handler/openapi.yaml) |
| 响应错误映射、Cookie 与 CORS | [handler/response.go](../../backend/internal/handler/response.go)、[handler/auth.go](../../backend/internal/handler/auth.go)、[cmd/server/main.go](../../backend/cmd/server/main.go) |
| app 中心对象与领域服务装配 | [app/service.go](../../backend/internal/app/service.go) |
| service 别名 facade | [service/aliases_types.go](../../backend/internal/service/aliases_types.go) |
| GORM repository 与 SQL | [repository/repository.go](../../backend/internal/repository/repository.go) |
| 表清单和迁移 | [database/schema.go](../../backend/internal/database/schema.go)、[database/migrations.go](../../backend/internal/database/migrations.go) |
| 任务状态、输入与恢复 | [model/models_task.go](../../backend/internal/model/models_task.go) |
| worker claim、lease 和恢复 | [app/task_worker.go](../../backend/internal/app/task_worker.go)、[task lease fencing test](../../backend/internal/repository/task_lease_fencing_test.go) |
| Redis / 本地协调降级 | [platform/coordination.go](../../backend/internal/platform/coordination.go) |
| 视频任务时限策略与测试 | [platform/runtime_policy.go](../../backend/internal/platform/runtime_policy.go)、[runtime_policy_test.go](../../backend/internal/app/runtime_policy_test.go) |
| Provider 合同与生成 engine | [generation/types.go](../../backend/internal/generation/types.go)、[generation/engine.go](../../backend/internal/generation/engine.go)、[protocol](../../backend/internal/protocol/) |
| 对象存储和出站安全 | [storage](../../backend/internal/storage/)、[assets](../../backend/internal/assets/)、[outbound](../../backend/internal/outbound/) |
| Agent Node 和远程执行 | [agent/runtime/runtime.go](../../backend/internal/agent/runtime/runtime.go)、[agent/yingceagent/client.go](../../backend/internal/agent/yingceagent/client.go) |
| Agent 端到端回归 | [cloud_agent_runtime_e2e_test.go](../../backend/internal/app/cloud_agent_runtime_e2e_test.go) |
| 生产服务拓扑 | [docker-compose.deploy.yml](../../docker-compose.deploy.yml) |

## 17. 自审与未验证范围

### 17.1 四轮自审记录

| 轮次 | 检查范围 | 结果 |
| --- | --- | --- |
| 第一轮：事实与缺口 | 当前实现、证据等级、capability profile、owner、operation catalog、存量资产与在途映射 | 已补入文档；tenant/RLS、Provider callback inventory、真实部署合同仍为 `UNKNOWN/DEFERRED` |
| 第二轮：失败、安全与资金 | `UNKNOWN`、重复 create、回调 replay、owner scope、对象删除、settle、日志和恢复 | 已补 failure matrix、删除决策、billing/settlement 与安全默认；后续画布 HTTP/PG 基线已执行，不替代 Provider/资金/对象存储故障验收 |
| 第三轮：术语、链接、测试与交付 | SKILL 术语、相对链接、测试入口、迁移阶段、回滚/forward-only | 已完成相对链接核对和测试入口补齐；文档构建、Java wire test、真实环境验收未执行 |
| 第四轮：空杯复审 | 当前事实与目标约束是否混写；任务 fencing 是否过度断言；回调分类；同 key replay 例外；M0 出口；Go 在途状态映射 | 已收紧上述边界；旧版 11-profile、114-scenario 基线之后新增 `generation-writeback` provider-free text replay + SSE 游标重放，最新 `20261013-073513-51126` 已完成 90/115 版本完整双轮。新增项目功能门控、跨项目重关联及配额/限流专项 HTTP 证据。旧版 clean-review `3/3` 不再沿用，新版三轮复核已完成 `3/3`。Java wire test、Provider/Agent 写回运行和文档站构建仍未执行，Required 剩余路径、行为审批和完整套件目标持续推进 |

### 17.2 最终结论

如果今天开始实现：

- **保留**：现有 Web 外部合同作为兼容窗口、PostgreSQL 事实源、模块化单体、Agent 外部运行边界、已验证的 user/owner 授权。
- **重构**：把 Task/Provider/账务/资源/Agent 变成 owner 明确的 durable facts；以 operation catalog、transactional outbox、reconciler、CAS/fencing 和不可变 ledger 补齐恢复协议。
- **推翻或禁止**：非原子双写、无 receipt 且无经验证稳定幂等合同的自动重提、把 Redis/Spring Event/SSE 当事实源、跨模块直接写 JPA Entity、无引用检查的物理删除、`open-in-view` 长事务。
- **前三个架构启用步骤**：完成 M0 证据与 owner 账本；冻结不改 schema 的合同和架构测试；在隔离 PostgreSQL 上完成一个低成本 Provider 的 prepare → create → UNKNOWN → reconcile → settle 闭环。

会改变相应默认政策的证据包括：已验证的多租户/合规隔离要求、Provider 官方稳定幂等合同，以及真实切换演练证明旧 Go writer 能安全读取 Java 新格式；会推翻模块化单体与 PostgreSQL 状态机默认的证据，则必须是不可由 PostgreSQL + outbox + reconciler 满足的故障域、吞吐或隔离要求，并附 replacement test、观测触发器和回滚成本。

本轮完成文档与架构语义审阅、源码/测试静态核对，并在隔离 PostgreSQL 18 与 Go HTTP 服务上完成最新 `20261013-073513-51126` 的画布合同 runner 90 个 Required operation 对应的 115 场景/11-profile 全量两轮，包含实际崩溃/优雅重启；同一轮真实浏览器 companion 也完成 6 个场景两轮。两轮清单内 observed/wire 均为 `14163/14162`，每轮 71 条映射并执行路由、72 条实际触达路由、71 条至少返回过非错误响应的路由、6 条受控故障、4 条响应丢失、4 条 incomplete 和 0 条未分类中断；浏览器页面响应头分别为 170/171，阻断装饰均为 9 条。新增 SSE 单轮已被合并进全量，但只证明 provider-free operation 的当前 Go 行为，不证明 Provider、SSE 断线重连、creation-run 或 Agent 写回。分享资源本地 CDN/object-store 的 redirect、成功读取、GET 故障与恢复、并发关联、固定 HTTPS 导入、legacy purge 竞态等既有选定路径仍以对应版本证据为准；不代表生产平台、远端对象存储/CDN、Java 或完整浏览器用户旅程通过。没有运行 Java、生产数据库、真实模型调用、生产远端对象存储/CDN 故障注入、跨设备同步、真实 Agent 写回或生产负载测试。当前 checkout 没有 `docs/package.json`，未虚构文档站 build/types 命令。Go 的 foreign 删除/列举差异、missing task logs 空结果、purge 断链、内存上传会话、分片覆盖/超时及关联部分提交仍待裁决。此文档不是 Java 实施完成、套件完整目标完成或无停机结论；最新版本/报告和完整 Required 缺口以[画布合同计划](canvas-java-http-contract-test-plan.md#验证记录)为准。

迁移开始前应先补齐逐 endpoint、逐表、逐 Provider 和逐环境的清单，并使用真实数据副本完成至少一次迁移与回滚演练。
