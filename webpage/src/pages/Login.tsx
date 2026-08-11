import React, { useState, useEffect } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { Form, Input, Button, Typography, message, Divider, Dropdown, Space } from 'antd'
import { UserOutlined, LockOutlined, WifiOutlined, GlobalOutlined, SkinOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { useAppStore, wallpaperList, getWallpaperBg } from '../store/appStore'
import request from '../api/request'
import i18n from '../i18n'
import { useTableStyle } from '../hooks/useTableStyle'

const { Title, Text } = Typography

// ─────────────────────────────────────────────────────────────
//  拓扑背景：SVG 动态网络节点 + 数据流连线
// ─────────────────────────────────────────────────────────────
interface TopoNode { id: number; cx: number; cy: number; r: number; color: string; label: string }
interface TopoEdge { from: number; to: number; delay: number }

const TOPO_NODES: TopoNode[] = [
  { id: 0, cx: 50, cy: 46, r: 5.5,  color: '#0ea5e9', label: 'CORE'  }, // 中心枢纽
  { id: 1, cx: 18, cy: 14, r: 3.0,  color: '#1677ff', label: 'FRP'   },
  { id: 2, cx: 78, cy: 20, r: 2.8,  color: '#722ed1', label: 'DDNS'  },
  { id: 3, cx: 10, cy: 58, r: 2.8,  color: '#13c2c2', label: 'WG'    },
  { id: 4, cx: 85, cy: 60, r: 2.8,  color: '#1677ff', label: 'Caddy' },
  { id: 5, cx: 28, cy: 80, r: 2.8,  color: '#52c41a', label: 'DNS'   },
  { id: 6, cx: 70, cy: 82, r: 2.8,  color: '#722ed1', label: 'NPS'   },
  { id: 7, cx: 88, cy: 36, r: 2.4,  color: '#ff4d4f', label: 'FW'    },
  { id: 8, cx: 54, cy: 10, r: 2.4,  color: '#13c2c2', label: 'STUN'  },
  { id: 9, cx: 36, cy: 65, r: 2.4,  color: '#fa8c16', label: 'Cron'  },
]

const TOPO_EDGES: TopoEdge[] = [
  { from: 0, to: 1, delay: 0    },
  { from: 0, to: 2, delay: 300  },
  { from: 0, to: 3, delay: 600  },
  { from: 0, to: 4, delay: 900  },
  { from: 0, to: 5, delay: 1200 },
  { from: 0, to: 6, delay: 1500 },
  { from: 0, to: 7, delay: 1800 },
  { from: 0, to: 8, delay: 400  },
  { from: 0, to: 9, delay: 700  },
  { from: 1, to: 8, delay: 200  },
  { from: 2, to: 7, delay: 1100 },
  { from: 3, to: 9, delay: 1400 },
  { from: 5, to: 9, delay: 900  },
  { from: 4, to: 6, delay: 500  },
]

const TopologyBg: React.FC<{ opacity?: number }> = ({ opacity = 0.35 }) => (
  <svg
    viewBox="0 0 100 100"
    preserveAspectRatio="xMidYMid slice"
    style={{ position: 'absolute', inset: 0, width: '100%', height: '100%', opacity }}
    aria-hidden="true"
  >
    <defs>
      <filter id="topoGlow" x="-50%" y="-50%" width="200%" height="200%">
        <feGaussianBlur stdDeviation="1.2" result="blur" />
        <feMerge><feMergeNode in="blur" /><feMergeNode in="SourceGraphic" /></feMerge>
      </filter>
    </defs>

    {/* 连线：短虚线沿路径流动 */}
    {TOPO_EDGES.map((edge, i) => {
      const f = TOPO_NODES.find(n => n.id === edge.from)!
      const t = TOPO_NODES.find(n => n.id === edge.to)!
      return (
        <path
          key={i}
          d={`M ${f.cx} ${f.cy} L ${t.cx} ${t.cy}`}
          stroke={f.color}
          strokeWidth={0.3}
          strokeOpacity={0.45}
          fill="none"
          strokeDasharray="3 100"
          pathLength={103}
          style={{ animation: `dataFlow 3.6s linear ${edge.delay}ms infinite` }}
        />
      )
    })}

    {/* 底层静态连线（低透明度） */}
    {TOPO_EDGES.map((edge, i) => {
      const f = TOPO_NODES.find(n => n.id === edge.from)!
      const t = TOPO_NODES.find(n => n.id === edge.to)!
      return (
        <line
          key={`base-${i}`}
          x1={f.cx} y1={f.cy} x2={t.cx} y2={t.cy}
          stroke={f.color}
          strokeWidth={0.18}
          strokeOpacity={0.18}
        />
      )
    })}

    {/* 节点 */}
    {TOPO_NODES.map(node => (
      <g key={node.id} filter="url(#topoGlow)">
        {/* 脉冲外环 */}
        <circle
          cx={node.cx} cy={node.cy} r={node.r * 2.4}
          fill="none" stroke={node.color} strokeWidth={0.2}
          style={{
            animation: `nodeRing 3.2s ease-in-out ${node.id * 280}ms infinite`,
          }}
        />
        {/* 主节点 */}
        <circle
          cx={node.cx} cy={node.cy} r={node.r}
          fill={node.color}
          style={{ animation: `nodeBlink 4s ease-in-out ${node.id * 350}ms infinite` }}
        />
        {/* 高光点 */}
        <circle
          cx={node.cx - node.r * 0.28} cy={node.cy - node.r * 0.28}
          r={node.r * 0.3}
          fill="rgba(255,255,255,0.55)"
        />
        {/* 节点标签 */}
        <text
          x={node.cx} y={node.cy + node.r + 2.4}
          textAnchor="middle"
          fontSize={1.9}
          fontFamily="'MapleMono', monospace"
          fill={node.color}
          fillOpacity={0.65}
          letterSpacing={0.2}
        >
          {node.label}
        </text>
      </g>
    ))}
  </svg>
)

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
  const tableStyle = useTableStyle()
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
  // 左侧品牌区域：有壁纸时背景叠加暗色遮罩，文字始终用浅色
  const leftTextColor = (isDark || animeBg) ? '#fff' : '#1a1a2e'
  const leftSubColor = (isDark || animeBg) ? 'rgba(255,255,255,0.85)' : 'rgba(0,0,0,0.45)'
  // 右侧登录卡片：始终根据 uiMode 决定文字颜色
  const rightTextColor = isDark ? '#fff' : '#1a1a2e'
  const rightSubColor = isDark ? 'rgba(255,255,255,0.5)' : 'rgba(0,0,0,0.45)'
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
        <div style={{ position: 'absolute', inset: 0, zIndex: 0, background: 'rgba(0,0,0,0.5)' }} />
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
        overflow: 'hidden',
        /* 无壁纸时左侧自带渐变底色，拓扑 SVG 有色彩可渲染 */
        background: animeBg ? undefined : isDark
          ? 'linear-gradient(135deg, #04081a 0%, #0a1028 40%, #0d1540 100%)'
          : 'linear-gradient(135deg, #eef5ff 0%, #e8f0fe 50%, #f0f4ff 100%)',
      }}>
        {/* 拓扑动画背景（无壁纸时才展示，避免与壁纸冲突） */}
        {!animeBg && (
          <TopologyBg opacity={isDark ? 0.45 : 0.28} />
        )}
        {/* 品牌内容 */}
        <div style={{
          maxWidth: 480,
          opacity: visible ? 1 : 0,
          transform: visible ? 'translateX(0)' : 'translateX(-30px)',
          transition: 'all 0.8s cubic-bezier(0.16, 1, 0.3, 1)',
          ...(animeBg ? {
            background: 'rgba(0,0,0,0.6)',
            backdropFilter: 'blur(16px)',
            WebkitBackdropFilter: 'blur(16px)',
            borderRadius: 20,
            padding: '36px 32px',
            border: '1px solid rgba(255,255,255,0.1)',
            boxShadow: '0 20px 60px rgba(0,0,0,0.4)',
          } : {}),
        }}>
          <div style={{
            width: 72, height: 72, borderRadius: 20,
            background: 'linear-gradient(135deg, #1677ff 0%, #722ed1 100%)',
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            marginBottom: 32,
            boxShadow: '0 20px 50px rgba(22,119,255,0.3)',
            position: 'relative',
          }}>
            {/* 六边形网格纹理 */}
            <svg width={72} height={72} style={{ position: 'absolute', inset: 0, opacity: 0.18 }} aria-hidden="true">
              <defs>
                <pattern id="hexPat" x="0" y="0" width="10" height="11.5" patternUnits="userSpaceOnUse">
                  <polygon points="5,0 10,2.9 10,8.6 5,11.5 0,8.6 0,2.9" fill="none" stroke="white" strokeWidth="0.6"/>
                </pattern>
              </defs>
              <rect width="72" height="72" fill="url(#hexPat)" rx="20"/>
            </svg>
            <WifiOutlined style={{ color: '#fff', fontSize: 32, position: 'relative' }} />
          </div>

          <Title level={1} style={{
            color: leftTextColor, margin: 0, fontWeight: 800,
            fontSize: 42, letterSpacing: '-1px', lineHeight: 1.2,
            ...(animeBg ? { textShadow: '0 2px 12px rgba(0,0,0,0.5)' } : {}),
          }}>
            NetPanel
          </Title>
          <Text style={{
            color: leftSubColor, fontSize: 14,
            marginTop: 12, display: 'block', lineHeight: 1.8,
            ...(animeBg ? { textShadow: '0 1px 8px rgba(0,0,0,0.4)' } : {}),
          }}>
            {language === 'zh'
              ? '一站式网络管理平台 —— 端口转发、内网穿透、反向代理、DDNS、域名管理、SSL证书、防火墙策略，尽在掌控。'
              : 'All-in-one Network Management — Port forwarding, NAT traversal, reverse proxy, DDNS, domain management, SSL certificates, and firewall policies, all under your control.'}
          </Text>

          {/* 特性标签 — monospace 终端风格 */}
          <div style={{ marginTop: 32, display: 'flex', flexWrap: 'wrap', gap: 8 }}>
            {(language === 'zh'
              ? ['端口映射', '内网穿透', '反向代理', 'DDNS', '证书管理', '防火墙']
              : ['Port Forward', 'NAT Traversal', 'Reverse Proxy', 'DDNS', 'Cert Mgmt', 'Firewall']
            ).map((tag, i) => {
              const colors = ['#1677ff','#13c2c2','#722ed1','#52c41a','#fa8c16','#ff4d4f']
              const c = colors[i % colors.length]
              return (
                <span key={tag} style={{
                  padding: '4px 10px',
                  borderRadius: 5,
                  fontSize: 11,
                  fontFamily: "'MapleMono', monospace",
                  fontWeight: 500,
                  letterSpacing: '0.4px',
                  background: `${c}18`,
                  color: (isDark || animeBg) ? `${c}ee` : c,
                  border: `1px solid ${c}44`,
                  boxShadow: (isDark || animeBg) ? `0 0 8px ${c}22` : 'none',
                  transition: 'all 0.2s',
                }}>
                  {`> ${tag}`}
                </span>
              )
            })}
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
              color: rightSubColor,
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
              color: rightSubColor,
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
                color: rightSubColor,
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
          <Title level={3} style={{ color: rightTextColor, margin: 0, fontWeight: 700 }}>
            {isSiteAuth ? (language === 'zh' ? '身份认证' : 'Authentication') : t('login.login')}
          </Title>
          {isSiteAuth ? (
            <div style={{
              marginTop: 12, padding: '10px 14px',
              background: 'rgba(22,119,255,0.08)',
              border: '1px solid rgba(22,119,255,0.2)',
              borderRadius: 8,
            }}>
              <Text style={{ color: rightTextColor, fontSize: 13 }}>
                🔒 {language === 'zh' ? '正在访问' : 'Accessing'} <span style={{ color: '#1677ff', fontWeight: 600 }}>{siteHost}</span>
              </Text>
              <br />
              <Text style={{ color: rightSubColor, fontSize: 12 }}>
                {language === 'zh' ? '该站点需要身份认证，请登录后继续' : 'This site requires authentication. Please log in to continue.'}
              </Text>
            </div>
          ) : (
            <Text style={{ color: rightSubColor, fontSize: 13, marginTop: 6, display: 'block' }}>
              {language === 'zh' ? '登录以管理你的网络服务' : 'Sign in to manage your network services'}
            </Text>
          )}
        </div>

        {/* 登录表单 */}
        <Form name="login" onFinish={onFinish} size="large" autoComplete="off">
          <Form.Item name="username" rules={[{ required: true, message: t('login.username') }]} style={{ marginBottom: 16 }}>
            <Input
              prefix={<UserOutlined style={{ color: rightSubColor }} />}
              placeholder={t('login.username')}
              style={{
                background: inputBg, border: `1px solid ${inputBorder}`,
                borderRadius: 10, color: rightTextColor, height: 46,
              }}
            />
          </Form.Item>
          <Form.Item name="password" rules={[{ required: true, message: t('login.password') }]} style={{ marginBottom: 28 }}>
            <Input.Password
              prefix={<LockOutlined style={{ color: rightSubColor }} />}
              placeholder={t('login.password')}
              style={{
                background: inputBg, border: `1px solid ${inputBorder}`,
                borderRadius: 10, color: rightTextColor, height: 46,
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
              <Text style={{ color: rightSubColor, fontSize: 12 }}>{t('login.orThirdParty')}</Text>
            </Divider>
            <Space wrap style={{ width: '100%', justifyContent: 'center' }}>
              {providers.map(p => (
                <Button key={p.id} shape="round" onClick={() => handleOAuthLogin(p)}
                  style={{ background: inputBg, border: `1px solid ${inputBorder}`, color: rightTextColor }}>
                  {p.name}
                </Button>
              ))}
            </Space>
          </>
        )}

        {/* 底部版权 */}
        <div style={{ marginTop: 'auto', paddingTop: 32, textAlign: 'center' }}>
          <Text style={{ color: rightSubColor, fontSize: 11 }}>© 2024 NetPanel · Network Manager</Text>
        </div>
      </div>
    </div>
  )
}

export default LoginPage
