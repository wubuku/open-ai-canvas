---
title: 自动化回归测试与核心功能验收现状调研
status: draft
scope: research
updated: 2026-10-11
---

# 自动化回归测试与核心功能验收现状调研

> **目的**：帮助项目维护者和考虑重建同类 AI 创作工作台的开发者判断现有测试能证明什么、改动后如何建立回归信心，以及 CI/CD 还缺少哪些用户流程验收。
> **状态**：现状研究草稿；不构成生产质量保证或完整测试计划。
> **证据范围**：本次检查了 GitHub Actions 工作流、前端 package scripts、预演台 Chrome E2E、前后端测试文件和仓库验证约定。没有运行测试、启动服务或执行真实生成。

## 结论

当前项目已有覆盖面较广的前后端自动化测试，并在 PR CI 中运行后端全量 Go 测试、前端类型检查和测试套件，以及一组真实 Chrome 驱动的预演台 P0 E2E。后端 CI 还启动 PostgreSQL、提供 Redis 可执行程序，并阻断指定依赖型测试被跳过的情况。

但这不等于已有覆盖全产品核心业务流程的端到端回归验收集。现有 Chrome E2E 聚焦预演台；画布、生成、素材、Agent、后台等有大量专项单测、契约或集成测试，但从仓库工作流和浏览器脚本看，没有一套覆盖创作者与管理员主要完整用户旅程的通用 E2E 集。测试配置中也未发现覆盖率阈值。是否要求 PR 检查全部通过才能合并，取决于 GitHub 仓库的分支保护设置，本地仓库无法确认。

因此，修改一个功能后应组合使用：针对行为的专项回归测试、相关端的全量检查、CI 依赖型测试，以及与风险相称的真实页面或集成环境验收。有限测试只能提高信心，不能证明没有任何回归。

## 现有测试能力

| 层次 | 当前证据 | 能提供的信心 | 不能单独证明 |
| --- | --- | --- | --- |
| 前端纯逻辑与组件测试 | `web/test/` 中覆盖画布状态、节点、连线、素材引用、生成合同、Agent、后台 UI 等主题的 Bun 测试 | 局部规则、边界条件和回归行为没有被明显破坏 | 页面路由、真实浏览器布局、完整用户旅程或后端部署行为 |
| 前端协议与持久化测试 | 画布远端 revision、生成结果同步、素材资源引用、Agent 同步等专项测试 | 前后端数据合同、竞态处理和持久化适配的局部行为 | 多设备真实同步、真实对象存储、模型上游或长期运行表现 |
| Go 单元与服务/仓储测试 | `backend/**` 下的 `_test.go` 覆盖 app、repository、handler、provider、协议、资源、任务、权限、计费和 Agent | 服务规则、数据库读写、HTTP 合同及部分跨层流程 | 生产拓扑、真实外部供应商、真实流量和所有 SQL 方言差异 |
| 依赖型后端集成测试 | CI PostgreSQL service、Redis 安装步骤及依赖测试强制检查 | 指定 PostgreSQL/Redis 路径在 CI 环境确实执行，而不是静默 skip | 生产数据迁移、集群故障、真实多实例竞争或生产容量 |
| 浏览器 E2E | [预演台 Chrome E2E](../../web/scripts/previs-p0-chrome-e2e.mjs) 启动 Vite 与 headless Chrome，通过 CDP 驱动 `/dev/previs-repro` | 预演台的若干交互、失败态和恢复边界在真实 Chrome 中可操作 | 整个创作台/管理后台流程、生产构建产物、真实后端及真实模型生成 |

测试文件数量不是质量或覆盖率指标；应看场景是否对应业务风险、断言是否观察到用户可见结果，以及测试运行时是否包含真实依赖。

### 有代表性的专项覆盖

- 画布远端保存与 revision：[canvas-remote-revision.test.ts](../../web/test/canvas-remote-revision.test.ts) 覆盖陈旧保存协调、重试、编辑期间保存、用户 scope 隔离、历史恢复等行为。
- 生成结果同步：[canvas-generation-task-sync.test.ts](../../web/test/canvas-generation-task-sync.test.ts) 覆盖重复提交、结果落回被删除节点时的保护、生成期间移动/改名的保留和旧素材绑定清理。
- Agent 画布同步：[agent-canvas-sync.test.ts](../../web/test/agent-canvas-sync.test.ts) 覆盖批量事件合并、刷新不重叠、乱序冲突和画布身份隔离。
- 资源引用：[canvas-resource-references.test.ts](../../web/test/canvas-resource-references.test.ts) 覆盖引用 token、资源身份、连接排序和提示词提及之间的局部合同。
- 后端任务和数据边界：[app 测试](../../backend/internal/app/)、[repository 测试](../../backend/internal/repository/)和[handler 测试](../../backend/internal/handler/)中有生成任务恢复、取消、用户归属、画布历史、资源完整性、Agent 审批和工具分派等测试。

