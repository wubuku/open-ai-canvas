# 管理员参考

## 路由速查

| 分区 | 路由 | 用途 |
|---|---|---|
| 监控与排查 | `/admin` | 运行总览、趋势、导出 |
| 监控与排查 | `/admin/logs` | 请求明细和任务恢复入口 |
| 监控与排查 | `/admin/settings/system-performance` | 进程、磁盘、数据库、Redis和 Agent状态 |
| 模型与资源 | `/admin/channels` | 系统渠道与模型 |
| 模型与资源 | `/admin/prompt-templates` | 提示词模板版本 |
| 模型与资源 | `/admin/resources` | 资源记录、预览、下载、删除 |
| 用户与运营 | `/admin/users` | 用户角色、状态和详情 |
| 用户与运营 | `/admin/announcements` | 全体用户公告 |
| 用户与运营 | `/admin/banner-announcements` | 首页常驻通知 |
| 用户与运营 | `/admin/agent-lessons` | Agent 记忆巡查 |
| 用户与运营 | `/admin/payments` | 支付查单和对账 |
| 用户与运营 | `/admin/credit-operations` | 积分核对和人工调账 |
| 用户与运营 | `/admin/redemption-codes` | 兑换码批次 |
| 平台配置 | `/admin/plugins` | 插件平台状态 |
| 平台配置 | `/admin/skill-curation` | 技能分类和策展 |
| 平台配置 | `/admin/settings/appearance` | 品牌、主题和页面文案 |
| 平台配置 | `/admin/settings/features` | 功能开关和模型来源 |
| 平台配置 | `/admin/settings/drawing-engine` | 新绘图节点默认引擎 |
| 平台配置 | `/admin/settings/runtime-policy` | 配额、并发、超时、频控、熔断 |
| 平台配置 | `/admin/settings/access` | 注册、Linux.do和服务协议 |
| 平台配置 | `/admin/settings/email` | 验证码邮件和 SMTP |
| 平台配置 | `/admin/settings/storage` | 对象存储和用户自有存储 |
| 平台配置 | `/admin/settings/ark-private-assets` | 方舟可信参考素材同步 |
| 平台配置 | `/admin/settings/response-interception` | 用户可见错误文案规则 |
| 平台配置 | `/admin/settings/third-party` | LibTV等第三方参数 |
| 平台配置 | `/admin/settings/system-update` | 更新检查、更新和回退 |

## 常见状态词

- **系统默认**：当前值来自部署默认或环境配置，管理员尚未覆盖。
- **管理员配置**：当前值由后台保存，可能覆盖系统默认。
- **已配置**：页面只证明服务端存在配置，不返回密钥明文，也不代表上游可用。
- **待补充**：页面允许继续填写或当前内容为空，不代表接口失败。
- **立即生效**：保存后新请求会读取新值；已经运行的任务是否受影响，要看具体策略。

## 身份与完整性边界

- 用户、节点、任务、批次和资源 ID分别属于不同业务域，不能互相替代。
- `storageKey`用于定位物理对象，`assetId`用于业务资源身份；URL存在不能单独证明对象已物化。
- 任务 ID、任务版本、生成效果键和本地同步序列用途不同；排障时记录完整上下文。
- 删除资源前检查项目、画布、任务和其他业务引用；物理删除失败不能先删记录。

## 变更前检查表

1. 目标环境和管理员账号确认。
2. 影响对象、费用、权限和数据范围确认。
3. 旧值、回滚值、观察指标和截止时间记录。
4. 凭据使用最小权限，且不写入截图、日志或 URL。
5. 保存后检查真实用户路径；只读页面和健康检查不能替代写入验收。
