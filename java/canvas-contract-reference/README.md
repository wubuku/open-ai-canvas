# Canvas Contract Reference

这是一个面向未来 Java 画布移植的**类型参考桩**，不是生产后端的业务实现，也不是把 Go 包结构逐文件翻译成 Java。

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
| `core` | 17 | `/tmp/canvas-java-core-retry.log` |
| `share-public` | 10 | `/tmp/canvas-java-share-public-final3.log` |
| `project-integration` | 9 | `/tmp/canvas-java-project-integration-final3.log` |
| `project-workbench-core` | 15 | `/tmp/canvas-java-project-workbench-core-final3.log` |
| `resource-integrity` | 26 | `/tmp/canvas-java-resource-integrity-final3.log` |
| `import` | 8 | `/tmp/canvas-java-import-final3.log` |
| `legacy-assets` | 9 | `/tmp/canvas-java-legacy-assets-final3.log` |
| `public-resource` | 8 | `/tmp/canvas-java-public-resource-20261014-openapi-fix.log`（OpenAPI 修复后重启并复验） |
| `restart-recovery` | 8 | `/tmp/canvas-java-restart-recovery-final.log` |
| `generation-boundary` | 4 | `/tmp/canvas-java-generation-boundary-final3.log` |
| `generation-writeback` | 4 | `/tmp/canvas-java-generation-writeback-final3.log` |
| **合计** | **118** | 11/11 profile |

profile 日志位于本机 `/tmp`，不属于仓库交付物；提交前不应把账号、Cookie、token、完整画布正文或私有 H2 文件复制进仓库。

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