这些例子说明测试资产不是空白；同时它们多数验证单个模块、服务合同或受控夹具，不应被描述成真实用户端到端验收。

## CI/CD 实际执行内容

### PR 质量检查

[质量工作流](../../.github/workflows/quality.yml)的触发器是 `pull_request` 和 `workflow_call`。

后端 job：

- 按 `backend/go.mod` 安装 Go 工具链，并安装 Pi Agent runtime 依赖。
- 启动 PostgreSQL 16，并通过 `CANVAS_TEST_POSTGRES_DSN` 提供测试连接。
- 安装 `redis-server` 可执行程序；部分测试会自行启动临时 Redis。
- 检查 `gofmt`，运行 `go test -json -timeout=20m ./...`。
- 检查五个指定 PostgreSQL/Redis 用例是否实际运行并通过；仅没有失败但被 skip 不算通过。

前端 job：

- 使用固定 Bun 版本和冻结 lockfile 安装依赖。
- 检查改动文件格式、运行 TypeScript `typecheck`、ESLint 和 `bun run test`。其中 ESLint 的产品范围应结合 [AGENTS.md](../../AGENTS.md) 理解：当前约定将其定位为 UI 退场规则检查，不是全面风格或缺陷分析。
- 运行 `bun run test:previs:e2e`，用 headless Chrome 验收预演台 P0 场景。

质量工作流还会验证支付插件包，并在 PR 中对 amd64/arm64 执行支付插件镜像 smoke。该项是发布产物兼容性检查，不是业务用户流程验收。

### 发布检查与门禁边界

[镜像发布工作流](../../.github/workflows/publish-images.yml)在推送到 `main`、推送 `v*` 标签或手动触发时运行，并通过 `workflow_call` 执行上述质量检查。可变版本标签的推广和 Release 资产发布依赖 `checks` 成功。

镜像构建 job 与质量检查并行，构建的是不可变 commit SHA 镜像；可变标签推广等待质量检查。因此，检查失败可以阻止正式标签推广，但构建期间生成的 SHA 镜像不等同于“检查通过后的发布镜像”。

### 当前可见的 CI 空白

1. PR 的 Web job 没有显式运行 `bun run build`。[根目录 Dockerfile](../../Dockerfile)的 Vite 生产构建发生在 Web 镜像构建阶段，而镜像发布工作流不是 PR 工作流的一部分；因此常规 PR 检查并未直接证明生产 Vite bundle 可构建。
2. 浏览器 E2E 只有预演台 P0 专项，不构成覆盖画布创作、项目管理、生成任务、素材管理、管理员权限等完整主链路的验收套件。
3. 在本次检查的 package scripts 和工作流中未发现覆盖率报告或覆盖率阈值门禁。
4. 分支保护和 required status checks 属于 GitHub 仓库设置，不在当前 checkout 的工作流文件中；需要管理员在 GitHub 设置中确认。

## 预演台 E2E 的实际范围

脚本启动本地 Vite 和 Chrome，不启动 Go 后端；它访问确定性的 `/dev/previs-repro` 页面，通过 Chrome DevTools Protocol 执行真实鼠标、键盘、网络节流和 WebGL 上下文操作。六组场景包括：

1. 工作台打开、摆场/动画模式、时间轴显隐、工具提示、添加道具与撤销；
2. 本地 glTF 模型加载稳定性；
3. 缺失模型错误、用户重试及诊断事件；
4. 模型加载中删除对象，验证迟到结果不回流；
5. WebGL context lost/restore；
6. 保存失败时显示重试入口，并阻止用户误关闭未保存工作台。

脚本还会捕获浏览器异常、console error 和失败的网络请求。它是有价值的真实浏览器专项测试，但场景使用开发复现页和本地受控数据，不等于已验收生产环境、后端持久化或真实模型生成。

## 修改功能后的验证流程

### 第一步：定义变更合同

先写清用户可观察的成功路径、失败路径和并发/权限边界，并定位对应模块及现有测试。涉及持久化的数据改动要检查重载和旧数据；涉及身份的操作要检查用户归属；涉及异步任务要检查重复、取消、超时、重试和晚到结果。

### 第二步：先跑最相关的回归测试

新增或修复行为时，优先让专项测试能在修改前复现问题、修改后验证预期。前端可从 `web/package.json` 中的 `test:canvas` 或具体 `web/test/*` 文件开始；后端用对应包的 `go test`。专项通过只是第一层，不替代完整相关端检查。

### 第三步：运行端级完整检查

前端建议：

```bash
cd web
bun run typecheck
bun run lint
bun run test
bun run build
```

其中 `bun run build` 是建议的本地生产构建检查；它不在当前 PR Web job 的显式命令中。

后端建议：

```bash
cd backend
go test ./...
```

