# dev-seed-models —— 本地模型配置批量 seed 脚本

向本地后端批量注入渠道 + 渠道模型 + 前台逻辑模型，免去在 Web UI 里逐条手工填写。

> 只面向本地 / 可信开发环境。清单文件里含真实上游密钥和价格，**不要提交包含密钥的清单**（`seed.local.json` 已被 `.gitignore` 忽略，真实配置放那里即可）。

## 用法

```bash
# 先确保后端已启动（后端默认监听 :8080）
cd backend && CANVAS_BACKEND_DATA_DIR=../.local/project-workbench-debug go run ./cmd/server

# 另开一个终端，在仓库根目录执行
bun scripts/dev-seed-models/seed.ts          # 默认读 scripts/dev-seed-models/seed.local.json
bun scripts/dev-seed-models/seed.ts my-seed.json   # 指定清单

# 查看各 capability 下合法的 protocol 枚举
bun scripts/dev-seed-models/seed.ts --list-protocols
```

需要 bun；Node 22+ 亦可（脚本只用原生 `fetch`，无第三方依赖）。脚本通过后端 HTTP 管理 API（`/api/admin/*`）运行，走 Cookie 会话。

## 清单格式

参考 `seed.example.json`，把它复制成 `seed.local.json` 后再填真实值：

- `backendBaseUrl`：后端 API 根地址，默认 `http://localhost:8080/api`。
- `admin`：用于登录 / 注册的管理员账号。若该账号不存在，脚本会注册（**库中尚无用户时，首个注册用户自动成为管理员**）；若已存在则直接登录。
- `channels[]`：每个渠道对应一个上游接入：
  - `name`、`baseUrl`、`apiKey`（或 `secretKey`）、可选 `headers`。
  - `models[]`：该渠道下要启用的模型。

每个模型（`models[]` 元素）需显式给出：

| 字段 | 说明 |
| --- | --- |
| `modelKey` | 上游真实模型名（如 `gpt-4o`）。 |
| `capability` | `text` / `image` / `video` / `audio`。上游 `/models` 目录只返回模型名，无法推断，必须声明。 |
| `protocol` | 与上游实际协议一致的枚举，见下方表格。 |
| `pricing` | 计费方式与价格（microcredits，`1 积分 = 10000 microcredits`）。 |
| `displayName` | 可选，前台展示名，缺省用 `modelKey`。 |
| `code` | 可选，前台逻辑模型 code（2-80 位 `[a-z0-9][a-z0-9._-]*`），缺省由 `modelKey` 推导。 |
| `capabilityConfig` | 可选，覆盖内置能力模板（如上下文长度、图片最大张数、视频时长/比例/分辨率等）。 |

### 计费方式

- `fixed_request`（按次）/ `per_second`（按秒）：填 `unitPriceMicrocredits`（销售价）与 `costUnitPriceMicrocredits`（成本价）。
- `token`（按 Token）：填 `inputTokenPriceMicrocredits` / `outputTokenPriceMicrocredits` / `cachedTokenPriceMicrocredits` 及对应成本价。

成本价缺省时回落到对应销售价。

### 合法 protocol 枚举（按 capability）

| capability | 合法 protocol |
| --- | --- |
| `text` | `chat-completion`、`openai-response`、`claude-api` |
| `image` | `openai-image`、`grok-image`、`volcengine-ark-image`、`volcengine-ark-agent-plan-image`、`volcengine-jimeng-image`、`gemini-image`、`runninghub-workflow-image` |
| `audio` | `openai-audio`、`doubao-streaming-tts`、`async-audio`、`runninghub-workflow-audio` |
| `video` | `newapi`、`newapi-channel-1`、`newapi-channel-2`、`xai-video`、`volcengine-ark-video`、`volcengine-ark-agent-plan-video`、`volcengine-jimeng-video`、`gemini-veo`、`novita-video`、`minimax-video`、`agnes-video`、`runninghub-workflow-video` |

（也可用 `--list-protocols` 从脚本内置枚举直接打印最新值。）

## 执行了什么

脚本按渠道执行以下步骤：

1. `POST /admin/channels`：创建系统渠道，并一次性注入 `models` 模型名列表（不依赖上游 `/models` 端点即可建好渠道模型记录）。
2. `POST /admin/channels/:id/models/fetch`：拉取上游目录，仅作诊断——清单里写了但上游目录没有的模型会给出警告，不阻塞。
3. `GET /admin/channels/:id/models`：读取渠道模型 id。
4. 逐模型 `PATCH /admin/channels/:id/models/:mid`：写入能力契约 + 价格档 + 启用。
5. 逐模型 `POST /admin/logical-models`：创建前台逻辑模型，并把路由指向该渠道模型（`pricePolicy: "channel"`，价格跟随渠道）。

重复运行安全：渠道模型按 `modelKey` 去重；逻辑模型 `code` 冲突时会报错并跳过（可改 `code` 或先删掉旧逻辑模型）。

## 边界与注意事项

- **capability / protocol 无法从上游推断**，必须逐模型声明，且 protocol 必须与上游真实协议一致，否则运行时会失败。
- 脚本构造的 `capabilitySpec`（路由前端能力）由渠道模型能力一致投影，二者必须匹配，否则「前沿模型能力必须被路由覆盖」校验会拒绝。
- `capabilityConfig` 缺省时用内置文本 / 图片 / 视频能力模板（已验证能通过后端校验）；需要更大上下文、更多参考图、视频比例/分辨率等时用 `capabilityConfig` 覆盖。
- 渠道密钥只会发给后端，脚本日志不打印密钥、不写入 URL。