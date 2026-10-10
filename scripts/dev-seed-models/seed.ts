#!/usr/bin/env bun
/**
 * dev-seed-models —— 本地开发环境模型配置批量 seed 脚本。
 *
 * 只通过后端管理 API（走 HTTP + Cookie 会话）完成整条链路：
 *   登录/注册管理员 → 创建系统渠道 → 拉取上游目录(诊断) → 逐模型定价启用 → 创建前台逻辑模型(+路由)。
 *
 * 运行（需要 bun，Node 22+ 亦可）：
 *   bun scripts/dev-seed-models/seed.ts [seed.json]
 * 未指定文件时读取 scripts/dev-seed-models/seed.local.json。
 *
 * 重要边界：
 *  - 每个模型的 capability(text/image/video/audio) 和 protocol 必须显式声明，
 *    上游 /models 目录只返回模型名，无法推断能力或协议。
 *  - 渠道密钥(apiKey/secretKey)与价格全部来自清单文件，请勿提交含真实密钥的清单。
 *  - 重复运行是安全的：渠道模型按 modelKey 去重导入，逻辑模型 code 冲突时会报错并跳过。
 */

// ---------------------------------------------------------------------------
// 类型定义（与 backend/internal 的 JSON 契约对齐）
// ---------------------------------------------------------------------------

/** 计费方式：按次 / 按 Token / 按秒。 */
type BillingMode = "fixed_request" | "token" | "per_second";

/** 合法协议枚举（与 backend/internal/model/models.go 的 ChannelInterfaceType 对齐）。 */
const PROTOCOLS_BY_CAPABILITY: Record<string, string[]> = {
  text: ["chat-completion", "openai-response", "claude-api"],
  image: [
    "openai-image",
    "grok-image",
    "volcengine-ark-image",
    "volcengine-ark-agent-plan-image",
    "volcengine-jimeng-image",
    "gemini-image",
    "runninghub-workflow-image",
  ],
  audio: ["openai-audio", "doubao-streaming-tts", "async-audio", "runninghub-workflow-audio"],
  video: [
    "newapi",
    "newapi-channel-1",
    "newapi-channel-2",
    "xai-video",
    "volcengine-ark-video",
    "volcengine-ark-agent-plan-video",
    "volcengine-jimeng-video",
    "gemini-veo",
    "novita-video",
    "minimax-video",
    "agnes-video",
    "runninghub-workflow-video",
  ],
};

/** 单个模型的价格与成本（单位：microcredits，1 积分 = 10000 microcredits）。 */
interface ModelPricing {
  billingMode: BillingMode;
  /** fixed_request / per_second：销售单价。 */
  unitPriceMicrocredits?: number;
  inputTokenPriceMicrocredits?: number;
  outputTokenPriceMicrocredits?: number;
  cachedTokenPriceMicrocredits?: number;
  /** 成本价，缺省时回落到对应销售价。 */
  costUnitPriceMicrocredits?: number;
  costInputTokenPriceMicrocredits?: number;
  costOutputTokenPriceMicrocredits?: number;
  costCachedTokenPriceMicrocredits?: number;
}

/** 单个模型在清单中的声明。 */
interface ModelSpec {
  /** 上游真实模型名，如 "gpt-4o"、"gemini-2.0-flash"。 */
  modelKey: string;
  capability: "text" | "image" | "video" | "audio";
  /** 见 PROTOCOLS_BY_CAPABILITY；必须与上游实际协议一致。 */
  protocol: string;
  /** 前台展示名，缺省用 modelKey。 */
  displayName?: string;
  /** 前台逻辑模型 code（2-80 位 [a-z0-9][a-z0-9._-]*），缺省由 modelKey 推导。 */
  code?: string;
  /** 前台逻辑模型名称，缺省用 displayName / modelKey。 */
  logicalName?: string;
  pricing: ModelPricing;
  /** 覆盖内置能力模板；key 为 text/image/video，结构见 backend internal/app/model_capability.go。 */
  capabilityConfig?: Record<string, unknown>;
}

interface ChannelSpec {
  name: string;
  baseUrl: string;
  apiKey?: string;
  secretKey?: string;
  headers?: Record<string, string>;
  models: ModelSpec[];
}

interface SeedConfig {
  backendBaseUrl?: string;
  admin: { username: string; password: string; displayName?: string };
  channels: ChannelSpec[];
}

// ---------------------------------------------------------------------------
// 内置能力模板（默认值均能通过 backend 的 validate*CapabilityConfig 校验）
// ---------------------------------------------------------------------------

