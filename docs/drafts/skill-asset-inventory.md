---
title: 影策 SKILL 与关键 Prompt 资产盘点
status: draft
scope: inventory
updated: 2026-10-16
---

# 影策 SKILL 与关键 Prompt 资产盘点

> 本文是当前仓库的资产索引，不是用户手册，也不是某个单一 Skill 的使用说明。它把可独立加载的 `SKILL.md`、Skill 包内的参考卡片、后端硬编码的 Prompt/策略文本、数据库可编辑的 Prompt 模板、前端 Prompt 预设和模型族规则放在同一张地图上。
>
> 盘点以当前工作区源码为准。资产数量、技能目录和默认模板会随代码变化；文中“当前”均指本次静态扫描快照，不代表所有部署环境永远相同。

## 0. 先看结论

当前项目包含两类容易被混淆的 Skill：

1. **产品 Skill**：位于 `backend/builtin/`，面向影策创作者，由后端嵌入并同步到技能目录，供技能广场、画布 Agent 和普通创作入口按需读取。
2. **仓库协作 Skill**：位于 `.agents/skills/`，面向 Codex 等开发协作者，指导文档、浏览器探索和仓库操作，不属于产品用户 Skill。

本次扫描得到：

| 资产层 | 当前数量/范围 | 主要入口 | 是否直接改变模型能力 |
|---|---:|---|---|
| 产品内置 Skill 根文件 | 71 个 `backend/builtin/*/SKILL.md` | `backend/builtin/embed.go`、`backend/internal/skills/skills_builtin.go` | 否；作为本轮上下文参考 |
| 产品 Skill 支撑文件 | 742 个非 `SKILL.md` 文件，33 个包含 `cards/` 的技能包 | 各 Skill 目录内的 `cards/`、参考 Markdown 和许可文件；数量还包含 `embed.go` 等嵌入辅助文件 | 参考文件由运行时按需选取；辅助文件不作为上下文 |
| 仓库协作 Skill | 2 个 | `.agents/skills/project-docs/`、`.agents/skills/web-studio-user-manual/` | 不属于产品运行时 |
| Agent 固定策略文本 | `agent-system-policy.md`、`AGENTS.md`、`agent-media-policy.md` | `backend/internal/prompts/agent_policy.go` | 约束 Agent 行为、工具和媒体审批 |
| 后端默认 Prompt 模板 | 9 个操作定义 | `backend/internal/prompts/prompt_template_defaults.go` | 直接影响文本、资产、分镜和生成提示词任务 |
| 前端内置 Prompt 预设与辅助词库 | 多组常量/工厂函数 | `web/src/lib/prompts/`、`web/src/lib/canvas/` | 影响用户输入和提交给模型的 Prompt |

“内置了 Skill”不等于“每次请求自动使用全部 Skill”。用户需要先加入技能库，当前入口再按选择或 `@` 提及加载；当前前端 Skill Runtime 每轮最多选择 4 个技能，总链接上下文预算为 32,000 字符。

关于“某个 Skill 是否适用于某个功能”的运行边界，请同时阅读[技能广场与 Skill 运行机制调研](skills-usage-and-runtime-research.md)：当前系统有用户归属、加入状态、版本、上下文预算和真实工具能力限制，但没有按入口、媒体类型、模型或业务操作建立统一的结构化适用范围。`Tag`、技能正文中的适用说明和场景预设主要用于发现和引导，不能当成后端兼容性闸门。

## 1. 资产类型和盘点边界

### 1.1 纳入盘点

- 有明确入口文件、可被产品技能目录加载的 `SKILL.md`；
- Skill 包内被入口引用或用于运行时检索的 Markdown、文本、JSON 和卡片资料；
- 发送给 LLM/Agent 的固定系统策略、媒体策略和默认 Prompt 模板；
- 前端会预填、追加、改写或转换用户 Prompt 的常量和函数；
- 风格 Profile 中作为模型输入的 `prompt` 字段及其校验/归一化入口；
- 画布镜头词库、时间线预设、参考素材引用语法和模型族 Prompt 规则。

### 1.2 不把它们误算成 Prompt 资产

- 只描述 HTTP、Provider wire format 或模型能力的协议文档；
- 只用于错误提示、审计日志、UI 标签和数据库字段名的字符串；
- 工具 schema 中的字段说明。它们会影响 Agent 可调用边界，但不是创作 Prompt；
- 生成任务中由用户传入、但仓库没有默认内容的自由文本。它们是运行时数据，应在任务快照和审计中追踪。

### 1.3 三种证据状态

| 状态 | 含义 |
|---|---|
| 已实现 | 当前代码直接读取、嵌入、同步或调用该资产。 |
| 已声明 | 文档或 Skill 描述了某个工具/流程，但本次没有证明真实入口已经实例化并可调用。 |
| 待运行验证 | 静态入口和测试存在，但没有执行真实 Agent、多轮模型或媒体生成验收。 |

## 2. 产品内置 Skill 的运行事实

### 2.1 物理位置和嵌入路径

