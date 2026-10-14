# Canvas HTTP contract suite

独立于 Go/Java 后端内部包的画布黑盒 HTTP 合约测试套件。runner 只通过
配置的 HTTP base URL、真实登录 Cookie 和 JSON/HTTP 响应工作，适合先固定
Go 基线，再对 Spring Boot 实现和混合网关重复运行。

## 成果用途与证据职责

本套件的主体是 **HTTP 黑盒 runner**，不是浏览器自动化。HTTP runner 直接请求
被测后端，不启动 React 或浏览器，也不导入 Go/Java 产品内部包。这里的“黑盒”是
指以外部请求和可观察后置状态验收，不是只检查一次请求是否返回 200。

浏览器相关工作有两个不同目的，不能混用：

- **探索与取证**：确认真实页面会调用哪些 API、如何携带登录态及消费响应，为
  catalog/HTTP 场景设计提供依据；一次探索本身不成为可重复的合同测试。
- **可执行 companion 验收**：使用真实前端重复运行用户操作，检查请求组装、
  响应处理、保存重开、资源显示和账号缓存隔离。即使 HTTP 合同全部正确，
  前端仍可能错误解包、错误缓存或未显示资源；这一层不是“仅帮助编写 runner”。

仅验收后端 HTTP 合同无需真实浏览器；验收真实前端消费该合同的能力才使用
companion。两个 runner 分开运行和报告，浏览器请求、fixture 请求及 UI 场景
均不增加 HTTP operation/route/scenario 的覆盖分子，也不改变其分母。

