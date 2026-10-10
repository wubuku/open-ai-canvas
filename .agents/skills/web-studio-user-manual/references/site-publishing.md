# 手册站点构建与发布

本项目的用户手册计划位于 `docs/user-manual/`，可使用 VitePress 构建为独立静态站点。手册 Skill 只负责按用户确认的范围产生/维护内容；只有用户明确要求构建、预览或发布时，才执行本参考中的站点操作。

## 站点结构

```text
docs/user-manual/
├── package.json              # VitePress 依赖
├── package-lock.json
├── .vitepress/
│   ├── config.mjs            # 站点元信息、侧边栏、搜索、srcExclude
│   └── dist/                 # 构建产物（不提交）
├── creator/                  # 创作台手册
├── admin/                    # 管理员后台手册
└── build-site.sh             # 一键构建脚本
```

## 推荐构建流程

在仓库根目录执行：

```bash
cd docs/user-manual
./build-site.sh
```

脚本会依次：

1. 检查 `node`、`npm`、Node.js 18+；
2. 在缺少 `node_modules/vitepress` 时执行 `npm install --no-audit --no-fund`；
3. 清理旧的 `.vitepress/dist` 与 `.vitepress/cache`；
4. 执行 `npx vitepress build`；
5. 检查 `dist/index.html`、HTML/截图数量和是否残留未改写的 `.md` 链接。

首次安装依赖需要网络。若本机 npm registry 不可用，遵循项目网络环境使用临时 registry/代理参数；不要修改任何 `.env` 文件。

构建完成后：

```bash
./build-site.sh --preview                         # VitePress 预览：http://localhost:4173
```

预览属于只读验证，不应提交表单、触发生成任务或修改业务数据。

## 中文搜索、侧边栏和内部账本

- 中文全文搜索使用 `.vitepress/config.mjs` 的 `themeConfig.search.provider: 'local'`。
- 用户导航使用 `themeConfig.sidebar`；创作台和管理员后台各自分组。新增或改名用户页面后，必须补充/修改对应 sidebar 项。
- `rewrites` 将根目录 `README.md` 映射为站点首页。
- `srcExclude` 排除 `AUDIT.md`、`PROGRESS.md` 和其他内部账本。新增内部 Markdown 文件时，先把它加入 `srcExclude` 再构建。
- 站点内容应保持面向最终用户；不要把构建日志、数据库记录、浏览器凭据、Token、内部 URL 或截图采集证据写进正文或可发布资源。

## 子路径部署与发布边界

若站点最终地址为 `https://example.com/manual/`，修改 `.vitepress/config.mjs`：

```js
base: '/manual/',
```

然后重新运行 `./build-site.sh`。只把 `.vitepress/dist/` 整体交给静态 Web 服务器、对象存储静态托管或其他已获授权的发布渠道；不要上传源码目录来代替构建产物。构建本身不代表允许发布，远端覆盖、域名配置和生产部署必须另获用户明确授权。

## 构建后的双重检查

站点构建成功后仍要运行手册内容审计：

```bash
python3 .agents/skills/web-studio-user-manual/scripts/audit_manual.py \
  docs/user-manual/creator --phase gate-a
python3 .agents/skills/web-studio-user-manual/scripts/audit_manual.py \
  docs/user-manual/admin --phase gate-a
```

发布前再运行 `--phase final`，并完成真实浏览器 Gate B 回走。构建通过只证明 VitePress 能生成静态文件，不证明手册步骤仍与当前 UI 一致。
