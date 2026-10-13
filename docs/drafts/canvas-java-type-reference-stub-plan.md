---
title: 画布 Java Spring Boot 类型参考桩实施规划
status: draft
scope: java-reference-stub-plan
updated: 2026-10-13
audience: 负责实现画布 Java 移植、HTTP 合同验收和类型设计的架构师与开发团队
---

# 画布 Java Spring Boot 类型参考桩实施规划

> **目的**：规划一个可以启动、可以被现有 HTTP 黑盒合同套件调用、只使用 H2 文件数据库和 mock/seed 数据的 Java Maven 项目，为未来的 Java 画布移植版提供可运行的 DTO、wire format、OpenAPI 和端点类型参考。
> **状态**：规划阶段；本文件只定义实施方案，不表示 Java 项目已经创建、编译或通过黑盒套件。
> **目标目录**：`java/canvas-contract-reference/`
> **架构依据**：采用 `spring-boot-principal-architect` 的模式 B 思路和护栏，但本项目是 reference-only stub，不是生产重写，也不把 Go 的包结构或缺陷自动提升为 Java 架构。
> **证据范围**：当前 checkout 的 `tests/canvas-contract`、Go handler/fixture、OpenAPI、Web API 客户端、画布研究文档、迁移指引，以及该 Spring Boot 架构技能的合同、事实 owner、事务、恢复和模块边界原则。

## 1. 执行结论

本次要创建的不是“把 Go 业务逻辑翻译成 Java”的后端，而是一个**类型参考桩（type-reference stub）**：

1. 它以 Spring Boot + Spring WebMVC + Jackson + Bean Validation + Spring Data JPA/Hibernate + H2 file mode 为主技术栈。
2. 它以强类型 Controller 入参、返回值和 DTO 描述当前黑盒套件需要的 HTTP 合同；公共 API 不使用 `Map<String, Object>`、无类型 `Object`、`ResponseEntity<?>` 或直接暴露 JPA Entity。
3. 它只实现让合同套件能够运行所需的最小状态机和状态转换，不实现真实模型调用、计费、OAuth、生产对象存储、真实 Agent、真实 Provider 或完整业务策略。
4. 所有初始数据、媒体字节、任务、事件、用户和权限场景都是确定性的 seed/mock 数据；请求之间需要的身份、资源、revision、history、share、upload、task 和 event 状态持久化到 H2，不能靠内存全局变量维持。
5. 它以当前合同套件的 `REQUIRED` 范围为第一交付门槛：11 个 profile、91 个 Required operation、118 个 HTTP scenario。完整 manifest 目前登记 163 个 operation / 143 个唯一 method/path，另有 `DEFERRED` 和独立 suite 的 `OUT_OF_SCOPE` 范围，不在第一版 reference stub 的默认交付分母中。
6. 现有 Go runner 通过 TCP HTTP 验收 Java jar；不需要把 Go 产品代码作为 Java 的依赖，也不需要启动浏览器才能执行 HTTP 主套件。真实浏览器 companion 是后续前端配合验收，不是这个 Java 项目的启动前提。

### 1.1 成功定义

第一版只有同时满足以下条件，才可以称为“参考桩完成”：

- `java/canvas-contract-reference/` 是独立 Maven 项目，可在隔离数据目录启动并在进程重启后恢复 H2 已提交状态。
- `mvn test`、静态架构规则、DTO/wire 序列化测试和项目自身的启动冒烟通过。
- 以 jar 作为目标进程，使用现有 `tests/canvas-contract` 的相同 HTTP 请求和断言，11 个 `REQUIRED` profile 可执行；失败必须记录为具体 route/scenario/fixture 差异，不用健康检查或局部 profile 的绿色冒充全部通过。
- `/api/openapi.yaml` 或等价的公开 YAML OpenAPI 入口能够反映实际 Controller 和 DTO；每个 Required operation 有稳定、唯一且与 catalog 可映射的 `operationId`。
- H2 只被声明为 reference stub 的本地 durable state；不把 H2 结果当作 PostgreSQL 并发、RLS、生产迁移或真实对象存储的证明。

### 1.2 不改变的事实边界

- 当前 Go 的响应、错误、权限和缺陷观察是**被测事实或待裁决行为**，不是全部已批准的 Java 合同。
- `tests/canvas-contract/contract/manifest.yaml` 是范围和 scenario 映射索引；manifest 登记不等于 scenario 已执行，也不等于 Java 目标已兼容。
- 当前套件的 `v1-observed` schema 是 wire 观测基线，不是 Java Entity 生成模板。
- 当前 Go 的某些 `UNKNOWN` / `KNOWN_BUG` / `APPROVED_CHANGE` 行为决策仍待批准；reference stub 必须把“为了让当前套件通过而模拟的观察行为”和“未来 Java 应保留的不变量”分开记录。
- 浏览器 companion 只验证真实 Web 前端消费 API 的调用链、响应解包、缓存和页面结果；它不增加后端 HTTP 覆盖分母，也不是本规划的 HTTP 主套件替代品。

## 2. 现状和证据账本

### 2.1 当前合同范围

以下数字由当前 `manifest.yaml` 的 `profiles` / `operations` 计算得到；数量改变时必须重新生成新的 run-id 和审计说明，不能把旧请求数拼接到新版本：

| 口径 | 当前值 | 含义 |
|---|---:|---|
| catalog operation | 163 | 全部登记的操作，包括 Required、Deferred 和独立 suite 范围 |
| 唯一 method/path | 143 | 去重后的 HTTP 路径组合；同一路径的不同场景不重复计 route |
| Required profile | 11 | 第一版 reference stub 的 HTTP 主套件门槛 |
| Required operation | 91 | 11 个 Required profile 中的 operation 数 |
| Required HTTP scenario | 118 | Required operation 关联的去重 scenario 数 |
| 直接画布 method/path | 14 | core 7、share-public 5、import 2；项目、资源、任务等是依赖族 |
| Deferred operation | 35 | `project-workbench` catalog-only，需后续明确范围和证据 |
| 独立 suite operation | 37 | `provider-resource`、`agent-canvas`、`generation-runtime`，由独立 suite 负责 |

证据入口：

- [`tests/canvas-contract/contract/manifest.yaml`](../../tests/canvas-contract/contract/manifest.yaml)：profile、operation、replay 语义、scenario、行为决策和证据路径。
- [`tests/canvas-contract/README.md`](../../tests/canvas-contract/README.md)：HTTP runner、browser companion、fixture、报告和覆盖口径。
- [`docs/drafts/canvas-java-http-contract-test-plan.md`](canvas-java-http-contract-test-plan.md)：当前 Go baseline、未闭合路径、行为决策和 Java 验收边界；本规划不复制其完整执行账本。
- [`docs/drafts/canvas-development-navigation-audit.md`](canvas-development-navigation-audit.md)：画布直接端点及 project/resource/asset/generation 依赖族的导航和覆盖边界。
- [`docs/drafts/canvas-deep-research.md`](canvas-deep-research.md)：画布七层状态、资源完整性、同步、生成回写和恢复的设计背景。
- [`docs/drafts/go-to-spring-boot-migration-guide.md`](go-to-spring-boot-migration-guide.md)：Java 生产迁移的事实 owner、事务、切流和回滚原则；reference stub 只采纳适用的类型和边界护栏。

### 2.2 证据等级

本规划中的表述使用以下标记：

- `CURRENT FACT`：可由当前源码、manifest、测试或实际运行记录直接确认。
- `TARGET INVARIANT`：reference stub 及未来 Java 版都必须通过测试守住的可观察不变量，例如 owner 隔离、CAS、请求结果持久化。
- `REFERENCE DECISION`：为使类型参考桩可运行而做的局部选择，例如 H2 file mode、确定性 seed、mock provider。
- `DEFERRED`：不进入第一版；必须有触发器、补证实验、最迟决策点和安全默认。

没有证据的生产假设不写成 `CURRENT FACT`。H2 能够证明“该桩在重启后恢复自己的文件状态”，不能证明未来生产 PostgreSQL 的 SQL、锁、RLS、迁移和容量行为。

### 2.3 黑盒套件查漏补缺登记

本规划不是只根据已有的“绿色 profile”设计 Java 端点；实施前必须把当前黑盒套件已经暴露的缺口分成“本版必须补齐”“reference stub 需要登记但暂不实现”和“由独立 suite 负责”三类。以下登记来自当前 `manifest.yaml` 的 `remaining_paths`、`browser/scenarios.json`、runner/setup 请求和各 profile 测试代码。它们是规划输入，不得在实现后被删除来制造完整覆盖假象。

