#!/usr/bin/env bun
/**
 * dev-seed-models —— 本地开发环境模型配置批量 seed 脚本。
 *
 * 只通过后端管理 API（走 HTTP + Cookie 会话）完成整条链路：
 *   登录/注册管理员 → 创建系统渠道 → 拉取上游目录(诊断) → 逐模型定价启用 → 创建前台逻辑模型(+路由)。
 *
 * 运行（需要 bun，Node 22+ 亦可）：
 *   bun scripts/dev-seed-models/seed.ts [seed.json | seed.yaml]
 * 未指定文件时读取 scripts/dev-seed-models/seed.local.json（或 seed.local.yaml）。
 *
 * 清单支持 JSON 与 YAML 两种格式（按扩展名区分），并支持引用环境变量：
 *   apiKey: ${OPENAI_API_KEY}            —— 敏感密钥只放环境变量，绝不写进清单。
 *   password: ${ADMIN_PASSWORD:-admin}   —— 支持缺省值。
 *
 * 本地 dev 的「计费」无关紧要：模型的 pricing 可整段省略，脚本按「按次、0 价」补齐，
 * 用最少字段即可立即开测（生图 / 生视频 / 剧本解析等）。
 *
 * 重要边界：
 *  - 每个模型的 capability(text/image/video/audio) 和 protocol 必须显式声明，
 *    上游 /models 目录只返回模型名，无法推断能力或协议。
 *  - 渠道密钥(apiKey/secretKey)应通过环境变量注入，不提交含真实密钥的清单。
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
  /** 计费规则；本地 dev 可整段省略，脚本按「按次、0 价」补齐（计费在本地无关紧要）。 */
  pricing?: ModelPricing;
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

/** 平台对象存储（OSS / S3 兼容）设置；缺省整段则跳过存储 seed。 */
interface StorageSpec {
  /** s3 / aliyun / tencent / qiniu。启用模型输入参考图，本地 dev 建议用 S3 兼容存储。 */
  provider: "s3" | "aliyun" | "tencent" | "qiniu";
  enabled: boolean;
  /** s3 预设：aws / r2 / b2 / rustfs / custom；非 s3 时忽略。 */
  s3Preset?: string;
  region?: string;
  endpoint?: string;
  bucket?: string;
  accessKeyId?: string;
  accessKeySecret?: string;
  cdnBaseUrl?: string;
  publicBaseUrl?: string;
  pathPrefix?: string;
  pathStyle?: boolean;
  sessionToken?: string;
  allowUserS3?: boolean;
  cdnAuthMode?: string;
  allowPrivateProxy?: boolean;
}

/** 方舟可信素材库（资产库）设置；Seedance 上传参考图 + 仿真人肖像审核用。 */
interface ArkPrivateAssetsSpec {
  enabled: boolean;
  region: string;
  projectName: string;
  accessKeyId: string;
  accessKeySecret: string;
}

interface SeedConfig {
  backendBaseUrl?: string;
  admin: { username: string; password: string; displayName?: string };
  channels: ChannelSpec[];
  storage?: StorageSpec;
  arkPrivateAssets?: ArkPrivateAssetsSpec;
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

// 图片默认尺寸/比例预设，复刻 backend internal/app/model_capability_defaults.go 的
// legacyImageSizeValues()（前后端共同展示的基础预设）。
// 不能用 size.parameter="none"：backend 的 channelModelDefaultOptions 对 image 无条件输出
// defaults["size"]，而 size="none" 时能力规格里根本没有 size 选项，导致 sanitizeChannelModel
// 报「默认参数 size 不在前台模型能力范围内」，整个渠道模型被 /model-catalog 丢弃，
// 生图模型在设置页里就看不见了。
const DEFAULT_IMAGE_SIZE_VALUES = [
  "1:1", "3:2", "2:3", "4:3", "3:4", "16:9", "21:9", "9:16",
  "1024x1024", "1536x1024", "1024x1536",
];

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
      size: { parameter: "size", values: DEFAULT_IMAGE_SIZE_VALUES, default: "1:1", allowCustom: true },
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
        maxImages: 9,
        maxImageBytes: 31457280,
        maxVideos: 0,
        maxVideoBytes: 0,
        maxVideoDurationSeconds: 0,
        maxAudios: 0,
        maxAudioBytes: 0,
        maxAudioDurationSeconds: 0,
      },
      duration: { selection: "range", min: 1, max: 15, step: 1, default: 6 },
      ratios: ["16:9", "9:16", "1:1", "4:3", "3:4", "21:9"],
      defaultRatio: "16:9",
      resolutions: ["480p", "720p", "1080p", "1440p", "2160p"],
      defaultResolution: "720p",
      generateAudio: { supported: false, default: false },
      watermark: { supported: false, default: false },
      operations: ["text_to_video", "image_to_video"],
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
  const p = spec.pricing ?? {};
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