准备 Java 联调时，先读[独立运行矩阵](#独立运行边界)和[Web 配合条件](#web-前端与-java-的配合条件)，
再按[迁移指引的运行入口](../../docs/drafts/go-to-spring-boot-migration-guide.md#迁移验收运行边界与阅读入口)
选择后端直连或真实前端验收。此处维护执行器用途、配置与依赖事实；实施进度、
未闭合路径与文档更新触发器分别以[唯一执行账本](../../docs/drafts/canvas-java-http-contract-test-plan.md#当前进度)
和[维护责任](../../docs/drafts/canvas-java-http-contract-test-plan.md#文档落点与维护责任)为准。Java 类型参考桩的
启动、浏览器管理员 bootstrap、真实浏览器运行命令和当前 11-profile/6-browser 证据见
[`Java 项目 README`](../../java/canvas-contract-reference/README.md) 与[实施进度账本](../../docs/drafts/canvas-java-type-reference-stub-progress.md)。

| 成果/入口 | 目的与作用 | 能证明什么；不能证明什么 |
| --- | --- | --- |
| [`contract/manifest.yaml`](contract/manifest.yaml) | 统一端点、operation、profile、scenario、剩余路径与行为裁决索引，防止遗漏或重复计数 | 登记范围与用例映射；登记不等于运行通过，Go 观测不等于已批准的 Java 合同 |
| [`runner/`](runner) 与根目录 `*_contract_test.go` | 对 Go/Java/混合目标发送相同 HTTP 请求和状态流断言 | 响应、权限、CAS、重放、失败后状态；不证明内部 SQL/锁/事务交错或所有代码分支 |
| [`contract/schemas/`](contract/schemas) 与 [`wire.go`](contract/wire.go) | 检查响应信封、必需字段、类型与二进制/缓存/request variant 合同 | 已收到的完整 wire；不以响应头或丢失 ACK 推断未收到的正文 |
| [`fixtures/`](fixtures)、[`harness/`](harness) 与 [`cmd/`](cmd) | 提供真媒体、固定 HTTPS 上游、私有进程控制与响应丢失代理，使边界和故障可复现 | 对受控替身/阶段的结果；不冒充真实付费 Provider、OSS/CDN 或在途事务 crash |
| [`check-catalog.ts`](check-catalog.ts) 及其单测 | 核对真实注册路由、双向场景映射、schema 绑定和脱敏请求证据，拒绝伪覆盖 | 静态一致性与输入报告合规；不替代实际服务回归 |
| [`run-go-baseline.sh`](run-go-baseline.sh) | 建立全新隔离 PostgreSQL/Redis/Go 环境，重复运行并保存版本、hash、trace 和报告 | 指定代码/配置下的 Go 基线；不是独立 runner 本身，不用于启动 Java 验收 |
| [`response_loss_contract_test.go`](response_loss_contract_test.go) 与 [`restart_contract_test.go`](restart_contract_test.go) | 验证提交后丢 ACK 的未知结果恢复、CAS/幂等重放及重启后的持久状态 | 所登记的丢失/重启恢复；不证明任意写入中断、分布式交错或多实例会话恢复 |
| [`browser/`](browser) | 用真实 React/Vite 前端消费目标 API，补充 HTTP runner 无法覆盖的浏览器状态与用户操作 | 六个登记旅程的选定路径；页面响应头不等于完整 wire，不证明所有 UI/任务/Agent 路径 |
| [`ci.sh`](ci.sh) 与 [workflow](../../.github/workflows/canvas-contract.yml) | 集成脱离仓库编译、专项 race、审计和双轮基线，归档脱敏证据 | 本地执行已验证的门禁入口；workflow 文件存在不证明远端 CI/required check 生效 |
| [唯一执行账本](../../docs/drafts/canvas-java-http-contract-test-plan.md#当前进度)与[迁移总指引](../../docs/drafts/go-to-spring-boot-migration-guide.md) | 记录工作目的、当前证据、未闭合路径与 Java 设计/切流边界 | 工程进度不是覆盖率；测试 profile/fixture 依赖不是目标模块 import 图 |

维护时，新增端点或路径须同步 manifest、可执行场景和审计；实现、schema、
fixture 或环境配置变更后重新建立相应基线，不能借用旧 hash。仅说明文档变化
不伪称重新执行测试：保留原证据中的文件 hash，明确文档已前移，并核对可执行
输入是否仍匹配。最新状态只在执行账本维护，避免多个进度表相互竞争。

## Scope

manifest 当前登记 **163 个 operation / 143 个唯一 method/path**。
直接画布有 **14 个端点**（文档 7、分享 5、导入 2），创建与更新是同一个 PUT。
其中可由本 runner 执行的
profile 为：

| Profile | 当前状态 | 已覆盖的主要路径 |
| --- | --- | --- |
| `core` | 可执行，路径未全量 | 列表分页/筛选/排序、坏参数、缺版本/非法版本/大正文拒绝、CAS、条件 GET、压缩、历史、恢复、删除、并发、权限和修复 preimage |
| `share-public` | 可执行，路径未全量 | 分享状态、创建/旋转、撤销、坏输入/有效期边界/重放、真实到期与恢复、资源 allowlist/Range/live document、并发旋转与撤销收敛；分享资源 CDN redirect 与 object-store 成功/GET 故障/恢复已执行，生产远端 TTL/条件缓存仍 deferred |
| `project-integration` | 可执行，路径未全量 | 列表、关联/解除关联；权限/feature gate、坏输入、跨归属拒绝、重关联、多单元/默认/规范化角色及重放；拒绝前置校验后状态不变，但 PostgreSQL 角色超长的 500 会留下部分画布状态，单独冻结为 D-27 |
| `project-workbench-core` | 可执行，路径未全量 | 项目/单元生命周期、正文与顺序、15 条路由权限/feature gate；2500/2501 批量与 Unicode 240/241 标题、空/null/最后坏行、最大批量全部正文和失败状态不变；完整分镜、工作流、项目素材与角色库仍 deferred |
| `resource-integrity` | 可执行，路径未全量 | 上传/读取/访问、引用校验、purge 后修复；分片边界/乱序/重放/跨用户/并发与一次合并；URL 导入成功/重放/立即失败/SSRF/真实 90 秒超时；单文件/日/存储预留/会话槽位限制和并发准入、分用户限流/Retry-After/未调用上游；本地真实 TTL/304/续签，逐项核查列表/用量/元数据/原字节 |
| `import` | 可执行，路径未全量 | LibTV/TapNow 受控固定 HTTPS 上游、媒体/拓扑转换、新批次重放、owner/foreign/未认证、feature gate、业务/HTTP/JSON/大小/重定向/截断/空画布错误和真实 20 秒上游超时；真实 PNG/WAV/MP4 资源导入、素材登记、保存重开/CAS/跨用户拒绝（新增路径的运行结果见验证记录） |
| `legacy-assets` | 可执行 | 旧素材列表/批量读取/更新/删除、文件夹生命周期、批量删除和用户数据快照 |
| `public-resource` | 可执行，路径未全量 | 独立签名资源 URL、文件名、Range、篡改/缺失/过期签名、资源绑定、重放和安全缓存头；本地 S3-compatible origin 的成功/失败/恢复已由 HTTP fixture 覆盖，CDN/远端 TTL/条件缓存仍需专门 fixture |
| `restart-recovery` | 可执行，需私有进程控制和数据库 fixture | SIGKILL 与优雅重启后原 Cookie、文档/CAS/ETag、历史、分享/签名 URL、资源字节、项目单元/关联与删除状态；提交成功而客户端丢失 ACK 后重启、CAS/幂等重放和精确资源用量；持久化 upload reservation 过期后跨进程重新准入；Go 内存上传会话失效后重新上传 |
| `generation-boundary` | 可执行，路径未全量 | 任务/creation-run/Agent 只读 HTTP 边界：未认证拒绝、已有任务 owner 详情/日志读取、foreign 任务列表/详情/日志隔离、missing-ID、foreign creation-run 隔离；不启动模型、不扣费、不执行画布写回 |
| `generation-writeback` | 可执行，明确为 provider-free 子集 | 文本任务创建、owner/foreign 增量读取、`after` 游标、SSE 事件/游标重放、单次完成和终态拒写；不覆盖 Provider、creation-run 或 Agent 写回 |

以下 catalog 仍保留但不会被上述 profile 的绿色结果隐式覆盖：

- `public-resource`：分享资源的生产 CDN 交付、远端 TTL 边界和条件缓存矩阵仍未完全执行；分享 profile 已用本地 CDN/object-store fixture 验证 redirect 与成功/GET 故障/恢复，资源/public origin 的本地 S3-compatible 成功/失败/恢复、本地真实 TTL 和 Last-Modified 重验证也已执行，不冒充生产远端验收；
- `project-workbench`：项目、单元、分镜、工作流和项目素材的完整生命周期；当前仅 `project-workbench-core` 的 15 个项目/单元 operation 已纳入可执行 profile，其余工作台和角色库仍 deferred；
- `generation-runtime`、`agent-canvas`：Provider-backed 任务写入/恢复、creation-run 状态机、Agent run/tool/event、记忆和 profile 写入的独立合同族；`generation-writeback` 只覆盖上表明确列出的 provider-free text replay + SSE 子集，不能外推为完整生成写回。
- `provider-resource`：Ark 私有素材同步的外部提交、receipt 和恢复合同。

11 个 runner profile 共映射 **91 个 operation / 118 个 HTTP 场景**；另外 **35 个 deferred**、
**37 个 out-of-scope**。每个 Required profile 的缺失分支均保留在 manifest
`remaining_paths`，所以“可执行”和“通过”不表示 profile 的全部路径已经覆盖。
manifest 的 profile `dependencies` 是测试 fixture/跨套件门禁关系，不是 Java
模块 import 图；writer 未批准前标记未分配，兼容分类默认 `UNKNOWN`。

最近完整双轮证据目录、run-id 和每轮精确计数会在[画布合同计划](../../docs/drafts/canvas-java-http-contract-test-plan.md#验证记录)中维护；本 README 只固定统计口径：operation/scenario、完整请求/wire、映射/实际/非错误路由、受控故障、incomplete 和浏览器页面响应头分开统计。分享到期专项仍作为 fixture 专项证据保留，但其场景是否已合并进当前全量以计划文档为准，不与旧版本拼接。该结果不关闭 manifest 的 `remaining_paths`，也不代表 Java、混合网关、远端对象存储/CDN 或完整任务/Agent/UI 旅程已经验收。

本版本多出的实际路由为 setup 的 `POST /api/creation-runs`：它准备只读 fixture，
不属于 66 条 Required wire bindings，也不代表该写入 operation 的状态机已覆盖。
因此清单内完整请求数比 wire 校验多 1；四个丢失 ACK 的请求另计 incomplete，
不计入这两个完整响应数字。请求数量不是代码或分支覆盖率。

`import` 已覆盖固定 HTTPS 替身的真 PNG/WAV/MP4、选定拓扑/参数/过滤、1000/1001 节点与 5000/5001 连接边界，以及资源导入→素材登记→画布保存/重开/CAS/foreign 隔离。该 HTTP 串联不证明浏览器导入、全部转换组合或远端故障已覆盖。导入端点返回转换结果，不直接持久化画布/资源；重放产生新的 batch/node identity，不能宣称服务端幂等保存。

项目功能开关仅门控项目路由：关闭后已登录 owner/foreign 均为 `403/forbidden`，未登录仍为 `401`，画布文档/历史读取不受该开关影响。Go 跨项目重关联会残留旧项目 full-detail 链接，但分页按新的项目身份返回；这是待裁决现状，不是 Java 应复制的正确性要求。`contract/manifest.yaml` 的 `behavior_decisions` 登记 31 项观测/提案/审批状态；所有分类仍为 `UNKNOWN`，提案不等于批准，静态审计拒绝隐式批准或无依据批准。

[`project_link_roles_contract_test.go`](project_link_roles_contract_test.go) 的目的，是补齐同一画布关联多个单元时的角色规范化、唯一身份、重放、逐单元解除与完整解除，并检查错误后的完整状态。32/33 个 Unicode 字符分别验证 PostgreSQL 成功/失败边界；D-26 记录角色更新返回未持久化的新 link ID，D-27 记录 500 后画布归属/revision 已写入、重复失败再次推进，但角色/项目 revision 不提交。它们是 Go 缺陷观测，不是 Java 应复制的正确性要求；也不代表所有故障回滚或并发交错已覆盖。

[`project_link_race_contract_test.go`](project_link_race_contract_test.go) 的两个场景已补强拒绝后的完整 unit 正文核验、登记 D-28，并在历史专项及最新全量双轮执行，不借用初版专项或旧 102/104 场景证据。其目的，是分别验收首次同 key、已有 link 角色更新、不同 unit 和跨项目争用的完整 HTTP 结局；每种形态双方用户交换方向、重复三次，联验 revision、唯一持久身份、完整正文/历史、投影、foreign 拒绝和 stale CAS。跨项目场景还检查当前项目解除后旧链接残留，以及显式重新关联/解除的恢复。最新全量每轮 6 次首次同 key 竞争均为 `200 + 500/internal`，失败也推进 canvas revision，登记 D-28；两次成功的串行结局由源码支持，但当前双轮没有执行，不能宣称已覆盖这个分支。起跑屏障不控制数据库交错；仅允许两种明确结局，并核对其精确状态，不要求 Java 复制 Go 非原子步骤、错误或 revision 策略。

## Versioned response shapes

[`contract/schemas/bindings.v1.json`](contract/schemas/bindings.v1.json) 将全部
66 条 Required 路由绑定到 `v1-observed`：62 条 JSON data schema、4 条默认二进制。
四条 file 路由另有 `descriptor_schema` 和显式 307 binding：只根据请求 `access=1`
选择 typed access 描述，不根据 Content-Type 将 JSON 文件误认为业务信封。
trace 仅增加静态 `request_variant: access-description`，不保留签名查询串。
描述要求 200、private/no-store/no-referrer；307 要求空 body、安全唯一 Location、
隐私 headers，runner 不自动跟随。307 单测不代表远端 OSS/CDN 运行已验证。
Cache-Control 按必需指令集合校验，不冻结 Go 输出顺序或大小写；缺少 `private`
或 `no-store`、用 `no-cache` 代替、或同时声明 `public` 都会拒绝。
[`contract/wire.go`](contract/wire.go) 嵌入并离线编译这些文件，禁止外部引用加载；
每次完整响应在 typed decode 之前校验，并在 trace 中记录版本、schema、模式和
校验结果。静态审计检查 Required/binding 双向覆盖；运行审计拒绝缺失、错误版本、
错误 schema 或失败的 wire 证据。setup 请求的通用信封不代替路由 DTO。

校验覆盖必需字段、类型、RFC 3339 时间、安全整数 revision、分页字段原子性、
access/error 排他、错误信封和 HTTP/业务状态一致性；允许未知扩展字段。
二进制成功、304 空正文与 `ServeContent` 的 416 文本错误单独记录，仍由场景
检查实际字节、Range 和 headers，不能算作 JSON 校验。gzip 在校验前解压并限制
decoded body 为 12 MiB。诊断只输出静态 schema 位置，不输出实例值或完整库错误。

这是 **observed 响应形状**，不是请求 schema、完整画布业务 schema、OpenAPI
替代、Java Entity/模块架构或行为批准。D-11 单独记录 Go 项目画布分页的
`revision: 0`（查询漏选列），不放宽 authoritative document 的正 revision；
该分页值不得用作 CAS。D-12 登记项目同用户重名冲突与单元创建/导入的非幂等
重放；重复 POST 不可被客户端擅自当作安全重试。

工作台场景覆盖重放、跨项目单元拒绝/导入回滚、相同及相反首单元的并发排序，
以及排序与创建/删除/批量导入竞争。每个完整 HTTP 结果对应最终完整成员/排列、
正文、归属、revision 和 foreign 状态；成员变化后旧完整 ID 集合须被拒绝且状态不变。
start barrier 不控制数据库交错，也不证明具体死锁。D-20 的完整内部错误与 D-21
的成员竞争结局均待裁决，不自动批准 Java 错误、锁序或重试；受控交错/失败注入，
有活动任务或已填充 workflow/shot/项目素材的跨套件级联仍未覆盖。
实现分别见 [`project_reorder_race_contract_test.go`](project_reorder_race_contract_test.go)
与 [`project_membership_race_contract_test.go`](project_membership_race_contract_test.go)。
工作台状态比较按列表顺序逐个读取完整 unit DTO；列表投影和导入 ACK 都清空
`sourceText`，不能只比较摘要就宣称正文持久化或未被失败请求/跨项目操作修改。
D-17 的最大批量场景逐个核对全部 2500 个正文、身份、顺序和失败后的完整状态，
不只抽查首尾；Go 数量/Unicode 限额和摘要策略仍需裁决，不能复制为 Java 内部实现。

[`resource_description_contract_test.go`](resource_description_contract_test.go) 记录 D-18：
分享撤销后分享的字节/描述路径返回 404，但此前描述签发的独立资源链接仍可读取。
签名到期不等于撤权；Java 必须按负责人批准的派生能力撤权范围和时限设计，不
自动复制 Go 现状。资源描述的成功/失败/变体请求不改变字节、元数据或用量。
[`project_import_body_contract_test.go`](project_import_body_contract_test.go) 记录 D-19：
32 MiB 的四章真实正文完整持久化，一字节超预算的 JSON 文档拒绝且状态不变；
但小 JSON 后追加空白至 32 MiB + 1 仍被接受，解码预算不是绝对传输体预算。
这两项均为 `UNKNOWN/pending` 下的缺陷提案，不是已批准的 Java 验收规则。

上传竞争及恢复见 [`resource_completion_race_contract_test.go`](resource_completion_race_contract_test.go)
和 [`resource_transport_contract_test.go`](resource_transport_contract_test.go)。缺片/合法覆盖与完成
竞争必须按 ACK 推导精确最终字节，用量只增加一次。D-22 记录 Go 的失败覆盖会删除
已经确认的暂存分片：短/长覆盖和传输中断后 complete 400，显式补片才能恢复。
这是待裁决缺陷，不绑定 Java 的文件、锁或会话架构。

真实请求体中断由 [`runner/interrupted_upload.go`](runner/interrupted_upload.go) 实施：声明
13 字节、仅发送 6 字节后半关闭 HTTP 写端；支持证书验证的 HTTPS，不关闭 TLS 校验。
manifest 的 `controlled_faults` 精确绑定故障 ID、profile、scenario、PUT 路由、字节数和次数。
该类请求仍要求完整 400/error wire；未知故障、错场景/端点/字节数、不完整响应、transport
failure 或缺失 wire 均失败。每个资源 profile 必须有两条实际故障请求，不能只靠场景通过。
trace 的 `body_complete` 指响应体，`controlled_fault_requests` 另计故意中断的请求体；
`incomplete_requests: 0` 不表示从未发生请求体中断，也不证明连接重置/响应丢失或多实例恢复。

[`response_loss_contract_test.go`](response_loss_contract_test.go) 另行验证三条受控响应丢失：
canvas PUT、resource POST、upload complete POST。loopback 单次代理先收齐并校验目标
200/成功 wire，再关闭客户端连接，客户端必须读到 EOF 且收到零响应字节。manifest
精确要求固定场景/路由/次数、请求摘要、目标完整响应 witness、第四代 crash marker 和后续
恢复请求。三条计入 `incomplete_requests` / `controlled_response_loss_requests`，不计作
完整请求或客户端 wire；任何未分类中断仍导致审计失败。场景核对提交文档/历史/资源
存续、旧 CAS 的 409、Go 旧上传会话的 404、原幂等键返回原 identity/字节且用量不增。
这不是在途事务被杀、chunk 传输 reset、任意上游故障或多实例恢复的证据。

## Real browser companion

### 独立运行边界

“独立 HTTP runner”和“真实前端 companion”是同一目录内的两套执行器，
不是一个完全由 Go 构成、无需前端的可执行包：

| 验收层 | 执行器 | 需要的被测服务 | 是否需要 Go 产品后端 |
| --- | --- | --- | --- |
| 后端 HTTP 合同 | 独立 `canvas-contract` Go module，或它编译出的测试二进制 | 实现目标合同的 HTTP 服务及隔离 fixture；无需 Web UI/浏览器 | 不需要；可以请求 Java 或混合网关。从源码编译/运行需 Go 工具链，不等于运行 Go 产品后端 |
| 真实前端补充验收 | Python + Playwright + Chromium 的 `browser/run.py` | 已启动的真实 Web 应用、同源 `/api` 后端/网关及隔离 fixture | 不要求；但 Java-only 目标须提供场景和 fixture 所需的全部依赖 API，不能只有画布 CRUD |
| Go 基线启动器 | `run-go-baseline.sh` / `ci.sh` | 启动器管理的隔离 Go/PostgreSQL/Redis；传 `--browser` 时再启动真实 Vite | 有意需要，它只负责建立 Go 基线，不是 Java 验收入口 |

浏览器脚本通过真实页面的 `get_by_role`、填表、click、focus/Enter 等操作
触发前端请求，并检查页面和持久化结果。它也通过 HTTP 创建用户、上传媒体、
登记 Asset、预置节点/连接以及核验保存后状态；这些是 setup/audit，不是 UI 操作。
因此目前验证的是“预置拓扑在文本编辑/保存后仍保留”，不是“用户拖线创建拓扑”；
节点和媒体预置也不证明 UI 上传或节点创建已验收。页面网络证据只来自
`page.on("response")`，API setup/audit 不加入页面触达统计。

### Web 前端与 Java 的配合条件

当前 Web 是独立的 React/Vite 应用，其 `dev/build` 不启动或编译 Go 后端
（[`web/package.json`](../../web/package.json)）。它不依赖后端实现语言，但
**依赖当前 HTTP/业务合同**；“可脱离 Go 进程”不等于“可脱离所有后端能力”。

- 连接边界：[`request.ts`](../../web/src/services/api/request.ts) 默认 `/api`，
  使用 Cookie，解包 `{ code, data, msg, reason }`；Java 须提供兼容路径/字段、
  status/code/reason、登录/session、revision/CAS/ETag、资源身份/访问及相应状态流。
- 开发连接：[`vite.config.ts`](../../web/vite.config.ts) 的
  `VITE_API_PROXY_TARGET` 可以指向 Java 或混合网关；默认目标的 8080 端口并不
  代表必须是 Go。前端仍使用 `/api`。静态部署中
  [`nginx.conf`](../../nginx.conf) 的 `backend:8080` 是部署绑定，须指向目标服务
  或替换为网关；不能只部署静态页面而遗漏 API 代理。
- 依赖边界：只移植画布 API 时，登录/session、Resource/Asset 和页面辅助 API
  仍需有明确提供者，可由混合网关路由到未迁移服务。完全不启动 Go 的验收则
  必须由 Java/其他声明的目标服务提供所需合同。页面还会读取公告、模型目录、
  技能、钱包、任务等；六个场景成功不证明这些辅助 API 的所有分支均兼容。
- fixture 边界：当前浏览器 setup 使用 `POST /api/admin/users`、认证、资源列表/
  上传、Asset PUT、Canvas GET/PUT。Java-only 目标也必须支持这些隔离准备合同，
  或先经明确方案改造 fixture；`--implementation=java` 只是报告标签，不会补齐
  API 或自动启动 Java/前端。完整任务/Agent 验收还需要对应 SSE/审批/恢复合同。
- 变更边界：保持已批准合同通常只需连接配置；有意改动 wire、登录或状态语义时
  需要明确兼容 adapter 或同步修改前端与合同。不要为了省前端改造无条件复制
  Go 缺陷，也不要因 Java 语言不同而默认重写前端。

以下是 Java 已运行在 8081 时的开发连接示例，**不是已完成的 Java 验证**。
使用仓库现有 Bun 依赖，保持 `VITE_CANVAS_BACKEND_URL` 未设置或为 `/api`：

```bash
cd web
VITE_API_PROXY_TARGET=http://127.0.0.1:8081 bun run dev --host 127.0.0.1 --port 18184 --strictPort
```

然后使用下文的 `browser/run.py --web-origin=http://127.0.0.1:18184` 验收。
无需生产部署，运行真实 Vite 即可；浏览器脚本本身不启动前端。当前 companion
只支持同源 API，并阻断未声明跨 origin 请求；不要将绝对后端 URL 配置直接
套用于它。现有源码扫描型 `web/test` 中有读取 Go 文件的测试，这是测试源码
耦合，不能与 Web 构建或浏览器运行依赖 Go 进程混为一谈。

[`browser/scenarios.json`](browser/scenarios.json) 与 HTTP manifest 分开统计。目前 6 个
真实 React/Vite 场景：错误密码与登录、UI 新建/重命名/重开、真实富文本编辑及资源图片
渲染/拓扑持久化、匿名分享/撤销、foreign 登录与画布库隔离、同浏览器账号切换及另一页
重载隔离。媒体 fixture 通过 HTTP
建立完整 Resource/Asset/Canvas 数据，再用全新浏览器 context 冷启动；不直接注入 store、
IndexedDB 或产品内存。只证明冷启动和列明的编辑链，未证明同 context 外部更新合并。

页面 HTTP 证据记录 `real-page-network-response-headers`；不把响应头事件充当完整
body/wire 校验。每个场景必须至少有声明路径的 2xx 页面请求和实际 UI/持久化后置断言，
准备用 APIRequestContext 的请求不能冒充页面网络。报告保留未闭合路径；不合并进
111 个 HTTP 场景或源码覆盖率。分享 token、签名 query、动态 ID 和凭据均脱敏。

运行准备（独立 Python venv，不改前端依赖/锁文件）：

```bash
python3 -m venv .local/cache/canvas-contract-browser-venv
.local/cache/canvas-contract-browser-venv/bin/pip install -r tests/canvas-contract/browser/requirements.txt
.local/cache/canvas-contract-browser-venv/bin/python -m playwright install chromium
```

对已有 Java/Go/混合目标，可将整个套件目录复制到其他 checkout；只需真实前端与
同源 `/api` 网关已经启动，并通过环境变量提供隔离目标的 test-admin 凭据。**不运行
Go launcher，不编译/启动 Go 后端**：

```bash
python3 tests/canvas-contract/browser/run.py --help
python3 tests/canvas-contract/browser/run.py \
  --implementation=java --web-origin=http://127.0.0.1:18184 \
  --run-id=java-browser-01 --report=.local/browser-report.json --isolated-fixture
```

凭据变量是 `CANVAS_CONTRACT_BROWSER_ADMIN_USERNAME` / `CANVAS_CONTRACT_BROWSER_ADMIN_PASSWORD`；
上例需使用已经安装 Playwright 的 Python。目标必须可通过现有 admin API 建两个普通测试
用户，测试数据由隔离环境销毁，不自动删除真实用户。前端用 Bun 安装既有锁定依赖。
Go 专用复现为 `PATH="$(pwd)/.local/cache/canvas-contract-browser-venv/bin:$PATH" bash
tests/canvas-contract/run-go-baseline.sh --profiles=core --rounds=2 --browser`；它独占
18183/18184 端口并清理自身后端/真实 Vite 进程，端口占用时拒绝，不杀其他服务。

所有未声明的跨 origin 页面请求都阻断并判失败。仅当前默认登录背景的两个固定
视频/海报 URL 作为 `blocked_decorations` 明确登记，仍阻断联网，记录静态 fixture ID；
不是域名放行，更不允许 API/provider 请求。文本编辑按钮在本轮 fixture 的 pointer
点击被正文层拦截，失败报告保留；当前场景验证真实 focus/Enter 键盘激活，不用
`force=True` 或 DOM dispatch 伪造点击，不宣称 pointer 路径通过。同浏览器账号切换与
另一页显式重载已有选定路径证据；live 跨标签通知/并发缓存写、本地保存失败/CAS UI、
历史/导入/项目关联及任务/Agent E2E 仍未完成。

[`legacy_ownership_contract_test.go`](legacy_ownership_contract_test.go) 覆盖匿名全部对象写入、
双方 foreign folder/move/upsert/purge，以及 mixed own/foreign/missing ID 的移动/批量删除。
另外，legacy-assets 通过六次并发画布引用/素材删除的 HTTP 结局检查，以及本地
object-store DELETE 故障后的异步物理对象保留检查；两者分别记录当前 Go purge 的
引用断链语义和 outbox 待重试状态，不替代任务/项目引用源和生产对象存储重试时序。
每个失败后比较双方列表、逐素材完整 DTO、folder 与 resource metadata、用量和实际文件字节。
D-23 记录 foreign upsert 的 Go 500/internal；D-31 记录显式 purge 与画布引用竞态以及
本地 object-store DELETE 故障，虽然数据隔离和最终可观察状态通过，该错误分类、purge
是否允许产生断链、生产 outbox 重试时序仍待裁决，不要求 Java 依赖主键冲突实现权限。

历史保留 D-16 场景验证快速普通写入的快照合并、20 次强制恢复的逐次完整 preimage、
单调 revision、最多 20 条降序历史、驱逐 ID/foreign/stale 拒绝后的状态不变。
它不证明普通 5 分钟定时捕获或底层物理资源引用清理，保留预算也不自动批准。

资源补测包含存储预留/32 会话槽位、并发准入、不同/相同分片并发和重复合并。
本地真实 TTL 场景只在隔离 runtime-policy fixture 中把访问有效期设为 60 秒，
验证同签名窗 URL、刷新时间、Last-Modified/304、真实过期后的 403（包括条件请求）、
重新签发及元数据/字节不变。D-13 不把该本地结果外推到 OSS/CDN。
URL 慢上游场景等待服务端实际 90 秒下载超时，专用 HTTP 客户端期限至少 110 秒；
只有获得完整服务端错误且确认上游取消、无自动重试、列表/用量不变才算通过。
D-14 把超时预算/错误语义列为待裁决，而非 Java 客户端默认值；D-15 单独登记
同分片不同内容仍可覆盖的观测，不能称作内容幂等。

## Build without a server

```bash
cd tests/canvas-contract
go test ./... -run '^$'
go test ./contract ./runner ./harness -count=1 -race
```

第一条命令只编译测试套件，第二条运行 schema/客户端/证据/进程控制的本地单测；不需要产品后端、数据库或 Redis，单测会启动 HTTP fixture 和自身测试二进制子进程。

## Reproduce an isolated Go baseline

从仓库根目录执行：

```bash
bash tests/canvas-contract/run-go-baseline.sh --rounds=3
```

需要 Go、Bun（支持 `Bun.YAML`）、可用的 Docker daemon、curl、lsof、openssl
和 shasum；会使用 `postgres:18`、`redis:7-alpine` 镜像。脚本构建 Go 服务，
每个 profile/轮次创建全新数据库和资源目录，通过 HTTP bootstrap 两个普通
测试用户；Redis 容器只供本轮使用，每个 profile 前清空其数据。不使用普通
dev 数据库，不关闭权限/SSRF 校验，不调用付费 Provider。

可选 `--rounds=1..10`（默认 1）、`--profiles=core,share-public` 和
`--backend-port=18183`（默认端口）。占用端口会直接失败，绝不杀其他进程；
PostgreSQL/Redis 使用随机 localhost 端口。此脚本只启动 Go 基线，Java 和混合
网关仍由各自环境启动后运行相同 HTTP runner。

`resource-integrity` 自动启动随机 loopback 端口的资源 HTTP fixture；被测后端
和 runner 须共享网络命名空间，并仅精确放行 `127.0.0.1`。远端独立机器尚无
资源 fixture 地址适配器，不把不能访问该 fixture 的失败算作 Java 合同差异。

`import` 的 Go 目标会另行交叉编译为 Docker daemon 架构的 Linux 静态 binary，
使用同一份 Go 源码和 PostgreSQL 主版本。产品目标与 CONNECT/TLS 替身仅接入
本轮 `--internal` Docker 网络，产品不直接发布端口；仅一个固定目标的 HTTP
gateway 通过 `127.0.0.1` 发布 API。替身仅接受 LibTV/TapNow API 域及
`files.tapnow.media` 这三个固定 CONNECT 目标，媒体域仅响应固定 fixture 文件路径，
**没有任何真实上游转发代码**。临时 CA 仅通过目标容器 `SSL_CERT_FILE` 注入，
不改宿主信任库，不禁用 TLS/SSRF，不启动模型任务。目标容器通过单独的 hosts
映射将 `files.tapnow.media` 解析为 `203.0.113.10`；该文档保留地址通过当前资源
下载的通用出站地址检查，实际连接只进入不转发的 HTTPS 替身。没有私网放行，
这不是公网 DNS/重绑定策略验收，也不用于自定义模型渠道的更严格地址策略。
Go 当前模型导入端点只返回转换数据；基本 fixture 的 `https://fixture.invalid/`
URL 不下载，新增 rich fixture 才通过资源 HTTP API 下载真正的 PNG/WAV/MP4，
先登记同用户素材再保存画布；HTTP 映射不声称浏览器导入/自动居中已验收。
这不是生产切流网关，也不是 Java 已通过的证据；Java 环境必须另行验证其 HTTP
client 的受控 proxy 与临时信任根配置，再运行相同 profile。
若 Java 的 SSRF 策略拒绝文档保留地址，应为隔离测试目标提供可审计的 DNS/出站
fixture 适配，并继续精确限制这三个主机及非转发代理；不能为通过 Go fixture
而放宽生产地址策略或全局信任根，也不能把未完成的 fixture 适配当 HTTP 合同差异。

`restart-recovery` 使用标准库 Go 编写的 `cmd/target-control` 启动并管理自己的
目标子进程，控制 API 只绑定随机 loopback 端口并验证随机 bearer token；凭据
不写入日志，也不传给它启动的产品子进程。目标端口已占用时拒绝启动，不接管
现有服务。分片临时文件也置于本次 private 数据目录，崩溃后残片由退出清理。

结果保存在 Git 忽略目录 `.local/project-workbench-debug/canvas-contract-review/<run-id>/`：

- `evidence.txt`：checkout、套件输入文件的 SHA256、临时运行 binary 的身份 hash、工具/数据库版本、容器 image ID 和选定 profiles；普通 hash 行可在退出清理后复核，`runtime_binary_sha256` 行只记录已删除的临时 binary，不表示对应路径仍存在。
- `<round>-<profile>/result.jsonl`：测试事件与脱敏请求事实；`routes.log` 只保留 Gin 路由注册，不保存完整服务访问日志。
- `report-round-<round>.json`：对应轮次的真实执行审计，失败或 skip 不生成通过报告。

退出（包括失败）时只删除自身服务、容器及匿名卷、binary、上传文件、settings key
和服务日志；保留以上脱敏证据。测试进程的 Cookie/密码只在内存/临时环境中，
不进入报告。进程被强制 `SIGKILL` 或 Docker 不可用时不能保证自动清理；此时仅
按本次 run-id 清理，不做宽范围删除。

## Run against a server

`CANVAS_CONTRACT_BASE_URL` 是后端根地址，例如 `http://127.0.0.1:18080`；不要
重复附加 `/api`。默认要求测试环境已经准备两个账号：

```bash
cd tests/canvas-contract
CANVAS_CONTRACT_BASE_URL=http://127.0.0.1:18080 \
CANVAS_CONTRACT_USER_A_USERNAME=canvas-a \
CANVAS_CONTRACT_USER_A_PASSWORD='replace-me' \
CANVAS_CONTRACT_USER_B_USERNAME=canvas-b \
CANVAS_CONTRACT_USER_B_PASSWORD='replace-me' \
go test ./... -count=1 -v
```

对于一次性空数据库，可以让 runner 通过 HTTP 建立账号。它会注册首个
bootstrap 管理员，再通过管理员 HTTP API 创建 A/B 两个普通用户；不会直接写数据库。
每个 profile 的注册模式必须使用全新数据库，非空库使用前面的显式凭据模式：

```bash
cd tests/canvas-contract
CANVAS_CONTRACT_BASE_URL=http://127.0.0.1:18080 \
CANVAS_CONTRACT_REGISTER_USERS=true \
go test ./... -count=1 -v
```

可选环境变量：

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `CANVAS_CONTRACT_PROFILE` | `core` | 可选 `core`、`share-public`、`project-integration`、`project-workbench-core`、`resource-integrity`、`import`、`legacy-assets`、`public-resource`、`restart-recovery`、`generation-boundary`、`generation-writeback`；未知 profile 直接失败 |
| `CANVAS_CONTRACT_TIMEOUT` | `20s`，`import` 为 `35s` | 单次 HTTP 请求超时；导入 runner 必须晚于目标的上游超时 |
| `CANVAS_CONTRACT_IMPLEMENTATION` | `unknown` | 写入报告/失败信息的被测实现标签 |
| `CANVAS_CONTRACT_DB_PROFILE` | `unknown` | 写入报告/失败信息的数据库标签，例如 `postgres-18` |
| `CANVAS_CONTRACT_RUN_ID` | 自动生成 | 用于测试账号和画布 ID；不要复用固定共享数据 |
| `CANVAS_CONTRACT_CONTROL_URL` | 空 | `restart-recovery` 必需，私有 loopback HTTP origin，无路径/查询串/凭据 |
| `CANVAS_CONTRACT_CONTROL_TOKEN` | 空 | `restart-recovery` 必需，至少 24 字节；其他 profile 不需要控制服务 |
| `CANVAS_CONTRACT_FIXTURE_URL` | 空 | `core`、`share-public`、`resource-integrity` 必需，私有 loopback HTTP origin；只提供隔离数据库准备动作，不是被测 API |
| `CANVAS_CONTRACT_FIXTURE_TOKEN` | 空 | 上述 profile 必需，至少 24 字节；fixture 只接受 Bearer token，不进入测试报告 |
| `CANVAS_CONTRACT_IMPORT_FIXTURE` | `false` | `import` 必需显式设为 `true`，仅允许在已建立固定域 TLS 替身的隔离环境使用，禁止对生产服务声明该标志 |
| `CANVAS_CONTRACT_SETTINGS_FIXTURE` | `false` | 两个项目 profile 和 `resource-integrity` 必须显式设 `true`，只可对隔离目标临时修改全局功能开关/运行配额与频控；缺失直接失败，不 skip。Go 启动器自动声明自身隔离目标 |
| `CANVAS_CONTRACT_ADMIN_USERNAME` / `CANVAS_CONTRACT_ADMIN_PASSWORD` | 空 | `import`、两个项目 profile 和 `resource-integrity` 的显式账号模式需隔离测试管理员；注册模式自动使用 bootstrap 管理员。资源限额场景另经管理员 HTTP 创建隔离账号以避免已有用量/限流窗口污染 |

测试账号密码不会写入输出。不要在 CI artifact 中保存 Cookie、账号密码、
提示词全文、签名 URL 或画布正文。

`core` 的 updated-at 并列排序、`share-public` 的真实到期场景和
`resource-integrity` 的 reservation 到期场景都不能靠伪造客户端时间或长时间等待完成。
Go 启动器会为这些 profile 启动仅绑定当前隔离 PostgreSQL database 的私有
`cmd/share-fixture`（历史目录名保留，但它现在是数据库 fixture adapter），runner
通过 loopback + Bearer token 请求一次性准备动作；最终断言仍全部通过目标 HTTP API。
Java/混合目标不得让 runner 直接写其数据库，应提供语义等价的隔离 fixture adapter，
并保留相同的安全边界、精确对象范围和清理语义。

### Database fixture adapter protocol

数据库 fixture adapter 是测试准备面，不是被测 API，也不是要求 Java 暴露生产数据库写入接口。当前 Go adapter 的最小 HTTP 合同如下；Java 或混合目标可以使用不同实现，但必须提供等同的语义、隔离范围和确认信息：

| 方法/路径 | 请求 JSON | 成功确认 | 用途与原子性 |
| --- | --- | --- | --- |
| `POST /expire-share` | `{ "projectId": "..." }` | `{ "expired": true, "projectId": "..." }` | 将指定项目分享设置为已过期；只允许一个精确项目，找不到即失败 |
| `POST /set-canvas-updated-at` | `{ "projectIds": ["..."], "updatedAt": "RFC3339Nano" }` | `{ "updated": N, "updatedAt": "..." }` | 在一个数据库事务中为全部指定画布写入同一时间；任何 ID 不存在都整体失败，用于证明并列排序 tie-break |
| `POST /expire-upload-reservation` | `{ "uploadId": "..." }` | `{ "expired": true, "uploadId": "..." }` | 使指定上传预留立即过期；随后由目标 HTTP API 证明提交拒绝、额度不被错误释放及同目标重试语义 |

三条路径都要求 `Authorization: Bearer <fixture-token>`、JSON body 不超过 16 KiB、拒绝未知字段和尾随 JSON；adapter 只监听字面量 loopback，禁止 URL 凭据、重定向跟随和把 token/正文写入报告。其数据库连接只指向本次运行的隔离 PostgreSQL，adapter 的写入不是 HTTP 覆盖证据；Java 实现应在测试配置中启动等价的私有 adapter，而不是让 runner 直接连接或修改 Java 数据库。

## Restart fixture and CI

重启 profile 不允许因没有控制 fixture 而 skip；缺少配置会直接失败。Go 启动器
会自动建立它。Java CI 可构建相同控制器，用 `--target-bin=<java 可执行文件>`、
多次 `--target-arg=`（例如 `-jar` 和隔离测试 Jar）、`--target-origin=`、
`--health-path=`、`--ready-file=` 提供自己的启动方式。产品仍仅通过 HTTP 被测，
runner 不导入 Go backend 或 Java 内部包。控制请求包含 `crash`/`graceful` 模式，
返回新的进程 generation；数据库、Redis、settings key 和对象目录跨重启保留。

从仓库根目录运行 `bash tests/canvas-contract/ci.sh`：将整个 suite 复制到临时
独立目录，`GOWORK=off` 编译和生成 HTTP test binary，运行 race 单测、审计与
格式检查，再跑两轮隔离 Go 基线。临时副本自动清理；该入口额外需要 `rg`。
已新增独立工作流 [Canvas HTTP contract](../../.github/workflows/canvas-contract.yml)，
PR/manual/call 运行同一入口，仅上传白名单脱敏结果，不上传 private 或完整服务
日志。工作流配置不等于远端 CI 已通过；分支保护将 `Canvas Go HTTP baseline`
设为 required check 的状态尚未核验。

## Baseline discipline

Go 与 Java 应使用同一 profile、同一 PostgreSQL 主版本、同一 fixture 语义，
但分别重置数据库和对象存储前缀。SQLite 只能作为本地 smoke，不能作为
Java PostgreSQL 兼容性的迁移基线。

固定一次 Go 基线时，至少保存：

- `CANVAS_CONTRACT_IMPLEMENTATION=go`；
- Go commit、服务配置、PostgreSQL 版本和 runner commit；
- `go test ./... -count=1 -json` 的完整结果与脱敏请求证据；
- profile、skip/deferred 列表和失败差异。

HTTP runner 不启动被测服务，也不依赖当前仓库的 Go `internal` 包；上面的
可选 Go 启动器负责环境生命周期。可执行场景与
`contract/manifest.yaml` 的 operation catalog 通过稳定 scenario ID 对应；Java
实现必须运行同一命令和 profile，而不是复制一套 Java 专用断言。

## Audit coverage

从仓库根目录执行，不需要服务：

```bash
bun tests/canvas-contract/check-catalog.ts
bun test tests/canvas-contract/check-catalog.test.ts
```

离线检查使用 Bun 的内置 YAML 解析（需要支持 `Bun.YAML` 的版本，本轮使用
Bun 1.3.9），扫描当前 Go 注册形状并检查 evidence/profile/scenario 双向索引；
它是 Go catalog 审计工具，不是 HTTP runner 的依赖。不支持任意 Go 动态语法，
因此真实基线还应对照 Gin 注册日志：

```bash
bun tests/canvas-contract/check-catalog.ts \
  --routes-log=.local/example-service.log \
  --results=.local/example-core-result.jsonl
```

结果文件来自 `go test ./... -count=1 -json`；可以多次传入 `--results` 汇总
多个 profile。未提供结果时 `operation_executed` / `scenario_executed` 为 `null`；
失败、真实测试 skip、映射场景缺失、缺少逐请求证据或场景未发出映射路由请求直接非零
退出；旧的仅有 subtest 结果不能用于新执行审计。报告列出 deferred、
out-of-scope 和 remaining paths，不宣称内部代码覆盖率。
Go 的无测试辅助 CLI 包必须明确输出 `[no test files]` 才不算测试 skip；
任何具名测试 skip 或无法解释的 package skip 仍然阻断。

报告口径：`operation_executed` 是通过场景及请求证据核对的 operation 映射；
`method_paths_mapped_executed` 是这些映射的路由去重数；
`method_paths_exercised` 是真实完整 HTTP 请求的路由去重数，包含 setup 和
拒绝路径；`method_paths_successful` 是至少一次获得小于 400 响应的路由数
（含 304，不证明业务全链路成功）。`requests_observed` 是清单范围内完整请求数，
`incomplete_requests` 是 trace 中网络/读取失败数。`wire_validated_requests` 是已绑定
路由中带有效版本/schema/模式证据的完整请求数；含二进制/304/416，不代表全部都是
JSON 校验或全部业务路径已覆盖。请求日志只含方法、脱敏路径、状态、完整性和 wire
facet；移除查询串、分享 token 和文件名，不保存 headers 或正文。

运行审计逐项要求每个 `operation/scenario` 都有该场景自己的 method/path 请求，
不能由其他映射场景或 setup 请求补证。并发 CAS 也记录两个实际 PUT；登出场景
既验证 CookieJar 清空后的拒绝，也重放原 Cookie 验证会话撤销，不输出 Cookie。

审计先匹配路由，再拒绝未分类的不完整请求，不允许额外网络失败由绿色场景隐藏。
当前没有通用“允许错误”开关；后续有意网络中断须登记受控故障、请求匹配和强
后置断言，不能恢复原先的直接跳过。相关 catalog 审计单测共 36 项，另有 9 项
Python browser audit 单测。

当前 manifest 登记 163 个 operation、143 个唯一 method/path、91 个 Required operation 和 118 个 HTTP 场景；115 场景版双轮仍是历史证据，最新 run-id、每轮 observed/wire 请求、路由触达、受控故障和真实浏览器观测数只维护在[唯一执行账本](../../docs/drafts/canvas-java-http-contract-test-plan.md#当前进度)及其版本化证据目录，不在本 README 复制动态计数。执行报告必须同时保留 11 个 profile、浏览器 6 个场景、`remaining_paths`、31 项行为裁决和 Java/混合网关门禁；页面响应头、HTTP 完整 wire 和 fixture 请求始终分开统计。`generation-boundary` 的任务 owner/foreign task read 已纳入当前证据；`generation-writeback` 已覆盖 provider-free text replay 与 SSE `text-events` 游标重放，但不覆盖 Provider、SSE 断线重连或完整 Agent/creation-run 写回；`canvas.share.resource-object-storage-failure` 通过本地 CDN/object-store fixture 验证分享资源 redirect、成功读取、GET 故障和恢复；这不等同于生产 CDN、TTL 或条件缓存验收。资源 URL 导入、legacy 素材 purge 的提交后响应丢失/崩溃/重放/引用竞态、本地物理删除失败和 durable upload reservation 跨进程重新准入也已登记；它们使用本地受控 fixture，不等同于远端对象存储恢复、后台 reconciler 或生产 outbox 重试时序。当前 118 场景全量双轮已完成，证据和动态计数只以执行账本为准；若 manifest、runner、fixture 或可执行场景再变化，必须建立新的 run-id，不得与旧版本请求数拼接。

71 场景稳定的 schema/完整单元正文状态版已通过独立 CI 两轮及全新启动器第三轮：**72 个 operation 映射、
71 个顶层 scenario、57 条触达/有非错误响应路由、清单内 1470 次完整请求及 wire
证据、0 次不完整请求**。证据版本为 `20261012-065734-10599`；每轮实际记录
1554 条 trace，另 84 条认证/admin/私有控制不在 catalog 请求数中。12 项行为决策
未批准，报告列出全部 unresolved ID。第三轮 `20261012-070155-13114` 已通过；
三轮共 4662 条 trace，非 README 套件文件 hash 已核对。新增配额/TTL 的
74 场景版本的专项证据为 `20261012-072719-25241`（17 场景、296 请求/wire）。
77 场景版本随后在 `20261012-073633-29754` 两轮和 `20261012-074530-35910` 第三轮
完整通过：每轮 9 profile、1682 请求/wire、57 条触达/非错误路由、0 incomplete；
三轮共 5394 条 trace 和 49 个非 README 源文件 hash 已核对。
79 场景/严格不完整请求审计版随后在 `20261012-080234-44305` 完整 CI 两轮通过：
每轮 9 profile、9375 请求/wire、9491 条 trace、57 条触达/非错误路由、0 incomplete；
两轮共 18982 条 trace，该版本 50 个非 README 文件 hash 已核对，没有第三轮证据。
82 场景版的资源描述/撤权三 profile 专项 `20261012-082817-58248` 已通过：
35 场景、567 请求/wire、630 条 trace、23 条描述请求、0 incomplete；没有远端 307。
历史 83 场景版：工作台字节预算/解码边界专项 `20261012-083302-60064`
通过：11 场景、8308 请求/wire、8321 条 trace、0 incomplete；对应版本的全量见下文，
不能借用 79 或 77 场景报告。该 83 场景版后来在 `20261012-083449-61789`
完整 CI 两轮通过：每轮 9510 请求/wire、9626 条 trace；两轮 19252 条 trace，
54 个非 README 文件 hash 已核对。随后相反首单元排序和成员写竞争初步专项通过，
87 场景强化版在 `20261012-085506-72220` 完整 CI 三轮通过：每轮全部 9 个 profile，
87 场景版三轮请求/wire 10119 / 10119 / 10120，共 30706 条 trace。后续 90 场景/D-22 版 `20261012-095745-450` 全部 9 个 profile 完整两轮通过：响应完整请求/wire 10288 / 10284、20840 条 trace、46 条描述请求、每轮 2 条精确半关闭请求体故障、0 incomplete、60 个非 README 源文件 hash 核对。当时登记 92 场景/23 项待裁决行为；legacy 权限/混合 ID 回滚专项 `20261012-100957-7257` 两轮通过：每轮 7 场景、1060 请求/wire、1065 条 trace、0 incomplete，61 个非 README 源文件 hash 核对。D-23 登记后的 92 场景版全量 CI `20261012-101144-8154` 两轮通过：每轮 92 场景、11320 请求/wire、57 条触达路由、2 条精确受控故障、0 incomplete，两轮 22908 条 trace、61 个非 README 源文件 hash 核对。remaining_paths 可显式为 []，静态审计仍不批准合同。全量绿不等于 clean-review 或 Required 剩余路径完成；各版本 0 条远端 307。执行轮数不等于 clean-review 轮数，完整 review 仍为 0/3。

历史 D-24 的 9-profile 全量 CI 两轮证据目录为
`20261012-104544-22132`；随后新增的 `generation-boundary` 专项证据为
`20261012-121747-54843`（3 个场景、9 个 operation 映射、20 条请求、0 incomplete）。96 场景版的 10-profile 全量基线位于
`20261012-123147-59864`：两轮分别执行 96 个场景和 81 个 operation，清单内完整请求/wire 为 11354/11349，`method_paths_mapped_executed` 为 66、`method_paths_exercised` 为 67、`method_paths_successful` 为 65，`wire_validated_requests` 为 11353/11348，`incomplete_requests` 为 0；每轮含 2 条精确受控请求体故障。D-25（missing task logs 返回 200 空结果）仍待裁决，Required `remaining_paths`、行为批准、Java/混合网关/远端对象存储/浏览器门禁仍未闭合。

历史 99 场景版源码登记；新增 `canvas.import.media-topology`、
`canvas.import.conversion-limits` 和 `canvas.import.resource-save-and-isolation`。
rich fixture 包含真实 PNG/MPEG-4 MP4/PCM WAV、unsafe URL/重复/坏数据/删除节点、
短 ID alias/handle 拓扑、参数和失败历史/占位/风格转换；数量上限使用真实
1000/1001 节点和 5000/5001 连接正文。后续 HTTP 链验证资源下载/重放/按真实字节
计费、失败恢复、Resource-only 保存拒绝、同用户 Asset 登记后保存重开、CAS
与 foreign 资源/画布/引用拒绝。不声称浏览器导入或远端对象存储已验。
专项完整 CI `20261012-140833-1753` 两轮通过：每轮 8 个场景、2 个映射 operation、
381 个清单内完整请求/wire、387 条全部 trace、11 条实际触达路由、0 incomplete；
67 个非 README 文件 hash 和隔离资源清理已核对。前三次专项失败来自 runner 的
Asset 登记遗漏、JSON 键顺序比较及 WAV MIME 预期，均保留为失败证据。
后端 observed WAV MIME 是 `audio/wav`，不是 fixture/标准库的 `audio/wave`。
随后相同实现的完整 CI `20261012-141056-3518` 两轮通过：每轮全部 10 个
Required profile、81 个 operation、99 个场景，清单内完整请求为 11587/11596、
wire 为 11586/11595、映射触达路由 66、实际触达路由 67、0 incomplete，
每轮仍含 2 条精确受控半关闭故障。全部 trace 为 11726/11735，合计 23461；
67 个非 README 文件 hash、自有容器/网络/private 目录/端口清理已核对。
旧 96 场景版不能作为当前 99 场景版的通过证据；当时登记的 24 条 Required 剩余声明、
当时 25 项 pending 行为和未来 Java/混合/远端/浏览器门禁均未闭合，clean-review 为 0/3。
这些是历史版本记录；当前版本的计数和浏览器双轮结果以本节上方的最新证据与合同计划为准。

旧 `20261012-060647-82150` / `20261012-061136-84898` 的三轮是 schema 引入前的
67-scenario/920-request 基线；`20261012-064730-4212` / `20261012-065330-8124`
各两轮为 71-scenario/1268-request 但只比较单元摘要的版本。旧结果不能代替最新
schema、安全诊断和完整单元正文后置断言，不与最新源码/manifest hash 混用。
新增恢复 operation 是既有路由的重启状态流分支，没有扩大路由数量。旧的
64 operation / 40 scenario / 240 请求报告保留为历史，不冒充新版本证据。
三条导入路由均已有受控成功路径；旧 61-scenario、57-scenario 三轮和更早 40-scenario
报告只保留为历史，不与新 source/manifest 版本混用。57/143 约 39.9% 是该清单的
路由触达率，不是执行路径、内部代码或“画布功能”覆盖率；
Required 的剩余路径和合同批准仍阻断 Gate 0。完整规划见
[画布 HTTP 合同测试计划](../../docs/drafts/canvas-java-http-contract-test-plan.md)。

Go baseline 目前仍断言 foreign canvas DELETE `200` 无删除效果、foreign project
canvas list（`GET /api/projects/:id/canvases`）和 `GET /api/projects/:id/units`
为 `500/internal`、foreign project DELETE 为 `404`、foreign share POST/DELETE
为 `500/internal`。这些是待批准事实，
不是 Java 设计要求；批准变更时同步 manifest、差异决策和断言，不能只放宽状态码。

资源 URL 重放仍重新下载上游，只比较 kind/MIME/size；同大小不同内容会返回
旧资源而不替换旧字节，上游失败的重放也会失败。LibTV/TapNow 的空画布和
上游错误当前为 `500/internal`，foreign/missing canvas 也为 `500/internal`。
这些均是需评审的 Go 事实，不自动批准为 Java 规则。分享/引用/旧素材/重启的
图片 fixture 使用同一个合法 PNG，不要求目标后端接受伪装成图片的文本字节。

Go 的素材 DELETE/批量删除目前是显式 purge，可令现有及历史引用断链。
资源 profile 分别验证素材替换拒绝、purge 后严格保存拒绝、显式修复保留
损坏 preimage 和旧历史恢复拒绝。物理清理通过异步作业执行；这些 HTTP
结果不证明清理作业重试、对象存储失败或内部普通删除保护已被本套件验证。
