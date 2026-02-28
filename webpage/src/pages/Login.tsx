import React, { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { Form, Input, Button, Typography, message } from 'antd'
import { UserOutlined, LockOutlined, WifiOutlined, ArrowRightOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { useAppStore } from '../store/appStore'
import request from '../api/request'

const { Title, Text } = Typography

// 动态粒子背景
const AnimatedBackground: React.FC = () => {
  return (
    <div style={{ position: 'absolute', inset: 0, overflow: 'hidden', pointerEvents: 'none' }}>
      {/* 网格线 */}
      <div style={{
        position: 'absolute', inset: 0,
        backgroundImage: `
          linear-gradient(rgba(22,119,255,0.05) 1px, transparent 1px),
          linear-gradient(90deg, rgba(22,119,255,0.05) 1px, transparent 1px)
        `,
        backgroundSize: '60px 60px',
      }} />

      {/* 主光晕 */}
      <div style={{
        position: 'absolute',
        width: 700, height: 700,
        top: '-20%', left: '-15%',
        borderRadius: '50%',
        background: 'radial-gradient(circle, rgba(22,119,255,0.18) 0%, transparent 65%)',
        animation: 'orbFloat1 22s ease-in-out infinite',
      }} />
      <div style={{
        position: 'absolute',
        width: 550, height: 550,
        bottom: '-15%', right: '-10%',
        borderRadius: '50%',
        background: 'radial-gradient(circle, rgba(9,88,217,0.14) 0%, transparent 65%)',
        animation: 'orbFloat2 28s ease-in-out infinite',
      }} />
      <div style={{
        position: 'absolute',
        width: 300, height: 300,
        top: '45%', right: '22%',
        borderRadius: '50%',
        background: 'radial-gradient(circle, rgba(82,196,26,0.07) 0%, transparent 65%)',
        animation: 'orbFloat3 18s ease-in-out infinite',
      }} />

      {/* 装饰圆环 */}
      {[
        { size: 220, top: '12%', right: '8%', opacity: 0.06 },
        { size: 140, bottom: '18%', left: '6%', opacity: 0.08 },
        { size: 90, top: '58%', right: '28%', opacity: 0.07 },
        { size: 60, top: '30%', left: '20%', opacity: 0.05 },
      ].map((c, i) => (
        <div key={i} style={{
          position: 'absolute',
          width: c.size, height: c.size,
          borderRadius: '50%',
          border: `1px solid rgba(22,119,255,${c.opacity * 4})`,
          top: c.top, right: c.right, bottom: c.bottom, left: c.left,
        }} />
      ))}

      {/* 浮动点 */}
      {[
        { size: 4, top: '20%', left: '30%', delay: '0s' },
        { size: 3, top: '65%', left: '15%', delay: '1.5s' },
        { size: 5, top: '40%', right: '15%', delay: '0.8s' },
        { size: 3, bottom: '30%', right: '35%', delay: '2s' },
        { size: 4, top: '75%', left: '55%', delay: '1.2s' },
      ].map((p, i) => (
        <div key={i} style={{
          position: 'absolute',
          width: p.size, height: p.size,
          borderRadius: '50%',
          background: 'rgba(22,119,255,0.6)',
          top: p.top, left: p.left, right: p.right, bottom: p.bottom,
          animation: `pulse 3s ease-in-out ${p.delay} infinite`,
        }} />
      ))}
    </div>
  )
}

const LoginPage: React.FC = () => {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { setToken, setUsername } = useAppStore()
  const [loading, setLoading] = useState(false)
  const [focused, setFocused] = useState<string | null>(null)

  const onFinish = async (values: { username: string; password: string }) => {
    setLoading(true)
    try {
      const res: any = await request.post('/v1/auth/login', values)
      setToken(res.data?.token)
      setUsername(values.username)
      message.success(t('login.loginSuccess'))
      navigate('/dashboard')
    } catch {
      // 错误已在拦截器处理
    } finally {
      setLoading(false)
    }
  }

  const inputStyle = (name: string) => ({
    background: focused === name ? 'rgba(22,119,255,0.08)' : 'rgba(255,255,255,0.04)',
    border: `1px solid ${focused === name ? 'rgba(22,119,255,0.5)' : 'rgba(255,255,255,0.1)'}`,
    borderRadius: 10,
    color: '#fff',
    height: 48,
    fontSize: 14,
    transition: 'all 0.2s',
    boxShadow: focused === name ? '0 0 0 3px rgba(22,119,255,0.12)' : 'none',
  })

  return (
    <div style={{
      minHeight: '100vh',
      background: 'linear-gradient(160deg, #050c1a 0%, #091525 40%, #0c1c38 70%, #080e1c 100%)',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      position: 'relative',
      overflow: 'hidden',
fontFamily: "'MapleMono', monospace",
    }}>
      <AnimatedBackground />

      {/* 登录卡片 */}
      <div style={{
        width: 420,
        background: 'rgba(255,255,255,0.03)',
        backdropFilter: 'blur(30px)',
        WebkitBackdropFilter: 'blur(30px)',
        border: '1px solid rgba(255,255,255,0.08)',
        borderRadius: 24,
        boxShadow: '0 40px 100px rgba(0,0,0,0.6), 0 0 0 1px rgba(255,255,255,0.04), inset 0 1px 0 rgba(255,255,255,0.08)',
        padding: '52px 44px',
        position: 'relative',
        zIndex: 1,
        animation: 'pageEnter 0.5s ease-out forwards',
      }}>
        {/* 顶部高光线 */}
        <div style={{
          position: 'absolute', top: 0, left: '15%', right: '15%', height: 1,
          background: 'linear-gradient(90deg, transparent, rgba(22,119,255,0.7), rgba(82,196,26,0.3), transparent)',
          borderRadius: 1,
        }} />

        {/* 角落装饰 */}
        <div style={{
          position: 'absolute', top: 20, right: 20,
          width: 60, height: 60,
          borderTop: '1px solid rgba(22,119,255,0.2)',
          borderRight: '1px solid rgba(22,119,255,0.2)',
          borderRadius: '0 8px 0 0',
        }} />
        <div style={{
          position: 'absolute', bottom: 20, left: 20,
          width: 60, height: 60,
          borderBottom: '1px solid rgba(22,119,255,0.15)',
          borderLeft: '1px solid rgba(22,119,255,0.15)',
          borderRadius: '0 0 0 8px',
        }} />

        {/* Logo */}
        <div style={{ textAlign: 'center', marginBottom: 44 }}>
          <div style={{
            width: 68, height: 68, borderRadius: 20,
            background: 'linear-gradient(135deg, #1677ff 0%, #0958d9 100%)',
            display: 'inline-flex', alignItems: 'center', justifyContent: 'center',
            marginBottom: 22,
            boxShadow: '0 16px 40px rgba(22,119,255,0.5), 0 0 0 1px rgba(22,119,255,0.3), inset 0 1px 0 rgba(255,255,255,0.2)',
            position: 'relative',
          }}>
            <WifiOutlined style={{ color: '#fff', fontSize: 30 }} />
            {/* Logo 光晕 */}
            <div style={{
              position: 'absolute', inset: -8,
              borderRadius: 28,
              background: 'radial-gradient(circle, rgba(22,119,255,0.2) 0%, transparent 70%)',
            }} />
          </div>
          <Title level={2} style={{
            color: '#fff', margin: 0, fontWeight: 700,
            fontSize: 28, letterSpacing: '-0.5px',
          }}>
            NetPanel
          </Title>
          <Text style={{
            color: 'rgba(255,255,255,0.35)', fontSize: 12,
            marginTop: 8, display: 'block', letterSpacing: '2px',
            textTransform: 'uppercase',
          }}>
            {t('login.subtitle')}
          </Text>
        </div>

        <Form name="login" onFinish={onFinish} size="large" autoComplete="off">
          <Form.Item
            name="username"
            rules={[{ required: true, message: `请输入${t('login.username')}` }]}
            style={{ marginBottom: 14 }}
          >
            <Input
              prefix={<UserOutlined style={{ color: 'rgba(255,255,255,0.3)', marginRight: 6 }} />}
              placeholder={t('login.username')}
              onFocus={() => setFocused('username')}
              onBlur={() => setFocused(null)}
              style={inputStyle('username')}
            />
          </Form.Item>

          <Form.Item
            name="password"
            rules={[{ required: true, message: `请输入${t('login.password')}` }]}
            style={{ marginBottom: 32 }}
          >
            <Input.Password
              prefix={<LockOutlined style={{ color: 'rgba(255,255,255,0.3)', marginRight: 6 }} />}
              placeholder={t('login.password')}
              onFocus={() => setFocused('password')}
              onBlur={() => setFocused(null)}
              style={inputStyle('password')}
            />
          </Form.Item>

          <Button
            type="primary"
            htmlType="submit"
            loading={loading}
            block
            icon={!loading ? <ArrowRightOutlined /> : undefined}
            style={{
              height: 50, borderRadius: 12, fontSize: 15, fontWeight: 600,
              background: 'linear-gradient(135deg, #1677ff 0%, #0958d9 100%)',
              border: 'none',
              boxShadow: '0 8px 24px rgba(22,119,255,0.5), inset 0 1px 0 rgba(255,255,255,0.15)',
              letterSpacing: '0.5px',
              display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 8,
            }}
          >
            {t('login.login')}
          </Button>
        </Form>

        {/* 底部版权 */}
        <div style={{ textAlign: 'center', marginTop: 32 }}>
          <div style={{
            width: 40, height: 1,
            background: 'rgba(255,255,255,0.1)',
            margin: '0 auto 16px',
          }} />
          <Text style={{ color: 'rgba(255,255,255,0.18)', fontSize: 11, letterSpacing: '1px' }}>
            NETPANEL · NETWORK MANAGEMENT
          </Text>
        </div>
      </div>
    </div>
  )
}

export default LoginPage