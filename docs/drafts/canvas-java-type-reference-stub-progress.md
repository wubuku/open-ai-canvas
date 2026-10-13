---
title: 画布 Java Spring Boot 类型参考桩实施进度
status: implementation-complete-reference-stub
scope: java-reference-stub-progress
updated: 2026-10-14
audience: 继续实施画布 Java 类型参考桩的开发者、评审者和合同测试维护者
---

# 画布 Java Spring Boot 类型参考桩实施进度

> 本文是实施接续账本，不替代实施规划。每次关键阶段都记录实际命令、证据和未解决阻塞项；没有证据的内容不得写成“通过”。

## 1. Goal 与交付边界

创建 `java/canvas-contract-reference/`：一个可启动的 Maven + Spring Boot + Spring WebMVC + H2 file + Flyway + seed/mock 项目。它只为未来 Java 移植版提供**可运行的类型/wire/OpenAPI 参考**并接受现有 HTTP 黑盒合同套件；不实现真实业务、Provider、对象存储、Agent 或生产 PostgreSQL 行为。

交付分母仍以规划文档冻结的 11 个 Required profile、91 个 Required operation、118 个 HTTP scenario 为准；Deferred 和 OUT_OF_SCOPE 不能被局部绿色结果掩盖。

## 2. 不可降级的核心类型要求

涉及画布文档直接操作的端点必须使用强类型领域 DTO，并在 Controller 方法、DTO 类型和字段上使用 OpenAPI/Swagger 注解。以下任何一种在核心公共 API 中出现，都算阻塞问题，必须修复后才能进入最终验收：

- `Map<String, Object>`、`List<Object>`、裸 `Object`；
- `JsonNode` 或其子类作为整个画布请求/响应、核心方法参数或返回值；
- 一个泛化 `WireData`/`ReferenceObjectDto` 承载多个核心资源而没有领域类型；
- 缺失唯一 `operationId`、`@Operation`、成功/错误 `@ApiResponse` 或 DTO `@Schema`。

允许例外：persistence 层的 JSON codec、明确命名且受限的 `OpaqueJsonValue` 扩展字段，以及只服务 setup/fixture/mock 配置的辅助端点。例外必须在代码和规划文档中说明用途，不能用于逃避画布核心建模。

这样做的目的，是让读者通过 Controller 定义或 OpenAPI 文档直接获得 wire format 参考信息，而不必深入探索弱类型实例的创建路径。只有 API 的设计意图本身就是版本化/客户端自定义动态扩展时，才可使用弱类型；这类例外必须有命名字段、文档说明和边界测试。

## 3. 当前完成度

截至 2026-10-14，类型参考桩本轮总体进度为 **100%（reference-stub 交付范围）**：Java 项目、强类型核心 API、H2 durable state、11/11 Required profile、OpenAPI 静态审计、重启后专项复验和文档交付均已完成。这个百分比只表示本轮 reference-stub 交付范围，不是 API 或代码覆盖率，也不表示生产 Java 迁移完成。

| 阶段 | 状态 | 证据/说明 |
|---|---|---|
| Maven/Spring WebMVC/H2/Flyway 骨架 | 已完成 | `mvn -q -DskipTests package` 已通过；JDK 23、Maven 3.9.14 |
| Auth/admin/setup 最小路径 | 已完成（Required gate 最小路径） | 现有 Go runner 已成功完成 register、admin users、login；完整生产管理能力不属于本桩目标 |
| H2 file + seed + 重启持久化骨架 | 已完成（reference 边界） | Flyway V1、H2 file、seed、crash/graceful restart 和已提交状态恢复已由 `restart-recovery` 验证；不等价于生产 PostgreSQL/多实例证明 |
| 画布核心状态机 | 已完成 core 基线 | create/read/list/CAS/history/restore/delete/ETag、分页排序、压缩、错误边界已由 core 黑盒覆盖 |
| 画布核心强类型 DTO 重构 | 已完成核心公共面 | 核心 Controller 不再使用完整 `JsonNode`/`WireData` 作为 request/response；内部 state codec 和命名 `OpaqueJsonValue` 例外仍受约束 |
| Required 11 profiles | `11/11` 已验证 | 118 个子场景全部通过；profile 和证据文件见 `java/canvas-contract-reference/README.md` |
| OpenAPI/Swagger 对齐与静态门禁 | 已完成（reference 边界） | 87 个 operationId 唯一；Required 91 operations 归并为 71 条唯一路由且全部映射；核心公共面无禁止动态签名 |
| README/运行报告/文档链接 | 已完成本轮 | Java README、规划、进度、合同计划、迁移入口和索引已互相链接；本轮最终检查后不再声称生产迁移通过 |