| 缺口 ID | 当前缺口 | 证据/影响 | 补全动作 | 第一版状态 |
|---|---|---|---|---|
| G-01 | `core` 没有真实 transport timeout 和普通 history capture interval 的可重复场景 | profile `remaining_paths` 明确列出；当前 history/forced-restore 不能证明所有时间窗口 | 增加可控 clock/transport fault fixture，分别断言 timeout、无部分写入和 history capture interval；把 forced-restore retention 与 restart gate 分开计数 | 规划并登记；Java 第一版按现有可执行路径，补缺测试列为 Gate 1 |
| G-02 | share/public resource 只覆盖 local fixture，未证明 remote CDN delivery、远端 TTL 边界和 conditional cache | `share-public`、`public-resource` 都声明 remote gap；local redirect/GET 故障不能外推到 CDN | 增加受控 remote-origin/CDN fixture，覆盖 redirect、TTL、304、撤权后的 derived capability 和故障恢复；报告中继续区分 local/remote | 规划并登记；不把 H2/本地 fixture 写成远端通过 |
| G-03 | project link/member/workbench 并发只证明 HTTP settlement，不证明数据库交错、失败回滚和 in-flight crash | start barrier 不是数据库 interleaving proof；role-overflow/first-insertion 只是选定路径 | 增加真实隔离数据库或可控 repository barrier，记录锁序、失败注入、部分状态和恢复；Java reference 先实现可观察结果，不伪造内部锁证明 | 规划并登记；生产迁移前必须补真实 PostgreSQL gate |
| G-04 | project-workbench core 缺 workflow 初始化失败、active-task cascade、populated workflow/shot/asset cascade，以及 body trailing JSON policy 的完整决策 | profile `remaining_paths` 和 D-19 明确指出；当前 15 Required route 不是完整 project workbench | 为失败注入、已占用 task、填充 cascade 和 framing policy 建立独立 scenario；把 Java `ObjectMapper`/body limit 行为锁到批准的 policy | 第一版只实现 15 Required operation；缺口进入后续 gate，不冒充完整 project-workbench |
| G-05 | resource/upload 缺独立 background reconciler、突发连接 reset、remote object-store delivery matrix | 当前有 fresh-process admission、half-close 和 local fixture，但不能证明后台/远端时序 | 增加可控 reservation reconciler、RST/partial upload fault、remote object fixture 和恢复报告；明确 admission 与 cleanup 的 owner | H2 reference 只覆盖 durable state；补全测试列为远端/生产 gate |
| G-06 | import 缺 imported-media-specific response loss、远端 object-store failure 和更丰富 conversion limit 矩阵 | fixed upstream 成功/失败/timeout 已执行，但 response-loss 主要覆盖 resource operations | 增加 import result/bytes 分阶段 response-loss、媒体保存失败、边界 graph fixture；无 receipt 时保持 `UNKNOWN`，禁止自动重提 | Java 第一版使用 deterministic/loopback mock；缺口仍列为 Required profile 的补测项 |
| G-07 | legacy asset 缺 task/project reference sources 和物理删除 retry timing | 当前执行 owner/foreign、引用竞态和 local object-store failure，但 reference source / retry timing 是 separate gate | 增加 task/project/canvas 引用源矩阵、删除失败重试和 reconciler evidence；继续验证引用存在不物理删除 | Java 第一版记录 reference source DTO/state，真实重试留后续 gate |
| G-08 | restart recovery 没覆盖 crash during in-flight write/object operation，也没覆盖多实例 upload session | 当前只证明提交后 response loss、已提交事实和单进程 crash/graceful restart | 增加 in-flight crash window、对象写入半完成、双实例 session/lease 方案；没有证据前默认单实例 | Java 第一版必须通过已提交事实 restart；in-flight/multi-instance 标为 deferred |
| G-09 | generation boundary 缺 deterministic Agent run owner/foreign durable fixtures；writeback 缺 live SSE disconnect、task retry/cancel/recovery、creation-run/Agent writes | 两个 generation profile 明确是 read-only/text replay subset | 增加不启动 Provider 的 Agent durable fixture、SSE disconnect/reconnect fault、task recovery 和 creation-run/Agent state matrix；Provider 仍由独立 suite | Java 第一版补齐现有 9+5 Required subset；其余进入独立 suite |
| G-10 | browser companion 仍缺 CAS conflict UI、local-only save failure/network recovery、history/import/project/upload UI、task/Agent SSE、跨 tab cache race | `browser/scenarios.json` 的 `remaining_paths` 明确列出；HTTP 主套件不覆盖 UI | 为真实 React/Vite 页面新增 scenario；每个 scenario 记录 UI action、真实网络证据、最终页面状态，并独立于 HTTP 分母 | 不阻断 Java HTTP stub；Java+真实 Web 联调前需单独验收 |
| G-11 | manifest/catalog 没有把 setup/auth/session/private fixture 作为业务 operation，但 runner 实际依赖它们 | 当前 runner 使用 `/api/auth/session`、admin feature/runtime/OSS/libtv/channels/models/users 和三个 private fixture；此前只检查业务 manifest | 增加 setup/fixture contract inventory、typed request/response、未知字段/trailing JSON/token/loopback 安全测试；把 `/api/auth/session` 加入 Java 计划和 README | Java 第一版必须提供 runner 所需 setup；不计入 91 个画布 operation |
| G-12 | 当前 catalog checker 校验 manifest、scenario、wire evidence，但没有自动把目标 Java OpenAPI 与 manifest/DTO 做双向比对 | Go baseline 报告可绿不代表 Java Controller/DTO 对齐；Java 特有动态类型违规也不会被 Go runner 捕获 | 增加 `java/canvas-contract-reference` 的 OpenAPI export check：Required operationId/path/method、schema、error response、binary/SSE 绑定双向校验 | Java 交付阻断门禁 |
| G-13 | 当前 HTTP runner 的通过主要是行为/线上的可观察证据，不提供目标代码公共签名“无 Map/Object”的静态门禁 | 黑盒不能发现 Controller 返回 `Map<String,Object>` 但恰好 wire 正确 | 增加 Java ArchUnit/反射扫描和编译期 API review，扫描 Controller、DTO、port、OpenAPI schema | Java 交付阻断门禁 |
| G-14 | Deferred `project-workbench` 35 operations 与 `provider-resource`/`agent-canvas`/`generation-runtime` 独立 suite 未闭合 | manifest 当前分别标记 `DEFERRED` / `OUT_OF_SCOPE`，不能把 91 Required green 外推到 163 全量 | 保持 operation catalog 可见，分别建立 owner、scenario、fixture、验收命令和最迟决策点；需求进入时才扩展 Java stub | 第一版不实现，但必须在 README/报告显示 |

缺口处理原则：

1. G-01～G-10 是现有 Required 或 browser/远端验收的真实缺口；实现 Java reference stub 时至少要让缺口在报告中可定位，并在对应 profile/阶段留下补测任务，不能用 seed 或 mock response 静默绕过。
2. G-11～G-13 是“套件能测行为，但不能独立证明 setup、OpenAPI 和强类型设计”的门禁缺口；它们必须在 Java 项目交付前补上。
3. G-14 不是本版遗漏，而是显式范围边界；只有完整补齐独立 suite 或正式批准扩范围后，才可把全 manifest 作为 Java 通过分母。
4. 每个缺口关闭时要新增 scenario/fixture/checker 或补充证据，并更新 `manifest.yaml`、本文件和[合同测试计划](canvas-java-http-contract-test-plan.md)；只修改文字不能视为关闭。

## 3. Goal、非目标和交付物

### 3.1 Goal

创建可独立运行的 `canvas-contract-reference` Maven 项目，提供：

- Required HTTP route 的强类型 Controller 和 DTO；
- 与现有 runner 兼容的统一成功/错误 envelope、Cookie session、HTTP status、header、ETag、压缩、Range、304/416、SSE 和二进制响应；
- 可被后续请求引用的 H2 durable state 和 request ledger；
- 可重复的 seed/mock fixture，覆盖 owner、foreign、anonymous、admin、share、resource、project、task 和 Agent boundary；
- `springdoc-openapi` 注解和生成文档，解释端点、参数、响应、错误、权限、replay 和 reference-only 限制；
- jar 启动、健康检查、重启、fixture 控制和黑盒套件执行脚本；
- 与迁移指引、合同测试计划、测试 README 和文档索引之间的可追溯链接。

### 3.2 非目标

以下内容不能因“为了测试方便”悄悄加入：

- 真实用户注册安全、密码恢复、OAuth、短信、生产 session 集群和完整 Spring Security 产品配置；
- 真实 LLM、图像、视频、音频 Provider、Agent runtime、支付、计费、配额计费和外部 webhook；
- 真实对象存储/CDN、OSS 复制、后台 reconciler、生产 outbox 投递和跨实例 SSE；
- 把 Go 的内部包、GORM 查询、锁顺序、错误文本或已识别缺陷逐行翻译成 Java；
- 将 H2 作为 PostgreSQL 的替代验收证据；
- 实现 manifest 中 `DEFERRED` 的 35 个 `project-workbench` operation，或 `OUT_OF_SCOPE` 的 provider/Agent/generation runtime operation，除非后续重新批准范围；
- 用 `Map<String, Object>`、`Object`、动态 Controller 参数或通用 JSON 逃生舱快速“把接口接起来”；
- 通过浏览器自动化替代 HTTP 黑盒套件；
- 通过内存 map 保存用户、session、revision、upload、task、event 或 request ledger，使重启后的行为与启动前不同。

