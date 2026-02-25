import React, { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Form, Input, Button, Typography, message } from 'antd'
import { UserOutlined, LockOutlined, WifiOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { useAppStore } from '../store/appStore'
import request from '../api/request'

const { Title, Text } = Typography

// 动态粒子背景
const ParticleBackground: React.FC = () => (
  <div style={{ position: 'absolute', inset: 0, overflow: 'hidden', pointerEvents: 'none' }}>
    {/* 网格线 */}
    <div style={{
      position: 'absolute', inset: 0,
      backgroundImage: `
        linear-gradient(rgba(22,119,255,0.06) 1px, transparent 1px),
        linear-gradient(90deg, rgba(22,119,255,0.06) 1px, transparent 1px)
      `,
      backgroundSize: '64px 64px',
    }} />
    {/* 光晕 */}
    {[
      { w: 500, h: 500, top: '-10%', left: '-5%', color: 'rgba(22,119,255,0.15)' },
      { w: 400, h: 400, bottom: '-5%', right: '-5%', color: 'rgba(9,88,217,0.12)' },
      { w: 300, h: 300, top: '40%', right: '20%', color: 'rgba(82,196,26,0.06)' },
    ].map((g, i) => (
      <div key={i} style={{
        position: 'absolute',
        width: g.w, height: g.h,
        borderRadius: '50%',
        background: `radial-gradient(circle, ${g.color} 0%, transparent 70%)`,
        top: g.top, left: g.left, bottom: g.bottom, right: g.right,
      }} />
    ))}
    {/* 装饰圆圈 */}
    {[
      { size: 200, top: '15%', right: '10%', opacity: 0.04 },
      { size: 120, bottom: '20%', left: '8%', opacity: 0.06 },
      { size: 80, top: '60%', right: '30%', opacity: 0.05 },
    ].map((c, i) => (
      <div key={i} style={{
        position: 'absolute',
        width: c.size, height: c.size,
        borderRadius: '50%',
        border: `1px solid rgba(22,119,255,${c.opacity * 5})`,
        top: c.top, right: c.right, bottom: c.bottom, left: c.left,
        opacity: c.opacity * 10,
      }} />
    ))}
  </div>
)

const LoginPage: React.FC = () => {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { setToken, setUsername } = useAppStore()
  const [loading, setLoading] = useState(false)

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

  return (
    <div style={{
      minHeight: '100vh',
      background: 'linear-gradient(160deg, #060d1f 0%, #0a1628 40%, #0d1f3c 100%)',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      position: 'relative',
      overflow: 'hidden',
      fontFamily: "'DM Sans', -apple-system, sans-serif",
    }}>
      <ParticleBackground />

      {/* 登录卡片 */}
      <div style={{
        width: 420,
        background: 'rgba(255,255,255,0.03)',
        backdropFilter: 'blur(24px)',
        WebkitBackdropFilter: 'blur(24px)',
        border: '1px solid rgba(255,255,255,0.08)',
        borderRadius: 20,
        boxShadow: '0 32px 80px rgba(0,0,0,0.5), inset 0 1px 0 rgba(255,255,255,0.1)',
        padding: '48px 40px',
        position: 'relative',
        zIndex: 1,
      }}>
        {/* 顶部高光线 */}
        <div style={{
          position: 'absolute', top: 0, left: '20%', right: '20%', height: 1,
          background: 'linear-gradient(90deg, transparent, rgba(22,119,255,0.6), transparent)',
          borderRadius: 1,
        }} />

        {/* Logo */}
        <div style={{ textAlign: 'center', marginBottom: 40 }}>
          <div style={{
            width: 64, height: 64, borderRadius: 18,
            background: 'linear-gradient(135deg, #1677ff 0%, #0958d9 100%)',
            display: 'inline-flex', alignItems: 'center', justifyContent: 'center',
            marginBottom: 20,
            boxShadow: '0 12px 32px rgba(22,119,255,0.45), 0 0 0 1px rgba(22,119,255,0.3)',
          }}>
            <WifiOutlined style={{ color: '#fff', fontSize: 28 }} />
          </div>
          <Title level={2} style={{
            color: '#fff', margin: 0, fontWeight: 700,
            fontSize: 26, letterSpacing: '-0.5px',
          }}>
            NetPanel
          </Title>
          <Text style={{
            color: 'rgba(255,255,255,0.4)', fontSize: 13,
            marginTop: 6, display: 'block', letterSpacing: '0.3px',
          }}>
            {t('login.subtitle')}
          </Text>
        </div>

        <Form name="login" onFinish={onFinish} size="large" autoComplete="off">
          <Form.Item
            name="username"
            rules={[{ required: true, message: `请输入${t('login.username')}` }]}
            style={{ marginBottom: 16 }}
          >
            <Input
              prefix={<UserOutlined style={{ color: 'rgba(255,255,255,0.25)', marginRight: 4 }} />}
              placeholder={t('login.username')}
              style={{
                background: 'rgba(255,255,255,0.05)',
                border: '1px solid rgba(255,255,255,0.1)',
                borderRadius: 10, color: '#fff', height: 48,
                fontSize: 14,
              }}
            />
          </Form.Item>

          <Form.Item
            name="password"
            rules={[{ required: true, message: `请输入${t('login.password')}` }]}
            style={{ marginBottom: 28 }}
          >
            <Input.Password
              prefix={<LockOutlined style={{ color: 'rgba(255,255,255,0.25)', marginRight: 4 }} />}
              placeholder={t('login.password')}
              style={{
                background: 'rgba(255,255,255,0.05)',
                border: '1px solid rgba(255,255,255,0.1)',
                borderRadius: 10, color: '#fff', height: 48,
                fontSize: 14,
              }}
            />
          </Form.Item>

          <Button
            type="primary"
            htmlType="submit"
            loading={loading}
            block
            style={{
              height: 48, borderRadius: 10, fontSize: 15, fontWeight: 600,
              background: 'linear-gradient(135deg, #1677ff 0%, #0958d9 100%)',
              border: 'none',
              boxShadow: '0 6px 20px rgba(22,119,255,0.45)',
              letterSpacing: '0.3px',
            }}
          >
            {t('login.login')}
          </Button>
        </Form>

        {/* 底部版权 */}
        <div style={{ textAlign: 'center', marginTop: 28 }}>
          <Text style={{ color: 'rgba(255,255,255,0.2)', fontSize: 12 }}>
            NetPanel · Network Management Platform
          </Text>
        </div>
      </div>
    </div>
  )
}

export default LoginPage

