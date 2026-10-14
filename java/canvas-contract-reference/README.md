# Canvas Contract Reference

这是一个面向未来 Java 画布移植的**类型参考桩**，不是生产后端的业务实现，也不是把 Go 包结构逐文件翻译成 Java。

## 阅读入口与文档索引

如果你是第一次接触本项目，按下面顺序阅读，可以直接从“参考桩如何运行”进入“真实 Java 移植如何验收”，不需要重新扫描整个 Go/Web 仓库：

1. 先读本文，启动参考桩、查看 `/v3/api-docs`，再运行一个 HTTP profile。
2. 读[类型参考桩实施规划](../../docs/drafts/canvas-java-type-reference-stub-plan.md)，理解为什么这是强类型 wire/OpenAPI 参考，而不是业务实现。
3. 读[类型参考桩实施进度](../../docs/drafts/canvas-java-type-reference-stub-progress.md)，查看已验证的 11 个 Required profile、118 个 HTTP scenario、证据和未覆盖边界。
4. 读[HTTP 合同测试计划](../../docs/drafts/canvas-java-http-contract-test-plan.md)与[独立 Go HTTP 合同套件 README](../../tests/canvas-contract/README.md)，理解 profile、fixture、HTTP runner 与浏览器 companion 的分工。
5. 开始真实移植前，再读 [Go → Spring Boot 迁移指引](../../docs/drafts/go-to-spring-boot-migration-guide.md)、[画布开发导航审计](../../docs/drafts/canvas-development-navigation-audit.md) 和[画布系统深度调研](../../docs/drafts/canvas-deep-research.md)，分别获取迁移边界、代码入口和状态/同步设计背景。

更大的文档导航见 [`docs/index.md`](../../docs/index.md)；研究草稿的生命周期与入口见 [`docs/drafts/README.md`](../../docs/drafts/README.md)。本文记录本项目的可执行 runbook，进度文档记录证据账本；两者都不替代真实移植的生产门禁。

## 用途与边界

- 使用 Spring Boot 3.4、Spring WebMVC、Jackson、Bean Validation、Spring Data JPA、H2 file mode、Flyway 和 Springdoc OpenAPI。
- 用确定性的 seed/mock 数据和 H2 持久化状态，提供当前 HTTP 黑盒合同套件需要的端点、wire format、状态依赖和重启恢复观察面。
- 画布文档、节点、连线、历史、恢复、分享和导入使用领域 DTO；核心公共 Controller 不用 `Map<String, Object>`、无类型 `Object`、`List<Object>` 或完整 `JsonNode` 表达请求/响应。
- `OpaqueJsonValue` 只表示明确命名的客户端扩展字段；OpenAPI 的扩展字段显式引用这个 schema，并标明它不能承载完整画布文档。`JsonNode` 只留在 H2 state codec、fixture 和内部状态转换边界。
- 不实现真实 Provider、计费、OAuth、对象存储、生产 PostgreSQL、Agent runtime、分布式并发或完整业务策略。黑盒通过只表示本 reference stub 满足当前受控 wire/状态观察，不表示真实 Java 迁移已经完成。

## 启动

```bash
mvn -q -f java/canvas-contract-reference/pom.xml -DskipTests package
CANVAS_REFERENCE_JDBC_URL='jdbc:h2:file:./data/canvas-contract-reference;MODE=PostgreSQL;AUTO_SERVER=TRUE' \
CANVAS_CONTRACT_FIXTURE_TOKEN='replace-with-an-isolated-token-at-least-24-bytes' \
java -jar java/canvas-contract-reference/target/canvas-contract-reference-0.1.0-SNAPSHOT.jar
```

默认只监听 `127.0.0.1:18183`。可用 `CANVAS_REFERENCE_PORT`、`CANVAS_REFERENCE_BIND_ADDRESS` 和 `CANVAS_REFERENCE_JDBC_URL` 指定端口、绑定地址和数据文件；不要把本地 H2 数据目录提交到 Git。

启动后可查看：