### 3.3 交付物

| 交付物 | 位置 | 作用 |
|---|---|---|
| Maven 项目 | `java/canvas-contract-reference/` | 可运行的 Java reference stub |
| 架构和类型实现 | `src/main/java/` | Controller、DTO、state、persistence、seed、OpenAPI 配置 |
| H2 配置和迁移 | `src/main/resources/` | file mode、schema、contract profile、确定性配置 |
| 自身测试 | `src/test/java/` | 强类型、wire、状态、权限、重启和架构护栏 |
| 启动脚本 | `scripts/run-reference.sh` | 使用指定数据目录启动 jar，便于 harness 拥有和重启 |
| 套件脚本 | `scripts/run-http-contract-suite.sh` | 调用现有 Go runner，显式传递 profile、fixture 和报告路径 |
| Java README | `java/canvas-contract-reference/README.md` | 前置条件、运行方式、范围、限制和故障排查 |
| 本规划 | `docs/drafts/canvas-java-type-reference-stub-plan.md` | 设计理由、任务分解、验收矩阵和未来维护入口 |

## 4. 目标 Maven 项目结构

第一版采用一个可部署的模块化单体，而不是拆成多个微服务。包按**事实 owner / 能力边界**划分，不能仅按 controller/service/repository 三层平铺：

```text
java/canvas-contract-reference/
├── pom.xml
├── README.md
├── scripts/
│   ├── run-reference.sh
│   └── run-http-contract-suite.sh
├── src/main/java/com/wubuku/canvas/contract/reference/
│   ├── CanvasContractReferenceApplication.java
│   ├── config/
│   │   ├── ContractProfileConfiguration.java
│   │   ├── JacksonConfiguration.java
│   │   ├── OpenApiConfiguration.java
│   │   ├── SecurityConfiguration.java
│   │   └── WebMvcConfiguration.java
│   ├── api/
│   │   ├── common/
│   │   ├── auth/
│   │   ├── canvas/
│   │   ├── share/
│   │   ├── project/
│   │   ├── resource/
│   │   ├── importcontract/
│   │   ├── legacy/
│   │   ├── generation/
│   │   └── admin/
│   ├── contract/
│   │   ├── policy/
│   │   ├── state/
│   │   └── fixture/
│   ├── persistence/
│   │   ├── entity/
│   │   ├── repository/
│   │   └── converter/
│   └── seed/
│       ├── ReferenceSeedRunner.java
│       ├── SeedDataset.java
│       └── MockMediaFixtures.java
├── src/main/resources/
│   ├── application.yml
│   ├── application-contract.yml
│   └── db/migration/
│       ├── V1__reference_schema.sql
│       └── V2__reference_seed_support.sql
└── src/test/java/com/wubuku/canvas/contract/reference/
    ├── architecture/
    ├── api/
    ├── contract/
    ├── persistence/
    └── restart/
```

### 4.1 依赖方向

固定为：

```text
api Controller
  -> contract application/service ports
  -> persistence repository ports
  -> entity/converter adapter
  -> H2 database
```

规则：

- `api` 不直接调用 `JpaRepository`、不访问 Entity 字段写入、不包含 SQL 和跨域策略。
- `contract` 拥有可观察状态转换、权限检查、fixture policy、replay policy 和 mock adapter port。
- `persistence` 实现 contract port；Controller 不知道 H2 表名。
- `seed` 只能通过 owner port 或显式 seed repository 写入，并在代码中标注为测试初始化路径。
- 任何跨模块写入都通过 typed command/transition，禁止 Controller 直接修改其他模块的状态。
- 用 ArchUnit 或等价架构测试阻止 `api -> persistence.entity`、`api -> persistence.repository` 和模块循环依赖；第一版不引入真实业务的 mega-service。

## 5. 技术栈和运行配置

### 5.1 技术选型

| 能力 | 选择 | 选择理由和限制 |
|---|---|---|
| JDK | Java 21 LTS | 支持 record、sealed interface、现代 Spring 生态；不依赖虚拟线程作为容量方案 |
| 构建 | Maven Wrapper + Maven | 让黑盒 harness 在固定目录用一致命令构建和启动 |
| Web | Spring Boot 3.x + Spring WebMVC | 明确使用 Servlet/MVC；不使用 WebFlux；SSE 用 `SseEmitter` |
| JSON | Jackson | DTO、record、sealed polymorphic node 和受控 `OpaqueJsonValue` 的统一序列化 |
| 校验 | Jakarta Bean Validation | `@Valid`、`@NotBlank`、`@Size`、`@Min`、自定义 revision/body 约束 |
| 持久化 | Spring Data JPA + Hibernate | 让 Entity/Repository 与 API DTO 分离；状态事实落盘 |
| 数据库 | H2 file mode | 本地可运行、可重启恢复；不作为 PostgreSQL 等价证明 |
| 迁移 | Flyway 或同等 append-only migration | 固定 schema，禁止 `ddl-auto=update` 形成隐式结构 |
| OpenAPI | `springdoc-openapi` WebMVC starter | Controller/DTO 注解生成可读 YAML/JSON 文档和 Swagger UI |
| 测试 | Spring Boot Test、MockMvc、JUnit、ArchUnit | 本地快速类型/结构测试；最终仍以 TCP HTTP 黑盒 runner 为主门禁 |

精确依赖版本在实现阶段写入 `pom.xml` 并锁定，不使用动态版本或“取最新”。项目不引入 Lombok 作为类型语义的替代；优先使用显式 record、不可变 DTO 和小型 value object。

### 5.2 H2 file mode

`application-contract.yml` 必须满足：

- 数据库 URL 来自环境变量或启动参数，例如 `jdbc:h2:file:${CANVAS_REFERENCE_DB_FILE};MODE=PostgreSQL;AUTO_SERVER=TRUE`；不得固定写入仓库或用户目录。
- `spring.jpa.hibernate.ddl-auto=validate`；schema 由迁移文件建立。
- `spring.jpa.open-in-view=false`；Controller 序列化前必须已在短事务内构建完整 DTO。
- 关闭 H2 console 的公网暴露；若本地临时开启，必须只绑定 loopback 且不能进入合同启动脚本。
- 进程退出后不删除 `.mv.db` 文件；`run-reference.sh` 只清理当前 run 自己创建的临时日志和 PID，不删除用户指定的 reference data directory。
- seed 使用数据库中的 `seed_version` / `reference_dataset` 标记，重复启动不重复插入；测试 runner 应为每轮提供新的隔离目录。
- H2 `MODE=PostgreSQL` 只帮助 SQL 语法接近，不声明锁、事务隔离、索引、RLS、JSONB、时间、并发和迁移行为等价。

### 5.3 HTTP 运行边界

- 默认监听 `127.0.0.1:${CANVAS_REFERENCE_PORT}`，默认健康路径 `/api/health`。
- `CANVAS_CONTRACT_PROFILE` 或 `reference.contract.enabled=true` 才开启 fixture-only 行为；非 contract profile 时私有 fixture 路由返回 404。
- 服务必须拒绝 URL credentials、非法 Host、超大 body、未知 JSON 字段和 trailing JSON，保持 fixture 与主套件的输入安全边界。
- Java jar 必须能被现有 `target-control` 通过 wrapper script 启停；健康检查只证明进程可达，黑盒套件才证明 API 行为。

## 6. API 端点覆盖规划

### 6.1 Required profile 总表

| Profile | Operation | 主要状态事实 | 主要 mock/seed 依赖 | 第一版结论 |
|---|---:|---|---|---|
| `core` | 13 | canvas、revision、history、resource reference、ETag | owner/foreign 用户、canvas、history、resource | 必须实现 |
| `share-public` | 5 | share token、allowlist、expiry、revoke、public resource | share、匿名 session、resource bytes | 必须实现 |
| `project-integration` | 4 | project-canvas link、unit link、owner/foreign | project、unit、canvas | 必须实现 |
| `project-workbench-core` | 15 | project、unit、feature gate、bulk import、reorder | project/unit seed、feature settings | 必须实现 |
| `resource-integrity` | 11 | resource、access signature、usage、upload reservation/chunk | bytes、limits、upload session、fixture upstream | 必须实现 |
| `import` | 2 | fixed conversion response、failure/timeout/replay boundary | loopback fixed mock upstream、feature gate | 必须实现 |
| `legacy-assets` | 12 | old asset/folder、ownership、delete protection、snapshot | legacy assets、folder、physical mock file | 必须实现 |
| `public-resource` | 2 | signed resource、named file、Range、304/416、expiry | resource bytes、signature record | 必须实现 |
| `restart-recovery` | 13 | H2 committed facts、response-loss recovery、restart | 上述依赖、同一 data dir、target-control | 必须实现 |
| `generation-boundary` | 9 | task/creation-run/agent read ownership | owner/foreign task/run/agent seed | 必须实现，read-only |
| `generation-writeback` | 5 | text delta、completion、cursor、SSE replay | deterministic task/event seed，无 Provider | 必须实现，provider-free |