// ---------------------------------------------------------------------------
// 清单加载：JSON / YAML + 环境变量引用展开
// ---------------------------------------------------------------------------

/** 展开字符串中的 ${VAR} 与 ${VAR:-默认值}；变量未设置且无默认值时抛错。 */
function expandEnvString(value: string): string {
  return value.replace(/\$\{([A-Za-z_][A-Za-z0-9_]*)(:-[^}]*)?\}/g, (_m, name: string, fallback?: string) => {
    const env = process.env[name];
    if (env !== undefined && env !== "") return env;
    if (fallback !== undefined) return fallback.slice(2); // 去掉 ":-"
    throw new Error(`清单引用了未设置的环境变量 \${${name}}，请先 export ${name}=...`);
  });
}

/** 递归展开配置树里所有字符串叶子中的环境变量引用。 */
function expandEnv(value: unknown): unknown {
  if (typeof value === "string") return expandEnvString(value);
  if (Array.isArray(value)) return value.map(expandEnv);
  if (isPlainObject(value)) {
    const out: Record<string, unknown> = {};
    for (const [k, v] of Object.entries(value)) out[k] = expandEnv(v);
    return out;
  }
  return value;
}

/** 读取并解析清单（按扩展名选 JSON / YAML），随后展开环境变量引用。 */
async function loadConfig(path: string): Promise<SeedConfig> {
  let raw: string;
  try {
    raw = await Bun.file(path).text();
  } catch (e) {
    fail(`无法读取清单 ${path}：${(e as Error).message}\n用法：bun scripts/dev-seed-models/seed.ts [seed.json|seed.yaml]`);
  }
  raw = raw.replace(/^﻿/, "");

  let parsed: unknown;
  if (/\.ya?ml$/i.test(path)) {
    let yamlMod: { parse: (text: string) => unknown };
    try {
      yamlMod = await import("yaml");
    } catch {
      fail("读取 YAML 清单需要 yaml 依赖，请先执行：cd scripts/dev-seed-models && bun install（JSON 清单无需安装）");
    }
    parsed = yamlMod.parse(raw);
  } else {
    try {
      parsed = JSON.parse(raw);
    } catch (e) {
      fail(`清单不是合法 JSON：${(e as Error).message}（YAML 清单请使用 .yaml / .yml 后缀）`);
    }
  }
  return expandEnv(parsed) as SeedConfig;
}

