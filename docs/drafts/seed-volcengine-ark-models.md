---
title: 用火山方舟（Ark）密钥 Seed 模型数据
status: draft
scope: guide
updated: 2026-10-10
---

# 用火山方舟（Ark）密钥 Seed 模型数据

> **目的**：给出「从外部 `api-server/.env` 导入环境变量 → 用 seed 脚本灌入 Seedance 2.0 视频 / 即梦 Seedream 5.0 Lite 图片 / OpenAI-Next Grok 文本模型」的可重放步骤，让开发者无需手工在 UI 里逐条填模型配置。
> **状态**：指南草稿；命令与协议已按本机源码核对，模型 ID 与 baseUrl 来自上游权威配置 `application.yml`（`ai-model-catalog`）。
> **前置阅读**：`scripts/dev-seed-models/README.md`；`AGENTS.md` 第 7 节（`CANVAS_BACKEND_DATA_DIR`=`.local/` 本地开发）。

## 关键边界（先说清）

- **密钥绝不进 YAML**：清单 `seed.ark.yaml` 里只用 `${VOLCENGINE_SEEDANCE_API_KEY}`、`${VOLCENGINE_SEEDREAM_API_KEY}`、`${OPENAI_NEXT_GROK_API_KEY}` 三个变量引用，值在运行时由 shell 环境注入。
- **绝不直接读取/打印原始 `.env`**：交接与排障一律看脱敏说明 `seedance-research/docs/ENV-REFERENCE-REDACTED.md`（只列变量名与用途，无等号右侧值）。
- **模型 ID 不是杜撰的**：一律来自上游 `api-server/src/main/resources/application.yml` 的 `ai-model-catalog`（见下「模型 ID 来源」），持久化在数据库的是 `upstream-model-id`，不是控制台短名。

## 环境变量来源

三个密钥对应三种上游 Provider（能力/协议映射已在 `seed.ark.yaml` 里写死）：

| 清单里的 `${VAR}` | 用途 | 对应 capability/protocol |
| --- | --- | --- |
| `VOLCENGINE_SEEDANCE_API_KEY` | Seedance 2.0 视频（`provider-name: official`） | `video` / `volcengine-ark-video` |
| `VOLCENGINE_SEEDREAM_API_KEY` | 即梦 Seedream 生图 Provider（**国内** seedream，`arcane` Bearer 单 key） | `image` / `volcengine-ark-image` |
| `OPENAI_NEXT_GROK_API_KEY` | OpenAI-Next Grok 中转（OpenAI Chat Completions） | `text` / `chat-completion` |

> 注意：即梦 5.0 Lite 用的是**国内** `VOLCENGINE_SEEDREAM_API_KEY`，不是 `MFHW_SEEDREAM_*`（那是海外 ModelArk，目录项默认 `enabled=false`）。环境变量属于另一个项目（seedance-research / api-server）的展开，与本 repo 协议层无对应的变量一律不引用（README 里「协议应与其上游真实协议一致」，未知含义者宁可不用）。

## 模型 ID 来源（authoritative）

`application.yml` 的 `ai-model-catalog.models` 里，三个模型的 `upstream-model-id` 即真实上游模型 ID：

| 渠道 | `upstream-model-id` | 目录项 | 说明 |
| --- | --- | --- | --- |
| Seedance 2.0 | `doubao-seedance-2-0-260128` | `video.seedance-2-standard` | `provider-name: official`，`default-for-kind: true` |
| 即梦 5.0 Lite | `doubao-seedream-5-0-lite-260128` | `image.seedream-5-lite` | `provider-name: seedream`（国内），`default-for-kind: true`，`image-model-variant: LITE` |
| Grok 文本 | `grok-4.5`（另有 `grok-4.6` `grok-4.7`） | `openai-next.default-model: grok-4.5` | 上游 `/models` 实际返回这三者 |

## 可重放步骤

前置：后端已启动（`./scripts/dev.sh` 或手动 `CANVAS_BACKEND_DATA_DIR=../.local/project-workbench-debug go run ./cmd/server`）。

```bash
# 0) 首次使用 YAML 清单时安装一次 yaml 依赖
cd scripts/dev-seed-models && bun install && cd ../..

# 1) 把外部 api-server/.env 的所有非注释行解析为 KEY=VALUE 并 export 到当前 shell
#    （把 <path> 换成你本机 .env 的实际位置；这一步只在本 shell 生效，不落盘、不写回 .env）
export $(cat <path>/api-server/.env | grep -v '^#' | xargs)

# 2) 用 seed 脚本灌入模型（本机配了 http_proxy 时加 NO_PROXY 前缀，避免 localhost 请求走代理 502）
NO_PROXY=127.0.0.1,localhost \
  bun scripts/dev-seed-models/seed.ts scripts/dev-seed-models/seed.ark.yaml
```

> `export $(cat .env | grep -v '^#' | xargs)` 的含义：`grep -v '^#'` 去掉注释行，`xargs` 把每行 `KEY=VALUE` 拼成一条命令的参数，`export` 一次性把它们写进当前 shell 环境变量。它只影响当前 shell 进程，`.env` 本身不被修改。

