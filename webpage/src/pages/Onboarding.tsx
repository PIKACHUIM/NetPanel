import React, { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Alert, Button, Card, Col, Row, Spin, Tag, Typography, message } from 'antd'
import {
  ApiOutlined,
  CheckCircleFilled,
  CloudOutlined,
  GlobalOutlined,
  LinkOutlined,
  SafetyOutlined,
  ToolOutlined,
} from '@ant-design/icons'
import { initApi } from '../api'

const { Title, Text, Paragraph } = Typography

interface NetworkInfo {
  public_ip_v4: string
  public_ip_v6: string
  has_ipv4: boolean
  has_ipv6: boolean
  tun_capable: boolean
}

interface GuideCard {
  number: string
  category: string
  title: string
  description: string
  action: string
  path: string
  icon: React.ReactNode
  accent: string
  tag?: string
  recommended?: boolean
}

const Onboarding: React.FC = () => {
  const navigate = useNavigate()
  const [loading, setLoading] = useState(true)
  const [networkError, setNetworkError] = useState(false)
  const [netInfo, setNetInfo] = useState<NetworkInfo | null>(null)

  const checkNetwork = () => {
    setLoading(true)
    setNetworkError(false)
    setNetInfo(null)
    initApi.networkInfo()
      .then((res: any) => {
        if (res?.data) setNetInfo(res.data)
        else setNetworkError(true)
      })
      .catch(() => setNetworkError(true))
      .finally(() => setLoading(false))
  }

  useEffect(() => {
    try { localStorage.setItem('netpanel-onboarded', '1') } catch { /* ignore */ }
    checkNetwork()
  }, [])

  const finish = () => {
    message.success('欢迎使用 NetPanel！')
    navigate('/dashboard', { replace: true })
  }

  const hasPublicIPv4 = Boolean(netInfo?.has_ipv4)
  const hasTun = Boolean(netInfo?.tun_capable)
  const cards: GuideCard[] = [
    {
      number: '01',
      category: '公网访问',
      title: '端口转发',
      description: hasPublicIPv4
        ? `检测到公网 IPv4 ${netInfo?.public_ip_v4 || ''}。将内网端口映射到公网前，请确认路由器转发与防火墙规则。`
        : '适用于拥有公网地址且可配置路由器的环境。当前未检测到公网 IPv4，可先了解条件，或选择 Cloudflare Tunnel。',
      action: hasPublicIPv4 ? '配置端口转发' : '查看接入方式',
      path: hasPublicIPv4 ? '/port-forward' : '/help',
      icon: <GlobalOutlined />,
      accent: '#1687ff',
      tag: networkError ? '检测未完成' : hasPublicIPv4 ? '检测到公网 IPv4' : '需要公网地址',
      recommended: hasPublicIPv4,
    },
    {
      number: '02',
      category: '公网访问',
      title: 'Cloudflare Tunnel',
      description: hasPublicIPv4
        ? '无需在路由器开放入站端口；适合已有 Cloudflare 账号并希望通过域名发布服务的场景。'
        : '未检测到公网 IPv4 时的推荐选择。通过 Cloudflare 建立出站隧道，减少路由器端口配置。',
      action: '配置 Cloudflare Tunnel',
      path: '/cftunnel',
      icon: <CloudOutlined />,
      accent: '#f0a12b',
      tag: !hasPublicIPv4 ? '推荐方式' : '免端口映射',
      recommended: !hasPublicIPv4,
    },
    {
      number: '03',
      category: '远程穿透',
      title: 'FRP 客户端',
      description: '连接到已部署的 FRP 服务端，转发 TCP、UDP 或 HTTP 服务。开始前需要准备服务端地址与认证信息。',
      action: '配置 FRP 客户端',
      path: '/frp/client',
      icon: <LinkOutlined />,
      accent: '#7357e8',
      tag: '需要服务端',
    },
    {
      number: '04',
      category: '私有网络',
      title: '异地组网',
      description: hasTun
        ? 'TUN 设备可用。使用 EasyTier 将多台设备接入同一虚拟网络，适合远程访问内部设备。'
        : '当前未检测到可用 TUN 设备。Docker 部署通常需要映射 /dev/net/tun 并授予相应权限。',
      action: hasTun ? '配置 EasyTier' : '查看部署说明',
      path: hasTun ? '/easytier/client' : '/help',
      icon: hasTun ? <ApiOutlined /> : <ToolOutlined />,
      accent: hasTun ? '#16a87a' : '#d58b18',
      tag: hasTun ? 'TUN 可用' : '需检查环境',
    },
    {
      number: '05',
      category: '域名与证书',
      title: '配置域名与 HTTPS',
      description: '添加域名、DNS 账号与证书，为通过反向代理或隧道发布的服务启用可信 HTTPS 访问。',
      action: '管理域名证书',
      path: '/domain/cert',
      icon: <GlobalOutlined />,
      accent: '#1687ff',
      tag: '发布服务后配置',
    },
    {
      number: '06',
      category: '访问安全',
      title: '检查安全防护',
      description: '查看 WAF 与访问规则，根据实际发布的服务配置防护策略；先确认规则范围，再启用拦截。',
      action: '打开安全中心',
      path: '/security/waf',
      icon: <SafetyOutlined />,
      accent: '#17a66a',
      tag: '建议配置',
    },
  ]

  if (loading) {
    return (
      <main style={{ minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
        <Spin size="large" tip="正在检测网络环境..." />
      </main>
    )
  }

  const networkSummary = networkError
    ? '检测服务暂不可用 · 以下为通用配置入口'
    : hasPublicIPv4
      ? `公网 IPv4 ${netInfo?.public_ip_v4 || '已检测'}${hasTun ? ' · TUN 可用' : ' · TUN 不可用'}`
      : `未检测到公网 IPv4${hasTun ? ' · TUN 可用' : ' · TUN 不可用'}`

  return (
    <main style={{
      minHeight: '100vh',
      padding: 'clamp(24px, 5vw, 64px) 24px 36px',
      background: 'radial-gradient(ellipse at 12% 0%, rgba(22, 135, 255, 0.10), transparent 38%), var(--bg-body)',
    }}>
      <div style={{ width: 'min(1120px, 100%)', margin: '0 auto' }}>
        <header style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 20, marginBottom: 42 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
            <div style={{
              width: 42, height: 42, borderRadius: 13, display: 'grid', placeItems: 'center',
              color: '#fff', fontSize: 21, fontWeight: 700,
              background: 'linear-gradient(145deg, #1687ff, #4f59dc)',
              boxShadow: '0 8px 24px rgba(22, 135, 255, 0.25)',
            }}>N</div>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
              <Text strong style={{ color: 'var(--text-primary)', letterSpacing: '.02em' }}>NETPANEL</Text>
              <Text type="secondary" style={{ fontSize: 11, letterSpacing: '.12em' }}>SELF-HOSTED NETWORK CONSOLE</Text>
            </div>
          </div>
          <Button type="text" onClick={finish}>跳过引导</Button>
        </header>

        <section style={{ marginBottom: 30 }}>
          <Text style={{ color: '#1687ff', fontSize: 12, fontWeight: 700, letterSpacing: '.14em' }}>从这里开始 · GETTING STARTED</Text>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-end', gap: 28, marginTop: 10 }}>
            <div style={{ maxWidth: 660 }}>
              <Title level={1} style={{ margin: '0 0 10px', fontSize: 'clamp(28px, 4vw, 40px)', letterSpacing: '-.035em' }}>
                欢迎使用 NetPanel
              </Title>
              <Paragraph type="secondary" style={{ margin: 0, maxWidth: 610, fontSize: 15, lineHeight: 1.8 }}>
                先选一种符合当前网络条件的连接方式，再配置域名和访问防护。高级协议与运维工具可随时从主导航进入。
              </Paragraph>
            </div>
            <div style={{
              flex: '0 0 auto', padding: '12px 16px', border: '1px solid var(--border-color)',
              borderRadius: 14, background: 'var(--bg-card)', minWidth: 240,
            }}>
              <Text type="secondary" style={{ display: 'block', fontSize: 11, marginBottom: 4 }}>网络环境检测</Text>
              <Text strong style={{ fontSize: 13 }}>{networkSummary}</Text>
            </div>
          </div>
          {networkError && (
            <Alert
              showIcon
              type="warning"
              style={{ marginTop: 18, borderRadius: 12 }}
              message="未能完成网络检测"
              description={<span>可先浏览通用入口，或 <Button type="link" size="small" onClick={checkNetwork} style={{ paddingInline: 4 }}>重新检测</Button> 后查看推荐。</span>}
            />
          )}
        </section>

        <section aria-label="NetPanel 六项快速入口">
          <Row gutter={[16, 16]}>
            {cards.map(card => (
              <Col xs={24} sm={12} lg={8} key={card.number}>
                <Card
                  hoverable
                  style={{
                    height: '100%', minHeight: 230, borderRadius: 16,
                    borderColor: card.recommended ? 'rgba(22, 135, 255, .38)' : 'var(--border-color)',
                    background: 'var(--bg-card)',
                    boxShadow: card.recommended ? '0 10px 30px rgba(22, 135, 255, .08)' : 'var(--shadow-card)',
                  }}
                  styles={{ body: { height: '100%', padding: 20, display: 'flex', flexDirection: 'column' } }}
                >
                  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 22 }}>
                    <div style={{
                      width: 42, height: 42, borderRadius: 13, display: 'grid', placeItems: 'center',
                      color: card.accent, fontSize: 19,
                      background: `color-mix(in srgb, ${card.accent} 12%, transparent)`,
                    }}>{card.icon}</div>
                    <Text type="secondary" style={{ fontSize: 12, letterSpacing: '.08em', fontVariantNumeric: 'tabular-nums' }}>{card.number}</Text>
                  </div>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap', marginBottom: 8 }}>
                    <Text type="secondary" style={{ fontSize: 11, letterSpacing: '.06em' }}>{card.category}</Text>
                    {card.tag && <Tag color={card.recommended ? 'blue' : 'default'} style={{ marginInlineEnd: 0, borderRadius: 99, fontSize: 11 }}>{card.tag}</Tag>}
                  </div>
                  <Title level={4} style={{ margin: '0 0 8px', fontSize: 17 }}>{card.title}</Title>
                  <Paragraph type="secondary" style={{ flex: 1, margin: '0 0 16px', fontSize: 13, lineHeight: 1.7 }}>
                    {card.description}
                  </Paragraph>
                  <Button
                    type={card.recommended ? 'primary' : 'link'}
                    onClick={() => navigate(card.path)}
                    style={card.recommended ? { alignSelf: 'flex-start', borderRadius: 9 } : { alignSelf: 'flex-start', paddingInline: 0, fontWeight: 600 }}
                  >
                    {card.action} <span aria-hidden="true">→</span>
                  </Button>
                </Card>
              </Col>
            ))}
          </Row>
        </section>

        <footer style={{
          display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 16,
          marginTop: 28, paddingTop: 20, borderTop: '1px solid var(--border-color)',
        }}>
          <Button type="link" onClick={() => navigate('/help')} style={{ paddingInline: 0 }}>
            查看部署说明与常见问题 <span aria-hidden="true">↗</span>
          </Button>
          <Button type="primary" icon={<CheckCircleFilled />} onClick={finish} style={{ borderRadius: 9, height: 40, paddingInline: 18 }}>
            完成，进入仪表盘
          </Button>
        </footer>
      </div>
    </main>
  )
}

export default Onboarding