/** 回环后端 + 本机 HTTP 代理未排除回环时提示（bun 从 OS 环境读代理，脚本内改 process.env 无效）。 */
function warnLoopbackProxy(base: string) {
  let host = "";
  try {
    host = new URL(base).hostname;
  } catch {
    return;
  }
  if (host !== "localhost" && host !== "127.0.0.1" && host !== "::1") return;
  const proxySet = ["http_proxy", "https_proxy", "all_proxy"].some((k) => process.env[k]);
  if (!proxySet) return;
  const noProxy = (process.env.no_proxy ?? process.env.NO_PROXY ?? "").toLowerCase();
  if (noProxy.includes("localhost") || noProxy.includes("127.0.0.1") || noProxy === "*") return;
  console.warn(
    "\n⚠ 检测到本机 HTTP 代理，但回环地址未排除：fetch 到 localhost 后端可能被代理误导返回 502。\n" +
      "  请在命令前加 NO_PROXY 前缀，例如：\n" +
      "  NO_PROXY=127.0.0.1,localhost bun scripts/dev-seed-models/seed.ts ...\n",
  );
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

/** 按名字查已有系统渠道（复用，实现幂等），未命中返回 null。 */
async function findChannelByName(api: ApiClient, name: string): Promise<{ id: string } | null> {
  const page = await api.request("GET", `/admin/channels?keyword=${encodeURIComponent(name)}&limit=100`);
  const channels: any[] = page?.channels ?? [];
  return channels.find((c) => c.name === name) ?? null;
}

/** 预取所有前台逻辑模型的 code，用于跳过已存在项（幂等）。 */
async function fetchExistingLogicalCodes(api: ApiClient): Promise<Set<string>> {
  try {
    const res = await api.request("GET", "/admin/logical-models");
    const models: any[] = res?.models ?? [];
    return new Set(models.map((m) => m.code as string).filter(Boolean));
  } catch (e) {
    console.warn(`  · 读取已有前台逻辑模型失败（忽略，仍尝试直接创建）：${(e as Error).message}`);
    return new Set();
  }
}

async function seedChannel(api: ApiClient, channel: ChannelSpec, existingCodes: Set<string>): Promise<void> {
  const modelKeys = channel.models.map((m) => m.modelKey);

  // 1. 复用同名渠道（幂等）：命中已存在渠道则 PATCH 更新，未命中才 POST 新建。
  const payload = {
    name: channel.name,
    baseUrl: channel.baseUrl,
    apiKey: channel.apiKey ?? "",
    secretKey: channel.secretKey ?? "",
    models: modelKeys,
    headers: Object.entries(channel.headers ?? {}).map(([k, v]) => ({ key: k, value: v })),
  };
  const existing = await findChannelByName(api, channel.name);
  let channelId: string;
  if (existing) {
    const updated = await api.request("PATCH", `/admin/channels/${existing.id}`, payload);
    channelId = updated.channel.id;
    console.log(`  ✓ 渠道「${channel.name}」已存在并更新（id=${channelId}）`);
  } else {
    const created = await api.request("POST", "/admin/channels", payload);
    channelId = created.channel.id;
    console.log(`  ✓ 渠道「${channel.name}」已创建（id=${channelId}）`);
  }

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
      await seedOneModel(api, channelId, modelId, spec, channel, existingCodes);
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
  existingCodes: Set<string>,
): Promise<void> {
  const capabilityConfig = buildCapabilityConfig(spec);

  // 4a. 保存渠道模型：能力契约 + 价格档 + 启用。
  await api.request("PATCH", `/admin/channels/${channelId}/models/${modelId}`, {
    modelKey: spec.modelKey,
    providerModelKey: spec.modelKey,
    displayName: spec.displayName ?? spec.modelKey,
    capability: spec.capability,
    protocol: spec.protocol,
    billingMode: spec.pricing?.billingMode ?? "fixed_request",
    capabilityConfig,
    priceTiers: [buildPriceTier(spec)],
    enabled: true,
  });
  console.log(`  ✓ 渠道模型「${spec.modelKey}」已定价启用（capability=${spec.capability}, protocol=${spec.protocol}）`);

  // 4b. 创建前台逻辑模型：能力规格由渠道模型能力投影，路由指向该渠道模型，价格跟随渠道。
  const capabilitySpec = capabilitySpecFromConfig(spec.capability, capabilityConfig);
  const code = spec.code ?? slugifyModelKey(spec.modelKey);
  if (existingCodes.has(code)) {
    console.log(`  · 前台逻辑模型「${code}」已存在，跳过创建（如需重建请先删除旧逻辑模型）`);
    return;
  }
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

async function seedStorage(api: ApiClient, storage: StorageSpec): Promise<void> {
  const req: Record<string, unknown> = {
    enabled: storage.enabled,
    provider: storage.provider,
    region: storage.region ?? "",
    endpoint: storage.endpoint ?? "",
    cdnBaseUrl: storage.cdnBaseUrl ?? "",
    bucket: storage.bucket ?? "",
    accessKeyId: storage.accessKeyId ?? "",
    accessKeySecret: storage.accessKeySecret ?? "",
    publicBaseUrl: storage.publicBaseUrl ?? "",
    pathPrefix: storage.pathPrefix ?? "",
    s3Preset: storage.s3Preset ?? (storage.provider === "s3" ? "custom" : ""),
    pathStyle: storage.pathStyle ?? false,
    sessionToken: storage.sessionToken ?? "",
    allowUserS3: storage.allowUserS3 ?? false,
    cdnAuthMode: storage.cdnAuthMode ?? "",
    allowPrivateProxy: storage.allowPrivateProxy ?? false,
  };
  // S3 兼容存储在被「启用」前，后端要求先通过连接测试（写入/读取/删除探针对象，限流 6 次/分）。
  // 测试失败直接抛错，不继续 PATCH，避免后端返回「S3 关键配置尚未通过连接测试」的次生错误。
  if (storage.provider === "s3" && storage.enabled) {
    await api.request("POST", "/admin/settings/oss/test", req);
    console.log("  ✓ 存储连接测试通过（写入/读取/删除探针对象）");
  }
  await api.request("PATCH", "/admin/settings/oss", req);
  console.log(`  ✓ 存储已配置（provider=${storage.provider}${storage.bucket ? `, bucket=${storage.bucket}` : ""}${storage.enabled ? ", 已启用" : ", 未启用"}）`);
}

async function seedArkPrivateAssets(api: ApiClient, ark: ArkPrivateAssetsSpec): Promise<void> {
  await api.request("PATCH", "/admin/settings/ark-private-assets", {
    enabled: ark.enabled,
    region: ark.region,
    projectName: ark.projectName,
    accessKeyId: ark.accessKeyId,
    accessKeySecret: ark.accessKeySecret,
  });
  console.log(`  ✓ 方舟可信素材库已配置（project=${ark.projectName}, region=${ark.region}${ark.enabled ? ", 已启用" : ", 未启用"}）`);
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

  let configPath = args.find((a) => !a.startsWith("--"));
  if (!configPath) {
    // 缺省先看 seed.local.json，没有再看 seed.local.yaml（二者都被 .gitignore 忽略）。
    configPath = "scripts/dev-seed-models/seed.local.json";
    if (!(await Bun.file(configPath).exists())) {
      const alt = "scripts/dev-seed-models/seed.local.yaml";
      if (await Bun.file(alt).exists()) configPath = alt;
    }
  }
  const config: SeedConfig = await loadConfig(configPath);

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
      if (!m.pricing) {
        console.log(`  · 模型「${m.modelKey}」未声明 pricing，按「按次、免费」补齐（本地 dev 计费无关紧要）`);
      }
    }
  }

  const base = config.backendBaseUrl ?? "http://localhost:8080/api";
  console.log(`目标后端：${base}`);
  warnLoopbackProxy(base);
  const api = new ApiClient(base);
  await api.ensureAdmin(config.admin);

  let okCount = 0;
  let failCount = 0;
  const existingCodes = await fetchExistingLogicalCodes(api);

  // 平台级配置（存储 / 方舟素材库）先于渠道，失败不阻塞渠道 seed，但会显式告警。
  if (config.storage) {
    console.log("\n=== 存储服务 ===");
    try {
      await seedStorage(api, config.storage);
    } catch (e) {
      console.warn(`  ✗ 存储配置失败：${(e as Error).message}`);
    }
  }
  if (config.arkPrivateAssets) {
    console.log("\n=== 方舟可信素材库（资产库） ===");
    try {
      await seedArkPrivateAssets(api, config.arkPrivateAssets);
    } catch (e) {
      console.warn(`  ✗ 方舟素材库配置失败：${(e as Error).message}`);
    }
  }

  for (const channel of config.channels) {
    console.log(`\n=== 渠道：${channel.name} ===`);
    try {
      await seedChannel(api, channel, existingCodes);
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