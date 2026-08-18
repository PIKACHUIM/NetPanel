import React from 'react'
import { Card, Col, Row, Typography, Collapse, Tag, Space } from 'antd'
import {
  RocketOutlined, CloudOutlined, SafetyOutlined, LinkOutlined,
  GlobalOutlined, TeamOutlined, QuestionCircleOutlined,
} from '@ant-design/icons'

const { Title, Text, Paragraph } = Typography

const helpItems = [
  { icon: <RocketOutlined style={{ color: '#0071e3' }} />, title: '快速上手', desc: '从安装到接入公网的完整流程，5 分钟跑通 NetPanel 核心功能。' },
  { icon: <CloudOutlined style={{ color: '#ff9f0a' }} />, title: 'Cloudflare 隧道', desc: 'Quick / Named 两种隧道的创建、配置与常见报错排查。' },
  { icon: <SafetyOutlined style={{ color: '#34c759' }} />, title: '安全中心', desc: 'WAF 规则、封禁策略、黑白名单与拦截页面的配置说明。' },
  { icon: <LinkOutlined style={{ color: '#bf5af2' }} />, title: 'NPS / FRP / EasyTier', desc: '各穿透协议的服务端部署、客户端接入与性能对比。' },
  { icon: <GlobalOutlined style={{ color: '#0a84ff' }} />, title: 'DDNS 与域名解析', desc: '绑定 Cloudflare / DNSPod 等解析商，动态 IP 自动更新。' },
  { icon: <TeamOutlined style={{ color: '#f472b6' }} />, title: '账号与权限', desc: '多管理员协作、最后管理员保护、改名与密码重置规则。' },
]

const faqs = [
  { q: '忘记管理员密码怎么办？', a: '登录页点击「忘记密码」，由其他管理员在「用户与权限」中为该账号重置密码；若无其他管理员，可联系部署者重置。' },
  { q: '为什么不能删除最后一名管理员？', a: '系统必须保留至少一名启用的管理员，避免管理面板完全失控。当只剩最后一名管理员时，删除按钮会自动禁用。' },
  { q: '快速隧道重启后会失效吗？', a: '会。Quick 隧道使用随机 trycloudflare.com 域名，进程退出后地址失效。如需长期稳定服务，请使用命名隧道 Named。' },
  { q: '安全中心如何与反向代理联动拦截？', a: '安全中心封禁的 IP 会自动写入系统防火墙（iptables/nftables/ufw/firewalld/Windows）生成 deny 规则，实现真实网络层拦截。' },
  { q: '如何切换界面主题模式？', a: '点击顶栏右侧的太阳/月亮图标即可在亮色与暗色主题间切换，选择会自动保存。' },
]

const Help: React.FC = () => {
  return (
    <div style={{ padding: 4 }}>
      <Title level={4} style={{ marginBottom: 2 }}>帮助与说明</Title>
      <Text type="secondary">功能指引 · 常见问题 · 使用文档</Text>

      <Row gutter={[14, 14]} style={{ marginTop: 18 }}>
        {helpItems.map(item => (
          <Col xs={24} sm={12} lg={8} key={item.title}>
            <Card size="small" hoverable style={{ borderRadius: 'var(--radius-md)', height: '100%' }}>
              <div style={{ fontSize: 20, marginBottom: 8 }}>{item.icon}</div>
              <b style={{ fontSize: 14 }}>{item.title}</b>
              <Paragraph type="secondary" style={{ fontSize: 12, marginTop: 6, marginBottom: 0 }}>{item.desc}</Paragraph>
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