function defaultTextCapability() {
  return {
    version: 1,
    text: {
      streaming: true,
      contextWindowTokens: 128000,
      maxOutputTokens: 16384,
      references: {
        promptMaxChars: 100000,
        maxImages: 8,
        maxImageBytes: 10485760,
        maxVideos: 0,
        maxVideoBytes: 0,
      },
    },
  };
}

function defaultImageCapability() {
  return {
    version: 1,
    image: {
      references: {
        promptMaxChars: 100000,
        maxImages: 4,
        maxImageBytes: 10485760,
        maskSupported: false,
      },
      size: { parameter: "none" },
      quality: { supported: false },
      transparentBackground: { supported: false },
      responseFormat: { supported: false },
      outputFormat: { supported: false },
      maxOutputs: 1,
    },
  };
}

function defaultVideoCapability() {
  return {
    version: 1,
    video: {
      references: {
        promptMaxChars: 8000,
        minImages: 0,
        maxImages: 4,
        maxImageBytes: 10485760,
        maxVideos: 0,
        maxVideoBytes: 0,
        maxVideoDuration: 15,
        maxAudios: 0,
        maxAudioBytes: 0,
        maxAudioDuration: 15,
      },
      duration: { selection: "range", min: 1, max: 10, step: 1, default: 5 },
      ratios: ["16:9"],
      defaultRatio: "16:9",
      resolutions: ["720p"],
      defaultResolution: "720p",
      generateAudio: { supported: false, default: false },
      watermark: { supported: false, default: false },
      operations: ["text_to_video"],
      defaultOperation: "text_to_video",
    },
  };
}

function buildCapabilityConfig(spec: ModelSpec): Record<string, unknown> {
  if (spec.capabilityConfig) {
    const base = capabilityTemplateFor(spec.capability);
    return mergeDeep(base, spec.capabilityConfig);
  }
  return capabilityTemplateFor(spec.capability);
}

function capabilityTemplateFor(capability: string): Record<string, unknown> {
  switch (capability) {
    case "text":
      return defaultTextCapability();
    case "image":
      return defaultImageCapability();
    case "video":
      return defaultVideoCapability();
    default:
      // audio 模型没有可编辑的能力 JSON。
      return { version: 1 };
  }
}

// ---------------------------------------------------------------------------
// CapabilityConfig -> CapabilitySpec 投影
// 复刻 backend internal/app/model_capability.go 的 CapabilitySpecFromModelCapabilityConfig。
// 这是「前沿模型能力必须被路由覆盖」校验所需，脚本必须与渠道模型能力保持一致。
// ---------------------------------------------------------------------------

interface InputConstraint {
  min: number;
  max: number;
}
interface OptionConstraint {
  values?: unknown[];
  min?: number;
  max?: number;
  step?: number;
}

function anyValues(values: unknown[]): OptionConstraint {
  return { values };
}
function boolValues(supportsTrue: boolean): OptionConstraint {
  const values: unknown[] = [false];
  if (supportsTrue) values.push(true);
  return { values };
}
function numericRange(minimum: number, maximum: number, step: number): OptionConstraint {
  return { min: minimum, max: maximum, step };
}
function addInput(inputs: Record<string, InputConstraint>, name: string, min: number, max: number) {
  if (min <= 0 && max <= 0) return;
  inputs[name] = { min, max };
}

function capabilitySpecFromConfig(capability: string, config: any): any {
  const spec: any = { version: 1, capability, inputs: {}, options: {} };
  if (capability === "audio") return spec;

  if (capability === "text") {
    const t = config.text;
    addInput(spec.inputs, "image", 0, t.references.maxImages);
    addInput(spec.inputs, "video", 0, t.references.maxVideos);
  } else if (capability === "image") {
    const img = config.image;
    addInput(spec.inputs, "image", 0, img.references.maxImages);
    if (img.references.maskSupported) addInput(spec.inputs, "mask", 0, 1);
    if (img.size.parameter !== "none") {
      spec.options.size = imageSizeOptionConstraint(img.size);
      spec.imageSize = imageSizeFromConfig(img.size);
    }
    if (img.quality.supported) spec.options.quality = anyValues(img.quality.values);
    spec.options.transparentBackground = boolValues(img.transparentBackground.supported);
    spec.options.count = numericRange(1, img.maxOutputs, 1);
  } else if (capability === "video") {
    const v = config.video;
    spec.operations = [...v.operations];
    addInput(spec.inputs, "image", v.references.minImages, v.references.maxImages);
    addInput(spec.inputs, "video", 0, v.references.maxVideos);
    addInput(spec.inputs, "audio", 0, v.references.maxAudios);
    if (v.duration.selection === "enum") {
      spec.options.videoSeconds = { values: v.duration.values };
    } else {
      spec.options.videoSeconds = numericRange(v.duration.min, v.duration.max, v.duration.step);
    }
    if (v.ratios.length) spec.options.size = anyValues(v.ratios);
    if (v.resolutions.length) spec.options.vquality = anyValues(v.resolutions);
    spec.options.videoGenerateAudio = boolValues(v.generateAudio.supported);
    spec.options.videoWatermark = boolValues(v.watermark.supported);
  }
  return spec;
}