本地若未设置 `CANVAS_TEST_POSTGRES_DSN` 或机器没有 `redis-server`，相应依赖测试可能 skip。CI 会准备这些依赖并强制检查指定测试运行成功；涉及数据库方言、迁移、租约或缓存语义的改动，应在具备对应依赖的环境中验证。

### 第四步：按风险验收真实用户路径

UI、路由或关键交互改动应在真实应用页面验收，而不是仅依赖源码断言。生成链路改动要检查任务提交、等待/轮询、失败、重试、取消、结果回写和资源访问；涉及收费模型的真实调用应使用隔离账号和明确预算，并单独记录，不能由单测通过推定。

推荐的高风险用户旅程至少包括：

| 旅程 | 关键检查点 |
| --- | --- |
| 登录后创建/打开画布并编辑 | 节点与连线、保存状态、刷新重开、权限和用户 scope |
| 提交图片/视频/音频生成 | 输入合同、任务状态、超时/取消/重试、结果只回写目标节点 |
| 素材上传、引用、替换和删除 | 业务身份、物理对象、引用保护、失败恢复 |
| Agent 读取、审批、写画布和恢复会话 | 工具是否可调用、权限、SSE/断线、重复操作和结果归属 |
| 管理员配置模型、权限或计费策略 | 服务端鉴权、保存重载、用户端效果及错误反馈 |

## 面向新系统的建议

以下是从当前测试资产和缺口归纳出的新实现建议，不代表当前项目已经完成这些工作：

1. **先建立风险驱动的核心验收清单。** 将登录、创建画布、编辑保存、提交生成、结果回写、素材可访问、权限隔离、失败恢复列为发布级场景；按能力分层，避免只累计孤立单测。
2. **把稳定的核心旅程自动化到真实浏览器。** 使用隔离测试后端、种子数据和可替换的模型适配器；关键流程不应依赖收费模型才能在每次 PR 中运行。将少量真实上游 smoke 留给定时任务或发布前人工验收。
3. **生产构建应进入 PR 门禁。** 在质量工作流中直接运行前端生产构建，使合并前的质量结论对应最终静态 bundle，而不只对应 TypeScript 与开发服务器。
4. **用测试类别表达证据。** 区分纯单测、数据库集成、真实浏览器、外部供应商 smoke、容量/混沌验收；报告通过范围和环境，不能用一个“tests passed”标签覆盖所有层次。
5. **并发和异步能力要有确定性测试夹具。** 以可控时钟、可编排的 Promise/future、模拟供应商和真实数据库租约来验证超时、重复、迟到结果、重试和租约过期；再为少量高风险边界保留多进程或真实依赖验收。
6. **覆盖率用于发现盲区，不应替代行为门槛。** 若引入覆盖率门槛，应优先要求关键域和关键路径覆盖，而不是用一个全仓百分比掩盖薄弱的资金、权限、资源和并发边界。
7. **分支保护要与工作流配套。** 将必需 CI job 设为 required check，并明确 main、release tag 和手动发布路径的门禁，防止工作流存在但合并策略没有强制执行。

## 证据与验证边界

本次只读检查的主要入口：

- [quality.yml](../../.github/workflows/quality.yml)：PR/复用质量检查及各 job 命令。
- [publish-images.yml](../../.github/workflows/publish-images.yml)：CI/CD 触发条件、镜像构建与推广依赖。
- [check-dependency-tests.py](../../.github/scripts/check-dependency-tests.py) 和 [test_check_dependency_tests.py](../../.github/scripts/test_check_dependency_tests.py)：指定依赖测试的运行/通过检查。
- [web/package.json](../../web/package.json)：前端构建、类型检查、测试、lint 和预演台 E2E 命令。
- [previs-p0-chrome-e2e.mjs](../../web/scripts/previs-p0-chrome-e2e.mjs)：预演台浏览器验收场景。
- [Dockerfile](../../Dockerfile)：前端 Vite 生产 bundle 的容器构建位置。
- [web/test](../../web/test/) 和 [backend 测试目录](../../backend/)：前后端专项测试资产。
- [AGENTS.md](../../AGENTS.md) 的“验证纪律”：项目按风险选择构建、专项测试、集成/冒烟和真实页面验收的要求。

本次没有运行前端/后端测试、生产构建或 E2E，也没有检查 GitHub 仓库分支保护设置。没有启动真实后端服务，没有调用真实模型，没有执行真实支付、跨设备同步或生产性能压测。以上结论是对当前仓库文件的静态审查，不是运行验收记录。

## 推广条件

若后续要把稳定流程提升为团队正式测试规范，应先补齐 PR 生产构建门禁、确认 GitHub required checks，并建立覆盖画布编辑/保存、生成回写、资源完整性、权限和 Agent 恢复的受控 E2E。完成真实运行验证后，再将稳定的命令和验收矩阵增量整理到正式开发文档；在此之前，本报告留在研究草稿区。
