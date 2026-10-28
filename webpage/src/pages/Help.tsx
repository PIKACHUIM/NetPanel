import React from 'react'
import { useNavigate } from 'react-router-dom'
import { Button, Card, Col, Row, Typography, Collapse, Tag, Space } from 'antd'
import {
  RocketOutlined, CloudOutlined, SafetyOutlined, LinkOutlined,
  GlobalOutlined, TeamOutlined, QuestionCircleOutlined,
} from '@ant-design/icons'

const { Title, Text, Paragraph } = Typography

const helpItems = [
  { icon: <RocketOutlined style={{ color: '#0071e3' }} />, title: '快速上手', desc: '根据当前网络条件选择接入方式，再逐步完成配置与验证。', path: '/onboarding', action: '打开引导' },
  { icon: <CloudOutlined style={{ color: '#ff9f0a' }} />, title: 'Cloudflare 隧道', desc: '管理免端口映射的隧道，并检查域名与访问规则。', path: '/cftunnel', action: '管理隧道' },
  { icon: <SafetyOutlined style={{ color: '#34c759' }} />, title: '安全中心', desc: '查看 WAF 规则、封禁策略、黑白名单与拦截状态。', path: '/security/waf', action: '查看安全状态' },
  { icon: <LinkOutlined style={{ color: '#bf5af2' }} />, title: '穿透与组网', desc: '按部署条件管理 FRP、NPS、EasyTier 与 WireGuard。', path: '/frp/client', action: '查看连接' },
  { icon: <GlobalOutlined style={{ color: '#0a84ff' }} />, title: 'DDNS 与域名解析', desc: '管理动态域名、解析账号与证书生命周期。', path: '/ddns', action: '管理域名服务' },
  { icon: <TeamOutlined style={{ color: '#f472b6' }} />, title: '账号与权限', desc: '管理用户、角色与 OAuth 登录配置。', path: '/admin/users', action: '管理用户' },
]

const faqs = [
  { q: '忘记管理员密码怎么办？', a: '登录页点击「忘记密码」，由其他管理员在「用户与权限」中为该账号重置密码；若无其他管理员，可联系部署者重置。' },
  { q: '为什么不能删除最后一名管理员？', a: '系统必须保留至少一名启用的管理员，避免管理面板完全失控。当只剩最后一名管理员时，删除按钮会自动禁用。' },
  { q: '快速隧道重启后会失效吗？', a: '会。Quick 隧道使用随机 trycloudflare.com 域名，进程退出后地址失效。如需长期稳定服务，请使用命名隧道 Named。' },
  { q: '安全中心如何与反向代理联动拦截？', a: '安全中心封禁的 IP 会自动写入系统防火墙（iptables/nftables/ufw/firewalld/Windows）生成 deny 规则，实现真实网络层拦截。' },
  { q: '如何切换界面主题模式？', a: '点击顶栏右侧的太阳/月亮图标即可在亮色与暗色主题间切换，选择会自动保存。' },
]

const Help: React.FC = () => {
  const navigate = useNavigate()

  return (
    <div style={{ padding: 4 }}>
      <Title level={4} style={{ marginBottom: 2 }}>帮助与说明</Title>
      <Text type="secondary">功能指引 · 常见问题 · 使用文档</Text>

      <Row gutter={[14, 14]} style={{ marginTop: 18 }}>
        {helpItems.map(item => (
          <Col xs={24} sm={12} lg={8} key={item.title}>
            <Card size="small" style={{ borderRadius: 'var(--radius-md)', height: '100%' }}>
              <div style={{ fontSize: 20, marginBottom: 8 }}>{item.icon}</div>
              <b style={{ fontSize: 14 }}>{item.title}</b>
              <Paragraph type="secondary" style={{ fontSize: 12, marginTop: 6, marginBottom: 8 }}>{item.desc}</Paragraph>
              <Button type="link" size="small" onClick={() => navigate(item.path)} style={{ paddingInline: 0 }}>
                {item.action} →
              </Button>
            </Card>
          </Col>
        ))}
      </Row>

      <Card size="small" title={<Space><QuestionCircleOutlined style={{ color: '#0071e3' }} />常见问题</Space>}
        style={{ marginTop: 16, borderRadius: 'var(--radius-md)' }}>
        <Collapse
          ghost
          items={faqs.map((f, i) => ({
            key: i,
            label: <Space><Tag color="blue">Q{i + 1}</Tag><span>{f.q}</span></Space>,
            children: <Paragraph style={{ marginBottom: 0, fontSize: 13 }}>{f.a}</Paragraph>,
          }))}
        />
      </Card>
    </div>
  )
}

export default Help