// 图片 size 投影（仅当 image.size.parameter != "none" 时触发；默认模板用 "none" 跳过）。
function imageSizeOptionConstraint(size: any): OptionConstraint {
  const values: unknown[] = [];
  const seen = new Set<string>();
  for (const value of size.values ?? []) {
    const v = String(value).trim();
    if (!v || v === "*" || seen.has(v)) continue;
    seen.add(v);
    values.push(v);
  }
  if (size.allowCustom) {
    if (values.length === 0) {
      for (const legacy of ["1:1", "3:2", "2:3", "4:3", "3:4", "16:9", "21:9", "9:16", "1024x1024", "1536x1024", "1024x1536"]) {
        values.push(legacy);
      }
    }
    values.push("*");
  }
  return { values };
}

function imageSizeFromConfig(size: any): any {
  if (!size.parameter || size.parameter === "none") return undefined;
  const result: any = { parameter: size.parameter, allowCustom: size.allowCustom };
  result.presets = (size.presets ?? [])
    .filter((p: any) => p.tier && p.ratio && p.size)
    .map((p: any) => ({ size: p.size, tier: p.tier, ratio: p.ratio, width: p.width, height: p.height }));
  return result;
}

// ---------------------------------------------------------------------------
// 价格档构造（CostPricing + 销售价），复刻 channel_models_pricing.go 的最低要求
// ---------------------------------------------------------------------------

function buildPriceTier(spec: ModelSpec) {
  const p = spec.pricing;
  const billingMode = p.billingMode ?? "fixed_request";

  const tier: any = {
    selector: {}, // 空 selector = 任意规格兜底档
    billingMode,
    priceConfigured: true,
    enabled: true,
    costPricing: { configured: true },
  };

  if (billingMode === "token") {
    tier.inputTokenPriceMicrocredits = p.inputTokenPriceMicrocredits ?? 0;
    tier.outputTokenPriceMicrocredits = p.outputTokenPriceMicrocredits ?? 0;
    tier.cachedTokenPriceMicrocredits = p.cachedTokenPriceMicrocredits ?? 0;
    tier.costPricing.inputTokenPriceMicrocredits = p.costInputTokenPriceMicrocredits ?? p.inputTokenPriceMicrocredits ?? 0;
    tier.costPricing.outputTokenPriceMicrocredits = p.costOutputTokenPriceMicrocredits ?? p.outputTokenPriceMicrocredits ?? 0;
    tier.costPricing.cachedTokenPriceMicrocredits = p.costCachedTokenPriceMicrocredits ?? p.cachedTokenPriceMicrocredits ?? 0;
  } else {
    // fixed_request / per_second
    const unit = p.unitPriceMicrocredits ?? 0;
    tier.unitPriceMicrocredits = unit;
    tier.costPricing.unitPriceMicrocredits = p.costUnitPriceMicrocredits ?? unit;
  }
  return tier;
}

// ---------------------------------------------------------------------------
// 小工具
// ---------------------------------------------------------------------------

function slugifyModelKey(modelKey: string): string {
  const normalized = modelKey.toLowerCase().trim().replace(/[^a-z0-9._-]+/g, "-").replace(/^-+|-+$/g, "");
  return normalized.slice(0, 80);
}

function isPlainObject(v: unknown): v is Record<string, unknown> {
  return typeof v === "object" && v !== null && !Array.isArray(v);
}
function mergeDeep(base: Record<string, unknown>, override: Record<string, unknown>): Record<string, unknown> {
  const out: Record<string, unknown> = { ...base };
  for (const [k, v] of Object.entries(override)) {
    if (isPlainObject(v) && isPlainObject(out[k])) {
      out[k] = mergeDeep(out[k] as Record<string, unknown>, v);
    } else {
      out[k] = v;
    }
  }
  return out;
}

function fail(msg: string): never {
  console.error(`\n[错误] ${msg}`);
  process.exit(1);
}

// ---------------------------------------------------------------------------
// API 客户端（原生 fetch + 手动 Cookie 管理）
// ---------------------------------------------------------------------------

const SESSION_COOKIE = "open_ai_canvas_session";

