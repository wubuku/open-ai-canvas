# 创作台手册进度

## 当前阶段
创作台首版手册已完成图文内容、真实浏览器回走和最终机械审计；当前任务清单为 13 个，截图覆盖页面状态和关键操作中间态。

## 任务状态
| 任务 | 状态 | 说明 |
|---|---|---|
| register-and-login | verified | 登录页截图；当前登录态入口已回走，未退出账号注册 |
| use-canvas | verified | 旗舰流程，5 张截图；界面入口已回走，真实生成/导入导出保留限制 |
| create-modes | verified | 图片/视频/文本/Agent 模式和操作截图 |
| generate-image | verified | 提交前控件已回走，未触发真实生成 |
| generate-video | verified | 提交前控件、历史状态和超时排障路径已回走，未触发真实生成 |
| browse-tasks | verified | 历史页和失败筛选已回走 |
| manage-assets | verified | 资产页和操作菜单已回走；编辑入口按素材类型区分 |
| browse-skills | verified | 搜索、详情和技能页已回走 |
| use-plugins | verified | 插件中心分类、文档和状态已回走 |
| browse-inspirations | verified | 灵感列表、详情和带入创作已回走 |
| manage-projects | verified | 项目页和新短剧展开入口已回走，未提交章节生成 |
| use-wallet | verified | 钱包页和消费记录已回走 |
| personal-settings | verified | 设置页、个人渠道入口和模型选择入口已回走 |

## 已知限制
- 未启动真实图片/视频/文本模型生成、支付、删除、发布或权限变更。
- 自由画布的真实节点创建、连线、撤销/重做持久化、导入导出和分享发布未在共享开发账号上触发。
- 视频超时说明强调“前端超时不等于任务失败”，用户应先查创作历史，避免重复提交和重复扣费。
- 管理员后台手册（`docs/user-manual/admin/`）尚未开始。