| 事实 | 代码锚点 |
|---|---|
| Skill 包以 `backend/builtin/<目录>/SKILL.md` 为入口，目录内可带卡片和其他文本文件 | [`backend/builtin/`](../../backend/builtin/) |
| `embed.FS` 将整个 `backend/builtin` 嵌入后端二进制 | [`backend/builtin/embed.go`](../../backend/builtin/embed.go) 的 `FS` |
| 启动时读取 frontmatter、校验目录、计算内容包并 upsert 到技能目录和版本表 | [`backend/internal/skills/skills_builtin.go`](../../backend/internal/skills/skills_builtin.go) 的 `EnsureBuiltinSkills`、`loadBuiltinSkillPackagesFromFS` |
| 服务启动流程执行内置 Skill 同步，再执行 Skill 包补齐 | [`backend/cmd/server/main.go`](../../backend/cmd/server/main.go) 的 `EnsureBuiltinSkills`、`EnsureSkillPackages` |
| 技能市场、详情、文件读取和加入关系通过 HTTP handler 暴露 | [`backend/internal/handler/skills.go`](../../backend/internal/handler/skills.go) |
| 普通创作入口加载已加入、已选择的技能文本，并记录版本和文件 hash | [`web/src/services/skill-runtime.ts`](../../web/src/services/skill-runtime.ts) |

### 2.2 当前静态数量

本次扫描命令：

```bash
find backend/builtin -mindepth 2 -maxdepth 2 -name SKILL.md -type f | wc -l
find backend/builtin -type f ! -name SKILL.md | wc -l
find backend/builtin -mindepth 2 -maxdepth 2 -type d -name cards | wc -l
```

结果为：71 个入口 Skill、742 个非 `SKILL.md` 文件、33 个包含 `cards/` 目录的技能包。第二个数字包含 `embed.go` 和许可文件等非运行时正文文件；分类来自每个入口 frontmatter 的 `metadata.tag`，当前为：

| `metadata.tag` | 产品分类显示名 | 数量 |
|---|---|---:|
| `drama` | 短剧影视 | 25 |
| `ecommerce` | 电商营销 | 12 |
| `creative` | 创意设计 | 23 |
| `social` | 社媒内容 | 8 |
| `others` | 其他 | 3 |
| **合计** |  | **71** |

这与本机开发站点技能广场的 71 条公开目录快照相符；目录快照仍受数据库 seed、策展开关、启用状态和部署数据影响，不能把页面数量当作仓库文件数量的永久保证。

## 3. 71 个产品内置 Skill 根入口

下面的表只列根入口和可发现的名称。每一行的链接是后续阅读该技能完整指令、卡片目录、来源和边界的锚点；用途分类是导航摘要，具体以入口文件正文为准。

### 3.1 短剧影视（25）

| Skill | 根入口 |
|---|---|
| AI 表演实验室 | [`ai-表演实验室/SKILL.md`](../../backend/builtin/ai-表演实验室/SKILL.md) |
| AI演员微表情导演 | [`ai演员微表情导演/SKILL.md`](../../backend/builtin/ai演员微表情导演/SKILL.md) |
| 一图成片-电影广告全能导演 | [`一图成片-电影广告全能导演/SKILL.md`](../../backend/builtin/一图成片-电影广告全能导演/SKILL.md) |
| 一镜到底短剧流水线 | [`一镜到底短剧流水线/SKILL.md`](../../backend/builtin/一镜到底短剧流水线/SKILL.md) |
| 三幕式短剧工厂 | [`三幕式短剧工厂/SKILL.md`](../../backend/builtin/三幕式短剧工厂/SKILL.md) |
| 世界观美术设定 | [`世界观美术设定/SKILL.md`](../../backend/builtin/世界观美术设定/SKILL.md) |
| 分镜语法 | [`分镜语法/SKILL.md`](../../backend/builtin/分镜语法/SKILL.md) |
| 剧本工业化机器 | [`剧本工业化机器/SKILL.md`](../../backend/builtin/剧本工业化机器/SKILL.md) |
| 剧本撰写 | [`剧本撰写/SKILL.md`](../../backend/builtin/剧本撰写/SKILL.md) |
| 剧本资产视频一条龙创作 | [`剧本资产视频一条龙创作/SKILL.md`](../../backend/builtin/剧本资产视频一条龙创作/SKILL.md) |
| 剪辑节奏 | [`剪辑节奏/SKILL.md`](../../backend/builtin/剪辑节奏/SKILL.md) |
| 叙事短片导演分镜 | [`叙事短片导演分镜/SKILL.md`](../../backend/builtin/叙事短片导演分镜/SKILL.md) |
| 对白工坊 | [`对白工坊/SKILL.md`](../../backend/builtin/对白工坊/SKILL.md) |
| 情景喜剧工坊 | [`情景喜剧工坊/SKILL.md`](../../backend/builtin/情景喜剧工坊/SKILL.md) |
| 故事开发 | [`故事开发/SKILL.md`](../../backend/builtin/故事开发/SKILL.md) |
| 故事结构引擎 | [`故事结构引擎/SKILL.md`](../../backend/builtin/故事结构引擎/SKILL.md) |
| 相声手艺 | [`相声手艺/SKILL.md`](../../backend/builtin/相声手艺/SKILL.md) |
| 短剧实战手册 | [`短剧实战手册/SKILL.md`](../../backend/builtin/短剧实战手册/SKILL.md) |
| 短片五阶段结构 | [`短片五阶段结构/SKILL.md`](../../backend/builtin/短片五阶段结构/SKILL.md) |
| 系列弧光规划 | [`系列弧光规划/SKILL.md`](../../backend/builtin/系列弧光规划/SKILL.md) |
| 脱口秀手艺 | [`脱口秀手艺/SKILL.md`](../../backend/builtin/脱口秀手艺/SKILL.md) |
| 视听理论基础 | [`视听理论基础/SKILL.md`](../../backend/builtin/视听理论基础/SKILL.md) |
| 角色设计 | [`角色设计/SKILL.md`](../../backend/builtin/角色设计/SKILL.md) |
| 角色锻造工坊 | [`角色锻造工坊/SKILL.md`](../../backend/builtin/角色锻造工坊/SKILL.md) |
| 影策短剧制作工作流 | [`影策短剧制作工作流/SKILL.md`](../../backend/builtin/影策短剧制作工作流/SKILL.md) |

