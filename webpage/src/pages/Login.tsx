import React, { useState, useEffect } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { Form, Input, Button, Typography, message, Divider, Dropdown, Space } from 'antd'
import { UserOutlined, LockOutlined, WifiOutlined, GlobalOutlined, SkinOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { useAppStore, wallpaperList, getWallpaperBg } from '../store/appStore'
import request from '../api/request'
import i18n from '../i18n'

const { Title, Text } = Typography

interface OAuthProvider {
  id: number
  name: string
  type: string
  icon: string
  display_order: number
}

// 壁纸选项（登录页使用）
const wpOptions = wallpaperList

const LoginPage: React.FC = () => {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const { token, setToken, setUsername, uiMode, setUIMode, wallpaper, setWallpaper, language, setLanguage } = useAppStore()
  const [loading, setLoading] = useState(false)
  const [providers, setProviders] = useState<OAuthProvider[]>([])
  const [visible, setVisible] = useState(false)

  // 判断是否为站点认证模式
  const redirectUrl = searchParams.get('redirect') || ''
  const isSiteAuth = redirectUrl.startsWith('http://') || redirectUrl.startsWith('https://')
  const siteHost = isSiteAuth ? (() => { try { return new URL(redirectUrl).host } catch { return redirectUrl } })() : ''

  // 主题相关
  const isDark = uiMode === 'dark'
  const animeBg = getWallpaperBg(wallpaper)

  useEffect(() => {
    // 如果用户已登录且是站点认证模式，直接跳转（cookie 已存在）
    if (token && isSiteAuth) {
      window.location.href = redirectUrl
      return
    }
    // 如果用户已登录且不是站点认证，跳转到面板
    if (token && !isSiteAuth) {
      navigate('/dashboard', { replace: true })
      return
    }
    setVisible(true)
    request.get('/v1/auth/oauth/providers').then((res: any) => {
      if (res.data) setProviders(res.data)
    }).catch(() => {})
  }, [])

  useEffect(() => { i18n.changeLanguage(language) }, [language])

  const onFinish = async (values: { username: string; password: string }) => {
    setLoading(true)
    try {
      const res: any = await request.post('/v1/auth/login', values)
      if (isSiteAuth) {
        message.success(t('login.loginSuccess'))
        window.location.href = redirectUrl
        return
      }
      setToken(res.data?.token)
      setUsername(values.username)
      message.success(t('login.loginSuccess'))
      navigate(redirectUrl || '/dashboard')
    } catch {
    } finally {
      setLoading(false)
    }
  }

  const handleOAuthLogin = (provider: OAuthProvider) => {
    window.location.href = `/api/v1/auth/oauth/${provider.name}/authorize`
  }

  // 颜色变量
  const textColor = isDark ? '#fff' : '#1a1a2e'
  const subColor = isDark ? 'rgba(255,255,255,0.5)' : 'rgba(0,0,0,0.45)'
  const inputBg = isDark ? 'rgba(255,255,255,0.06)' : 'rgba(0,0,0,0.02)'
  const inputBorder = isDark ? 'rgba(255,255,255,0.12)' : 'rgba(0,0,0,0.08)'
  const cardBg = isDark ? 'rgba(15,20,35,0.85)' : 'rgba(255,255,255,0.92)'

  const wpMenuItems = wpOptions.map(w => ({
    key: w.key,
    label: <span>{w.icon} {w.name}{wallpaper === w.key ? ' ✓' : ''}</span>,
    onClick: () => setWallpaper(w.key),
  }))

  return (
    <div style={{
      minHeight: '100vh',
      display: 'flex',
      position: 'relative',
      overflow: 'hidden',
      background: animeBg ? undefined : (isDark ? '#080d1a' : '#f4f7fb'),
    }}>
      {/* 背景层 */}
      {animeBg && (
        <div style={{
          position: 'absolute', inset: 0, zIndex: 0,
          backgroundImage: `url(${animeBg})`,
          backgroundSize: 'cover',
          backgroundPosition: 'center',
        }} />
      )}
      {animeBg && (
        <div style={{ position: 'absolute', inset: 0, zIndex: 0, background: 'rgba(0,0,0,0.3)' }} />
      )}

      {/* 左侧：品牌介绍区 */}
      <div style={{
        flex: 1,
        display: 'flex',
        flexDirection: 'column',
        justifyContent: 'center',
        alignItems: 'center',
        padding: '60px 48px',
        position: 'relative',
        zIndex: 1,
        minHeight: '100vh',
      }}>
        {/* 品牌内容 */}
        <div style={{
          maxWidth: 480,
          opacity: visible ? 1 : 0,
          transform: visible ? 'translateX(0)' : 'translateX(-30px)',
          transition: 'all 0.8s cubic-bezier(0.16, 1, 0.3, 1)',
          ...(animeBg ? {
            background: 'rgba(0,0,0,0.45)',
            backdropFilter: 'blur(16px)',
            WebkitBackdropFilter: 'blur(16px)',
            borderRadius: 20,
            padding: '36px 32px',
            border: '1px solid rgba(255,255,255,0.08)',
            boxShadow: '0 20px 60px rgba(0,0,0,0.3)',
          } : {}),
        }}>
          <div style={{
            width: 72, height: 72, borderRadius: 20,
            background: 'linear-gradient(135deg, #1677ff 0%, #722ed1 100%)',
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            marginBottom: 32,
            boxShadow: '0 20px 50px rgba(22,119,255,0.3)',
          }}>
            <WifiOutlined style={{ color: '#fff', fontSize: 32 }} />
          </div>

          <Title level={1} style={{
            color: textColor, margin: 0, fontWeight: 800,
            fontSize: 42, letterSpacing: '-1px', lineHeight: 1.2,
          }}>
            NetPanel
          </Title>
          <Text style={{
            color: subColor, fontSize: 14,
            marginTop: 12, display: 'block', lineHeight: 1.8,
          }}>
            {language === 'zh'
              ? '一站式网络管理平台 —— 端口转发、内网穿透、反向代理、DDNS、域名管理、SSL证书、防火墙策略，尽在掌控。'
              : 'All-in-one Network Management — Port forwarding, NAT traversal, reverse proxy, DDNS, domain management, SSL certificates, and firewall policies, all under your control.'}
          </Text>

          {/* 特性标签 */}
          <div style={{ marginTop: 32, display: 'flex', flexWrap: 'wrap', gap: 10 }}>
            {(language === 'zh'
              ? ['端口映射', '内网穿透', '反向代理', 'DDNS', '证书管理', '防火墙']
              : ['Port Forward', 'NAT Traversal', 'Reverse Proxy', 'DDNS', 'Cert Mgmt', 'Firewall']
            ).map(tag => (
              <span key={tag} style={{
                padding: '6px 14px',
                borderRadius: 20,
                fontSize: 12,
                fontWeight: 500,
                background: isDark ? 'rgba(22,119,255,0.12)' : 'rgba(22,119,255,0.08)',
                color: '#1677ff',
                border: `1px solid rgba(22,119,255,0.2)`,
              }}>
                {tag}
              </span>
            ))}
          </div>
        </div>
      </div>

      {/* 右侧：登录卡片 */}
      <div style={{
        width: 460,
        minWidth: 380,
        display: 'flex',
        flexDirection: 'column',
        justifyContent: 'center',
        padding: '40px 48px',
        position: 'relative',
        zIndex: 1,
        background: cardBg,
        backdropFilter: 'blur(40px)',
        WebkitBackdropFilter: 'blur(40px)',
        borderLeft: `1px solid ${isDark ? 'rgba(255,255,255,0.06)' : 'rgba(0,0,0,0.04)'}`,
        boxShadow: isDark ? '-20px 0 60px rgba(0,0,0,0.3)' : '-10px 0 40px rgba(0,0,0,0.04)',
        opacity: visible ? 1 : 0,
        transform: visible ? 'translateX(0)' : 'translateX(30px)',
        transition: 'all 0.7s cubic-bezier(0.16, 1, 0.3, 1) 0.1s',
      }}>
        {/* 语言 + UI模式 + 壁纸 切换 */}
        <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 8, marginBottom: 32 }}>
          <Button
            size="small" type="text"
            icon={<GlobalOutlined />}
            onClick={() => setLanguage(language === 'zh' ? 'en' : 'zh')}
            style={{
              color: subColor,
              background: inputBg,
              border: `1px solid ${inputBorder}`,
              borderRadius: 8, padding: '2px 10px',
            }}
          >
            {language === 'zh' ? 'EN' : '中文'}
          </Button>
          <Button
            size="small" type="text"
            onClick={() => setUIMode(isDark ? 'light' : 'dark')}
            style={{
              color: subColor,
              background: inputBg,
              border: `1px solid ${inputBorder}`,
              borderRadius: 8, padding: '2px 10px',
            }}
          >
            {isDark ? '🌙' : '☀️'}
          </Button>
          <Dropdown menu={{ items: wpMenuItems }} placement="bottomRight" trigger={['click']}>
            <Button
              size="small" type="text"
              icon={<SkinOutlined />}
              style={{
                color: subColor,
                background: inputBg,
                border: `1px solid ${inputBorder}`,
                borderRadius: 8, padding: '2px 10px',
              }}
            >
              {wpOptions.find(w => w.key === wallpaper)?.icon || '🎯'}
            </Button>
          </Dropdown>
        </div>

        {/* 标题 */}
        <div style={{ marginBottom: 32 }}>
          <Title level={3} style={{ color: textColor, margin: 0, fontWeight: 700 }}>
            {isSiteAuth ? (language === 'zh' ? '身份认证' : 'Authentication') : t('login.login')}
          </Title>
          {isSiteAuth ? (
            <div style={{
              marginTop: 12, padding: '10px 14px',
              background: 'rgba(22,119,255,0.08)',
              border: '1px solid rgba(22,119,255,0.2)',
              borderRadius: 8,
            }}>
              <Text style={{ color: textColor, fontSize: 13 }}>
                🔒 {language === 'zh' ? '正在访问' : 'Accessing'} <span style={{ color: '#1677ff', fontWeight: 600 }}>{siteHost}</span>
              </Text>
              <br />
              <Text style={{ color: subColor, fontSize: 12 }}>
                {language === 'zh' ? '该站点需要身份认证，请登录后继续' : 'This site requires authentication. Please log in to continue.'}
              </Text>
            </div>
          ) : (
            <Text style={{ color: subColor, fontSize: 13, marginTop: 6, display: 'block' }}>
              {language === 'zh' ? '登录以管理你的网络服务' : 'Sign in to manage your network services'}
            </Text>
          )}
        </div>

        {/* 登录表单 */}
        <Form name="login" onFinish={onFinish} size="large" autoComplete="off">
          <Form.Item name="username" rules={[{ required: true, message: t('login.username') }]} style={{ marginBottom: 16 }}>
            <Input
              prefix={<UserOutlined style={{ color: subColor }} />}
              placeholder={t('login.username')}
              style={{
                background: inputBg, border: `1px solid ${inputBorder}`,
                borderRadius: 10, color: textColor, height: 46,
              }}
            />
          </Form.Item>
          <Form.Item name="password" rules={[{ required: true, message: t('login.password') }]} style={{ marginBottom: 28 }}>
            <Input.Password
              prefix={<LockOutlined style={{ color: subColor }} />}
              placeholder={t('login.password')}
              style={{
                background: inputBg, border: `1px solid ${inputBorder}`,
                borderRadius: 10, color: textColor, height: 46,
              }}
            />
          </Form.Item>
          <Form.Item style={{ marginBottom: 16 }}>
            <Button
              type="primary" htmlType="submit" block loading={loading}
              style={{
                height: 46, borderRadius: 10, fontSize: 15, fontWeight: 600,
                background: 'linear-gradient(135deg, #1677ff 0%, #0958d9 100%)',
                border: 'none', boxShadow: '0 8px 24px rgba(22,119,255,0.3)',
              }}
            >
              {t('login.login')}
            </Button>
          </Form.Item>
        </Form>

        {/* 第三方登录 */}
        {providers.length > 0 && (
          <>
            <Divider style={{ borderColor: inputBorder, margin: '12px 0' }}>
              <Text style={{ color: subColor, fontSize: 12 }}>{t('login.orThirdParty')}</Text>
            </Divider>
            <Space wrap style={{ width: '100%', justifyContent: 'center' }}>
              {providers.map(p => (
                <Button key={p.id} shape="round" onClick={() => handleOAuthLogin(p)}
                  style={{ background: inputBg, border: `1px solid ${inputBorder}`, color: textColor }}>
                  {p.name}
                </Button>
              ))}
            </Space>
          </>
        )}

        {/* 底部版权 */}
        <div style={{ marginTop: 'auto', paddingTop: 32, textAlign: 'center' }}>
          <Text style={{ color: subColor, fontSize: 11 }}>© 2024 NetPanel · Network Manager</Text>
        </div>
      </div>
    </div>
  )
}

export default LoginPage