## 4. 已执行验证与结果

### 2026-10-13：第一次 Java core 黑盒尝试

- 命令：`mvn -q -DskipTests package`，通过。
- 启动：`mvn -q spring-boot:run`，H2 file、Flyway、Tomcat 18183 启动成功。
- 命令：`go test ./... -run '^TestCanvasContract$' -count=1 -v`，环境为 `CANVAS_CONTRACT_PROFILE=core`、注册用户模式、Java reference target。
- 结果：未通过。register/admin/login 成功；core 请求大部分已到达 Java，但 wire schema 报错，主要是错误 envelope 的 `data` 被 Jackson 非空策略省略，以及 canvas summary/document/history 缺少合同必需字段。
- 处理：已加入 envelope 的显式 `data: null`、canvas `createdAt/updatedAt/nodeCount/previewNodes` 和 history summary 字段；这些修改随后由 `core-java-7` 完整验证。
- 额外发现：当时核心 request/response 仍使用 `JsonNode`/`ReferenceObjectDto`；该问题已在后续强类型重构中关闭。

## 5. 当前接续顺序

1. 完成本轮文档交叉复核，保证进度、规划、合同计划和迁移入口没有把旧阶段结果当成当前状态。
2. 提交并推送当前 Java reference stub 与文档；工作区恢复干净。
3. 真实 Java 业务移植开始后，复用同一 runner，另行证明 PostgreSQL、混合网关、远端对象存储、Provider、Agent、SSE 断线恢复和切流回滚。

## 6. 当前阻塞项与边界

- B-01：reference stub 的 11 个 Required profile 已关闭；manifest 的 deferred/separate-suite、Required `remaining_paths`、远端生产门禁和真实 Java 业务迁移仍未关闭。
- B-02：核心 DTO 的 OpenAPI/强类型门禁已通过本轮静态审计；后续新增核心字段必须重新运行同一审计。
- B-03：`/api/openapi.yaml` 不是 Java 桩的生成入口；Java 实际生成文档为 `/v3/api-docs`，运行/限制说明已记录在 `java/canvas-contract-reference/README.md`。

## 7. 维护规则

- 每完成一个 profile 或发现一个行为差异，先更新本文，再继续下一 profile。
- 任何为了 runner 通过而添加的特殊行为，写明对应 scenario、状态前提和“仅 reference stub”边界。
- 不把浏览器 companion 的通过写成 Java HTTP 主套件通过；不把 H2 结果写成 PostgreSQL/生产迁移证明。

### 2026-10-13：核心强类型重构与 core profile 通过

- 修正前置状态：加入私有 fixture `POST /reset`，避免 H2 文件中上一次运行的账号/会话污染下一次运行。
- 核心公共签名已从 `CanvasWriteRequest(JsonNode)` / `WireData.ReferenceObjectDto` 改为领域 DTO：`CanvasDocumentDto`、`CanvasNodeDto`、`CanvasConnectionDto`、`CanvasSummaryDto`、`CanvasHistorySnapshotDto`、`CanvasWriteRequest`、`CanvasRestoreRequest`、`CanvasLibraryResponse`、`CanvasHistoryResponse`、`CanvasHistoryReadResponse`。
- `JsonNode` 只留在内部 H2 JSON codec/状态转换，以及命名的 `OpaqueJsonValue` 扩展字段；不再作为完整画布请求/响应的 Controller 公共类型。
- 命令：`mvn -q -DskipTests package`，通过。
- 命令：`CANVAS_CONTRACT_PROFILE=core ... go test . -run '^TestCanvasContract$' -count=1 -v`，`core-java-7`，通过。
- 结果：`TestCanvasContract` 及 17 个 core 子场景全部 PASS；wire schema、认证、CAS 并发、ETag/304、gzip、分页排序、历史 coalescing/20 条保留、非法输入、注销均通过。
- 当日阶段性记录：share/public、project/workbench、resource/upload、import、legacy asset、public resource、restart、generation 两个 profile 尚未完成；后续已在 2026-10-14 的 11/11 总账中关闭 reference stub gate。

### 2026-10-13：share-public profile 通过与两处合同修复