### 3.2 电商营销（12）

| Skill | 根入口 |
|---|---|
| AI造型师高转化模特图 | [`ai造型师高转化模特图/SKILL.md`](../../backend/builtin/ai造型师高转化模特图/SKILL.md) |
| TikTok网红带货视频 | [`tiktok网红带货视频/SKILL.md`](../../backend/builtin/tiktok网红带货视频/SKILL.md) |
| 发布片三件套 | [`发布片三件套/SKILL.md`](../../backend/builtin/发布片三件套/SKILL.md) |
| 定位与命名策略 | [`定位与命名策略/SKILL.md`](../../backend/builtin/定位与命名策略/SKILL.md) |
| 广告全链路打法 | [`广告全链路打法/SKILL.md`](../../backend/builtin/广告全链路打法/SKILL.md) |
| 广告文案四步系统 | [`广告文案四步系统/SKILL.md`](../../backend/builtin/广告文案四步系统/SKILL.md) |
| 消费心理转化设计 | [`消费心理转化设计/SKILL.md`](../../backend/builtin/消费心理转化设计/SKILL.md) |
| 珠宝电商图文视频一站式 | [`珠宝电商图文视频一站式/SKILL.md`](../../backend/builtin/珠宝电商图文视频一站式/SKILL.md) |
| 社媒素人App宣传 | [`社媒素人app宣传/SKILL.md`](../../backend/builtin/社媒素人app宣传/SKILL.md) |
| 穿搭上身试穿视频生成 | [`穿搭上身试穿视频生成/SKILL.md`](../../backend/builtin/穿搭上身试穿视频生成/SKILL.md) |
| 顶级波普视觉广告导演 | [`顶级波普视觉广告导演/SKILL.md`](../../backend/builtin/顶级波普视觉广告导演/SKILL.md) |
| 高能未来电子产品广告视频 | [`高能未来电子产品广告视频/SKILL.md`](../../backend/builtin/高能未来电子产品广告视频/SKILL.md) |

### 3.3 创意设计（23）

| Skill | 根入口 |
|---|---|
| 15秒极致夸张反转提示词 | [`一五秒极致夸张反转提示词/SKILL.md`](../../backend/builtin/一五秒极致夸张反转提示词/SKILL.md) |
| 15秒视觉特效 | [`一五秒视觉特效/SKILL.md`](../../backend/builtin/一五秒视觉特效/SKILL.md) |
| 3D国漫风转身变身特效 | [`三d国漫风转身变身特效/SKILL.md`](../../backend/builtin/三d国漫风转身变身特效/SKILL.md) |
| Freestyle 风格库 | [`freestyle-风格库/SKILL.md`](../../backend/builtin/freestyle-风格库/SKILL.md) |
| H3 视频提示词套件 | [`h三-视频提示词套件/SKILL.md`](../../backend/builtin/h三-视频提示词套件/SKILL.md) |
| Seedance 2.5 导演工艺 | [`seedance-二-五-导演工艺/SKILL.md`](../../backend/builtin/seedance-二-五-导演工艺/SKILL.md) |
| Seedance 提示词工程 | [`seedance-提示词工程/SKILL.md`](../../backend/builtin/seedance-提示词工程/SKILL.md) |
| 体验设计五原则 | [`体验设计五原则/SKILL.md`](../../backend/builtin/体验设计五原则/SKILL.md) |
| 决策思维模型格森 | [`决策思维模型格森/SKILL.md`](../../backend/builtin/决策思维模型格森/SKILL.md) |
| 名导十五秒视频风格资产引擎 | [`名导十五秒视频风格资产引擎/SKILL.md`](../../backend/builtin/名导十五秒视频风格资产引擎/SKILL.md) |
| 喜剧 Mechanics | [`喜剧-mechanics/SKILL.md`](../../backend/builtin/喜剧-mechanics/SKILL.md) |
| 图片图层拆分 | [`图片图层拆分/SKILL.md`](../../backend/builtin/图片图层拆分/SKILL.md) |
| 图片标注编辑 | [`图片标注编辑/SKILL.md`](../../backend/builtin/图片标注编辑/SKILL.md) |
| 图片编辑工作流 | [`图片编辑工作流/SKILL.md`](../../backend/builtin/图片编辑工作流/SKILL.md) |
| 提示词工程模式 | [`提示词工程模式/SKILL.md`](../../backend/builtin/提示词工程模式/SKILL.md) |
| 散文质地工艺 | [`散文质地工艺/SKILL.md`](../../backend/builtin/散文质地工艺/SKILL.md) |
| 模型路由交互 | [`模型路由交互/SKILL.md`](../../backend/builtin/模型路由交互/SKILL.md) |
| 珠宝设计出款 | [`珠宝设计出款/SKILL.md`](../../backend/builtin/珠宝设计出款/SKILL.md) |
| 珠宝设计进化 | [`珠宝设计进化/SKILL.md`](../../backend/builtin/珠宝设计进化/SKILL.md) |
| 系列套图生成 | [`系列套图生成/SKILL.md`](../../backend/builtin/系列套图生成/SKILL.md) |
| 视觉基调设计 | [`视觉基调设计/SKILL.md`](../../backend/builtin/视觉基调设计/SKILL.md) |
| 视觉提示词工程 | [`视觉提示词工程/SKILL.md`](../../backend/builtin/视觉提示词工程/SKILL.md) |
| 角色精灵图动画产线 | [`角色精灵图动画产线/SKILL.md`](../../backend/builtin/角色精灵图动画产线/SKILL.md) |