Required operation 的完整映射由 manifest 维护；实现时不得只凭本表新增或删除 endpoint。以下清单把 operationId 与 path 固定到类型设计和 OpenAPI 注解的工作项。

### 6.2 Core、share 和项目关联

```text
core
  canvas.list                       GET    /api/canvas-projects
  canvas.create                     PUT    /api/canvas-projects/{id}
  canvas.read                       GET    /api/canvas-projects/{id}
  canvas.update                     PUT    /api/canvas-projects/{id}
  canvas.history.list               GET    /api/canvas-projects/{id}/history
  canvas.history.read               GET    /api/canvas-projects/{id}/history/{snapshotId}
  canvas.history.restore            POST   /api/canvas-projects/{id}/history/{snapshotId}/restore
  canvas.delete                    DELETE  /api/canvas-projects/{id}
  canvas.authz                      GET    /api/canvas-projects/{id}
  canvas.authz.foreign-delete      DELETE  /api/canvas-projects/{id}
  canvas.conditional-read           GET    /api/canvas-projects/{id}
  canvas.compression                GET    /api/canvas-projects/{id}
  canvas.update.repair-missing-resources PUT /api/canvas-projects/{id}

share-public
  canvas.share.read                 GET    /api/canvas-projects/{id}/share
  canvas.share.create               POST   /api/canvas-projects/{id}/share
  canvas.share.revoke              DELETE  /api/canvas-projects/{id}/share
  canvas.share.public-read          GET    /api/public/canvas-shares/{token}
  canvas.share.resource-read        GET    /api/public/canvas-shares/{token}/resources/{resourceId}/file

project-integration
  project.canvas.list               GET    /api/projects/{id}/canvases
  project.canvas.link               POST   /api/projects/{id}/canvas-links
  project.canvas.unlink-unit       DELETE  /api/projects/{id}/canvas-links/{canvasId}/units/{unitId}
  project.canvas.unlink            DELETE  /api/projects/{id}/canvases/{canvasId}
```

### 6.3 Project/workbench、resource、import 和 legacy

```text
project-workbench-core
  project.list                      GET    /api/projects
  project.create                    POST   /api/projects
  project.read                      GET    /api/projects/{id}
  project.core                      GET    /api/projects/{id}/core
  project.overview                  GET    /api/projects/{id}/overview
  project.update                    PATCH  /api/projects/{id}
  project.delete                   DELETE  /api/projects/{id}
  project.unit.create               POST   /api/projects/{id}/units
  project.unit.list                GET    /api/projects/{id}/units
  project.unit.read                GET    /api/projects/{id}/units/{unitId}
  project.unit.workspace            GET    /api/projects/{id}/units/{unitId}/workspace
  project.unit.import               POST   /api/projects/{id}/units/import
  project.unit.reorder              PATCH  /api/projects/{id}/units/reorder
  project.unit.update               PATCH  /api/projects/{id}/units/{unitId}
  project.unit.delete              DELETE  /api/projects/{id}/units/{unitId}

resource-integrity
  canvas.resource.access            POST   /api/resources/access
  canvas.resource.list              GET    /api/resources
  canvas.resource.usage             GET    /api/resources/storage-usage
  canvas.resource.create            POST   /api/resources
  canvas.resource.import            POST   /api/resources/import
  canvas.resource.read              GET    /api/resources/{id}
  canvas.resource.file              GET    /api/resources/{id}/file
  canvas.resource.upload.create     POST   /api/resources/uploads
  canvas.resource.upload.chunk      PUT    /api/resources/uploads/{id}/chunks/{index}
  canvas.resource.upload.complete   POST   /api/resources/uploads/{id}/complete
  canvas.resource.reference-validation PUT /api/canvas-projects/{id}

import
  canvas.import.libtv               POST   /api/canvas-projects/{id}/import/libtv
  canvas.import.tapnow              POST   /api/canvas-projects/{id}/import/tapnow

legacy-assets
  canvas.legacy-asset.list          GET    /api/assets
  canvas.legacy-asset.batch-read    POST   /api/assets/batch
  canvas.legacy-asset.batch-delete  POST   /api/assets/batch-delete
  canvas.legacy-asset.folder-list   GET    /api/asset-folders
  canvas.legacy-asset.folder-create POST   /api/asset-folders
  canvas.legacy-asset.folder-update PATCH  /api/asset-folders/{id}
  canvas.legacy-asset.folder-delete DELETE /api/asset-folders/{id}
  canvas.legacy-asset.folder-move   PATCH  /api/assets/folder
  canvas.legacy-asset.read           GET   /api/assets/{id}
  canvas.legacy-asset.update         PUT   /api/assets/{id}
  canvas.legacy-asset.delete        DELETE /api/assets/{id}
  canvas.legacy-user-data.snapshot   GET   /api/user-data/snapshot

public-resource
  canvas.public-resource.file        GET   /api/public/resources/{id}/file
  canvas.public-resource.file-named  GET   /api/public/resources/{id}/file/{filename}
```

### 6.4 Restart、generation boundary 和 writeback

`restart-recovery` 不增加新的唯一 route；它要求同一组 entity 在 crash/graceful restart、response loss、deleted state、upload reservation 和 committed response 场景下仍可观察。必须用同一 H2 file path 启动新进程，不能靠 seed 重新伪造结果。

```text
generation-boundary
  canvas.task.list                   GET   /api/tasks
  canvas.task.read                   GET   /api/tasks/{id}
  canvas.task.logs                   GET   /api/tasks/{id}/logs
  canvas.creation-run.list           GET   /api/creation-runs
  canvas.creation-run.read           GET   /api/creation-runs/{id}
  canvas.agent.capabilities          GET   /api/agent/capabilities
  canvas.agent.run.read              GET   /api/agent/runs/{id}
  canvas.agent.skills-usage          GET   /api/agent/skills/usage
  canvas.agent.profile.read          GET   /api/agent/profile

generation-writeback
  canvas.task.create                POST  /api/tasks
  canvas.task.text-delta-create    POST  /api/tasks/{id}/text-deltas
  canvas.task.text-delta-list       GET  /api/tasks/{id}/text-deltas
  canvas.task.events                GET  /api/tasks/{id}/text-events
  canvas.task.text-replay-complete POST  /api/tasks/{id}/text-replay-complete
```

第一版 generation 实现只提供 provider-free deterministic text replay、SSE cursor replay 和 owner/foreign/anonymous 边界；不实现真实任务执行、Provider callback、Agent runtime 或媒体回写。

### 6.5 必须提供但不计入画布 operation 的 setup API

这些路径供 runner 建立测试身份、开关和 mock 环境，不计入画布 Required operation 覆盖率，但必须有明确 DTO、权限和 OpenAPI 注解：

```text
POST /api/auth/register
POST /api/auth/login
POST /api/auth/logout
GET /api/auth/session
POST /api/admin/users
GET|PATCH /api/admin/settings/features
GET|PUT /api/admin/settings/runtime-policy
PATCH /api/admin/settings/libtv
PATCH /api/admin/settings/oss
POST /api/admin/settings/oss/test
POST /api/admin/channels
POST /api/admin/channels/{id}/models
GET /api/health
```

setup API 的约束：

- `register/login/logout` 使用明确的 `RegisterRequest`、`LoginRequest`、`SessionDto` 和 `LogoutResponse`；不接受客户端自报 `userId` 作为身份。
- `GET /api/auth/session` 使用明确的 `SessionDto`；浏览器 companion 依赖它确认当前 Cookie session，不能只实现 login 而省略 session read。
- admin API 使用 `AdminUserCreateRequest`、`FeatureSettingsDto`、`RuntimePolicyDto`、`ChannelDto`、`ModelDto`；不把配置行直接序列化为 map。
- `libtv`、`oss` 和 `oss/test` 是 contract fixture/setup 端点，不是生产 Provider API；必须在 README 中注明它们只用于隔离测试。
- bootstrap admin 只能通过 contract profile 的 seed 或显式环境变量创建；普通 profile 不自动产生默认管理员。
- 每个 runner 轮次使用隔离 H2 data directory 和独立用户名，避免 seed、session、quota 和资源跨轮污染。

### 6.6 私有 fixture API

当前 runner 的 database/share fixture 需要以下控制操作：

```text
POST /expire-share
POST /set-canvas-updated-at
POST /expire-upload-reservation
```

规划要求：

- 仅 `reference.contract.enabled=true` 时注册；生产/普通运行配置不注册。
- 只接受 loopback 请求和长度至少 24 字节的 `Authorization: Bearer` token；使用常量时间比较。
- 使用 `@RequestBody` 强类型 `ExpireShareFixtureRequest`、`SetCanvasUpdatedAtFixtureRequest`、`ExpireUploadReservationFixtureRequest`。
- 拒绝未知字段、trailing JSON、超出 body 上限、URL credentials 和 redirect；响应使用命名的 fixture result DTO。
- 不出现在业务 manifest；OpenAPI 默认隐藏，或单独标记 `x-reference-fixture: true` 并在 README 中说明不可用于生产。
- fixture 通过 owner port 更新 H2 状态，不直接在 Controller 中执行 JPA 查询；操作结果要验证受影响行数和目标 ID。

