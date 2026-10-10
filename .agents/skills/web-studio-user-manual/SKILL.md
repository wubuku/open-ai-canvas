---
name: web-studio-user-manual
description: 为影策（open-ai-canvas）的 Web UI 创建或更新面向最终用户的中文使用手册。Web UI 分为"创作台"（普通创作者使用）和"管理员后台"（管理员使用）两部分。通过项目静态扫描与真实浏览器探索建立任务清单，在用户确认高频和重要业务操作后，按优先级编写任务导向的教程、操作指南、参考和排障内容；使用经过 DOM/ARIA/网络证据验证的步骤、目标元素高亮截图、截图清单和浏览器回走审计。用于"给创作台或管理员后台写用户手册/操作手册/使用说明/帮助文档/培训指南""生成带截图的产品文档"或审查现有手册是否与真实 UI 一致。不要用于开发产品功能、纯代码 API 文档或未经用户授权便开始生成手册。
---

# Web Studio 用户手册

为影策 Web UI 生成任务导向、证据可追溯、可增量维护的中文用户手册。页面覆盖只是手段；核心目标是让目标用户独立完成高频、重要和高风险操作。

## 产品边界

影策 Web UI 分为两部分，手册也必须分开编写和维护：

| 部分 | 目标用户 | 主要入口 | 手册目录 |
|---|---|---|---|
| **创作台** | 普通创作者 | `/create`、`/inspirations`、`/tasks`、`/assets`、`/skills`、`/plugins`、`/wallet`、`/settings`、`/projects`、`/canvas` | `docs/user-manual/creator/` |
| **管理员后台** | 管理员 | `/admin`（含用户、渠道、模型、插件、支付、公告、资源、额度、设置等子页） | `docs/user-manual/admin/` |

两部分共享同一套方法论、审计脚本和截图标准，但拥有各自独立的 `task-inventory.yml`、截图 Manifest 和侧边栏。不要把管理员操作混入创作台手册。

将当前 `SKILL.md` 所在目录记为 `<skill-dir>`。所有 `references/` 和 `scripts/` 均从该目录解析；不要假设 Skill 固定安装在 `.agents/`、`.claude/` 或某个绝对路径。

## 0. 权限边界

- Skill 被创建、安装或提及时，不自动启动应用、操作浏览器、截图或写手册。只有用户明确要求创建、更新或审查用户手册时才执行。
- 不修改生产业务代码、测试代码、数据库迁移或 `.env` / `.env.*`。需要配置时只使用当前命令的临时环境变量。
- 不使用生产数据、真实用户隐私数据或共享环境做可变操作。不得触发真实视频/图片/文本生成等高成本 Provider 调用。
- 删除、发布、充值、权限变更、额度调整、不可逆写入及外部通知始终先取得用户单独许可；截图任务不构成此类许可。
- 保护现有工作区修改。开始前检查 `git status --short`，不使用 `stash`、`reset --hard`、`checkout --` 或 `clean`。
- 截图只用于说明，不是功能正确性的证据。每个操作步骤必须先通过 DOM、ARIA 状态、网络请求/响应或安全的只读数据证据验证。

执行前读取仓库根 `AGENTS.md`、`docs/index.md`，以及与本次手册范围直接相关的产品文档。

## 1. 建立计划，但先只做快速扫描

复杂手册任务使用 plan，并只保留一个进行中步骤。第一阶段限于低成本、低副作用的快速扫描：

1. 检查当前分支、工作区状态、可用启动方式、前后端端口和浏览器工具。
2. 扫描 `web/src/router.tsx`、`web/src/layouts/`、主要页面组件、权限守卫（`RequireAuth`、`RequireFeature`）、已有测试和产品文档。
3. 如果用户已提供正在运行的 URL 或浏览器标签页，可只读查看应用壳、一级导航和当前角色身份；不要进入深层流程或提交表单。
4. 从页面入口而非仅从 URL 枚举候选用户任务。一个任务应表达用户目标，例如"创建项目并生成第一个分镜视频"，而不是"打开项目详情页"。
5. 标记证据来源和置信度。代码、测试数量和导航位置只能提供候选优先级，不能代替真实用户频率。

快速扫描阶段不创建 `docs/user-manual/`、不拍摄正式截图、不编写正文。

## 2. 强制执行"高频 / 重要任务确认门"

Agent 不得自行最终决定什么是高频或重要。快速扫描后，先向用户提交精简候选表：

