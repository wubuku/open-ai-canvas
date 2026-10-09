# 文档体系建设计划

> **目的**：在不大范围改动上游文档的前提下，逐步建立可导航、可验证、适合 AI Agent 使用的项目文档体系。
> **状态**：初始计划；后续研究优先增量修改本目录。
> **更新时间**：2026-10-09

## 当前状态评估

### 已有文档资产

- 根入口：`README.md`、`AGENTS.md`、`CONTRIBUTING.md`、`CHANGELOG.md`、`SECURITY.md`。
- 发布文档源：`docs/content/docs/`，包含功能、代码地图、本地开发、HTTP API、数据库、插件和待测试专题。
- 设计与决策：`docs/design/`、`docs/adr/`、`docs/plans/`、`docs/superpowers/`。
- 代码内协议文档：`backend/internal/protocol/docs/` 和 `plugin-packages/*/docs/`。
- 新研究区：`docs/drafts/`，由本计划和草稿入口统一管理。

### 已确认缺口

- 当前 checkout 没有 `docs/package.json`、`docs/source.config.ts` 或 Next.js/Fumadocs 应用入口。
- README 和 AGENTS 记录的 `cd docs && bun run types:check` / `bun run build` 当前不可执行。
- `README.md` 引用的 `quick-start.mdx`、`canvas-node-manual.mdx`、`todo.mdx` 不存在。
- `docs/content/docs/backend/meta.json` 引用的 `protocol-plugins`、`canvas-data-structure` 页面不存在，且未覆盖全部现有后端页面。
- 产品总览、后端调用链、任务生成链、Agent 运行时和部署拓扑分散在多个文件中，缺少一篇统一架构入口。
- 文档内容和发布站构建边界尚未在仓库中形成可执行的单一事实源。

## 目标

1. 让新 Agent 能在 5 分钟内找到项目定位、入口、分层、验证命令和安全边界。
2. 让维护者能从用户功能定位到页面、API、handler、app、repository、provider 和测试。
3. 让调研结论先进入低冲突草稿区，再按稳定性推广到正式文档。
4. 让文档构建、链接检查和“已验证/待验证”状态可被自动或半自动检查。

## 分阶段计划

### R0：草稿区和事实入口（当前阶段）

- [x] 创建 `docs/drafts/README.md`，定义研究区用途、命名、证据和生命周期。
- [x] 创建第一篇项目整体认知与文档系统审计草稿。
- [x] 迁移并适配 `.agents/skills/project-docs/`，默认研究结果落到 `docs/drafts/`。
- [x] 在 `.gitignore` 中允许提交 `docs/drafts/`。
- [ ] 决定文档站应用是否应纳入当前仓库，或明确它属于外部发布工程。

### R1：统一架构与导航（低冲突增量）

- [x] 将前端、后端、任务/模型、Agent、插件和部署的统一调用链提升为正式架构总览。
- [x] 在架构总览中建立“功能 -> 代码入口 -> API -> 持久化/上游 -> 测试”的检索路径。
- [x] 在 `docs/index.md` 增加正式架构页和研究草稿入口。
- [x] 为 `docs/content/docs/overview/` 增加最小导航元数据；其他专题导航待发布工程归属明确后再补齐。

### R2：链接和构建合同

- [ ] 统一 README、`docs/index.md`、MDX 页面和 `meta.json` 的页面命名。
- [ ] 修复或删除已确认失效的文档链接，避免为了兼容旧路径添加无意义别名。
- [ ] 如果文档站在本仓库内，补齐 `docs/package.json`、构建配置和 CI 文档检查。
- [ ] 如果文档站在仓库外，更新当前仓库的命令和责任边界，避免继续记录不可执行命令。

### R3：正式文档推广

- [ ] 将经过评审、链接和命令验证的架构内容提升到正式文档。
- [ ] 将稳定的架构选择沉淀为 ADR，而不是长期保留在研究草稿中。
- [ ] 草稿推广后保留迁移记录，不维护两份相互竞争的事实源。

## 依赖关系

- 文档站归属明确后，才能决定正式 MDX 的导航和构建修复方式。
- 统一架构草稿依赖当前代码地图、任务执行链和 Agent 运行时事实核对。
- 链接修复应在页面命名和发布边界明确后进行，避免反复改路径。
- 正式推广前必须完成命令可执行性和关键代码入口核验。

## 当前验证记录

- 已检查当前分支、remote、目录结构、入口代码、部署文件、文档索引和链接引用。
- `cd docs && bun run types:check`：失败，Bun 报告 `Script not found "types:check"`。
- `cd docs && bun run build`：失败，Bun 报告 `Script not found "build"`。
- 本阶段未启动前后端服务，也未运行全量前端/后端测试；因此不把静态阅读称为运行验收。

## 推广条件

一篇草稿满足以下条件后，才考虑移入 `docs/content/docs/`：

- 事实均有当前代码、配置、测试或运行日志依据；
- 已实现、设计意图和待验证范围明确区分；
- 代码路径和仓库内链接有效；
- 关键命令已实际运行，或明确说明环境限制；
- 有明确的正式文档归属，不会与现有页面重复维护；
- 改动不会无必要扩大与 `upstream` 合并时的冲突面。