## 7. 强类型 API 和 DTO 设计

### 7.1 公共方法签名规则

所有业务 Controller 方法必须符合以下形态之一：

```java
ResponseEntity<ApiEnvelope<CanvasProjectDto>> read(...)
ResponseEntity<ApiEnvelope<CanvasProjectListDto>> list(...)
ResponseEntity<ApiEnvelope<UploadSessionDto>> createUpload(...)
ResponseEntity<byte[]> readResourceFile(...)
ResponseEntity<ResourceFileDescriptorDto> describeResourceFile(...)
ResponseEntity<SseEmitter> streamTextEvents(...)
```

禁止：

- `Map<String, Object>`、`Map<String, ?>`、无类型 `Map`、无类型 `Object` 作为 request/response 或 Service port；
- `ResponseEntity<?>`、`ResponseEntity<Object>`、`List<Map<...>>`；
- Controller 直接返回 Entity、`JsonNode` 或 Hibernate proxy；
- 用一个 `GenericRequest` / `GenericResponse` 承载所有 profile；
- 用 `@RequestParam Map<String, String>` 隐式接受未登记参数；
- 用异常文案或 `msg` 作为客户端稳定分支依据。

### 7.2 Envelope、错误和通用 value object

定义并复用：

- `ApiEnvelope<T>`：`code`、`data`、`msg`、`reason`，成功/失败字段语义与现有前端和合同计划一致。
- `ApiErrorDto`：稳定 `code`、`reason`、`details`，其中 `details` 使用各错误场景的命名 DTO；不能用通用 map 扩展。
- `PageQuery`、`PageInfoDto`、`CursorDto`、`RevisionDto`、`EtagDto`、`RequestIdDto`、`AuthenticatedUser`。
- `OpaqueJsonValue`：仅作为已经登记的开放扩展字段 value object，内部可以由 Jackson tree 保留未知 JSON，但不会成为所有 API 的动态逃生舱；每个使用点必须有字段名、大小上限、允许 JSON 类型和保留/丢弃策略。
- `BinaryResponseMetadata`、`ResourceFileDescriptorDto`、`SseTextEventDto`：把二进制和流式响应的可观察元数据显式化。

### 7.3 领域 DTO 分组

| DTO 组 | 典型类型 | 强制规则 |
|---|---|---|
| canvas | `CanvasProjectDto`、`CanvasDocumentDto`、`CanvasNodeDto`、`CanvasConnectionDto`、`CanvasViewportDto`、`CanvasHistorySnapshotDto` | revision、ID、nodes、connections、metadata/extensions 有明确类型；保留未知扩展必须走 `OpaqueJsonValue` |
| share | `CanvasShareDto`、`CreateShareRequest`、`PublicCanvasDto`、`ShareResourceDescriptorDto` | token 是 opaque value；公开读取不暴露 owner session；expiry/revoke 状态显式 |
| project | `ProjectDto`、`ProjectCoreDto`、`ProjectOverviewDto`、`UnitDto`、`UnitImportRequest`、`UnitReorderRequest`、`CanvasLinkDto` | project/unit/link 归属和 role 使用 enum/value object；批量 import 有明确 list 类型和预算约束 |
| resource | `ResourceDto`、`ResourceAccessRequest`、`ResourceAccessDto`、`UploadSessionDto`、`UploadChunkReceiptDto`、`StorageUsageDto` | bytes、mime、kind、status、checksum、signature 和 expiry 不使用字符串拼 JSON |
| legacy | `LegacyAssetDto`、`AssetFolderDto`、`BatchAssetRequest`、`UserDataSnapshotDto` | folder、reference、delete protection、physical mock object 状态分开表达 |
| import | `LibTvImportRequest`、`TapnowImportRequest`、`ImportedCanvasDto`、`ImportedNodeDto`、`ImportedConnectionDto`、`ImportWarningDto` | conversion result 强类型；固定 mock 上游失败/超时不返回动态错误正文 |
| generation | `TaskDto`、`TextDeltaDto`、`TextEventDto`、`CreationRunDto`、`AgentProfileDto`、`AgentCapabilityDto` | task/run owner 和 cursor 明确；SSE data 是 `TextEventDto`，不把任务 ID当 cursor |
| admin/setup | `RegisterRequest`、`LoginRequest`、`SessionDto`、`FeatureSettingsDto`、`RuntimePolicyDto`、`ChannelDto`、`ModelDto` | 仅支持 runner 所需最小操作；不扩展为生产管理后台 |

Canvas node 采用 sealed interface + 明确 subtype 或“固定字段 + 命名扩展 value object”的方案。实现阶段先从合同 fixture 中出现的 node types 建立 subtype；未知 node type 必须有显式 `UnknownCanvasNodeDto`，不能在 Controller 中退化为 `Map`。

### 7.4 Entity 和 DTO 隔离

- `CanvasProjectEntity`、`ResourceEntity` 等只表示持久化事实；不加 `@Schema` 让 Entity 直接变成 OpenAPI model。
- DTO 组装在 transaction 内完成，返回前完成 lazy data 读取；`open-in-view=false` 下不会序列化时访问数据库。
- Entity 的 `createdAt/updatedAt`、内部 hash、mock file path、fixture flags、request ledger 状态不默认暴露给 API。
- DTO 的字段名称、`null`/省略语义和数字类型由 wire golden test 固定；不能因为 Java 类型方便而把 `int64`、时间、bytes 或 enum 改成宽泛字符串。

## 8. Durable state、seed 和最小状态机

### 8.1 数据表/Entity 规划

第一版 H2 schema 至少包含下列事实族；每一族只有一个写入 owner：

| 事实族 | 参考 Entity | 关键字段 | 写入 owner |
|---|---|---|---|
| identity/session | `ReferenceUserEntity`、`SessionEntity` | userId、role、password hash/mock verifier、cookie id、expiresAt、revokedAt | auth |
| canvas | `CanvasProjectEntity` | id、ownerId、document JSON/value、revision、updatedAt、deletedAt | canvas |
| canvas history | `CanvasHistoryEntity` | snapshotId、canvasId、revision、preimage、createdAt、evictedAt | canvas-history |
| share | `CanvasShareEntity` | canvasId、token hash、enabled、expiresAt、allowlist、updatedAt | share |
| project/workbench | `ProjectEntity`、`UnitEntity`、`CanvasLinkEntity` | ownerId、feature state、revision、order、role、link state | project |
| resource | `ResourceEntity`、`ResourceAccessEntity` | ownerId、kind、mime、size、checksum、status、bytes/mock object id | resource |
| upload | `UploadReservationEntity`、`UploadChunkEntity` | uploadId、ownerId、size、chunkCount、received indexes、expiresAt、completedAt | upload |
| legacy assets | `LegacyAssetEntity`、`AssetFolderEntity` | ownerId、folderId、references、physicalObjectState | legacy-asset |
| generation read/writeback | `TaskEntity`、`TextDeltaEntity`、`TextEventEntity`、`CreationRunEntity` | ownerId、status、cursor、payload snapshot、event id | generation |
| agent boundary | `AgentRunEntity`、`AgentProfileEntity`、`AgentMemoryEntity` | ownerId、visibility、mock capability/profile/memory | agent |
| request ledger | `RequestLedgerEntity` | operationId、actor、fingerprint、resource key、policy、result snapshot、applied state | contract-state |
| seed/config | `SeedVersionEntity`、`FeatureFlagEntity`、`RuntimePolicyEntity`、`ChannelEntity`、`ModelEntity` | seed version、flags、mock channel/model metadata | setup/seed |

JSON 文档字段必须有明确 owner、schema version、大小限制和 Jackson codec；不把所有 JSONB/文本字段暴露为动态 API。H2 中可用 CLOB/BLOB 或序列化文本保存 reference data，但 DTO 组装必须经过显式转换器。

### 8.2 Seed 数据集

`ReferenceSeedRunner` 在 contract profile 启动时一次性写入 deterministic dataset：

- bootstrap admin、seed owner、seed foreign user、seed anonymous-compatible public records；密码只由测试配置注入，不在仓库保存真实密码。
- 至少一个 canvas、多个 revision/history、未知 node type、嵌套 extension、缺失/有效 resource reference。
- 至少一个 project、多个 unit、不同 role、可关联/已关联/跨项目 unit、feature gate 组合。
- 至少一个 ready resource、一个描述型 resource、一个可 Range 读取的 mock file、一个 expired/invalid signature 场景。
- 至少一个 legacy asset/folder、引用保护关系、batch 操作数据和 user-data snapshot。
- owner/foreign task、creation-run、agent run/profile/memory，以及可重放的 text delta/event 序列。
- mock import fixture 的成功、空转换、错误、超时和边界数据；不调用真实上游。

Seed 不应预先污染 runner 的“空库行为”断言。需要空库的场景使用新的 data directory 或清晰的 test reset command；不能在请求中偷偷删除并重建整个数据库。

### 8.3 请求之间的依赖

请求依赖采用两层方案：