### 3.4 社媒内容（8）

| Skill | 根入口 |
|---|---|
| ASMR沉浸式潮玩开箱导演 | [`asmr沉浸式潮玩开箱导演/SKILL.md`](../../backend/builtin/asmr沉浸式潮玩开箱导演/SKILL.md) |
| 人像捏脸导演 | [`人像捏脸导演/SKILL.md`](../../backend/builtin/人像捏脸导演/SKILL.md) |
| 先锋服装商拍视觉指导 | [`先锋服装商拍视觉指导/SKILL.md`](../../backend/builtin/先锋服装商拍视觉指导/SKILL.md) |
| 小红书素人穿搭配方 | [`小红书素人穿搭配方/SKILL.md`](../../backend/builtin/小红书素人穿搭配方/SKILL.md) |
| 故事营销八阶段 | [`故事营销八阶段/SKILL.md`](../../backend/builtin/故事营销八阶段/SKILL.md) |
| 数字人搞笑采访视频生成 | [`数字人搞笑采访视频生成/SKILL.md`](../../backend/builtin/数字人搞笑采访视频生成/SKILL.md) |
| 治愈系原创IP孵化助手 | [`治愈系原创ip孵化助手/SKILL.md`](../../backend/builtin/治愈系原创ip孵化助手/SKILL.md) |
| 绿幕猫咪跳舞素材生成 | [`绿幕猫咪跳舞素材生成/SKILL.md`](../../backend/builtin/绿幕猫咪跳舞素材生成/SKILL.md) |

### 3.5 其他（3）

| Skill | 根入口 |
|---|---|
| OC魅力诊断师 | [`oc魅力诊断师/SKILL.md`](../../backend/builtin/oc魅力诊断师/SKILL.md) |
| 二维动画线稿批量上色 | [`二维动画线稿批量上色/SKILL.md`](../../backend/builtin/二维动画线稿批量上色/SKILL.md) |
| 选包门房 | [`选包门房/SKILL.md`](../../backend/builtin/选包门房/SKILL.md) |

## 4. 视频模型提示词与镜头语言资产

这组技能共同构成当前项目中最接近“镜头语言编写 Skill”的能力。它们不是一个统一的导演 Prompt，而是可以按任务组合的多个方法包。

### 4.1 可复用的核心技能

| 技能 | 可复用内容 | 适配边界 |
|---|---|---|
| [`Seedance 提示词工程`](../../backend/builtin/seedance-提示词工程/SKILL.md) | 八块结构：主体、场景、动作、运镜、分时段、转场/特效、音频、风格；摄影机词库；参考图职责；延长/编辑边界。 | 结构和镜头方法可迁移；Seedance 专属参数和 Provider 约束不能当成通用协议。 |
| [`Seedance 2.5 导演工艺`](../../backend/builtin/seedance-二-五-导演工艺/SKILL.md) | 资产锁定、连续性不变量、模式选择、时间线审计、动作与运镜的物理兼容。 | 可迁移的是连续性和时序原则；具体模式名、模型限制和调用方式需按当前入口核对。 |
| [`分镜语法`](../../backend/builtin/分镜语法/SKILL.md) | 景别、轴线、视点、镜头衔接、主客观视角和镜头选择。 | 适合主正文和分镜设计；不能把卡片内容当成工具授权。 |
| [`叙事短片导演分镜`](../../backend/builtin/叙事短片导演分镜/SKILL.md) | 镜头叙事目的、动作—反应、镜头组、节奏和可执行分镜。 | 其工具名和生产流程必须与当前 Agent 工具合同逐项核对。 |
| [`视觉提示词工程`](../../backend/builtin/视觉提示词工程/SKILL.md) | 视觉锚点、风格 DNA、可观察细节、去除空泛形容词和视觉反向拆解。 | 图像和视频的通用表达可借鉴；静态生图模板不能直接替代视频时序。 |
| [`一图成片-电影广告全能导演`](../../backend/builtin/一图成片-电影广告全能导演/SKILL.md) | 参考图视觉基因、视频类别、分镜表、镜头纯内容和分段生成边界。 | 工具链、批量生成和拼接流程只在真实入口支持时成立。 |