| 候选任务 | 目标角色 | 扫描证据 | 推测频率 | 业务影响/失败代价 | 建议篇幅 | 待确认问题 |
|---|---|---|---|---|---|---|

判断维度：

- **频率**：核心日常 / 常用 / 偶发 / 罕见；
- **影响**：关键 / 高 / 中 / 低；
- **角色**：普通创作者 / 管理员 / 外部协作者；
- **失败代价**：费用（视频生成消耗额度）、数据丢失、发布错误、权限风险、长时间等待或无法恢复；
- **文档厚度**：旗舰流程 / 完整 how-to / 简明 how-to / 仅参考。

证据优先级：

1. 用户确认、产品数据、客服/培训反馈；
2. UI 入口显著性和权限守卫；
3. 已有测试覆盖；
4. 路由存在性（最弱证据）。

用户可以直接确认提案，也可以明确委托 Agent 定级。没有这两者之一，不进入正式成稿。

## 3. 真实浏览器探索与取证

优先使用用户指定的 URL 或浏览器会话。当前项目 dev server 通常在 `http://localhost:28300/`。按以下顺序使用第一个可用浏览器工具：

1. 用户明确指定的浏览器、标签页或浏览器会话；
2. 当前 Agent 运行时提供的原生计算机使用/浏览器控制工具；
3. 本地 Playwright 临时脚本。

不能控制真实浏览器时停止并说明缺口。不得仅凭源码、路由或静态截图假装已经验证运行行为。

每个步骤至少记录：

- route；
- 用户看到的入口和准确 UI 文本；
- role/name/label 等稳定 locator；
- 操作前后 DOM/ARIA 状态；
- 关键网络请求的 method、path、status 和响应业务结果；
- 成功判据；
- 失败时可恢复路径。

优先通过可访问性树和 DOM 判断状态。截图像素只用于解释界面，不用于确认按钮是否可用、请求是否成功或数据是否持久化。

## 4. 截图标准

默认：

- desktop viewport：`1440x900`（画布页面建议 `1920x1080`）；
- locale：`zh-CN`；
- 100% zoom；
- light/dark 只选一个，除非主题本身在范围内；
- `prefers-reduced-motion: reduce`；
- 等待 `document.fonts.ready`、网络稳定和关键 locator 可见；
- 同一批截图保持相同浏览器、viewport 和主题。

截图类型：

- **概览图**：保留导航和页面上下文；
- **步骤图**：裁剪至足以理解动作的区域；
- **结果图**：展示可识别的成功状态；
- **恢复图**：只用于高影响错误或容易误解的恢复动作。

### 目标高亮

使用 `<skill-dir>/scripts/highlight-target.js`：

1. 用 role/name/label 找到目标元素；
2. 取得元素自身的 bounding rect；
3. 在实时 DOM 中添加红框和步骤编号；
4. 截图；
5. 调用清理函数移除 overlay。

不要事后凭估计在 PNG 上画框。目标被 sticky header、滚动容器或弹窗遮挡时，先滚动和验证可见性，再计算位置。

Playwright 示例：

```javascript
await page.addScriptTag({ path: `${skillDir}/scripts/highlight-target.js` });
const target = page.getByRole("button", { name: "新建画布", exact: true });
await target.scrollIntoViewIfNeeded();
await target.evaluate((element) =>
  window.webStudioManualHighlight.show(element, { step: 1 }),
);
await page.screenshot({ path: screenshotPath });
await page.evaluate(() => window.webStudioManualHighlight.clear());
```

`show(...)` 是异步函数，必须 `await`。截图失败时也要在 `finally` 中调用 `clear()`，避免高亮污染后续截图。

## 5. 采用任务导向的中文结构

默认使用以下结构，不按页面清单写成截图目录。`<scope>` 为 `creator` 或 `admin`：

```text
docs/user-manual/<scope>/
├── README.md
├── 00-quickstart.md
├── 10-tasks/
├── 20-reference.md
├── 30-concepts.md
├── 90-troubleshooting.md
├── task-inventory.yml
├── PROGRESS.md
├── AUDIT.md
└── screenshots/
    └── manifest.yml
```

- **Quickstart / Tutorial**：最短成功路径，只承诺一个可完成结果。
- **How-to**：一页解决一个用户目标；高频/高影响任务步骤、截图、前置条件和恢复路径更完整。
- **Reference**：字段、状态、权限、设置、限制和快捷键，保持简洁准确。
- **Explanation**：解释概念和流程原因，不塞进操作步骤。
- **Troubleshooting**：来自真实探索的症状 → 原因 → 修复 → 仍失败时如何求助。