1. **API 可观察结果**：runner 从上一个 response 提取 user/project/canvas/resource/upload/task/token/revision/cursor，再作为下一个请求输入；这验证 wire 类型和 ID 可用性。
2. **H2 durable state**：Java 端把受理后的实体、revision、history、share、resource、upload、task、event 和 request result 写入 H2；下一个请求从 owner port 读取，不依赖 Controller 内存字段。

因此，必须实现最小状态转换，但不实现真实业务策略。比如 `PUT canvas` 需要 revision CAS、history preimage 和 reference 检查，`POST upload complete` 需要 chunk 状态和一次性完成状态；这些是测试合同所需的可观察状态，不是完整产品逻辑。

## 9. Request ledger、重放和 response-loss

### 9.1 ledger 字段

`RequestLedgerEntity` 至少保存：

- `operationId` / contract profile / policy version；
- actor/session scope、resource identity、公共 `X-Idempotency-Key`（若有）和 request fingerprint；
- `PREPARED`、`APPLIED`、`REPLAYABLE`、`CONSUMED`、`UNKNOWN` 等 reference 状态；
- response status、typed response snapshot 或关联实体 ID；
- appliedAt、createdAt、fixture fault marker、process generation；
- 仅用于测试恢复的脱敏摘要，不保存 Cookie、Authorization、密码、签名 URL 或完整敏感正文。

### 9.2 不同 operation 的策略

不能用一个全局“所有 POST 都幂等”的 middleware：

| operation 类别 | reference policy |
|---|---|
| canvas PUT/CAS | 由 canvas revision 保护；response loss 后重试旧 revision 返回 conflict，不新增 history/revision |
| upload chunk | `uploadId + chunkIndex` 范围内可安全 replay；内容 hash/length 不匹配拒绝覆盖 |
| upload complete | `complete-once`；当前 response-loss 场景在提交后重启，再次 complete 返回 `404/not-found`，不再生成资源 |
| resource import | 只有提供批准/观测要求的 idempotency key 才从 ledger 返回同一 resource；同键异 fingerprint 拒绝 |
| direct resource create | 当前 response-loss 场景中，同一个 `X-Idempotency-Key` 的后续 multipart POST 必须返回一个新的 distinct ready resource；这是待批准的 Go 观察，不得因为存在 header 就擅自升级成全局去重，ledger 只记录证据而不强制 replay |
| project create | 同一用户重复 project name 返回 `409/project_name_conflict`；未冻结公共 idempotency key，未知结果不自动重试 |
| unit create/import | 当前观察为每次请求追加 fresh unit identity、推进一次 revision；没有安全的自动重试，ledger 不把相同正文当作同一业务操作 |
| fixed import conversion | 纯 conversion mock 可以重复生成新 batch；不自动持久化 canvas/history/resource，除非 scenario 明确要求 |
| read/conditional/SSE replay | 只读；cursor/ETag/Last-Modified 是独立身份，不与 request ledger 混同 |

任何 `UNKNOWN` 结果都不能偷偷重新发起非幂等外部副作用；reference stub 没有真实外部副作用时，也要把 `UNKNOWN`/response-loss 行为显式模拟，避免未来实现把测试桩的便利误当成生产安全证明。

### 9.3 H2 restart 要求

重启验收必须：

1. 以隔离 data directory 启动 jar；
2. 通过真实 TCP HTTP 完成写入或 response-loss fault；
3. 强制 crash 或 graceful stop；
4. 使用相同 jar、相同 H2 file path、相同配置重新启动；
5. 验证 canvas/history/resource/share/project/upload/task/event 和 ledger 的可观察状态；
6. 验证旧 Cookie 是否按 reference policy 失效或恢复，不把内存 session 恢复误写成生产集群能力。

## 10. mock provider、文件和流式边界

### 10.1 资源和文件

- mock media 使用固定 PNG/文本/JSON bytes；内容、size、mime、checksum 和 named filename 来自 `MockMediaFixtures`。
- resource bytes 可以存 H2 BLOB 或当前 run 的受控文件目录；数据库记录是 resource identity 和状态 source of truth，物理 mock object 只作为投影。
- `GET /file` 的 descriptor、bytes、Range、If-None-Match、If-Modified-Since、304、416、expiry、redirect 和故障注入分别测试；不能用 JSON 200 代替二进制 wire。
- 不生成真实签名 URL，不把 URL 当授权身份；签名/expiry 仅作为可观察的 mock capability record。

### 10.2 import mock

定义 `ImportGateway` port 和两个 reference adapter：

- `DeterministicImportGateway`：默认返回固定 libtv/tapnow 转换结果，适合单测和无外部进程运行。
- `LoopbackContractImportGateway`：contract profile 可选，用于连接现有固定 HTTPS mock fixture，验证 TLS/host allowlist/timeout/invalid JSON/oversized/redirect/HTTP failure；只允许显式 loopback/fixture host，不允许任意 URL。

Java HTTP client 必须显式设置 connect/read/overall timeout、关闭自动重试，并记录“未发出 / 已受理但结果未知 / 已得到终态”的状态；不能因超时自动再次调用非幂等 import。

### 10.3 generation 和 SSE

- `MockGenerationGateway` 不调用 Provider，只从 seed 和请求 DTO 生成 deterministic task/text delta/event。
- text delta 写入 `TextDeltaEntity`，event 写入 `TextEventEntity`，cursor 为递增 event id；不能把 task id 当游标。
- `GET /api/tasks/{id}/text-events` 使用 `SseEmitter`，支持 `Last-Event-ID` 或 `after`，断线后从 H2 event 表重放。
- SSE 连接是 projection/wake-up，不是 durable owner；进程重启后必须能从 H2 重新读取事件。
- 浏览器 companion 不作为本 profile 的必要启动条件；HTTP runner 可以直接验证 task/event wire。

## 11. OpenAPI / Swagger 规划

### 11.1 注解覆盖

每个公开 Controller 类使用 `@Tag`，每个方法使用 `@Operation`、`@ApiResponse` / `@ApiResponses`，参数使用 `@Parameter` / `@RequestBody`，DTO/字段使用 `@Schema`；枚举、错误 reason、分页、ETag、Range、SSE 和二进制 media type 必须写在注解或配置中。

至少为以下内容提供说明：

- 认证方式、Cookie/session 作用域、anonymous public share 例外和 owner/foreign 行为；
- 请求 body 必填/可选/null/省略语义、大小/数量/整数边界和 trailing JSON 拒绝；
- 成功 envelope、错误 envelope、HTTP status、`code`、`reason` 和可重试性；
- revision/CAS、history、idempotency/replay、response-loss 和 cursor 语义；
- `Content-Encoding`、ETag、304 空 body、Range/416、Content-Disposition 和 redirect；
- reference-only 限制：mock data、无真实 Provider、H2 非生产证据、私有 fixture 不可用于生产。

### 11.2 operationId 映射

- Required operation 的 `@Operation(operationId = "...")` 优先使用 manifest 中的 canonical operation ID，例如 `canvas.read`、`canvas.update`、`canvas.resource.import`。
- 一个 method/path 可对应多个 scenario，但不能为每个 scenario 复制 Controller route；scenario 映射留在合同测试 manifest。
- setup API 使用 `reference.auth.login` 等前缀，私有 fixture 使用 `reference.fixture.*`；它们不伪装成画布业务 operation。
- OpenAPI 生成结果必须做静态校验：operationId 唯一、manifest Required operation 可映射、方法/path/参数一致、DTO schema 非 `object` 无约束逃生舱。
- 若当前 Go OpenAPI 与 runner/前端事实冲突，记录差异并在参考桩 README/合同计划中链接；不为了生成文档而静默改写 wire。

### 11.3 文档入口

实现完成后提供：

- `/api/openapi.yaml`：供项目和现有文档入口读取的 YAML；
- `/v3/api-docs` / `/v3/api-docs.yaml`：springdoc 原生入口；
- `/swagger-ui/index.html`：本地开发查看，不作为黑盒通过证据；
- README 中的 profile、seed、H2、fixture 和限制说明。

## 12. 测试和验证策略

### 12.1 测试层次

| 层次 | 命令/工具 | 证明什么 | 不能证明什么 |
|---|---|---|---|
| 编译 | `./mvnw -q -DskipTests package` | Java source、依赖和 jar 可构建 | API 行为、重启、覆盖 |
| 架构 | `./mvnw test -Dtest=*Architecture*` | 包边界、无动态公共签名、无循环依赖 | H2 真实状态、HTTP |
| DTO/wire | `./mvnw test -Dtest=*Wire*` | JSON key、null、省略、数字、错误 envelope、SSE event shape | 真实端口、前端消费 |
| H2 state | `./mvnw test -Dtest=*Persistence*` | seed、owner、revision、history、ledger 状态可读写 | PostgreSQL 等价、生产并发 |
| MockMvc | `./mvnw test -Dtest=*Api*` | Controller mapping、validation、注解和局部状态 | TCP proxy、jar 重启、完整 scenario |
| jar 冒烟 | `scripts/run-reference.sh` + `curl` | 真实进程、health、login、OpenAPI、H2 file | 完整 Required coverage |
| HTTP 主套件 | `scripts/run-http-contract-suite.sh --profiles=...` | 现有 Go runner 对 Java 目标的外部 wire/状态结果 | 代码覆盖率、生产数据库、浏览器 |
| 重启矩阵 | runner `restart-recovery` | 同一 H2 file 的 crash/graceful/response-loss recovery | PostgreSQL lock/RLS、真实外部副作用 |
| 浏览器 companion | 明确启动 Web + Playwright 后单独运行 | 真实前端 API 调用链、缓存和页面结果 | 替代 HTTP endpoint 覆盖 |

