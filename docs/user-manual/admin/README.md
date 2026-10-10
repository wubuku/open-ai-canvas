# 影策管理员后台用户手册

本手册面向负责平台运行、模型渠道、用户运营和系统配置的管理员。它按“任务目标”组织内容，截图用于定位控件，正文步骤才是操作依据。

管理员后台的写入权限很大。发布公告、调整额度、删除资源、修改凭据、切换功能和执行更新，都应先确认影响范围，再在可回退的窗口内操作。本地开发环境的页面可以帮助你熟悉入口，但不能替代生产变更审批。

## 目录

### 开始使用

- [快速上手](00-quickstart.md)：认识后台分区和最短的只读巡检路径。
- [后台概念与权限](30-concepts.md)：理解状态、配置来源、凭据和高风险边界。

### 监控与运营

- [查看运行总览、请求明细和系统性能](10-tasks/monitor-and-troubleshoot.md)
- [管理用户、公告和 Agent 记忆](10-tasks/manage-users-and-notices.md)
- [处理支付、积分和兑换码](10-tasks/manage-billing-and-credits.md)

### 模型、资源与扩展

- [配置系统渠道、模型、提示词和资源](10-tasks/manage-models-and-resources.md)
- [管理插件与技能分类](10-tasks/manage-plugins-and-skills.md)

### 平台配置

- [配置站点外观、功能和绘图工具](10-tasks/configure-product-experience.md)
- [配置资源配额、任务并发和超时](10-tasks/configure-runtime-policy.md)
- [配置登录、注册和邮件服务](10-tasks/configure-access-and-email.md)
- [配置存储、方舟素材库和第三方参数](10-tasks/configure-storage-integrations.md)
- [配置响应拦截与系统更新](10-tasks/operate-safety-and-updates.md)

### 参考与排障

- [管理员参考](20-reference.md)：路由、字段、状态和安全边界速查。
- [常见问题与排障](90-troubleshooting.md)：按症状定位问题，包含视频任务超时的排查顺序。

## 阅读约定

- **粗体**表示页面上的按钮、菜单或字段名称，请以当前界面显示的中文为准。
- 截图统一来自已登录的本地开发环境，主要用于帮助你找到区域和控件；截图中的数据是开发数据。
- 本手册记录了只读浏览和安全展开的真实回走结果。没有执行支付、删除、调账、凭据保存、插件卸载、系统更新或回退。