## 执行后发现与预期

- 脚本按「渠道名复用、模型按 `modelKey` 去重、逻辑模型 `code` 已存在则跳过」幂等运行，可反复执行不产生重复数据。
- 三个渠道建立后在设置页可见；文本（Grok）、生图（即梦 5.0 Lite）、生视频（Seedance 2.0）即可走通（计费免费）。
- **上游 `/models` 目录的差异属预期**：
  - Grok 通道（OpenAI 兼容）能拉到目录，本次实跑回显 `grok-4.5, grok-4.6, grok-4.7`，恰好证明 key 与 `/models` 端点可用。
  - Seedance / Seedream 两个 Ark 通道拉不到目录（回显「模型服务未提供 /models 接口」），因为 Ark 的 Contents Generations / images/generations 端点不暴露 OpenAI 兼容 `/models`。seed 对此**只诊断、不阻塞**，模型照常写入。
- seed 只写配置、不调用生成端点；真正的生图/生视频/文本调用在 web UI 里发起时才会消耗真实额度并证明密钥有效。

## 已知注意点

1. **鉴权**：`volcengine-ark-video` / `volcengine-ark-image` 都是 `Authorization: Bearer <API_KEY>` 单密钥，**不是** AK/SK。即梦官方协议（`volcengine-jimeng-*`）才需要 `secretKey`，本清单未用即梦协议。
2. **能力参数必须显式给足，否则模型在设置页不可见**：生图模型若 `image.size.parameter="none"`，backend 的 `channelModelDefaultOptions` 会无条件输出 `size` 默认值但能力规格里没有 `size` 选项，`sanitizeChannelModel` 报「默认参数 size 不在前台模型能力范围内」并把整个渠道模型从 `/model-catalog` 丢弃。视频同理——脚本内置模板曾是 `resolutions:["720p"]`、`ratios:["16:9"]`、`maxImages:4`、无参考/音频的精简桩。本清单已为即梦与 Seedance 写死能力：图 size 收敛到 `["1:1","4:3","3:4","16:9","9:16","21:9"]`（默认 1:1）；视频按 `application.yml` 的 `ark-seedance-2-standard` 给到 480p/720p/1080p/4K 与 5 档比例、参考图 9 / 视频 3 / 音频 3、`generateAudio` 与 `reference_to_video`/`audio_to_video` 全开。脚本 `defaultImageCapability`/`defaultVideoCapability` 也已同步修正，不再产出不可路由的最小能力。
3. **baseUrl**：Ark 视频/图片插件会把裸主机 `https://ark.cn-beijing.volces.com` 自动补成 `/api/v3/...` 路径；写成带 `/api/v3` 也能被去重逻辑正确识别（见 `provider_http_client.go` 的 `apiURLWithDefaultPrefix`）。
4. **Grok baseUrl 带 `/v1`**：`chat-completion` 协议按「裸主机补 `/v1`、已带 `/v1` 则不重复」规则拼接，最终都请求 `https://api.openai-next.com/v1/chat/completions`（与 `application.yml` 的 `base-url` + `completions-path` 一致，见 `provider_test.go` 的 URL 组装用例）。
5. 密钥只在导出环境变量那一步进入 shell，脚本日志不打印密钥、不写入 URL；`seed.ark.yaml` 只含 `${VAR}` 引用，可安全提交（`seed.local.*` 才被 `.gitignore` 忽略）。

## 与本地开发闭环的关系

本清单是 `docs/drafts/local-dev-quickstart.md`「完整工作循环」里 `seed.ts` 的一种具体交付形态：清空（`dev-reset.sh`）→ 启动（`dev.sh`）→ Seed（本清单）。日常「测脏了重来」时，重复第 2 步即可，无需再改清单。

## 不 seed，改在 Web UI 里手动新增 / 查阅模型

seed 只是把「系统渠道 → 渠道模型 → 前台逻辑模型/价格」这一串写进数据库的一种方式；同一套数据也能在 Web UI 手工维护，入口与 seed 落库一一对应：

| 层 | seed 落库 | Web UI 入口 |
| --- | --- | --- |
| 系统渠道（名称/Base URL/API Key/协议） | `channels` 列表 | 管理后台「系统渠道」`/admin/channels`：左侧「渠道」→「新增渠道」 |
| 渠道模型（capability/protocol/能力/价格） | 渠道下的 `models` | 选中渠道后右侧「模型管理」→「新增模型」或「拉取模型」 |
| 前台逻辑模型 + 路由 | `logicalModels` | 「前台模型」`/admin/models`（需 `frontendModelsEnabled` 开关） |

关键澄清（这几条正是常见的"找不到入口"原因）：