### 4.2 模型和题材专项资产

这些入口适合在需求明确时作为专项模块，而不是默认全部叠加：

- `H3 视频提示词套件`：MiniMax H3 六段式提示词、参考资产绑定和风格选择；
- `15秒视觉特效`：单镜头视觉锚点、动态矢量和高密度特效表达；
- `一镜到底短剧流水线`：一镜到底的成本、时长变体、参考一致性和失败排查；
- `三幕式短剧工厂`：从一句话剧情到分集和多段视频提示词的完整流水线；
- `15秒极致夸张反转提示词`：短时长反转节奏和能量释放；
- `3D国漫风转身变身特效`：角色底图、变身阵营和批量独立生成；
- `数字人搞笑采访视频生成`：问答短片、切镜频率、甩镜/突进和喜剧音效；
- `穿搭上身试穿视频生成`、`高能未来电子产品广告视频`、`ASMR沉浸式潮玩开箱导演`：面向明确商品/题材的定制 Prompt。

### 4.3 这组资产当前能证明什么

从源码可以确认这些 Skill 文件存在、能被嵌入和按需读取；不能仅凭 Skill 正文确认：

- Skill 中列出的每个工具名都在当前主执行链实例化；
- 具体模型会稳定执行所有镜头词和时间轴；
- “批量生成”“自动拼接”“先出表单再生成”等流程在所有入口都可用；
- 某个 Skill 的模型名、时长、分辨率或参数能适用于其他 Provider。

## 5. 仓库协作 Skill

这两个 Skill 与产品技能目录隔离，不能混写成用户可选技能：

| Skill | 用途 | 入口 |
|---|---|---|
| `project-docs` | 研究优先的项目文档体系、草稿落档、链接和验证纪律。 | [`.agents/skills/project-docs/SKILL.md`](../../.agents/skills/project-docs/SKILL.md) |
| `web-studio-user-manual` | 真实浏览器探索、截图、操作记录和 Web UI 用户手册编写。 | [`.agents/skills/web-studio-user-manual/SKILL.md`](../../.agents/skills/web-studio-user-manual/SKILL.md) |

这类 Skill 只在仓库协作者上下文中起作用。它们不会因为存在于 `.agents/skills/` 就被后端产品 Agent 自动发现；当前 Pi runtime 的 filesystem skill 自动发现也不是产品 Skill 加载路径。

## 6. 后端固定 Prompt、策略和模板资产

### 6.1 Agent 系统和媒体策略

| 资产 | 锚点 | 作用 | 生命周期 |
|---|---|---|---|
| Agent 系统策略 | [`backend/internal/prompts/agent-system-policy.md`](../../backend/internal/prompts/agent-system-policy.md)；`LoadAgentPolicies()` | 用户目标优先级、画布读取/写入纪律、工具安全、技能读取边界、错误表达和输出风格。 | 文件嵌入二进制；策略带 ID、版本和 hash。 |
| Agent 操作规则 | [`backend/internal/prompts/AGENTS.md`](../../backend/internal/prompts/AGENTS.md)；`LoadAgentPolicies()` | 与系统策略拼接，约束工具参数、响应真实性、审批和失败处理。 | 与系统策略一起编译和 hash。 |
| Agent 媒体策略 | [`backend/internal/prompts/agent-media-policy.md`](../../backend/internal/prompts/agent-media-policy.md) | 媒体生成、授权额度、审批、计费、任务状态和资源回写边界。 | 单独带 ID、版本和 hash。 |
| 策略快照 | [`backend/internal/app/cloud_agent_policy.go`](../../backend/internal/app/cloud_agent_policy.go)；`compileCloudAgentPolicies` | 将策略版本/hash、技能元数据、画布摘要和 profile 组合成一轮 Agent 的固定系统上下文。 | 随 Agent run 保存和恢复，不能被历史文本静默重解释。 |

这三份文本是**行为控制面**，不能与用户可编辑的创作 Prompt 混为一谈。用户 Skill 或画布内容不能覆盖权限、审批、资源归属和工具安全策略。

### 6.2 后端默认 Prompt 模板目录

入口：[`backend/internal/prompts/prompt_template_defaults.go`](../../backend/internal/prompts/prompt_template_defaults.go)、[`backend/internal/prompts/prompt_template.go`](../../backend/internal/prompts/prompt_template.go)。当前默认操作定义包括：