class ApiClient {
  private cookie: string | null = null;
  constructor(private base: string) {}

  private extractSession(res: Response): string | null {
    const setCookies: string[] = [];
    const multi = (res.headers as any).getSetCookie?.();
    if (Array.isArray(multi)) setCookies.push(...multi);
    const single = res.headers.get("set-cookie");
    if (single) setCookies.push(single);
    for (const c of setCookies) {
      const m = c.match(new RegExp(`(?:^|;\\s*)${SESSION_COOKIE}=([^;]+)`));
      if (m) return m[1];
    }
    return null;
  }

  async request(method: string, path: string, body?: unknown): Promise<any> {
    const headers: Record<string, string> = { "Content-Type": "application/json" };
    if (this.cookie) headers.Cookie = `${SESSION_COOKIE}=${this.cookie}`;
    const res = await fetch(`${this.base}${path}`, {
      method,
      headers,
      body: body === undefined ? undefined : JSON.stringify(body),
    });
    const setSession = this.extractSession(res);
    if (setSession) this.cookie = setSession;

    let json: any = null;
    try {
      json = await res.json();
    } catch {
      /* 非 JSON 响应 */
    }
    if (json && json.code !== 0) {
      throw new Error(`${json.msg ?? "未知错误"}${json.reason ? ` (reason=${json.reason})` : ""}`);
    }
    if (!res.ok) {
      throw new Error(`HTTP ${res.status}`);
    }
    return json?.data;
  }

  async ensureAdmin(admin: SeedConfig["admin"]) {
    // 先尝试登录，失败(用户不存在)则注册——首个用户自动成为管理员。
    try {
      await this.request("POST", "/auth/login", { username: admin.username, password: admin.password });
      console.log(`  已登录管理员：${admin.username}`);
    } catch (e) {
      console.log(`  登录失败，尝试注册首个管理员（注册失败会退出）：${(e as Error).message}`);
      try {
        await this.request("POST", "/auth/register", {
          username: admin.username,
          password: admin.password,
          displayName: admin.displayName ?? admin.username,
          acceptedTerms: true,
        });
        console.log(`  已注册并成为管理员：${admin.username}`);
      } catch (regErr) {
        // 可能是用户已存在但密码不同等，给出明确提示。
        fail(`注册管理员失败：${(regErr as Error).message}`);
      }
    }
  }
}

// ---------------------------------------------------------------------------
// 主流程
// ---------------------------------------------------------------------------

async function seedChannel(api: ApiClient, channel: ChannelSpec): Promise<void> {
  const modelKeys = channel.models.map((m) => m.modelKey);

  // 1. 创建系统渠道，同时注入模型名列表（不依赖上游 /models 端点即可建好模型记录）。
  const created = await api.request("POST", "/admin/channels", {
    name: channel.name,
    baseUrl: channel.baseUrl,
    apiKey: channel.apiKey ?? "",
    secretKey: channel.secretKey ?? "",
    models: modelKeys,
    headers: Object.entries(channel.headers ?? {}).map(([k, v]) => ({ key: k, value: v })),
  });
  const channelId: string = created.channel.id;
  console.log(`  ✓ 渠道「${channel.name}」已创建/存在（id=${channelId}）`);

  // 2. 拉取上游目录（仅诊断/校验；失败不阻塞，因为模型记录已由第 1 步建立）。
  let upstreamModels: string[] = [];
  try {
    const fetched = await api.request("POST", `/admin/channels/${channelId}/models/fetch`);
    upstreamModels = (fetched?.models ?? []).map((m: string) => m.trim());
    console.log(`  · 上游目录（${upstreamModels.length} 个）：${upstreamModels.join(", ") || "(空)"}`);
    for (const m of modelKeys) {
      if (!upstreamModels.includes(m)) {
        console.warn(`  ⚠ 清单模型「${m}」不在上游目录中，请确认模型名或该渠道是否支持 /models 端点`);
      }
    }
  } catch (e) {
    console.log(`  · 拉取上游目录失败（忽略）：${(e as Error).message}`);
  }

  // 3. 读取渠道模型 id（modelKey -> id）。
  const list = await api.request("GET", `/admin/channels/${channelId}/models`);
  const byKey = new Map<string, string>();
  for (const item of list?.models ?? []) {
    byKey.set(item.modelKey, item.id);
  }

  // 4. 逐模型：定价 + 能力 + 启用；再创建前台逻辑模型 + 路由。
  for (const spec of channel.models) {
    const modelId = byKey.get(spec.modelKey);
    if (!modelId) {
      console.warn(`  ✗ 模型「${spec.modelKey}」未在渠道中找到 id，跳过`);
      continue;
    }
    try {
      await seedOneModel(api, channelId, modelId, spec, channel);
    } catch (e) {
      console.warn(`  ✗ 模型「${spec.modelKey}」配置失败：${(e as Error).message}`);
    }
  }
}