- 首轮 `share-java-10` 暴露两个问题：并发首次创建错误地返回两个成功管理结果；S3/mock object-storage 307 重定向缺少合同要求的隐私/安全响应头。
- 修复 `CanvasShareController`：对无既有 token 的首次创建增加进程内竞态闸门，使并发请求稳定表现为一个成功结果、一个合同允许的 `500 internal`；顺序重复创建仍返回当前 share，不改变 rotate/revoke 语义。
- 修复 `ReferenceCompressionFilter` 与 `ResourceController`：公开分享资源重定向保持 `private, no-store`、`Referrer-Policy: no-referrer` 和 `X-Content-Type-Options: nosniff`；普通本地分享资源仍保持 `Cache-Control: no-store`。
- 命令：`mvn -q -f java/canvas-contract-reference/pom.xml -DskipTests package`，通过。
- 聚焦验证：`canvas.share.concurrent-first-create`、`canvas.share.concurrent-rotate-and-revoke`、`canvas.share.resource-object-storage-failure` 均通过。
- 完整验证：`CANVAS_CONTRACT_PROFILE=share-public ... CANVAS_CONTRACT_RUN_ID=share-java-13 go test . -run '^TestCanvasContract$' -count=1 -v`，通过；10 个 share/public 子场景全部 PASS，包含 local resource allowlist、expiry/recovery、revoke、并发 rotate/first-create、mock object-store redirect、故障和恢复。
- 结论：`share-public` 进入 `2/11` 已验证；该结果只证明当前受控 HTTP fixture 和 H2 file reference stub 的合同兼容性，不证明生产 CDN、S3、TTL 或 PostgreSQL 等价性。

### 2026-10-13：project-integration profile 通过与并发状态修复

- 首轮 `project-roles-debug` 暴露：核心关联 `role` 的非字符串 JSON 值被 Jackson 默认强制转换为字符串；逐 unit 解除关联错误地递增画布 revision；已有 link 的超长 role 没有保持合同要求的 `500 internal`；同 key 首次并发拒绝请求缺少观察到的部分画布状态；并发请求偶发 JPA `ObjectOptimisticLockingFailureException`。
- 修复：新增字段级 `StrictStringDeserializer`，使核心 `CanvasLinkRequest.role` 仅接受 JSON string；逐 unit unlink 只更新项目关联 revision，不修改画布文档 revision；所有超长 role 和同 key 首次并发拒绝都保留画布 assignment/revision 的 reference-stub partial state；移除 `ReferenceStore.update()` 外层事务，让 `saveAndFlush()` 在 `synchronized` 状态锁内提交，避免锁释放后才提交导致的 JPA version 冲突。
- 命令：`mvn -q -f java/canvas-contract-reference/pom.xml -DskipTests package`，通过。
- 完整验证：`CANVAS_CONTRACT_PROFILE=project-integration CANVAS_CONTRACT_REGISTER_USERS=true CANVAS_CONTRACT_IMPLEMENTATION=java-reference CANVAS_CONTRACT_DB_PROFILE=h2-file CANVAS_CONTRACT_RUN_ID=project-integration-20261013-c CANVAS_CONTRACT_SETTINGS_FIXTURE=true go test ./... -run '^TestCanvasContract$' -count=1 -v`，通过；`project-integration` 的 9 个子场景全部 PASS：foreign、validation、feature-gate、lifecycle、cross-project-and-reassociation、multi-unit-roles-and-replay、role-overflow-partial-state、concurrent-link-settlement、concurrent-cross-project-reassociation。
- 运行隔离：服务使用全新的 H2 file 数据库；本次启动确认 Flyway v1、Tomcat `18183` 和 seed state 正常。此前测试因复用旧 H2 文件导致 bootstrap 用户冲突，不计为实现失败，后续已改用独立数据库重跑。
- 当日结论：`project-integration` 进入 `3/11` 已验证；这是 2026-10-13 的阶段性记录，不能覆盖后续 11/11 总账。并发通过只证明 reference stub 在当前单进程 H2/JPA 模型下复现已批准的 wire/状态观察，不等价于生产多实例并发保证。

### 2026-10-14：11 个 Required profile 与 OpenAPI 门禁完成

以下 11 个 profile 均以 jar 作为 TCP HTTP target 运行；总计 118 个子场景通过。各 profile 的命令环境相同，仅 `CANVAS_CONTRACT_PROFILE`、`CANVAS_CONTRACT_RUN_ID` 和隔离 H2 数据目录不同；完整命令形状与日志索引见 [`java/canvas-contract-reference/README.md`](../../java/canvas-contract-reference/README.md)。