### 12.2 黑盒套件执行方式

`scripts/run-http-contract-suite.sh` 负责：

1. 检查 JDK/Maven/jar、loopback 端口和干净的 contract data directory；不杀非本次进程。
2. 构建或接收已构建 jar，生成至少 24 字节的 control/fixture token，不把 token 写入日志。
3. 为每个 profile 建立独立 H2 file、日志、report 和 run-id；不把不同 profile 的请求数合并成一个 coverage 数字。
4. 设置 `CANVAS_CONTRACT_BASE_URL`、`CANVAS_CONTRACT_PROFILE`、用户/admin credentials、`CANVAS_CONTRACT_FIXTURE_URL`、`CANVAS_CONTRACT_FIXTURE_TOKEN`，必要时设置 import mock upstream。
5. 使用现有 `tests/canvas-contract` runner；不复制一份 Java 专属断言。
6. 对失败报告记录 profile、operationId、scenario、request sequence、HTTP status/headers、脱敏 response summary、H2 data path 和源码/jar hash。
7. 退出时只清理当前 run 创建的进程和临时 fixture，不删除用户指定的持久 reference data。

推荐执行顺序：`core` → `share-public` → `project-integration` → `project-workbench-core` → `resource-integrity` → `import` → `legacy-assets` → `public-resource` → `restart-recovery` → `generation-boundary` → `generation-writeback`。

### 12.3 通过判定

“通过”必须同时写清：

- profile 是否执行，是否有 skip/incomplete；
- manifest Required operation 是否都有 scenario 映射；
- 实际运行的 HTTP 请求数、wire 响应数、受控故障数、response-loss 数、restart generation；
- 哪些行为是当前 Go 观察兼容，哪些是 reference policy，哪些仍是 pending decision；
- 是否使用 H2、mock upstream、mock object bytes、fake session；
- 是否运行浏览器 companion；若没有，明确这不影响 HTTP 主套件结论。

不能把“所有 Controller 有单测”“健康检查 200”“MockMvc 绿色”“OpenAPI 可生成”或“core profile 绿色”写成完整黑盒通过。

## 13. 分阶段实施任务

### Phase 0：项目和合同骨架

- 创建 Maven wrapper、`pom.xml`、主类、配置 profile 和 README。
- 引入 WebMVC/Jackson/Validation/JPA/H2/Flyway/springdoc/test 依赖，锁定版本。
- 建立 `ApiEnvelope<T>`、error DTO、request context、typed response mapper。
- 建立 OpenAPI 配置和 `/api/openapi.yaml` 入口。
- 建立 ArchUnit 规则，先禁止动态公共 API 签名和 Controller 直连 repository。

**退出标准**：项目可编译，health/OpenAPI 可访问，公开方法签名静态规则已失败即停。

### Phase 1：身份、seed、H2 durable foundation

- 建立 schema migration、H2 file configuration、seed version 和 deterministic dataset。
- 实现 mock Cookie session、owner/foreign/admin/anonymous context；不启用默认 form login，不接受客户端自报 userId。
- 实现 setup/admin DTO 和最小端点。
- 实现 request ledger entity/repository/typed port 和 fixture token filter。

**退出标准**：新进程能从同一 H2 file 读回 seed、session policy、ledger；不同用户不能读取对方 seed 数据。

### Phase 2：Canvas、share、project integration

- 先实现 `core` 的 13 operations，包括 list/read/PUT/CAS/history/restore/delete/ETag/gzip。
- 实现 `share-public` 的 token、allowlist、expiry/revoke/public read/resource。
- 实现 `project-integration` 的 list/link/unlink 和 project/unit ownership。
- 加入 Canvas document typed polymorphism、unknown node/extension preservation 和 resource reference validation。

**退出标准**：`core`、`share-public`、`project-integration` 的 TCP HTTP profile 对 current runner 通过，且 H2 restart 能恢复 history/share/link。

### Phase 3：Project/workbench core

- 实现 project/unit CRUD、feature gate、workspace、bulk import、reorder、delete cascade 的最小状态转换。
- 对 duplicate/non-idempotent POST、body size、Unicode/title、role、cross-project 和并发结果按 manifest scenario 建立 policy。
- 不扩展到 catalog-only 的 `project-workbench` 35 operations。

**退出标准**：15 Required operations 和其 scenario 全部有 typed DTO、OpenAPI、runner 映射和 state assertions。

### Phase 4：Resource、upload、public delivery

- 实现 resource metadata/access/list/usage/create/import/read/file、upload reservation/chunk/complete。
- 实现 limits、quota mock、chunk checksum/length、completion once、resource reference protection。
- 实现 mock file bytes、signed capability record、Range、304/416、named filename 和 descriptor representation。
- 把 object storage failure/recovery 限定为 local mock fixture，不冒充真实 CDN/OSS。

**退出标准**：`resource-integrity`、`public-resource` profile 通过；`restart-recovery` 的资源、upload 和 usage 场景能够从 H2 恢复。

### Phase 5：Import、legacy assets、restart/recovery

- 实现 fixed mock import gateway、libtv/tapnow typed request/response、失败/空转换/timeout/oversized/redirect/SSRF policy。
- 实现 legacy asset/folder/batch/snapshot 和引用保护；物理删除只操作 mock object state。
- 实现 response-loss fault marker、replay ledger、crash/graceful restart 脚本和同一 data directory 验证。

**退出标准**：`import`、`legacy-assets`、`restart-recovery` 完整执行；未决 Go 行为以报告标注，不静默批准。

### Phase 6：Generation boundary/writeback

- seed owner/foreign task、creation-run、agent run/profile/memory。
- 实现 task/run/agent read boundary，明确 unauthenticated、owner、foreign、cross-user 结果。
- 实现 provider-free task create、text delta create/list、completion 和 SSE cursor replay。
- 不实现真实 Provider、Agent runtime、媒体回写或 generation runtime 独立 suite。

**退出标准**：`generation-boundary`、`generation-writeback` 通过，SSE 可在进程重启后从 H2 事件重放。

### Phase 7：文档、审计和交付

- 运行 Maven、静态架构、wire、jar smoke、HTTP suite 和 restart 验证。
- 对照 manifest 检查 operationId/path/DTO/OpenAPI/runner 双向映射。
- 更新 Java README、合同测试计划、迁移指引和文档索引；记录 H2/reference-only 边界。
- 对发现的计划或实现问题按“发现即修复、重新开始检查”的严格流程处理。

**退出标准**：实现证据、限制、未覆盖项、运行命令和文档入口都可追溯；没有用局部绿色冒充整体完成。

## 14. 验收矩阵

| 不变量 | 单元/静态 | H2 integration | TCP HTTP runner | restart/fault | 结果要求 |
|---|---|---|---|---|---|
| 公共 API 无动态签名 | ArchUnit、反射扫描 | N/A | OpenAPI schema scan | N/A | BLOCKER，失败即不能交付 |
| envelope/error/reason | DTO/wire golden | N/A | 所有 Required operation | response loss | status、code、reason 与类型一致 |
| owner/foreign/anonymous 隔离 | policy unit | 双用户 H2 | authz scenarios | restart session policy | 不泄露对象存在性或正文 |
| canvas CAS/history | transition unit | revision/history repository | core | lost response/restart | 不重复 revision/history |
| share expiry/revoke | token policy unit | share rows | share-public | fixture expiry/restart | token、expiry、allowlist 可判定 |
| project/unit/link | role policy unit | link uniqueness | project profiles | restart | 无跨项目/foreign 写入 |
| resource/upload | quota/chunk unit | bytes/reservation | resource profiles | response loss/restart | usage、chunk、completion 一致 |
| public file wire | range/signature unit | mock bytes | public-resource | expiry/restart | bytes/header/304/416 正确 |
| import boundary | gateway unit | no auto persistence unless scenario | import profile | timeout/failure | 无真实上游、无未知重试 |
| legacy protection | reference policy unit | asset references | legacy profile | restart | 引用存在时不误删 |
| task/writeback/SSE | cursor/event unit | event rows | generation profiles | restart/replay | cursor 单调且可重放 |
| request ledger | policy/static | persisted ledger | response-loss | crash/restart | 只按 operation policy 重放 |
| OpenAPI alignment | operationId scan | N/A | catalog check | N/A | manifest/Controller/DTO 三方一致 |

### 14.1 退出条件和失败处理

