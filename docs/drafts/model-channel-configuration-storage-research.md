---
title: 模型与渠道配置的存储方式研究
status: draft
scope: research
updated: 2026-10-09
---

# 模型与渠道配置的存储方式研究

> **目的**：回答「生图 / 视频 / LLM 模型的配置放在哪里——数据库还是配置文件」，并说明配置如何分层、由谁维护、如何路由到真实上游。
> **状态**：研究草稿；不替代正式 HTTP API、数据库或模型渠道专题文档。
> **证据范围**：当前 Git checkout 的源码与 `.env.example`。本文结论基于静态阅读，尚未启动服务验证后台管理写路径。

## 一、结论摘要

模型配置**主要存放在数据库中**，而非配置文件。整体遵循「代码内置协议适配器 + 数据库配渠道/模型/价格/路由」的分层：

- **怎么调各家 API（协议）** = 写死在代码里，编译进二进制，不可通过配置修改。
- **用哪些模型、价格多少、如何路由、密钥和上游地址是什么** = 全部持久化在数据库，通过后台管理 API 维护。
- **`.env` 只保留运行时开关和基础设施级地址**，不含任何模型目录或模型清单。

## 二、代码内置的协议适配器

`backend/internal/protocol/builtin.go:74` 的 `Builtins()` 返回一个进程级不可变注册表，内建各家供应商的协议适配器：

```go
func Builtins() *Registry {
    registry, err := NewRegistry(
        openAIChatAdapter(), openAIResponsesAdapter(), claudeAdapter(),
        openAIVideosAdapter(), newAPIChannel1Adapter(), ..., xAIVideosAdapter(),
        arkVideosAdapter(), jimengVideosAdapter(), geminiVeoAdapter(), ...
    )
}
```

这些 adapter 定义的是「调用协议」：请求构造、响应解析、轮询、取消等逻辑。覆盖 OpenAI、Claude、XAI、火山 Ark、即梦、Gemini Veo、MiniMax、Novita 等。新增一个供应商协议属于代码改动，不能通过数据库或配置注入。

## 三、数据库中的模型配置分层

以下结构均通过 GORM 持久化，是「生图 / 视频 / LLM 模型配置」的核心真源。

### 1. 渠道 `ModelChannel`

`backend/internal/model/models_channel.go:9`。一条渠道对应一个上游接入：

- `BaseURL`：上游地址。
- `APIKey` / `SecretKey`：均为 `json:"-"`，序列化时不外泄。
- `APIFormat`、`ConcurrencyLimit`、`Enabled`、`Scope`、`SortOrder` 等。

### 2. 渠道模型 `ChannelModel`

`backend/internal/model/models_channel.go:37`。一个渠道下的具体上游 SKU，关键字段：

- `Capability`（文本 / 图片 / 视频 / 音频）、`Protocol`（对应当前 adapter）。
- `ProviderModelKey`：上游真实模型名（如 `gpt-4o`）。
- `ModelKey`：在渠道内唯一（活动记录 + `channelID` 联合唯一）。
- 标量价格字段（`UnitPriceMicrocredits` 等）仅为旧调用兼容。

### 3. 渠道模型价格档 `ChannelModelPriceTier`

`backend/internal/model/models_channel.go:74`。系统渠道模型的**价格真相**：

- `SelectorJSON` 是规格选择器，例如 `{"operation":"image_to_video","vquality":"720p","videoSeconds":"10"}` 或 `{"operation":"text_to_image","quality":"2k"}`。
- `SelectorKey` 是规范化 JSON，用于 PostgreSQL 的活动规格唯一约束。
- 每个档持有独立的 `ProviderModelKey`、计费模式和价格；新版创作端报价必须按所选规格命中一个价格档。

### 4. 逻辑模型 `LogicalModel` + `Revision` + `Route`

`backend/internal/model/models_logical_model.go`。面向**前台**的可发布目录项：

- `LogicalModel`（`:13`）：前台模型主体，含 `Capability`、`PricePolicy`（如 `unified`）、价格与 `SourceChannelModelID`（标识由哪个系统渠道模型投影而来）。
- `LogicalModelRevision`（`:44`）：固化的「能力合同」，`CapabilitySpecJSON` / `DefaultOptionsJSON` 定义前台默认能力；默认值属于前台模型，不由供应商隐式决定。
- `LogicalModelRoute`（`:54`）：把前台模型路由到多个 `ChannelModelID`，带 `Priority` / `Weight`，支持多上游容灾与加权。
- `RouteAttempt`（`:65`）：记录每次任务实际经过的路由尝试，任务、账单仍固定引用其创建时的 ID 快照。

### 5. 前台价格 SKU `LogicalModelPriceSKU`

`backend/internal/model/models_logical_model.go:87`。统一定价模式下（`pricePolicy = unified`）的独立售价 SKU，由 `SelectorJSON` 表达复杂能力组合定价。

## 四、`.env` 中的模型相关配置

`.env.example` 中与模型直接相关的只有运行时开关和基础设施地址，**没有模型清单**：

```
ENABLE_PROVIDER_PLUGINS=false     # 是否启用插件化供应商
CANVAS_CHANNEL_CONCURRENCY=3      # 渠道并发上限
CANVAS_WHISPER_BASE_URL=          # 语音转写地址（基础设施）
```

具体模型、价格、路由与密钥均通过数据库和后台管理 API（含自定义渠道 `/api/ai/custom` 中转）维护。

## 五、本地开发快速 seed

本地启动 dev 环境后，模型配置没有一键 seed 命令，只有两条路：打开 Web UI 一条条手填，或脚本化调用后台管理 API。针对后者，仓库提供了 `scripts/dev-seed-models/`：

- `seed.ts`（bun/Node 原生 fetch，零依赖）：登录/注册管理员 → 创建系统渠道（注入模型名列表）→ 拉上游目录（诊断）→ 逐模型定价启用 → 创建前台逻辑模型 + 路由。
- `seed.example.json`：清单模板（无真实密钥）；真实密钥和价格放 `seed.local.json`（已被 `.gitignore` 忽略）。

脚本走的管理 API 与 Web UI 一致，`capability` / `protocol` 需逐模型显式声明（上游 `/models` 只返回模型名），价格以 microcredits 计。详见 `scripts/dev-seed-models/README.md`。

## 六、能力清单与数据流

- 初始化 / 同步入口：`backend/internal/app/channel_models.go:71` 的 `EnsureSystemChannelModels()`，以及 `ensureChannelModels`（`:424`）。
- 前端调用面：`web/src/services/api/`（业务 API 统一经 `request.ts` 的 `http`）；自定义渠道解析走 `custom-channel-relay.ts`，实际发出走 `channel-transport.ts`。
- 后台管理 handler（仅文件名，未逐文件阅读内容）：`backend/internal/handler/admin_channel_routes.go`、`admin_channel_model_response.go`、`channel_models.go`、`model_catalog.go`、`logical_models.go`。

## 七、待验证边界

- 后台管理对渠道 / 逻辑模型的完整写路径（创建、价格档命中、路由编排）尚未通过运行端到端验证。
- `.env` 之外是否还有编译期注入的供应商发现插件（`backend/internal/provider/registry.go` 的 `Register` / `discovery`）与内置渠道种子，尚未逐条核对。
- `EnsureSystemChannelModels` 与协议内置目录（`protocol.Builtins()`）之间的投影关系，仅基于文件名与注释推断，未跑通创建链路。