每个 how-to 至少包含：适用角色、目标、前置条件、入口、原子步骤、成功判据、取消/恢复、相关任务。正文使用产品中的真实中文标签；不要自行翻译或创造术语。

## 6. 两道交付闸门

### Gate A：覆盖率和机械完整性

- 每个"核心日常"任务有完整步骤和关键截图。
- 每个"关键/高影响"任务有前置条件、风险提示、成功判据和恢复路径。
- 每个 UI 标签与当前 DOM 文本一致；每个快捷键已实际执行验证。
- 每个截图文件存在、有描述性 alt、在 manifest 中登记且没有泄露敏感信息。
- 页面链接、标题层级、任务索引和术语一致。
- 运行：

```bash
python3 "<skill-dir>/scripts/audit_manual.py" docs/user-manual/<scope> --phase gate-a
```

### Gate B：真实浏览器回走

依据手册而不是作者记忆，从头重走每条旗舰流程和完整 how-to：

1. 在干净或可复现状态打开文档指定入口；
2. 逐字核对标签和每个原子步骤；
3. 验证预期请求/响应及最终可见状态；
4. 核对截图是否仍对应当前 UI；
5. 将发现记入 `AUDIT.md`，按 Blocker / Major / Minor 分级；
6. 修正文档后重走受影响流程，直到无 Blocker/Major。

回走完成后运行最终机械审计；此时任务状态只能是 `verified` 或带原因的 `excluded`：

```bash
python3 "<skill-dir>/scripts/audit_manual.py" docs/user-manual/<scope> --phase final
```

审查透镜、严重性和中文成稿规范见 [回走审计与中文质量](references/review-and-zh-cn.md)。

## 7. 完成标准

只有同时满足以下条件才报告完成：

- 用户确认的任务范围全部在 inventory 中闭环；
- 高频和高影响任务获得明显高于低频功能的篇幅与验证密度；
- Gate A 通过；
- Gate B 无 Blocker/Major，Minor 已修复或明确接受；
- 最终机械审计通过；
- `PROGRESS.md`、manifest 和 `AUDIT.md` 支持下一位 Agent 增量更新；
- 最终报告列出目标版本/URL、角色、深度、覆盖率、未覆盖项和已知限制。

除非用户另行要求，不因生成手册而修改产品代码、提交 Git、合并或 push。

## 8. 构建为 VitePress 静态手册站点

当用户明确要求构建、预览或发布手册网站时，使用 `docs/user-manual/` 下的 VitePress 站点；不要另起一套文档站点、搜索服务或构建配置。详细命令和发布方式见 [手册站点构建与发布](references/site-publishing.md)。

基本流程：

1. 先完成内容审计；新增或改名页面时同步 `.vitepress/config.mjs` 的 `sidebar`，并确认页面链接、截图和 `task-inventory.yml` 一致。
2. 在 `docs/user-manual/` 下执行 `./build-site.sh`（首次会安装依赖）。
3. 站点使用 VitePress local search，界面文案和中文搜索配置位于 `.vitepress/config.mjs`。
4. 站点通过 `rewrites` 将根目录 `README.md` 映射为首页，并通过 `themeConfig.sidebar` 提供中文侧边栏（创作台和管理员后台各自分组）。
5. 内部探索记录、审计和进度账本由 `srcExclude` 排除；新增内部 Markdown 文件时必须同步排除规则。
6. 站点只发布 `.vitepress/dist/` 的静态内容。若部署在子路径，先设置 `base` 再重新构建。

构建网站不等于发布网站：上传、覆盖远端静态目录或配置生产域名仍需用户明确授权。

## 9. 参考文件索引

| 文件 | 覆盖内容 |
|---|---|
| [references/priority-and-structure.md](references/priority-and-structure.md) | 候选任务来源、用户确认表、优先级、`task-inventory.yml` 规范 |
| [references/browser-and-screenshots.md](references/browser-and-screenshots.md) | 浏览器取证、环境安全、截图标准、Manifest |
| [references/review-and-zh-cn.md](references/review-and-zh-cn.md) | 回走审计透镜、严重性分级、防幻觉规则、中文成稿 |
| [references/site-publishing.md](references/site-publishing.md) | VitePress 构建、预览、发布边界 |
