import { defineConfig } from 'vitepress';

export default defineConfig({
  lang: 'zh-CN',
  title: '影策用户手册',
  description: '影策创作台与管理员后台的图文使用手册',
  head: [['link', { rel: 'icon', href: '/favicon.svg' }]],
  cleanUrls: true,
  lastUpdated: true,
  srcExclude: ['**/AUDIT.md', '**/PROGRESS.md', '**/task-inventory.yml', '**/screenshots/manifest.yml', '**/README.md'],
  themeConfig: {
    nav: [
      { text: '创作台', link: '/creator/' },
      { text: '管理员后台', link: '/admin/' },
    ],
    sidebar: {
      '/creator/': [
        {
          text: '开始使用',
          items: [
            { text: '创作台用户手册', link: '/creator/' },
            { text: '快速上手', link: '/creator/00-quickstart' },
          ],
        },
        {
          text: '核心创作',
          items: [
            { text: '使用自由画布', link: '/creator/10-tasks/use-canvas' },
            { text: '选择创作模式', link: '/creator/10-tasks/create-modes' },
            { text: '生成图片', link: '/creator/10-tasks/generate-image' },
            { text: '生成视频', link: '/creator/10-tasks/generate-video' },
            { text: '管理短剧项目', link: '/creator/10-tasks/manage-projects' },
          ],
        },
        {
          text: '素材与能力',
          items: [
            { text: '管理素材', link: '/creator/10-tasks/manage-assets' },
            { text: '浏览灵感库', link: '/creator/10-tasks/browse-inspirations' },
            { text: '使用技能', link: '/creator/10-tasks/browse-skills' },
            { text: '使用插件', link: '/creator/10-tasks/use-plugins' },
          ],
        },
        {
          text: '账户与任务',
          items: [
            { text: '创作历史', link: '/creator/10-tasks/browse-tasks' },
            { text: '积分与消费记录', link: '/creator/10-tasks/use-wallet' },
            { text: '个人设置', link: '/creator/10-tasks/personal-settings' },
            { text: '注册与登录', link: '/creator/10-tasks/register-and-login' },
          ],
        },
        {
          text: '参考与排障',
          items: [
            { text: '创作台参考', link: '/creator/20-reference' },
            { text: '创作台概念', link: '/creator/30-concepts' },
            { text: '常见问题', link: '/creator/90-troubleshooting' },
          ],
        },
      ],
      '/admin/': [
        {
          text: '开始使用',
          items: [
            { text: '管理员后台', link: '/admin/' },
            { text: '快速上手', link: '/admin/00-quickstart' },
          ],
        },
        {
          text: '监控与运营',
          items: [
            { text: '运行总览、请求明细和系统性能', link: '/admin/10-tasks/monitor-and-troubleshoot' },
            { text: '用户、公告和 Agent 记忆', link: '/admin/10-tasks/manage-users-and-notices' },
            { text: '支付、积分和兑换码', link: '/admin/10-tasks/manage-billing-and-credits' },
          ],
        },
        {
          text: '模型、资源与扩展',
          items: [
            { text: '系统渠道、模型、提示词和资源', link: '/admin/10-tasks/manage-models-and-resources' },
            { text: '插件与技能分类', link: '/admin/10-tasks/manage-plugins-and-skills' },
          ],
        },
        {
          text: '平台配置',
          items: [
            { text: '站点外观、功能和绘图工具', link: '/admin/10-tasks/configure-product-experience' },
            { text: '资源配额、并发和超时', link: '/admin/10-tasks/configure-runtime-policy' },
            { text: '登录、注册和邮件服务', link: '/admin/10-tasks/configure-access-and-email' },
            { text: '存储、方舟和第三方参数', link: '/admin/10-tasks/configure-storage-integrations' },
            { text: '响应拦截与系统更新', link: '/admin/10-tasks/operate-safety-and-updates' },
          ],
        },
        {
          text: '参考与排障',
          items: [
            { text: '管理员参考', link: '/admin/20-reference' },
            { text: '后台概念与权限', link: '/admin/30-concepts' },
            { text: '常见问题与排障', link: '/admin/90-troubleshooting' },
          ],
        },
      ],
    },
    search: { provider: 'local' },
    outline: { level: [2, 3] },
    socialLinks: [],
    footer: { message: '影策用户手册' },
  },
});