| Profile | 子场景 | 结果 |
| --- | ---: | --- |
| `core` | 17 | PASS |
| `share-public` | 10 | PASS |
| `project-integration` | 9 | PASS |
| `project-workbench-core` | 15 | PASS |
| `resource-integrity` | 26 | PASS |
| `import` | 8 | PASS |
| `legacy-assets` | 9 | PASS |
| `public-resource` | 8 | PASS；2026-10-14 OpenAPI 修复后 graceful restart、reset 隔离状态并重跑 |
| `restart-recovery` | 8 | PASS |
| `generation-boundary` | 4 | PASS |
| `generation-writeback` | 4 | PASS |
| **合计** | **118** | **11/11 profile PASS** |

已执行的构建/运行门禁：

- `mvn -q -f java/canvas-contract-reference/pom.xml -DskipTests package`：PASS。
- target-control graceful restart 后，`GET /api/health` 返回 typed `status=ok`，`GET /v3/api-docs` 返回 71,838 bytes OpenAPI JSON。
- OpenAPI 静态审计：87 个 operation、87 个唯一 `operationId`；91 个 Required operation 归并为 71 个 Required 唯一路由，71/71 映射到 Java OpenAPI；核心 `api.canvas` 公共签名没有 `Map<String,Object>`、`List<Object>`、裸 `Object`、`ResponseEntity<?>` 或完整 `JsonNode`，扩展字段引用命名的 `OpaqueJsonValue` schema。
- H2 reset 后的 `public-resource` 重启后复验：8 个子场景 PASS；首轮未 reset 的 403 是非空 H2 注册前置条件污染，已用私有 fixture reset 后重跑，不计为 API 实现失败。

### 2026-10-14：最终 OpenAPI schema 复核与回归

- 首次以实际 `/v3/api-docs` JSON 复核时发现：`CanvasConnectionDto.metadata` 以及若干命名扩展集合虽然 Java 类型是 `OpaqueJsonValue`，但 springdoc 默认仍把部分字段展示成 `#/components/schemas/JsonNode`；这违反“扩展字段也必须有命名边界”的核心类型门禁。
- 修复：为 `CanvasConnectionDto`、`CanvasImportDtos.ImportNodeDto` 的扩展字段显式加 `@Schema(ref = "#/components/schemas/OpaqueJsonValue")`；为 `ProjectDtos` 的扩展集合加 `@ArraySchema` 并显式引用同一 schema。没有把完整画布文档退回动态 JSON。
- 修复后重新执行 `mvn -q -f java/canvas-contract-reference/pom.xml test`、`package` 和真实服务 `/v3/api-docs` 审计：87 个 operation、87 个唯一 `operationId`、所有 operation 均有 response 描述；`CanvasDocument`、`CanvasNode`、`CanvasConnection`、`CanvasImportResponse`、项目扩展响应均不再含 `JsonNode` schema 属性，核心公共签名动态类型扫描为 0。
- 修复后重新执行 11 个 Required profile：118/118 HTTP scenario、11/11 profile PASS。重跑期间 `19387` 被外部 DeepSeek 进程占用，`legacy-assets` 改用隔离端口重跑并通过；该次是编排端口冲突，不是产品失败。
- 本轮结论：类型参考桩交付范围仍为 **100%**；严格三轮内容审计应以本次修复后的文档和代码为起点重新计数，生产 PostgreSQL、真实 Web/Chromium、Provider、Agent、远端对象存储/CDN 和多实例门禁仍不属于本桩已完成范围。

### 4.1 根因修复记录

- 画布核心 DTO：将 `CanvasDocumentDto`、`CanvasNodeDto`、`CanvasConnectionDto`、历史/分享/恢复/导入 DTO 接入 Controller 和 OpenAPI，`OpaqueJsonValue` 仅保留命名扩展边界。
- generation writeback：补齐 delta 的 `id`、`taskId`、`sequence`、`content`、`byteCount`，修复 SSE event ID 冲突、未认证内容协商 500 和 foreign completion 的 400。
- history restore：移除过早 `@NotNull`，让缺失 revision 由 Controller 返回 428。
- resource integrity：把 multipart 创建与配额检查合并为同一原子 state update；修复声明 `kind=video` 覆盖文本 bytes、半关闭 chunk 未标记 rejected 导致 completion 错误成功。
- sharing/OpenAPI：修复公开资源重定向安全 headers、重叠映射产生的 `operationId_1`，并增加 typed `/api/health`。