| Operation | 产出 | 主要锚点 |
|---|---|---|
| `chapter_assets_extract` | 章节角色、场景、道具资产 JSON | `defaultPromptDefinitions()` |
| `storyboard_plan` | 分镜规划 JSON | `DefaultStoryboardPromptTemplate()` |
| `storyboard_repair` | 分镜结构/复杂度修复 | `defaultPromptDefinitions()` |
| `storyboard_first_frame` | 分镜首帧图片 Prompt | `defaultPromptDefinitions()` |
| `storyboard_video` | 分镜视频执行 Prompt | `defaultPromptDefinitions()`，`StoryboardExecutionContract()` |
| `character_extract` | 角色卡 JSON | `defaultPromptDefinitions()` |
| `character_turnaround` | 角色三视图 Prompt | `defaultPromptDefinitions()` |
| `short_drama_outline` | 短剧标题、简介和章节 JSON | `defaultPromptDefinitions()` |
| `skill_draft` | 技能草稿 JSON | `defaultPromptDefinitions()` |

服务端启动会执行 `EnsureDefaultPromptTemplates()`：如果数据库中不存在对应操作的模板，就用默认内容创建；管理员可以维护模板，用户还可以按操作配置继承、追加或重写。也就是说，默认字符串是代码资产，但运行时最终内容可能来自数据库模板和用户定制。

### 6.3 风格 Profile 与全局风格片段

| 资产 | 锚点 | 作用与边界 |
|---|---|---|
| 风格 Profile JSON 规范与归一化 | [`backend/internal/prompts/style_profile.go`](../../backend/internal/prompts/style_profile.go) 的 `StyleProfileDocument`、`normalizeUserStyleProfileJSON` | 保存标题、标签、正向 `prompt`、`negativePrompt`、素材片段、模型/版本和执行策略；这是可持久化的业务数据结构，不等于固定系统 Prompt。 |
| 风格片段编译 | [`backend/internal/app/provider_style.go`](../../backend/internal/app/provider_style.go) 的 `resolveGenerationStyleExecution`、`reconcileGenerationStylePrompt` | 将 Profile 的正向、负向、触发词和资源片段拼接到生成请求；具体是否生效还取决于任务、Provider 和模型能力。 |

### 6.4 分镜默认 Prompt 的关键约束

[`backend/internal/prompts/storyboard_default.go`](../../backend/internal/prompts/storyboard_default.go) 中的 `DefaultStoryboardPromptTemplate()` 是当前项目最集中的镜头语言 Prompt 之一。它明确要求：

- 先确定观众跟随的主体，再选择景别、机位、构图和运镜；
- 一个镜头只保留一个主运镜，并写清起点、动机和停止点；
- 大远景用于空间/规模/处境建立，对白和反应优先使用中景、近景、过肩或反应镜头；
- 把不可见心理转成眼神、停顿、手部动作、走位、道具反应或环境变化；
- `description` 写可见画面和动作，`visualPrompt` 写构图/光线/材质，`videoPrompt` 写运动和结尾状态；
- 保持角色、服装、道具、空间关系、项目媒介和画风连续；
- 负面要求要针对换脸、服装变化、手部错误、乱码、闪烁、穿模、风格突变和动作僵硬等当前镜头风险。

它是可迁移的镜头语言参考，但它的 JSON 字段、`SchemaKey` 和分镜执行合同仍属于当前项目的结构化分镜协议，不能直接当成任意视频模型的通用 Prompt 格式。

## 7. 前端 Prompt 和镜头词库资产

### 7.1 直接影响创作输入的前端常量

| 文件/符号 | 资产内容 | 作用 |
|---|---|---|
| [`web/src/lib/prompts/catalog.ts`](../../web/src/lib/prompts/catalog.ts) / `CANVAS_BUILTIN_PRESETS` | 角色设定图、多机位视角、画面推演、连续镜头、电影光影优化、视频提示词优化 | 画布 Prompt 预设；按 text/image/video 模式筛选。 |
| 同文件 / `IMAGE_PROMPT_REVERSE` | 从参考图反推生图 Prompt 的固定指令 | 只输出 Prompt 正文，并覆盖主体、构图、风格、光线、色彩、材质和氛围。 |
| 同文件 / `workflowStarterPrompt` | 角色卡、三视图、分镜视频工作流起始 Prompt | 将上游产物传给下游创作步骤。 |
| 同文件 / `buildTextRewritePrompt` | 原文 + 修改要求的包装器 | 文本改写入口，不是独立创作 Skill。 |
| 同文件 / `buildVideoOperationPrompt` | 视频版本对比和编辑任务包装器 | 仅包装用户要求，具体执行仍受任务合同约束。 |
| 同文件 / `buildSubtitleHighlightSystemPrompt`、`buildSubtitleHighlightUserMessage` | 字幕关键词高亮的 JSON Prompt | 输出结构化高亮，不属于镜头创作 Prompt。 |
| [`web/src/lib/prompts/operations.ts`](../../web/src/lib/prompts/operations.ts) | `storyboard_plan`、`storyboard_video`、`skill_draft` 等操作 ID | Prompt 模板操作的稳定枚举，不是 Prompt 正文。 |