- 任一 Required profile 出现未解释的 fail、skip、incomplete、未知路由或 schema mismatch，reference stub 不得标记完成。
- 发现计划与代码/测试事实冲突时，先修改本规划或相关事实 owner 文档，再重新执行从 Round 1 开始的三轮检查；不能只在交付消息中口头解释。
- 发现与 H2 特性相关的假阳性时，必须把测试降级为“reference smoke”并补 PostgreSQL/生产迁移的 deferred 条目，不把它记为 Java 生产兼容。
- 发现需要真实前端才能解释的失败时，分离 HTTP runner 结果和 browser companion 结果；不把前端缺陷归咎于后端 DTO，也不把浏览器通过归因于所有 API 已覆盖。

## 15. Deferred、风险和安全默认

| 项目 | 状态 | 安全默认 | 触发器/最迟决策点 |
|---|---|---|---|
| PostgreSQL schema/锁/RLS 等价 | deferred | 不宣称等价；Java 生产验收必须用真实 PostgreSQL | 开始生产 Java 迁移或切流前 |
| 真实 Provider/Agent | deferred | mock，禁止网络和付费调用 | 首个真实 Provider operation 进入 Java |
| 35 个 project-workbench catalog-only operation | deferred | 不注册或返回明确 unsupported，不伪装成功 | 产品要求该 API 进入 Java cutover 前 |
| provider-resource/agent-canvas/generation-runtime | out of scope | 由独立 suite 和迁移阶段负责 | 独立 suite 具备 owner 和 fixture 后 |
| session 集群/多实例 SSE | deferred | 单实例 loopback；事件以 H2 durable record 为准 | 需要多实例部署前 |
| H2 BLOB vs mock file directory | reference decision | 以 H2 resource identity 为 source of truth，mock bytes 可替换 | 需要测量大文件或生产对象存储时 |
| OpenAPI 与 Go 当前缺陷冲突 | pending decision | 记录 observed 与 target invariant 两列，不静默选择 | 合同 owner 批准 wire 版本前 |
| 非幂等 POST 的 idempotency | pending decision | 按 manifest replay 字段逐 operation 处理，UNKNOWN 不自动重试 | Java 生产 operation catalog 冻结前 |
| 私有 fixture 是否单独端口 | reference decision | 默认同一 loopback origin 的隐藏 namespace + token；可替换为 sidecar | runner/部署需要独立 origin 时 |

## 16. 文档互链和维护责任

本规划完成后，必须在以下位置增加入口或反向链接，避免未来开发者重新猜测：

- `docs/index.md`：增加“画布 Java 类型参考桩实施规划”入口。
- `docs/drafts/README.md`：在画布/迁移草稿列表增加本文件，并说明它是实施计划，不是已完成 Java 项目。
- `docs/drafts/canvas-java-http-contract-test-plan.md`：在 Java 验收或当前进度附近链接本规划；该计划继续拥有 Go baseline、manifest、scenario 和 Java HTTP gate 的事实账本。
- `docs/drafts/go-to-spring-boot-migration-guide.md`：在 M1/画布专项处链接本规划；迁移指引继续拥有生产 Java 的事实 owner、事务、切流和回滚原则。
- `tests/canvas-contract/README.md`：Java reference stub README/脚本建立后，链接其独立运行方式；不把 Java 项目变成 Go runner 的内部依赖。
- `docs/drafts/canvas-development-navigation-audit.md`、`docs/drafts/canvas-deep-research.md`：只在相关画布状态/迁移入口增加链接，不复制完整 DTO 清单。

维护触发器：

1. manifest 新增/删除/改 path、operationId、replay 或 profile：同步检查 Java Controller、DTO、OpenAPI、runner 结论，并建立新的 run-id。
2. Go handler、Web API client、OpenAPI 或 fixture 合同改变：先更新合同测试计划和事实账本，再修改 reference stub。
3. 新增 Entity/迁移：更新本文件的事实 owner、seed、restart 和 DTO 影响；数据库 schema 变更同步正式数据库文档（若未来进入生产迁移）。
4. 新增 dynamic JSON 字段、异步 task、外部调用或 callback：必须重新做强类型、operation catalog、ambiguity policy 和安全审查。
5. reference stub 任何“为了通过测试”的特殊行为：在代码、README 和本规划中标记为 `REFERENCE DECISION`，并链接对应 scenario，不得沉淀为无名 workaround。

## 17. 实施前检查清单

- [ ] 确认 `tests/canvas-contract/contract/manifest.yaml` 当前 163/143/91/118 统计未漂移。
- [ ] 确认 11 个 Required profile 与 runner `ConfigFromEnv` 支持列表一致。
- [ ] 确认 setup API、private fixture、target-control 和 H2 data directory 的启动合同。
- [ ] 确认 WebMVC 而非 WebFlux，SSE 采用 `SseEmitter`，不把浏览器作为 HTTP 主套件依赖。
- [ ] 确认所有公共 Controller 方法和 DTO 的禁止动态类型规则可以通过静态扫描执行。
- [ ] 确认 `spring.jpa.open-in-view=false`、迁移策略、seed marker 和 H2 file restart 方案。
- [ ] 确认 request ledger 的 operation-specific replay policy，不采用全局 POST 幂等 middleware。
- [ ] 确认 import、file、Range、304/416、gzip、ETag、SSE、response-loss 和 restart 都有对应 wire/state test。
- [ ] 确认每个 deferred/out-of-scope 范围有安全默认、触发器和最迟决策点。
- [ ] 确认实现后更新本文件的“已验证/未验证”状态，而不是提前把规划写成成果。

## 18. 规划阶段验证记录

以下是本规划文档在当前 checkout 上实际执行过的静态/现状检查；它们验证的是规划依据和文档一致性，不代表 Java 项目已经创建或通过 HTTP 黑盒套件：

| 时间 | 检查 | 结果 |
|---|---|---|
| 2026-10-13 | 解析 `tests/canvas-contract/contract/manifest.yaml`，核对 operation、唯一 method/path、Required profile/operation/scenario、Deferred 和 OUT_OF_SCOPE 数量 | `163 / 143 / 11 / 91 / 118 / 35 / 37` 与文档一致 |
| 2026-10-13 | `bun test tests/canvas-contract/check-catalog.test.ts` | 36 pass、0 fail |
| 2026-10-13 | `PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s tests/canvas-contract/browser -p '*_test.py'` | 9 pass、0 fail |
| 2026-10-13 | setup/auth/session、admin fixture、private fixture 和 11 个 runner profile 源码交叉核对 | 计划已覆盖实际调用路径；setup 不计入 91 个画布 operation |
| 2026-10-13 | 新规划文档链接、索引、章节顺序、缺口 ID、敏感机器路径和 `git diff --check` | 通过 |

尚未执行且不能提前宣称通过：

- `java/canvas-contract-reference/` 尚未创建，因此没有 Maven compile/test、OpenAPI export、jar smoke 或 H2 restart 结果。
- 没有以 Java target 运行 `tests/canvas-contract` 的任何 profile，因此不能宣称 Java API、DTO、wire 或 response-loss 已兼容。
- 没有启动真实 Web 前端/Chromium，因此 browser companion 的 Java 直连或混合网关验收仍未完成。
- 没有执行真实 PostgreSQL、远端 CDN/object-store、Provider、Agent runtime 或生产切流验证；这些边界仍按 G-01～G-14 和 deferred 表管理。

本文件的严格三轮内容审计按用户要求在本轮对话中逐轮输出；若审计发现规划事实、范围、逻辑或缺口登记问题，必须先修改本文并把连续无修改计数重置为 `0/3`。

## 19. 最终交付判断

```text
IF THIS WERE MY REFERENCE STUB TODAY

Keep:
- 现有 Go HTTP runner 作为唯一黑盒执行器，不复制 Java 专属断言。
- 强类型 DTO、显式 operationId、OpenAPI 注解和独立 DTO/entity 边界。
- H2 file + seed + request ledger，让状态依赖和重启行为可重复。
- 按 Required profile 分阶段实现，Deferred/OUT_OF_SCOPE 保持可见。

Change:
- 只实现合同所需的最小状态机，不带入 Go 内部包结构和生产缺陷。
- 用 deterministic mock adapter 替代 Provider、对象存储、Agent 和 import 外部服务。
- 把 browser companion 明确放在 HTTP 主套件之后，作为前端消费链补充验收。

Reject:
- 用 Map/Object 快速接 API。
- 用内存状态或重新 seed 冒充 restart recovery。
- 用 H2/MockMvc/health 200 冒充生产 PostgreSQL、并发、RLS 或 Java 迁移通过。
- 用 core profile 或 OpenAPI 生成成功冒充 91 operation / 118 scenario 全部通过。

First implementation steps:
1. 建 Maven/WebMVC/OpenAPI/H2 schema 和架构护栏。
2. 建 typed envelope、auth/session、seed、request ledger 和 canvas core。
3. 用现有 HTTP runner 逐 profile 实跑，失败即回到对应 DTO/state/wire 证据修正。
```

本文件只是实施规划；完成后的 Java 项目、运行报告、未覆盖项和行为差异必须分别记录在 `java/canvas-contract-reference/README.md`、合同测试计划和迁移指引中。
