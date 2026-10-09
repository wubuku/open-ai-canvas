---
title: 画布开发导航审计
status: research
scope: documentation
updated: 2026-10-09
---

# 画布开发导航审计

## 结论

在新增正式页面前，画布相关内容不能被快速导航到。已有信息分散在以下位置：

- `docs/content/docs/backend/code-map.mdx`：包含云端 Agent 和部分页面入口，但不是画布开发指南。
- `docs/design/canvas-consistency-repair.mdx`：深入记录批量生成、持久化和节点可见性的一致性边界，但属于问题治理设计记录。
- `docs/design/canvas-final-performance-architecture.md`、`canvas-floating-controls.mdx`、`canvas-node-visual-contrast.mdx`：分别关注性能、浮动控件和视觉设计，不能作为代码入口总览。
- `docs/content/docs/plugins/plugin-system.mdx`、`plugin-surfaces.mdx`：覆盖插件和画布 surface，但不解释宿主画布 store、任务回写和远端同步。
- 源码本身已经形成较清晰的模块边界：`web/src/pages/canvas/`、`web/src/components/canvas/`、`web/src/stores/canvas/`、`web/src/lib/canvas/`、`web/src/services/api/user-data.ts`、`backend/internal/canvas/`。

因此，开发者若只阅读现有 `docs/index.md` 或代码地图，能找到部分画布入口，但仍需要自行推导以下关系：

```text
CanvasProject / CanvasNodeData / CanvasConnection
  -> 页面与 hook 编排
  -> Zustand + localforage + revision merge
  -> remote canvas API + history
  -> task admission + generation consumer
  -> Agent capability / plugin surface
```

## 需要稳定推广的内容

以下内容已经有当前代码依据，适合提升到正式开发导航：

1. 节点、连线、项目和 metadata 的持久化模型。
2. 页面、组件、store、纯函数和 service 的职责边界。
3. 本地 scope、storage revision、journal、tombstone 和远端 ETag/revision 的协作。
4. 生成任务提交、任务消费、最小增量回写、资源物化和幂等效果键。
5. Agent capability registry、结构化画布操作和插件节点的发现路径。
6. 画布性能、批量生成一致性和常见问题的最短排查路径。

这些内容已整合到 `docs/content/docs/canvas/canvas-development.mdx`，并通过 `docs/index.md`、`README.md`、`AGENTS.md` 和 `docs/content/docs/canvas/meta.json` 导航。

## 仍需验证或不应过度承诺的内容

- 当前 checkout 没有完整的文档站构建应用，不能把 MDX 页面当作文档站已经成功构建的证据。
- 画布真实登录、跨设备同步、SSE 断线、对象存储、真实模型上游和独立 `yingce-agent` 运行仍需要环境验收。
- 画布文档只描述当前实现入口，不替代具体 API、数据库、插件 Manifest 或设计记录。

## 维护规则

- 新增画布功能先在本文“按问题找入口”表中补入口，再决定是否增加专题页。
- 新的现场调查、性能数据和未确认根因优先放在 `docs/drafts/`，确认后只把稳定结论增量推广到正式页。
- 修改节点字段、生成合同、远端同步或 Agent capability 时，同时检查本文中的数据模型、调用链和验证清单，避免只更新 UI 文案。