- `GET /api/health`：typed health probe，只证明进程可达。
- `GET /v3/api-docs`：OpenAPI JSON；这是 DTO/wire 阅读入口，不是黑盒通过证据。
- `/swagger-ui/index.html`：本地查看 OpenAPI。

私有 fixture 路由（例如 `/reset`）只在 contract 模式启用，并要求 `Authorization: Bearer <CANVAS_CONTRACT_FIXTURE_TOKEN>`；它们不属于产品 API 或 91 个 Required operation。使用非空 H2 数据目录运行注册模式前，先对隔离目录执行 reset，或为每轮使用独立数据库和显式账号。

## 项目结构与状态流

### 目录职责

| 路径 | 责任 | 真实 Java 移植时的使用方式 |
| --- | --- | --- |
| `src/main/java/.../api/canvas/` | 画布文档、节点、连线、历史、恢复、分享、导入的 Controller 和 DTO | 优先阅读这里获取核心 wire format、字段约束和 OpenAPI 注解；不要复制其 mock 状态实现 |
| `src/main/java/.../api/project/` | 项目、单元、画布关联和工作台 DTO/Controller | 作为项目边界与画布关联合同的类型参考 |
| `src/main/java/.../api/resource/` | 资源、上传、分片、访问、Asset/legacy DTO/Controller | 作为资源引用、文件访问和上传状态合同的参考；不代表真实 OSS/CDN 实现 |
| `src/main/java/.../api/generation/` | mock task、text replay、SSE、creation-run 和 Agent 边界 | 只提供可观察 wire/state 形状，不实现 Provider 或 Agent runtime |
| `src/main/java/.../api/auth/`、`api/admin/` | mock 登录/session/logout、用户和设置 fixture | 为 HTTP runner 和浏览器 companion 提供辅助 API；真实服务需重新实现权限和安全策略 |
| `src/main/java/.../api/fixture/` | contract 模式的 reset、时间和过期准备动作 | 仅供隔离测试准备，不是产品 API，不应暴露到生产 |
| `src/main/java/.../api/common/` | envelope、错误、OpenAPI 配置、压缩/请求过滤和命名扩展类型 | 重点参考错误/wire 边界；`OpaqueJsonValue` 只用于明确命名的扩展字段 |
| `src/main/java/.../contract/` | `ReferenceService`、`ReferenceStore` 和 reference-only 状态操作 | 解释测试状态如何被保留，不是生产领域服务分层模板 |
| `src/main/java/.../persistence/`、`src/main/resources/db/migration/` | 单行 reference state entity/repository、Flyway schema | 只说明 H2 文件型状态如何持久化；不要把 state blob/entity 复制到生产模型 |

### 请求与状态流

```text
HTTP request
  -> api/*Controller
  -> ReferenceService（认证、统一错误和 mock 操作编排）
  -> ReferenceStore（读取/更新 reference state）
  -> ReferenceStateRepository / JPA
  -> H2 file + Flyway schema
  -> typed DTO / ApiEnvelope / HTTP headers
```

`ReferenceStore` 把多个 mock 集合压缩保存为一个持久化 JSON state blob，是为了用最小实现复现请求之间的依赖、CAS、重启恢复和 fixture 操作；它不是未来生产 Java 的数据库设计。核心画布 Controller 的请求/响应仍使用显式 DTO，完整画布文档不通过 `Map<String, Object>`、裸 `Object`、`List<Object>` 或完整 `JsonNode` 暴露。只有内部 state codec、fixture 和明确命名的 `OpaqueJsonValue` 扩展边界可以接触动态 JSON。

## HTTP 黑盒验收

HTTP 主套件是独立 Go module，不依赖 Go 产品内部包，也不需要启动 Web 前端或真实浏览器。它直接把同一组请求和断言发送到本 jar：

