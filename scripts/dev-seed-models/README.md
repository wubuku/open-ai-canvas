# dev-seed-models —— 本地模型配置批量 seed 脚本

向本地后端批量注入渠道 + 渠道模型 + 前台逻辑模型，免去在 Web UI 里逐条手工填写。

> 只面向本地 / 可信开发环境。清单文件可含真实上游地址和价格，但**密钥应通过环境变量注入，不要写进清单**（`seed.local.json` / `seed.local.yaml` 已被 `.gitignore` 忽略）。

## 快速上手（本地 dev 最快的路径）

本地 dev 的「计费」无关紧要，最短只要三件事：**注入密钥 → 写最小清单 → 跑一次**。

```bash
# 1. 先确保后端已启动（后端默认监听 :8080）
cd backend && CANVAS_BACKEND_DATA_DIR=../.local/project-workbench-debug go run ./cmd/server

# 2. 密钥放环境变量，写一个最小清单（可复制 seed.quick.yaml 改模型名）
export OPENAI_API_KEY=sk-...

# 3. 一键 seed（YAML 首次需要装个 yaml 依赖）
cd scripts/dev-seed-models && bun install && cd ../..
bun scripts/dev-seed-models/seed.ts seed.quick.yaml
```

最小清单长这样（`seed.quick.yaml` 的节选）：

```yaml
admin:
  username: admin
  password: ${ADMIN_PASSWORD:-admin123456}   # 第一个注册用户自动成为管理员

channels:
  - name: OpenAI 兼容
    baseUrl: https://api.openai.com/v1
    apiKey: ${OPENAI_API_KEY}               # ← 密钥只来自环境变量，不写进文件
    models:
      - { modelKey: gpt-4o,       capability: text,  protocol: chat-completion }
      - { modelKey: gpt-image-1,  capability: image, protocol: openai-image }
      - { modelKey: sora-2,       capability: video, protocol: newapi }
```

注意 `models[]` 里**每个模型都没写 `pricing`** —— 脚本会按「按次、免费」自动补齐，本地测试跑生图 / 生视频 / 剧本解析完全够用，不会中途停下来要你补配置。

## 用法

```bash
bun scripts/dev-seed-models/seed.ts                        # 默认读 seed.local.json / seed.local.yaml
bun scripts/dev-seed-models/seed.ts my-seed.json           # 指定 JSON 清单
bun scripts/dev-seed-models/seed.ts my-seed.yaml           # 指定 YAML 清单
bun scripts/dev-seed-models/seed.ts --list-protocols       # 查看各 capability 下合法 protocol
```

需要 bun；脚本通过后端 HTTP 管理 API（`/api/admin/*`）运行，走 Cookie 会话。

**依赖**：JSON 清单零依赖；YAML 清单需要 `yaml` 依赖（只在第一次用 YAML 时安装一次）：

```bash
cd scripts/dev-seed-models && bun install
```

**坑：本机 HTTP 代理**。若你本机 shell 配了 `http_proxy` / `https_proxy` 且未排除回环地址，bun 的 `fetch` 也会像 `curl` 一样把 localhost 请求走向代理、返回 **502**，看起来像「后端没起」。脚本检测到该情况会在启动时提示；解决方式是在命令前加 `NO_PROXY` 前缀：

```bash
NO_PROXY=127.0.0.1,localhost bun scripts/dev-seed-models/seed.ts my-seed.yaml
```

## 清单格式（JSON 与 YAML 等价）

参考 `seed.example.json`（JSON、完整字段）、`seed.quick.yaml`（YAML、最小字段）和 `seed.ark.yaml`（火山方舟 Seedance/Seedream/Doubao，密钥引用外部 `.env` 的 `${VAR}`）。字段如下：

- `backendBaseUrl`：后端 API 根地址，默认 `http://localhost:8080/api`。
- `admin`：用于登录 / 注册的管理员账号。若账不存在，脚本会注册（**库中尚无用户时，首个注册用户自动成为管理员**）；已存在则直接登录。
- `channels[]`：每个渠道对应一个上游接入：
  - `name`、`baseUrl`、`apiKey`（或 `secretKey`）、可选 `headers`。
  - `models[]`：该渠道下要启用的模型。

每个模型（`models[]` 元素）需显式给出：

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| `modelKey` | ✅ | 上游真实模型名（如 `gpt-4o`）。 |
| `capability` | ✅ | `text` / `image` / `video` / `audio`。上游 `/models` 目录只返回模型名，无法推断，必须声明。 |
| `protocol` | ✅ | 与上游实际协议一致的枚举，见下方表格。 |
| `pricing` | ❌ | 计费方式与价格。可省略，省略时按「按次、免费」补齐。 |
| `displayName` | ❌ | 前台展示名，缺省用 `modelKey`。 |
| `code` | ❌ | 前台逻辑模型 code（2-80 位 `[a-z0-9][a-z0-9._-]*`），缺省由 `modelKey` 推导。 |
| `capabilityConfig` | ❌ | 覆盖内置能力模板（如上下文长度、图片最大张数、视频时长/比例/分辨率等）。 |