- **「模型选择」页不是新增入口**：`/settings?section=models` 只让你在每个领域（生图 / 视频 / 文本 / 音频）勾一个**默认模型**，不能新增。它页面那行说明就是"按领域选择默认模型；模型能力与请求协议在渠道『模型与能力』中配置"。真正新增的地方是上表里的「系统渠道」（管理员）和「个人渠道」（用户）。
- **"生图 / 视频 / 文本 / 音频" 是在「新增模型」里选的**：`web/src/pages/admin/components/channel-model-manager.tsx` 的模型表格有「能力（capability）」与「请求协议（protocol）」两列，「新增模型」表单里选 capability + protocol，能力参数（尺寸 / 分辨率 / 画幅 / 参考图等）与定价在同一模型编辑里配。
- **两套渠道不冲突**：seed 写的是 `scope=system` 的「系统渠道」（模型选择页里显示成「系统 X/次」），后台 `/admin/channels` 维护的也是它；普通用户还能在「个人渠道」`/settings?section=channels` 加自己的渠道（`scope≠system`，见 `web/src/pages/settings/index.tsx:57` 的 `userChannels` 过滤）。但「个人渠道」这一栏受 `customChannelsEnabled` 开关控制（`web/src/pages/settings/index.tsx:42`），默认可能不显示——管理员在 `/admin/settings/features`（「功能开放」）里打开它才会出现。
- **管理后台自身在哪进**：首页没有后台入口链接，管理员入口在工作台左下角头像菜单 / 「我的账户」卡片里的「管理员后台」，或直接访问 `/admin`（详见 `local-dev-quickstart.md`「零、一键启动脚本」下"管理后台（/admin）入口在哪"）。

## 验证记录（本机实测 2026-10-10）

按「清空 → 启动 → Seed（真实密钥）→ 复查」完整跑了一遍闭环，最终确认模型 ID、渠道与逻辑模型都正确落库。

### 1) 清空 `./scripts/dev-reset.sh --yes`

删除 `.local/project-workbench-debug/open_ai_canvas.db(+wal/shm)`（连同上一轮用占位/错误 ID seed 出的残留），保留 `.settings-key`。

### 2) 启动 `./scripts/dev.sh`

```
[dev] 端口 3000 已被占用，前端改用 3001
[dev] 启动后端 :8080 ...
[dev] 启动前端 :3001（/api 代理 -> 127.0.0.1:8080）...
[dev] 后端 就绪 ✅
[dev] 前端 就绪 ✅
```

### 3) Seed（真实密钥，`export $(cat .env ...)` + seed 一条命令）

```
=== 渠道：Seedance 2.0 视频（火山方舟） ===
  ✓ 渠道已创建（id=CHANNEL_000001）
  · 拉取上游目录失败（忽略）：模型服务未提供 /models 接口 (reason=bad_gateway)
  ✓ 渠道模型「doubao-seedance-2-0-260128」已定价启用（capability=video, protocol=volcengine-ark-video）
  ✓ 前台逻辑模型已创建并绑定路由
=== 渠道：即梦 Seedream 5.0 Lite 图片（火山方舟） === …(CHANNEL_000002, image / volcengine-ark-image)
=== 渠道：Grok 文本（OpenAI-Next） ===
  · 上游目录（3 个）：grok-4.5, grok-4.6, grok-4.7
  ✓ 渠道模型 grok-4.5 / grok-4.6 / grok-4.7 均定价启用（capability=text, protocol=chat-completion），各建前台逻辑模型
完成：3 个渠道成功，0 个失败。
```

> Ark 视频/图片通道的「拉取上游目录失败：模型服务未提供 /models 接口」是 Ark 端点不暴露 `/models` 的**预期**现象，与密钥是否有效无关；Grok 通道成功拉到目录才是密钥生效的直接证明。

### 4) 独立复查（`/api/admin/channels` + `/api/admin/logical-models`）

- 渠道 3 个：`CHANNEL_000001 Seedance 2.0 视频（火山方舟） → https://ark.cn-beijing.volces.com [doubao-seedance-2-0-260128]`、`CHANNEL_000002 即梦 Seedream 5.0 Lite 图片（火山方舟） → https://ark.cn-beijing.volces.com [doubao-seedream-5-0-lite-260128]`、`CHANNEL_000003 Grok 文本（OpenAI-Next） → https://api.openai-next.com/v1 [grok-4.5,grok-4.6,grok-4.7]`。
- 前台逻辑模型 5 个：`doubao-seedance-2-0-260128`(video)、`doubao-seedream-5-0-lite-260128`(image)、`grok-4.5`/`grok-4.6`/`grok-4.7`(text)。

### 5) 侧记

- 首次 seed 曾误用旧/杜撰 ID（`seedance-2.0-pro`、Seedream 的 MFHW endpoint ID、`grok-3`），上游 `/models` 目录回显已暴露其不在目录中；最终以 `application.yml` 的 `upstream-model-id` 为唯一依据全部改正确认。
- 本记录全程未打印 `.env` 正文；密钥只经 `export` 进入 shell 环境后交后端保存，管理 API 读回时 `apiKey` 字段已由后端脱敏（返回空串）。
- 真正首次生成（生图/生视频/文本）会消耗真实额度并证明密钥有效，需在 web UI 里由你发起确认。