```bash
(
  cd tests/canvas-contract
  CANVAS_CONTRACT_BASE_URL=http://127.0.0.1:18183 \
  CANVAS_CONTRACT_FIXTURE_URL=http://127.0.0.1:18183 \
  CANVAS_CONTRACT_FIXTURE_TOKEN='replace-with-an-isolated-token-at-least-24-bytes' \
  CANVAS_CONTRACT_REGISTER_USERS=true \
  CANVAS_CONTRACT_IMPLEMENTATION=java-reference \
  CANVAS_CONTRACT_DB_PROFILE=h2-file \
  CANVAS_CONTRACT_PROFILE=core \
  go test ./... -run '^TestCanvasContract$' -count=1 -v
)
```

当前 Required gate 固定为 11 个 profile、91 个 Required operation、118 个 HTTP scenario。Required operation 中部分是 restart/recovery 或行为变体，复用同一个 HTTP method/path；因此静态 OpenAPI 审计以 71 个 Required 唯一路由和 87 个完整 reference operation 为方法/路径口径，不能把 operation 数量误读为路由数量或代码覆盖率。

已验证的 profile 和子场景数量：

| Profile | 子场景 | 运行证据 |
| --- | ---: | --- |
| `core` | 17 | `.local/canvas-contract-reference/browser-20261014-final/http-rerun/core.log` |
| `share-public` | 10 | `.local/canvas-contract-reference/browser-20261014-final/http-rerun/share-public.log` |
| `project-integration` | 9 | `.local/canvas-contract-reference/browser-20261014-final/http-rerun/project-integration.log` |
| `project-workbench-core` | 15 | `.local/canvas-contract-reference/browser-20261014-final/http-rerun/project-workbench-core.log` |
| `resource-integrity` | 26 | `.local/canvas-contract-reference/browser-20261014-final/http-rerun/resource-integrity.log` |
| `import` | 8 | `.local/canvas-contract-reference/browser-20261014-final/http-rerun/import.log` |
| `legacy-assets` | 9 | `.local/canvas-contract-reference/browser-20261014-final/http-rerun/legacy-assets.log` |
| `public-resource` | 8 | `.local/canvas-contract-reference/browser-20261014-final/http-rerun/public-resource.log` |
| `restart-recovery` | 8 | `.local/canvas-contract-reference/browser-20261014-final/http-rerun/restart-recovery.log` |
| `generation-boundary` | 4 | `.local/canvas-contract-reference/browser-20261014-final/http-rerun/generation-boundary.log` |
| `generation-writeback` | 4 | `.local/canvas-contract-reference/browser-20261014-final/http-rerun/generation-writeback.log` |
| **合计** | **118** | 11/11 profile |

以上为 2026-10-14 当前 Java jar 的隔离重跑证据，位于本机 Git 忽略目录 `.local/canvas-contract-reference/browser-20261014-final/http-rerun/`；它们不属于仓库输入。提交前不应把账号、Cookie、token、完整画布正文、私有 H2 文件或这些完整日志复制进仓库。

## 自动化验证 Runbook

下面把“能否启动参考桩”“HTTP 合同是否通过”和“真实 Web 调用链是否工作”分成独立层次。除特别说明外，命令均从仓库根目录执行。

### 1. 编译与 OpenAPI smoke check

```bash
mvn -q -f java/canvas-contract-reference/pom.xml test
mvn -q -f java/canvas-contract-reference/pom.xml -DskipTests package

curl -fsS http://127.0.0.1:18183/api/health
curl -fsS http://127.0.0.1:18183/v3/api-docs > /tmp/canvas-reference-openapi.json
```

`test`/`package` 是 Java 项目自身的编译和测试门禁；`health` 只证明进程可达，`v3/api-docs` 才是阅读 DTO/schema/operation 的自动化入口。OpenAPI JSON 不等于黑盒行为通过。

### 2. 启动隔离的 Java 参考桩

为每次合同运行使用新的 H2 文件目录，避免注册账号、session、revision 和上传状态互相污染：