### 环境变量引用（密钥不进清单）

清单里任意字符串值都可用 `${VAR}` 引用环境变量，支持缺省值 `${VAR:-默认值}`：

```yaml
admin:
  password: ${ADMIN_PASSWORD:-admin123456}
channels:
  - baseUrl: ${MY_UPSTREAM_URL}
    apiKey: ${OPENAI_API_KEY}
    secretKey: ${JIMENG_SECRET_KEY}
    headers: { "X-Custom": ${CUSTOM_HEADER} }
```

变量未设置且无缺省值时，脚本会在读取阶段直接报错并提示缺哪个变量，不会写进半成品数据。

### 计费方式（本地 dev 可忽略）

- `fixed_request`（按次）/ `per_second`（按秒）：填 `unitPriceMicrocredits`（销售价）与 `costUnitPriceMicrocredits`（成本价）。
- `token`（按 Token）：填 `inputTokenPriceMicrocredits` / `outputTokenPriceMicrocredits` / `cachedTokenPriceMicrocredits` 及对应成本价。

价格单位 microcredits（`1 积分 = 10000 microcredits`）；成本价缺省时回落到对应销售价。价格可为 `0`（本地测试免费）。`per_second` 仅限视频/音频，`token` 仅限文本/视频。

### 合法 protocol 枚举（按 capability）

| capability | 合法 protocol |
| --- | --- |
| `text` | `chat-completion`、`openai-response`、`claude-api` |
| `image` | `openai-image`、`grok-image`、`volcengine-ark-image`、`volcengine-ark-agent-plan-image`、`volcengine-jimeng-image`、`gemini-image`、`runninghub-workflow-image` |
| `audio` | `openai-audio`、`doubao-streaming-tts`、`async-audio`、`runninghub-workflow-audio` |
| `video` | `newapi`、`newapi-channel-1`、`newapi-channel-2`、`xai-video`、`volcengine-ark-video`、`volcengine-ark-agent-plan-video`、`volcengine-jimeng-video`、`gemini-veo`、`novita-video`、`minimax-video`、`agnes-video`、`runninghub-workflow-video` |

（也可用 `--list-protocols` 从脚本内置枚举直接打印最新值。）

## 执行了什么

脚本按渠道执行以下步骤（一次跑完，不中途打断）：

1. 按名字查找已有渠道：命中则 `PATCH /admin/channels/:id` 更新，未命中才 `POST /admin/channels` 新建；并一次性注入 `models` 模型名列表（不依赖上游 `/models` 端点即可建好渠道模型记录）。
2. `POST /admin/channels/:id/models/fetch`：拉取上游目录，仅作诊断——清单里写了但上游目录没有的模型会给出警告，不阻塞。
3. `GET /admin/channels/:id/models`：读取渠道模型 id。
4. 逐模型 `PATCH /admin/channels/:id/models/:mid`：写入能力契约 + 价格档 + 启用。
5. 逐模型 `POST /admin/logical-models`：创建前台逻辑模型，并把路由指向该渠道模型（`pricePolicy: "channel"`，价格跟随渠道）；code 已存在时跳过并提示。

重复运行安全：脚本幂等——渠道按名字复用（更新而非新建）；渠道模型按 `modelKey` 去重；前台逻辑模型 `code` 已存在时跳过并提示，可放心反复运行。

## 边界与注意事项

- **capability / protocol 无法从上游推断**，必须逐模型声明，且 protocol 必须与上游真实协议一致，否则运行时会失败（脚本会核对其是否属于该 capability 的合法枚举）。
- 脚本构造的 `capabilitySpec`（路由前端能力）由渠道模型能力一致投影，二者必须匹配，否则「前沿模型能力必须被路由覆盖」校验会拒绝。
- `capabilityConfig` 缺省时用内置文本 / 图片 / 视频能力模板（已验证能通过后端校验）；需要更大上下文、更多参考图、视频比例/分辨率等时用 `capabilityConfig` 覆盖。
- 渠道密钥只会发给后端，脚本日志不打印密钥、不写入 URL。
- 即梦官方协议（`volcengine-jimeng-image` / `volcengine-jimeng-video`）需要渠道同时配置 `apiKey`（Access Key）和 `secretKey`（Secret Key）。