async function seedOneModel(
  api: ApiClient,
  channelId: string,
  modelId: string,
  spec: ModelSpec,
  channel: ChannelSpec,
): Promise<void> {
  const capabilityConfig = buildCapabilityConfig(spec);

  // 4a. 保存渠道模型：能力契约 + 价格档 + 启用。
  await api.request("PATCH", `/admin/channels/${channelId}/models/${modelId}`, {
    modelKey: spec.modelKey,
    providerModelKey: spec.modelKey,
    displayName: spec.displayName ?? spec.modelKey,
    capability: spec.capability,
    protocol: spec.protocol,
    billingMode: spec.pricing.billingMode ?? "fixed_request",
    capabilityConfig,
    priceTiers: [buildPriceTier(spec)],
    enabled: true,
  });
  console.log(`  ✓ 渠道模型「${spec.modelKey}」已定价启用（capability=${spec.capability}, protocol=${spec.protocol}）`);

  // 4b. 创建前台逻辑模型：能力规格由渠道模型能力投影，路由指向该渠道模型，价格跟随渠道。
  const capabilitySpec = capabilitySpecFromConfig(spec.capability, capabilityConfig);
  const code = spec.code ?? slugifyModelKey(spec.modelKey);
  await api.request("POST", "/admin/logical-models", {
    code,
    name: spec.logicalName ?? spec.displayName ?? spec.modelKey,
    capability: spec.capability,
    enabled: true,
    pricePolicy: "channel",
    capabilitySpec,
    defaultOptions: {},
    routes: [{ channelModelId: modelId, enabled: true, priority: 100, weight: 100 }],
  });
  console.log(`  ✓ 前台逻辑模型「${code}」已创建并绑定路由`);
}

async function main() {
  const args = process.argv.slice(2);
  if (args.includes("--list-protocols")) {
    console.log("合法 protocol 枚举（按 capability 分组）：");
    for (const [cap, protocols] of Object.entries(PROTOCOLS_BY_CAPABILITY)) {
      console.log(`  ${cap}: ${protocols.join(", ")}`);
    }
    return;
  }

  const configPath = args.find((a) => !a.startsWith("--")) || "scripts/dev-seed-models/seed.local.json";
  let config: SeedConfig;
  try {
    const raw = await Bun.file(configPath).text();
    config = JSON.parse(raw);
  } catch (e) {
    fail(`无法读取清单 ${configPath}：${(e as Error).message}\n用法：bun scripts/dev-seed-models/seed.ts [seed.json]`);
  }

  // 清单静态校验。
  for (const channel of config.channels) {
    if (!channel.name || !channel.baseUrl) fail(`渠道缺少 name/baseUrl：${JSON.stringify(channel)}`);
    if (!channel.apiKey && !channel.secretKey) {
      console.warn(`⚠ 渠道「${channel.name}」未提供 apiKey/secretKey（可能适配免密钥上游，请确认）`);
    }
    for (const m of channel.models) {
      const valid = PROTOCOLS_BY_CAPABILITY[m.capability] ?? [];
      if (!valid.includes(m.protocol)) {
        fail(`模型「${m.modelKey}」protocol "${m.protocol}" 不属于 capability "${m.capability}"。合法值：${valid.join(", ")}`);
      }
      if (["text", "token"].includes(m.pricing.billingMode ?? "") && !m.pricing.inputTokenPriceMicrocredits && m.pricing.billingMode === "token") {
        console.warn(`⚠ 模型「${m.modelKey}」使用 token 计费但未设置 token 价格`);
      }
    }
  }

  const base = config.backendBaseUrl ?? "http://localhost:8080/api";
  console.log(`目标后端：${base}`);
  const api = new ApiClient(base);
  await api.ensureAdmin(config.admin);

  let okCount = 0;
  let failCount = 0;
  for (const channel of config.channels) {
    console.log(`\n=== 渠道：${channel.name} ===`);
    try {
      await seedChannel(api, channel);
      okCount++;
    } catch (e) {
      failCount++;
      console.warn(`  ✗ 渠道「${channel.name}」处理失败：${(e as Error).message}`);
    }
  }
  console.log(`\n完成：${okCount} 个渠道成功，${failCount} 个失败。`);
}

main().catch((e) => {
  console.error(String(e?.stack ?? e));
  process.exit(1);
});