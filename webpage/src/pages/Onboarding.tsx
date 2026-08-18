import React, { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Card, Row, Col, Typography, Button, Steps, message } from 'antd'
import { CloudOutlined, SafetyOutlined, QuestionCircleOutlined, CheckCircleFilled } from '@ant-design/icons'

const { Title, Text, Paragraph } = Typography

// 登录后新手指引（首次进入展示，可跳过）
const Onboarding: React.FC = () => {
  const navigate = useNavigate()
  const [step, setStep] = useState(0)

  useEffect(() => {
    // 标记已看过引导
    try { localStorage.setItem('netpanel-onboarded', '1') } catch { /* ignore */ }
  }, [])

  const finish = () => {
    message.success('欢迎使用 NetPanel！')
    navigate('/dashboard', { replace: true })
  }

  const cards = [
    {
      icon: <CloudOutlined style={{ fontSize: 26, color: '#0071e3' }} />,
      title: '接入公网（推荐先做）',
      desc: '使用 CF 隧道将内网服务安全暴露到公网，无需公网 IP 与端口映射。',
      action: () => navigate('/cftunnel', { replace: true }),
      actionText: '开始配置 →',
    },
    {
      icon: <SafetyOutlined style={{ fontSize: 26, color: '#34c759' }} />,
      title: '开启安全防护',
      desc: '启用 WAF 安全中心，自动拦截 SQL 注入、暴力破解等攻击，自动封禁恶意 IP。',
      action: () => navigate('/security/waf', { replace: true }),
      actionText: '立即开启 →',
    },
    {
      icon: <QuestionCircleOutlined style={{ fontSize: 26, color: '#bf5af2' }} />,
      title: '查看使用说明',
      desc: '每个功能页都内置说明文档与常见问题，随时可在「帮助与说明」查阅。',
      action: () => navigate('/help', { replace: true }),
      actionText: '查看帮助 →',
    },
  ]

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
          <Text type="secondary">已就绪 · 花 30 秒完成基础配置</Text>
          <div style={{ maxWidth: 320, margin: '20px auto 0' }}>
            <Steps size="small" current={step} items={[{ title: '入门' }, { title: '进阶' }, { title: '完成' }]} />
          </div>
        </div>

        <Row gutter={[16, 16]}>
          {cards.map((c, i) => (
            <Col xs={24} md={8} key={c.title}>
              <Card hoverable size="small" style={{ borderRadius: 'var(--radius-md)', height: '100%' }}
                onClick={() => { setStep(i + 1); c.action() }}>
                <div style={{ marginBottom: 12 }}>{c.icon}</div>
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
