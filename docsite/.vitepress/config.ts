import { defineConfig } from 'vitepress'

export default defineConfig({
  // 自定义域名部署在根路径
  base: '/',

  lang: 'zh-CN',
  title: 'NetPanel',
  description: '面向家庭和小型网络环境的内网穿透与网络管理面板',

  head: [
    ['link', { rel: 'icon', type: 'image/svg+xml', href: '/logo.svg' }],
    ['meta', { name: 'theme-color', content: '#1677ff' }],
    ['meta', { property: 'og:type', content: 'website' }],
    ['meta', { property: 'og:title', content: 'NetPanel' }],
    ['meta', { property: 'og:description', content: '面向家庭和小型网络环境的内网穿透与网络管理面板' }],
  ],

  themeConfig: {
    logo: '/logo.svg',
    siteTitle: 'NetPanel',

    // 顶部导航栏
    nav: [
      { text: '首页', link: '/' },
      { text: '快速开始', link: '/guide/installation' },
      {
        text: '功能文档',
        items: [
          {
            text: '网络穿透与组网',
            items: [
              { text: '端口转发', link: '/features/port-forward' },
              { text: 'STUN 内网穿透', link: '/features/stun' },
              { text: 'FRP 客户端', link: '/features/frp-client' },
              { text: 'FRP 服务端', link: '/features/frp-server' },
              { text: 'EasyTier 客户端', link: '/features/easytier-client' },
              { text: 'EasyTier 服务端', link: '/features/easytier-server' },
              { text: 'NPS', link: '/features/nps' },
            ]
          },
          {
            text: '域名与证书',
            items: [
              { text: '动态域名 (DDNS)', link: '/features/ddns' },
              { text: '域名账号', link: '/features/domain-account' },
              { text: '域名解析', link: '/features/dns-records' },
              { text: '域名证书', link: '/features/ssl-cert' },
            ]
          },
          {
            text: '网站与安全',
            items: [
              { text: '网站服务 (Caddy)', link: '/features/caddy' },
              { text: '网络防护 (WAF)', link: '/features/waf' },
              { text: '访问控制', link: '/features/access-control' },
              { text: '防火墙', link: '/features/firewall' },
            ]
          },
          {
            text: '辅助功能',
            items: [
              { text: '网络唤醒 (WOL)', link: '/features/wol' },
              { text: '解析服务 (DNSMasq)', link: '/features/dnsmasq' },
              { text: '计划任务', link: '/features/cron' },
              { text: '网络存储', link: '/features/storage' },
              { text: 'IP 地址库', link: '/features/ip-database' },
            ]
          },
          {
            text: '回调系统',
            items: [
              { text: '回调任务与账号', link: '/features/callback' },
            ]
          },
        ]
      },
      {
        text: '系统管理',
        items: [
          { text: '系统设置', link: '/guide/system' },
          { text: '用户管理', link: '/guide/users' },
          { text: '系统日志', link: '/guide/logs' },
          { text: '官方资源与下载', link: '/guide/resources' },
        ]
      },
    ],

    // 侧边栏
    sidebar: {
      '/guide/': [
        {
          text: '入门指南',
          collapsed: false,
          items: [
            { text: '安装部署', link: '/guide/installation' },
            { text: '系统设置', link: '/guide/system' },
            { text: '用户管理', link: '/guide/users' },
            { text: '系统日志', link: '/guide/logs' },
          ]
        },
        {
          text: '版本更新',
          collapsed: false,
          items: [
            { text: 'V0.3 更新说明', link: '/guide/release-v0.3' },
          ]
        },
        {
          text: '资源与下载',
          collapsed: false,
          items: [
            { text: '官方资源与下载', link: '/guide/resources' },
          ]
        },
        {
          text: '网络穿透与组网',
          collapsed: true,
          items: [
            { text: '端口转发', link: '/features/port-forward' },
            { text: 'STUN 内网穿透', link: '/features/stun' },
            { text: 'FRP 客户端', link: '/features/frp-client' },
            { text: 'FRP 服务端', link: '/features/frp-server' },
            { text: 'EasyTier 客户端', link: '/features/easytier-client' },
            { text: 'EasyTier 服务端', link: '/features/easytier-server' },
            { text: 'NPS', link: '/features/nps' },
          ]
        },
        {
          text: '域名与证书',
          collapsed: true,
          items: [
            { text: '动态域名 (DDNS)', link: '/features/ddns' },
            { text: '域名账号', link: '/features/domain-account' },
            { text: '域名解析', link: '/features/dns-records' },
            { text: '域名证书', link: '/features/ssl-cert' },
          ]
        },
        {
          text: '网站与安全',
          collapsed: true,
          items: [
            { text: '网站服务 (Caddy)', link: '/features/caddy' },
            { text: '网络防护 (WAF)', link: '/features/waf' },
            { text: '访问控制', link: '/features/access-control' },
            { text: '防火墙', link: '/features/firewall' },
          ]
        },
        {
          text: '辅助功能',
          collapsed: true,
          items: [
            { text: '网络唤醒 (WOL)', link: '/features/wol' },
            { text: '解析服务 (DNSMasq)', link: '/features/dnsmasq' },
            { text: '计划任务', link: '/features/cron' },
            { text: '网络存储', link: '/features/storage' },
            { text: 'IP 地址库', link: '/features/ip-database' },
          ]
        },
        {
          text: '回调系统',
          collapsed: true,
          items: [
            { text: '回调任务与账号', link: '/features/callback' },
          ]
        },
      ],
      '/features/': [
        {
          text: '网络穿透与组网',
          collapsed: false,
          items: [
            { text: '端口转发', link: '/features/port-forward' },
            { text: 'STUN 内网穿透', link: '/features/stun' },
            { text: 'FRP 客户端', link: '/features/frp-client' },
            { text: 'FRP 服务端', link: '/features/frp-server' },
            { text: 'EasyTier 客户端', link: '/features/easytier-client' },
            { text: 'EasyTier 服务端', link: '/features/easytier-server' },
            { text: 'NPS', link: '/features/nps' },
          ]
        },
        {
          text: '域名与证书',
          collapsed: false,
          items: [
            { text: '动态域名 (DDNS)', link: '/features/ddns' },
            { text: '域名账号', link: '/features/domain-account' },
            { text: '域名解析', link: '/features/dns-records' },
            { text: '域名证书', link: '/features/ssl-cert' },
          ]
        },
        {
          text: '网站与安全',
          collapsed: false,
          items: [
            { text: '网站服务 (Caddy)', link: '/features/caddy' },
            { text: '网络防护 (WAF)', link: '/features/waf' },
            { text: '访问控制', link: '/features/access-control' },
            { text: '防火墙', link: '/features/firewall' },
          ]
        },
        {
          text: '辅助功能',
          collapsed: false,
          items: [
            { text: '网络唤醒 (WOL)', link: '/features/wol' },
            { text: '解析服务 (DNSMasq)', link: '/features/dnsmasq' },
            { text: '计划任务', link: '/features/cron' },
            { text: '网络存储', link: '/features/storage' },
            { text: 'IP 地址库', link: '/features/ip-database' },
          ]
        },
        {
          text: '回调系统',
          collapsed: false,
          items: [
            { text: '回调任务与账号', link: '/features/callback' },
          ]
        },
      ]
    },

    // 内置全文搜索
    search: {
      provider: 'local',
      options: {
        locales: {
          root: {
            translations: {
              button: {
                buttonText: '搜索文档',
                buttonAriaLabel: '搜索文档'
              },
              modal: {
                noResultsText: '无法找到相关结果',
                resetButtonTitle: '清除查询条件',
                footer: {
                  selectText: '选择',
                  navigateText: '切换',
                  closeText: '关闭'
                }
              }
            }
          }
        }
      }
    },

    // 社交链接
    socialLinks: [
      { icon: 'github', link: 'https://github.com/PIKACHUIM/NetPanel' }
    ],

    // 页脚
    footer: {
      message: '基于 GPL-3.0 许可证发布',
      copyright: 'Copyright © 2024 NetPanel'
    },

    // 编辑链接
    editLink: {
      pattern: 'https://github.com/PIKACHUIM/NetPanel/edit/main/docs-site/:path',
      text: '在 GitHub 上编辑此页'
    },

    // 文档页脚导航
    docFooter: {
      prev: '上一页',
      next: '下一页'
    },

    // 大纲
    outline: {
      label: '页面导航',
      level: [2, 3]
    },

    // 最后更新时间
    lastUpdated: {
      text: '最后更新于',
      formatOptions: {
        dateStyle: 'short',
        timeStyle: 'medium'
      }
    },

    // 返回顶部
    returnToTopLabel: '回到顶部',

    // 侧边栏菜单
    sidebarMenuLabel: '菜单',

    // 深色模式切换
    darkModeSwitchLabel: '主题',
    lightModeSwitchTitle: '切换到浅色模式',
    darkModeSwitchTitle: '切换到深色模式',
  },

  lastUpdated: true,

  markdown: {
    lineNumbers: true,
    theme: {
      light: 'github-light',
      dark: 'github-dark'
    }
  }
})
