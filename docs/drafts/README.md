# 研究草稿

> **目的**：作为当前仓库的低冲突文档研究区，承载架构梳理、文档审计、调研结论和待确认设计。
> **状态**：持续维护；草稿不会自动成为发布文档。
> **更新规则**：新调研优先新增或增量修改本目录文件，稳定后再评估是否提升到 `docs/content/docs/`。

## 使用方式

1. 进入仓库后先读 `AGENTS.md`、`README.md`、`docs/index.md` 和本页。
2. 先看 `DOCUMENTATION_PLAN.md`，确认当前研究主题和优先级。
3. 新结论放在本目录，必须说明证据范围、验证命令和未确认边界。
4. 不要把文档中的计划、历史设计或待测试项目写成当前已实现事实。
5. 需要发布时，先确认链接、代码入口、命令和真实运行状态，再移动或整合到发布文档。

## 命名规范

- 使用小写英文短横线文件名，名称描述主题，不使用 `NOTES.md`、`TEMP.md` 等泛名。
- 研究和审计使用 `<topic>-research.md` 或 `<topic>-audit.md`。
- 设计方案使用 `<topic>-design.md`；阶段计划使用 `<topic>-plan.md`。
- 不以日期作为当前草稿的唯一标识；日期放在 frontmatter 或文档元信息中。

## 当前草稿

- [文档体系建设计划](DOCUMENTATION_PLAN.md)：当前文档状态、缺口、优先级和推广条件。
- [项目整体认知与文档系统审计](project-understanding-and-docs-system-audit.md)：项目架构、关键调用链、部署边界和文档构建/发现性审计。
- [模型与渠道配置的存储方式研究](model-channel-configuration-storage-research.md)：生图/视频/LLM 模型的配置分层——代码内置协议、数据库配渠道/模型/价格/路由、`.env` 仅保留运行时开关。
- [本地前后端构建与快速启动指引](local-dev-quickstart.md)：本地起前后端 dev 环境的最小步骤，含 3000 端口冲突与本机 HTTP 代理干扰两个坑，及「不常用端口」联动启动方法；并给出「清空测试残留 → 启动 → seed」的完整可重复循环（`dev-reset.sh` + `dev.sh` + `dev-seed-models/seed.ts`），以及管理后台 `/admin` 的入口位置（首页无链接，在头像菜单 / 账户卡片，管理员可见）。
- [用火山方舟（Ark）密钥 Seed 模型数据](seed-volcengine-ark-models.md)：从外部 `api-server/.env` 导入环境变量 → 用 seed 脚本灌入 Seedance 2.0 视频 / 即梦 5.0 Lite 图片 / Grok 文本模型的可重放步骤（`scripts/dev-seed-models/seed.ark.yaml`），并说明不 seed 时在 Web UI（系统渠道 `/admin/channels` / 个人渠道）手动新增模型的对照入口。
- [云端 Agent 深度调研报告](cloud-agent-deep-research.md)：Agent 能力矩阵、前端面板、Go 控制面、Pi bridge、Skill 生命周期、画布读写、媒体审批、持久化恢复、排障和开发者导读。
- [Agent 能力与技术实现全景调研](agent-capabilities-and-architecture-research.md)：对照画布云端 Agent 与助手消息内的创作交互，梳理产品能力、不同执行合同、持久化/审批边界和架构取舍。
- [技能广场与 Skill 运行机制调研](skills-usage-and-runtime-research.md)：解释 `/skills` 中产品技能的用途、加入和选用方式、云端 Agent 与普通创作入口的加载差异，以及技能文本和实际工具能力的边界。
- [画布系统深度调研报告](canvas-deep-research.md)：面向准备自建 AI 视频创作画布的开发者，讲七层状态模型、数据模型、三方合并、生成回写、资源完整性、渲染性能与实施清单。
- [预演台系统深度调研报告](previs-workbench-deep-research.md)：研究嵌入画布的 3D 预演工作台，覆盖场景 schema、动画语义、本地草稿与画布同步、Three.js 截帧录制、输出物化和 Agent 执行边界。
- [当前数据模型综述与导航](data-model-overview-and-navigation.md)：汇总浏览器状态、后端持久化、JSON 文档与物理资源的模型边界，按领域导航到源码和专题，并记录 schema 版本差异。
- [后端架构考古与 Spring Boot 移植评估](backend-architecture-review.md)：从当前 Go 源码与测试归纳后端架构得失，区分可迁移原则、项目特定选择和待验证范围。
- [Go 后端迁移到 Java Spring Boot 指引](go-to-spring-boot-migration-guide.md)：面向保留现有 Web UI 的 Java 重建，覆盖模块结构、数据与 API 兼容、异步生成、资源、Agent、切换和验收；[运行边界与阅读入口](go-to-spring-boot-migration-guide.md#迁移验收运行边界与阅读入口)说明保留前端的条件与分层验收准备。
- [画布 Java Spring Boot 类型参考桩实施规划](canvas-java-type-reference-stub-plan.md)：规划只使用 H2 file + seed/mock 数据的强类型 Spring WebMVC 参考桩，并把现有黑盒套件的 Required、浏览器、远端和独立 suite 缺口登记为后续补全任务；不是已完成实现。
- [画布 Java 迁移前置条件：HTTP 黑盒合同验证方案](canvas-java-http-contract-test-plan.md)：冻结画布端点/wire 合同、建立 Go 基线并验收 Java 与混合路由；[独立运行边界](canvas-java-http-contract-test-plan.md#http-与浏览器的独立运行边界)及[文档维护责任](canvas-java-http-contract-test-plan.md#文档落点与维护责任)说明 HTTP/浏览器执行器、前端依赖和后续变更应更新何处。
- [自动化回归测试与核心功能验收现状调研](test-regression-and-ci-research.md)：盘点前后端测试、PR CI/CD 实际门禁、预演台 Chrome E2E 覆盖及缺口，并给出修改功能后的分层验证步骤。

## 草稿质量要求

- 结论必须能回溯到代码、配置、测试、日志、提交或现有文档。
- 明确区分“已实现”“文档意图”“推断”和“待验证”。
- 命令只有实际执行过才能写成已验证；未执行的命令要明确标记。
- 尽量使用仓库相对路径，避免写入机器敏感路径、真实密钥或用户数据。
- 与 `upstream` 合并相关的改动优先留在本目录，减少发布文档冲突。

## 生命周期

```text
调研 / 审计 -> docs/drafts/ -> 评审确认 -> docs/content/docs/ 或 docs/design/
                                      \
                                       -> docs/adr/（稳定决策）
```

草稿完成后不自动删除。若内容已经发布，应在草稿末尾注明推广目标和对应的正式文档，避免重复维护两份事实源。