```bash
mkdir -p .local/canvas-contract-reference/run-01
CANVAS_REFERENCE_JDBC_URL="jdbc:h2:file:$(pwd)/.local/canvas-contract-reference/run-01/state;MODE=PostgreSQL;AUTO_SERVER=TRUE" \
CANVAS_CONTRACT_FIXTURE_TOKEN='replace-with-an-isolated-token-at-least-24-bytes' \
java -jar java/canvas-contract-reference/target/canvas-contract-reference-0.1.0-SNAPSHOT.jar
```

另开终端后，可先清空该 reference state：

```bash
curl -fsS -X POST http://127.0.0.1:18183/reset \
  -H 'Authorization: Bearer replace-with-an-isolated-token-at-least-24-bytes'
```

`/reset`、`/set-canvas-updated-at`、`/expire-upload-reservation` 和 `/expire-share` 是隐藏的私有 fixture；它们只在 `CANVAS_REFERENCE_CONTRACT_ENABLED=true` 且 token 匹配时可用，不属于产品 API。不要让 runner 直接读写 H2 文件。

### 3. HTTP 主套件

HTTP 主套件不需要 Web dev server、React 或 Chromium，只需要已经启动的目标后端和相应 fixture。最小可执行例子是 `core`：

```bash
(
  cd tests/canvas-contract
  CANVAS_CONTRACT_BASE_URL=http://127.0.0.1:18183 \
  CANVAS_CONTRACT_FIXTURE_URL=http://127.0.0.1:18183 \
  CANVAS_CONTRACT_FIXTURE_TOKEN='replace-with-an-isolated-token-at-least-24-bytes' \
  CANVAS_CONTRACT_REGISTER_USERS=true \
  CANVAS_CONTRACT_IMPLEMENTATION=java-reference \
  CANVAS_CONTRACT_DB_PROFILE=h2-file \
  CANVAS_CONTRACT_PROFILE=core \
  go test ./... -run '^TestCanvasContract$' -count=1 -v
)
```