### 7.2 画布镜头和时间线预设

| 文件/符号 | 资产内容 | 运行边界 |
|---|---|---|
| [`web/src/lib/canvas/cinematic-video-tools.ts`](../../web/src/lib/canvas/cinematic-video-tools.ts) / `CAMERA_MOVE_PRESETS` | 推近、后拉、摇镜、跟拍、升降、环绕、手持、移焦、甩镜和定镜等可执行中文镜头片段 | 会追加到用户 Prompt；不能把预设名当成模型一定理解的标准术语。 |
| 同文件 / `TIME_SEQUENCE_PRESETS` | 6 秒、9 秒、10 秒等阶段化时间线片段 | 提供起始、发展、转折和结束锚点；实际时长仍由任务参数校验。 |
| [`web/src/lib/canvas/camera-prompt-library.ts`](../../web/src/lib/canvas/camera-prompt-library.ts) / `CAMERA_PROFILES`、`LENS_PROFILES`、`CAMERA_PROMPT_TEMPLATE` | 虚拟机身、镜头、焦段、光圈和摄影结果描述 | 面向相机控制面板；相机参数是“摄影方向”，不是往画面里添加摄影设备。 |
| [`web/src/lib/canvas/cinematic-video-tools.ts`](../../web/src/lib/canvas/cinematic-video-tools.ts) / `VideoRule` | 视频参考素材大小、时长和格式约束 | 是输入校验，不是生成 Prompt。 |

### 7.3 模型族 Prompt 规则

[`web/src/lib/plugins/builtin/prompt-optimizer.ts`](../../web/src/lib/plugins/builtin/prompt-optimizer.ts) 的 `resolveModelAdaptationProfile()` 和相关 Prompt 优化模块按模型族返回规则。当前可识别的方向包括：

- Seedance / 即梦视频：单镜头时序描述，明确起始构图、动作节拍、镜头运动、环境变化和结尾状态；
- Veo：电影化自然语言，按主体、动作、镜头、环境和节奏组织；
- Kling：简洁连续的主体运动和摄像机运动；
- 通用视频模型：起始画面、动作过程、摄影机和结束画面；
- GPT Image、Stable Diffusion、Midjourney、设计类图片模型：各自的主体/构图/风格/参数边界。

这些规则是**模型适配层**，不能反过来覆盖用户明确要求，也不能把某个 Provider 的私有参数写成平台通用协议。

### 7.4 其他前端 Prompt 资产

| 文件 | 资产用途 |
|---|---|
| [`web/src/lib/image-reference-prompt.ts`](../../web/src/lib/image-reference-prompt.ts) | 参考图职责和图像 Prompt 组装。 |
| [`web/src/lib/canvas/skill-drafting.ts`](../../web/src/lib/canvas/skill-drafting.ts) | 创建/编辑产品 Skill 时的草稿提示和字段约束。 |
| [`web/src/lib/plugins/builtin/prompt-optimizer.ts`](../../web/src/lib/plugins/builtin/prompt-optimizer.ts) | Prompt 优化插件的模型族规则、形状建议和禁止项。 |
| [`web/src/lib/canvas/previs/previs-prompt-compiler.ts`](../../web/src/lib/canvas/previs/previs-prompt-compiler.ts) | 将预演台的镜头、摄影机、环境、角色、空间调度、灯光和资产引用编译成可交给生成链的结构化 Prompt。 |
| [`web/src/lib/inspirations/catalog.ts`](../../web/src/lib/inspirations/catalog.ts) | 灵感卡片的示例 Prompt 元数据，不是系统级指令。 |
| [`web/public/inspirations/`](../../web/public/inspirations/) | 随前端发布的示例 Prompt 数据，主要用于灵感浏览和复用。 |

## 8. 动态 Prompt 组装和“不是资产”的边界

### 8.1 Agent 运行时组装

| 入口 | 事实 |
|---|---|
| [`backend/internal/app/cloud_agent_policy.go`](../../backend/internal/app/cloud_agent_policy.go) | 把固定策略、技能元数据、画布摘要、profile 和创作锚点组装成 Agent 系统上下文。 |
| [`backend/internal/app/cloud_agent_media_prompt.go`](../../backend/internal/app/cloud_agent_media_prompt.go) | 将 `@图片N`、`@视频N`、`@音频N` 校验并补齐为真实参考素材提示，不产生新的创作方法。 |
| [`backend/internal/app/cloud_agent_character_create.go`](../../backend/internal/app/cloud_agent_character_create.go) 的 `cloudAgentCharacterReferencePrompt` | 把角色卡字段压缩为角色参考 Prompt。 |
| [`backend/internal/app/cloud_agent_canvas_state.go`](../../backend/internal/app/cloud_agent_canvas_state.go) 的 `cloudAgentMediaPrompt`、`cloudAgentStoryboardState` | 将画布节点中的 Prompt、分镜字段投影给 Agent，受分页、字段白名单和文本上限限制。 |
| [`backend/internal/app/provider_video.go`](../../backend/internal/app/provider_video.go) 的 `seedancePromptText`、`seedanceVideosPromptText` | 将已经确定的用户 Prompt 送入 Provider 请求；这些函数本身不构造导演规则。 |

