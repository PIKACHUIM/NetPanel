import React, { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Card, Row, Col, Typography, Button, Steps, message, Tag, Spin } from 'antd'
import {
  CloudOutlined,
  SafetyOutlined,
  QuestionCircleOutlined,
  CheckCircleFilled,
  GlobalOutlined,
  ApiOutlined,
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

// 登录后新手指引（首次进入展示，可跳过）
// 基于网络环境探测结果，推荐最合适的使用场景
const Onboarding: React.FC = () => {
  const navigate = useNavigate()
  const [step, setStep] = useState(0)
  const [loading, setLoading] = useState(true)
  const [netInfo, setNetInfo] = useState<NetworkInfo | null>(null)

  useEffect(() => {
    // 标记已看过引导
    try { localStorage.setItem('netpanel-onboarded', '1') } catch { /* ignore */ }

    initApi.networkInfo()
      .then((res: any) => setNetInfo(res?.data))
      .catch(() => {})
      .finally(() => setLoading(false))
  }, [])

  const finish = () => {
    message.success('欢迎使用 NetPanel！')
    navigate('/dashboard', { replace: true })
  }

  // 根据网络环境推荐场景卡片
  const buildCards = () => {
    const cards: Array<{
      icon: React.ReactNode
      title: string
      desc: string
      tag?: string
      action: () => void
      actionText: string
    }> = []

    if (!netInfo) {
      // 加载失败时显示默认推荐
      cards.push({
        icon: <CloudOutlined style={{ fontSize: 26, color: '#0071e3' }} />,
        title: '接入公网（推荐先做）',
        desc: '使用 CF 隧道将内网服务安全暴露到公网，无需公网 IP 与端口映射。',
        action: () => navigate('/cftunnel', { replace: true }),
        actionText: '开始配置 →',
      })
    } else {
      // 有公网 IP：推荐端口转发 + FRP
      if (netInfo.has_ipv4) {
        cards.push({
          icon: <GlobalOutlined style={{ fontSize: 26, color: '#0071e3' }} />,
          title: '端口转发',
          tag: '有公网 IP',
          desc: `检测到公网 IPv4：${netInfo.public_ip_v4}。可直接配置端口转发，将内网服务映射到公网。`,
          action: () => navigate('/port-forward', { replace: true }),
          actionText: '配置端口转发 →',
        })
      }

      // 无公网 IPv4：推荐 CF 隧道（免费免配置）
      if (!netInfo.has_ipv4) {
        cards.push({
          icon: <CloudOutlined style={{ fontSize: 26, color: '#0071e3' }} />,
          title: 'CF 隧道（免公网 IP）',
          tag: '推荐',
          desc: '未检测到公网 IPv4，推荐使用 Cloudflare Tunnel，免费将内网服务安全暴露到公网。',
          action: () => navigate('/cftunnel', { replace: true }),
          actionText: '配置 CF 隧道 →',
        })
      }

      // 有 TUN：推荐组网
      if (netInfo.tun_capable) {
        cards.push({
          icon: <ApiOutlined style={{ fontSize: 26, color: '#5e5ce6' }} />,
          title: '异地组网',
          tag: 'TUN 可用',
          desc: 'TUN 设备可用，可使用 EasyTier 或 WireGuard 将多台设备组成虚拟局域网。',
          action: () => navigate('/easytier/client', { replace: true }),
          actionText: '配置组网 →',
        })
      } else {
        cards.push({
          icon: <ToolOutlined style={{ fontSize: 26, color: '#ff9500' }} />,
          title: '异地组网（需配置）',
          tag: 'TUN 不可用',
          desc: '当前环境不支持 TUN 设备（Docker 需添加 --device /dev/net/tun），组网功能暂不可用。',
          action: () => navigate('/help', { replace: true }),
          actionText: '查看部署说明 →',
        })
      }
    }

    // 通用卡片
    cards.push({
      icon: <SafetyOutlined style={{ fontSize: 26, color: '#34c759' }} />,
      title: '开启安全防护',
      desc: '启用 WAF 安全中心，自动拦截 SQL 注入、暴力破解等攻击，自动封禁恶意 IP。',
      action: () => navigate('/security/waf', { replace: true }),
      actionText: '立即开启 →',
    })

    cards.push({
      icon: <QuestionCircleOutlined style={{ fontSize: 26, color: '#bf5af2' }} />,
      title: '查看使用说明',
      desc: '每个功能页都内置说明文档与常见问题，随时可在「帮助与说明」查阅。',
      action: () => navigate('/help', { replace: true }),
      actionText: '查看帮助 →',
    })

    return cards
  }

  if (loading) {
    return (
      <div style={{ minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
        <Spin size="large" tip="正在检测网络环境..." />
      </div>
    )
  }

  const cards = buildCards()

  return (
    <div style={{
      minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center', padding: 24,
      background: 'var(--bg-body)',
    }}>
      <div style={{ width: 860, maxWidth: '100%' }}>
        <div style={{ textAlign: 'center', marginBottom: 26 }}>
          <div style={{
            width: 60, height: 60, borderRadius: 18, margin: '0 auto 16px',
            background: 'linear-gradient(135deg, #0071e3, #5e5ce6)',
            display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: 28, color: '#fff', fontWeight: 700,
          }}>N</div>
          <Title level={3} style={{ marginBottom: 4 }}>欢迎使用 NetPanel</Title>
          <Text type="secondary">
            {netInfo?.has_ipv4
              ? `已检测到公网 IP · ${netInfo.public_ip_v4}`
              : '正在为你推荐最佳使用方式'}
          </Text>
          <div style={{ maxWidth: 320, margin: '20px auto 0' }}>
            <Steps size="small" current={step} items={[{ title: '入门' }, { title: '进阶' }, { title: '完成' }]} />
          </div>
        </div>

        <Row gutter={[16, 16]}>
          {cards.map((c, i) => (
            <Col xs={24} md={8} key={c.title}>
              <Card hoverable size="small" style={{ borderRadius: 'var(--radius-md)', height: '100%' }}
                onClick={() => { setStep(Math.min(i + 1, 2)); c.action() }}>
                <div style={{ marginBottom: 12 }}>
                  {c.icon}
                  {c.tag && <Tag color={c.tag === '推荐' ? 'blue' : c.tag.includes('不可用') ? 'orange' : 'green'} style={{ marginLeft: 8, fontSize: 11 }}>{c.tag}</Tag>}
                </div>
                <b>{c.title}</b>
                <Paragraph type="secondary" style={{ fontSize: 12.5, marginTop: 8, marginBottom: 10 }}>{c.desc}</Paragraph>
                <span style={{ color: '#0071e3', fontSize: 13, fontWeight: 600 }}>{c.actionText}</span>
              </Card>
            </Col>
          ))}
        </Row>

        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginTop: 24 }}>
          <Button type="text" onClick={finish}>跳过引导</Button>
          <Button type="primary" icon={<CheckCircleFilled />} onClick={finish}>完成，进入仪表盘</Button>
        </div>
      </div>
    </div>
  )
}

export default Onboarding