完整 Required gate 不是只运行 `core`：依次覆盖 `core`、`share-public`、`project-integration`、`project-workbench-core`、`resource-integrity`、`import`、`legacy-assets`、`public-resource`、`restart-recovery`、`generation-boundary` 和 `generation-writeback`。这些 profile 需要不同的隔离 fixture、管理员账号、固定 TLS 上游或进程控制；逐 profile 的前置条件、环境变量和边界以[合同套件 README 的服务运行与配置说明](../../tests/canvas-contract/README.md#run-against-a-server)和[类型参考桩进度账本](../../docs/drafts/canvas-java-type-reference-stub-progress.md)为准，不能把一个 `core` 绿色结果扩写成全量通过。当前参考桩已记录 11/11 profile、118/118 scenario 的运行证据，但证据日志在本机 `/tmp`，不是仓库输入。

### 4. 浏览器 companion

浏览器 companion 是另一套黑盒验收：它通过真实 React/Vite 页面进行用户操作，页面再向目标后端发 HTTP 请求。它不计入 HTTP runner 的 operation/route/scenario 覆盖分母，也不是 HTTP 主套件的替代品。脚本本身不会启动 Java、Vite 或 Chromium 服务。

准备一次 Python/Playwright 环境：

```bash
python3 -m venv .local/cache/canvas-contract-browser-venv
.local/cache/canvas-contract-browser-venv/bin/pip install \
  -r tests/canvas-contract/browser/requirements.txt
.local/cache/canvas-contract-browser-venv/bin/python \
  -m playwright install chromium
```

Java 后端已在 `18183` 运行时，另开终端启动真实 Web：

```bash
cd web
VITE_API_PROXY_TARGET=http://127.0.0.1:18183 \
bun run dev --host 127.0.0.1 --port 18184 --strictPort
```

再另开终端运行 companion：

```bash
CANVAS_CONTRACT_BROWSER_ADMIN_USERNAME='isolated-admin' \
CANVAS_CONTRACT_BROWSER_ADMIN_PASSWORD='isolated-admin-password' \
.local/cache/canvas-contract-browser-venv/bin/python \
  tests/canvas-contract/browser/run.py \
  --implementation=java \
  --web-origin=http://127.0.0.1:18184 \
  --run-id=java-browser-01 \
  --report=.local/browser-report.json \
  --isolated-fixture
```

`run.py` 不启动 Java、Vite 或 Chromium，也不创建管理员；它先使用 `CANVAS_CONTRACT_BROWSER_ADMIN_USERNAME` / `PASSWORD` 登录现有管理员，再通过管理员 API 创建两个普通测试用户。如果是刚 reset 的空 H2，先用隔离的浏览器管理员凭据调用 `POST /api/auth/register` 建立首个用户（参考桩会将空库首用户设为 admin），再运行脚本；不要在非隔离或生产数据上做此 bootstrap。运行前确认 `/api/health`、Vite proxy 和管理员登录都指向同一个隔离目标。

2026-10-14 对当前 Java jar 的执行证据：run id `java-browser-20261014-current`，报告位于 Git 忽略目录 `.local/canvas-contract-reference/browser-20261014-final/report-current-java-browser.json`，6/6 登记场景通过。报告收集了 188 个真实页面 HTTP 响应头事件，其中 101 个 500 来自十个未列入当前场景 required UI routes 的辅助 API：`/api/announcements`、`/api/auth/settings`、`/api/banner-announcements`、`/api/model-catalog`、`/api/plugins/status`、`/api/public/appearance`、`/api/skills`、`/api/skills/added`、`/api/skills/presets`、`/api/wallet`。所以 `passed=true` 仅表示六个登记旅程的指定 UI、HTTP 路径和持久化断言通过，不表示这些辅助端点或所有页面调用均通过；日志和完整 `remaining_paths` 仅存本机报告。不要将此 companion 结果计入 91 个 Required operation / 118 个 HTTP scenario，也不要据此宣称全页面 API 兼容。

浏览器旅程及各自必需页面请求登记在 [`browser/scenarios.json`](../../tests/canvas-contract/browser/scenarios.json)，包括登录失败/owner session、创建重开、文本/媒体/拓扑、匿名分享撤销、foreign 隔离和同上下文账号切换。Java 参考桩的当前完成度及与 HTTP profiles 的分离证据见[实施进度账本](../../docs/drafts/canvas-java-type-reference-stub-progress.md)。

### 5. Go/Web 开发环境的边界

```bash
./scripts/dev.sh --force=kill
```

该命令启动的是 Go backend（默认 `28080`）和 Web dev server（默认 `28300`），用于 Go 基线或现有产品探索，不会启动 Java 参考桩。Java 联调必须先单独启动 Java，再用上面的 `VITE_API_PROXY_TARGET` 启动 Vite；不能用 `scripts/dev.sh` 的绿色健康检查替代 Java 合同验证。

## 人工验证方法

人工验证用于快速确认“读者能否从类型和页面行为理解合同”，是自动化 HTTP/浏览器验收的补充，不替代它们，也不证明生产实现。建议每次 DTO 或 OpenAPI 变更后按以下顺序记录结果：

1. 打开 `http://127.0.0.1:18183/swagger-ui/index.html`，检查画布写入、读取、历史、恢复、分享和导入操作是否显示明确的 request/response schema；确认核心模型展示为 `CanvasDocumentDto`、`CanvasNodeDto`、`CanvasConnectionDto` 等命名类型，扩展字段只引用 `OpaqueJsonValue`，没有退化成完整 `JsonNode`。
2. 用 `curl` 检查 `/api/health`、`/v3/api-docs` 和带 fixture Bearer token 的 `/reset`；确认 reset 只作用于隔离 H2，不把 token 写入日志或报告。
3. 登录 Web 页面，执行“错误密码 → 正确登录 → 刷新 session”；预期错误密码被拒绝，正确登录建立 Cookie session，刷新后仍能识别当前用户。
4. 在创作台执行“新建画布 → 重命名 → 离开 → 重开”；预期名称和画布身份保持，且没有访问另一账号的画布。
5. 在画布中编辑文本、显示媒体并保存，再刷新页面；预期文本内容、媒体尺寸/资源身份和既有连线都保留，文本编辑不改变无关拓扑。
6. 创建分享链接，用匿名页面访问，再停止分享；预期匿名页面先能读到画布，撤销后同一链接被拒绝，owner 仍能读取自己的文档。
7. 用 foreign 账号访问 owner 的列表和详情，再切换同一浏览器账号并在另一页 reload；预期 foreign 看不到 owner 资源，缓存不会串账号。

浏览器脚本的 setup/audit 阶段可能直接调用 API 预置用户、媒体和画布；这些调用不应被误报为 UI 操作覆盖。若只想验证后端，运行 HTTP runner；若想验证前端是否正确消费合同，才启动 Vite 和 Chromium。

## 失败排查与证据归档

| 现象 | 优先检查 | 常见含义 |
| --- | --- | --- |
| `health` 连接失败 | Java 进程、`CANVAS_REFERENCE_PORT`、绑定地址和启动日志 | 服务未启动、端口被占用或绑定地址不匹配 |
| `/reset` 返回未授权 | `CANVAS_CONTRACT_FIXTURE_TOKEN` 与请求 Bearer token 是否完全一致 | fixture token 缺失、长度不足或目标不是 contract 模式 |
| 注册用户已存在 | H2 文件是否复用、是否先 reset | 测试隔离失败；换新的 `.local` 数据目录，不要删除共享/生产数据 |
| HTTP runner 要求 fixture/control/admin | 阅读 profile 配置表和 progress 账本 | 该 profile 需要语义等价的私有 fixture；不能让 runner 直接改 Java 数据库 |
| OpenAPI 出现 `JsonNode` 或弱类型核心字段 | 检查对应 DTO 的 `@Schema`/`@ArraySchema` 和 springdoc 输出 | 类型参考桩发生 schema 回退，应修复 DTO 注解后重新跑静态门禁 |
| Web 页面打开但 API 404/登录失败 | Vite `VITE_API_PROXY_TARGET`、同源地址、Cookie 和 Java 辅助 API | 前端依赖的 HTTP 合同或辅助端点尚未由 Java/混合网关提供 |
| 浏览器通过但 HTTP runner 失败 | 分别查看 `.local/browser-report.json` 和 Go test 输出 | 两套执行器独立统计，不能用 companion 结果覆盖 HTTP runner 缺口 |

证据应保留命令、目标 implementation、数据库 profile、run id、服务日志摘要和脱敏报告；不要提交 Cookie、密码、token、完整画布正文、H2 文件或含敏感 URL 的输出。

## 参考桩与真实移植的关系

未来 Java 业务移植应把本项目当成 wire/type/OpenAPI 参考和黑盒回归目标，而不是当成生产领域模型的直接实现：

1. 先阅读 [类型参考桩实施规划](../../docs/drafts/canvas-java-type-reference-stub-plan.md)、[实施进度](../../docs/drafts/canvas-java-type-reference-stub-progress.md) 和 [HTTP 合同测试计划](../../docs/drafts/canvas-java-http-contract-test-plan.md)。
2. 以当前 Go baseline、manifest、前端 API 客户端和批准的行为决策冻结外部合同；不要因为 reference stub 使用 H2/state codec 就复制其内部存储结构。
3. 真实 Java 服务必须重新证明 PostgreSQL、事务、权限、资源归属、Provider/outbox、对象存储、SSE、重启和切流；本桩的绿色结果不能替代这些门禁。
4. 保留现有 Web 时，HTTP 主套件不需要 Web；只有要验证真实前端调用链、响应解包、缓存和页面结果时，才额外启动 Vite/Web 与 Chromium companion。前端与 Java 的兼容性仍取决于批准的 HTTP/Cookie/SSE/资源合同。

关联入口：

- [Go → Spring Boot 迁移指引](../../docs/drafts/go-to-spring-boot-migration-guide.md)
- [画布开发导航审计](../../docs/drafts/canvas-development-navigation-audit.md)
- [画布系统深度调研](../../docs/drafts/canvas-deep-research.md)
- [合同套件 README](../../tests/canvas-contract/README.md)