### 8.2 结构化字段中的 Prompt

分镜节点、批量表、媒体节点和角色卡中可以保存 `videoMotionPrompt`、`imageGenerationPrompt`、`prompt`、`negativePrompt`、`characterPrompt`、`consistencyPrompt` 等字段。它们是业务数据里的 Prompt 载体，不应直接视为“内置 Prompt 资产”。盘点它们时要同时记录：

- 字段属于哪个节点/实体和版本；
- 是用户输入、Skill 产出、模板编译结果还是模型回写；
- 生成任务创建时是否冻结了它；
- 后续编辑是否允许覆盖；
- 是否需要与参考资源、角色版本和任务身份一起校验。

## 9. 管理和修改指南

### 9.1 修改 Skill

修改 `backend/builtin/<skill>/SKILL.md` 或卡片时，应同时检查：

1. frontmatter 的 `name`、`description`、`metadata.skillId`、`metadata.tag` 和版本；
2. 入口正文中引用的卡片路径是否仍存在；
3. Skill 宣称的工具名是否在目标入口真实注册；
4. 是否把模型专属参数写成通用规则；
5. 是否需要同步测试、Skill 目录摘要和用户手册；
6. 服务启动后的 Skill 包 hash、版本和用户已加入关系是否符合预期。

### 9.2 修改后端 Prompt 模板

修改 `backend/internal/prompts/` 下默认模板或策略时，应同时检查：

- 操作 ID、占位符、输出类型和 JSON schema 是否匹配；
- `EnsureDefaultPromptTemplates()` 对已有数据库模板的行为；
- 管理员模板、用户继承/追加/重写和历史任务快照边界；
- Agent policy 的 ID、版本、hash 和恢复逻辑；
- 分镜 Prompt 与结构化字段校验、首帧/视频生成链路是否一致。

### 9.3 修改前端 Prompt 预设或镜头词库

修改 `web/src/lib/prompts/`、`web/src/lib/canvas/cinematic-video-tools.ts` 或 `camera-prompt-library.ts` 时，应检查：

- 预设实际适用的 generation mode；
- 是否覆盖用户原始 Prompt 或只是追加上下文；
- 摄影术语是否同时给出主体、起点、方向、速度和结束状态；
- 时间线是否连续、不重叠并与真实时长一致；
- 参考素材 token 是否与 `referenceNodeIds` 和媒体数组顺序一致；
- Provider 是否真的支持新增的参数或格式。

## 10. 导航关系

- [技能广场与 Skill 运行机制调研](skills-usage-and-runtime-research.md)：用户如何浏览、加入、选用 Skill，以及云端 Agent/普通创作入口如何加载。
- [云端 Agent 深度调研报告](cloud-agent-deep-research.md)：策略、技能快照、Pi bridge、工具、审批和恢复的完整调用链。
- [Agent 能力与技术实现全景调研](agent-capabilities-and-architecture-research.md)：Agent 能力矩阵和工具边界。
- [画布系统深度调研报告](canvas-deep-research.md)：画布节点中的 Prompt、技能引用、生成任务输入和状态同步边界。
- [创作者技能浏览手册](../user-manual/creator/10-tasks/browse-skills.md)：面向最终用户的技能广场操作，不替代本资产盘点。
- [管理员技能策展手册](../user-manual/admin/10-tasks/manage-plugins-and-skills.md)：平台分类和策展操作，不替代 Skill 内容/Prompt 盘点。

## 11. 验证记录与未完成范围

本次完成了仓库静态盘点和路径核对：

- 统计了 `backend/builtin` 的 71 个入口 Skill、742 个支撑文件和 33 个 `cards/` 包；
- 核对了五个 `metadata.tag` 分类及数量；
- 核对了 `embed.FS`、服务启动 seed、技能 API 和前端 Skill Runtime 的加载入口；
- 核对了后端 Agent 策略、默认 Prompt 操作目录、分镜默认模板和前端 Prompt/镜头词库入口；
- 补充了风格 Profile 编译、模型族 Prompt 优化器和预演台 Prompt 编译器的代码锚点；
- 将视频提示词和镜头语言相关的 Skill、模型专项 Skill 与通用控制面边界分开记录。

本次没有执行：

- 全部 71 个 Skill 的逐个真实 Agent 调用；
- 真实模型生成、视频生成、音频生成或成片质量盲评；
- 跨标签页/跨设备的 Skill 版本竞争和上下文预算压力测试；
- 对所有 `web/public/inspirations` 示例 Prompt 的逐条版权、质量或模型适配评审；
- 文档站构建或前端全量测试。

因此，这份文档可以作为“资产发现和变更影响分析”的事实入口，但不能把 Skill 自述的能力直接当作已完成的产品验收结论。
