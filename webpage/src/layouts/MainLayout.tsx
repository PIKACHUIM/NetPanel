import React, { useState, useEffect } from 'react'
import { Outlet, useNavigate, useLocation } from 'react-router-dom'
import {
  Layout, Menu, Avatar, Dropdown, Space, Typography, Tooltip,
  theme as antTheme,
} from 'antd'
import type { MenuProps } from 'antd'
import {
  DashboardOutlined, SwapOutlined, GlobalOutlined, CloudServerOutlined,
  ApiOutlined, WifiOutlined, SafetyOutlined, ThunderboltOutlined,
  KeyOutlined, DatabaseOutlined, ClockCircleOutlined, FolderOpenOutlined,
  FilterOutlined, BellOutlined, SettingOutlined, UserOutlined,
  LogoutOutlined, MenuFoldOutlined, MenuUnfoldOutlined, LinkOutlined,
  ApartmentOutlined, ControlOutlined, NodeIndexOutlined,
  TranslationOutlined, BulbOutlined,
} from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { useAppStore } from '../store/appStore'
import i18n from '../i18n'

const { Sider, Header, Content } = Layout
const { Text } = Typography

// 玻璃背景组件
const GlassBackground: React.FC = () => (
  <div className="glass-bg-wrapper">
    <div className="glass-bg-orb glass-bg-orb-1" />
    <div className="glass-bg-orb glass-bg-orb-2" />
    <div className="glass-bg-orb glass-bg-orb-3" />
  </div>
)



const MainLayout: React.FC = () => {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const location = useLocation()
  const { username, collapsed, setCollapsed, logout, language, setLanguage, theme, setTheme } = useAppStore()
  const { token } = antTheme.useToken()
  const isDark = theme === 'dark' || theme === 'glass-dark'
  const isGlass = theme === 'glass-light' || theme === 'glass-dark'
  const isLight = !isDark

  // 切换暗黑：保持透明状态不变，只切换明暗
  const toggleDark = () => {
    if (isGlass) setTheme(isDark ? 'glass-light' : 'glass-dark')
    else setTheme(isDark ? 'light' : 'dark')
  }

  // 切换透明：保持明暗状态不变，只切换透明
  const toggleGlass = () => {
    if (isDark) setTheme(isGlass ? 'dark' : 'glass-dark')
    else setTheme(isGlass ? 'light' : 'glass-light')
  }

  // 同步语言到 i18n
  useEffect(() => {
    i18n.changeLanguage(language)
  }, [language])

  // 根据当前路径计算选中的菜单项
  const selectedKey = location.pathname.replace(/^\//, '') || 'dashboard'
  const openKeys = getOpenKeys(location.pathname)



  const menuItems: MenuProps['items'] = [
    {
      key: 'dashboard',
      icon: <DashboardOutlined />,
      label: t('menu.dashboard'),
    },
    {
      key: 'port-forward',
      icon: <SwapOutlined />,
      label: t('menu.portForward'),
    },
    {
      key: 'tunnel',
      icon: <NodeIndexOutlined />,
      label: t('menu.tunnel'),
      children: [
        { key: 'stun', icon: <WifiOutlined />, label: t('menu.stun') },
        { key: 'frp/client', icon: <ApiOutlined />, label: t('menu.frpc') },
        { key: 'frp/server', icon: <CloudServerOutlined />, label: t('menu.frps') },
        { key: 'nps/server', icon: <CloudServerOutlined />, label: t('menu.npsServer') },
        { key: 'nps/client', icon: <ApiOutlined />, label: t('menu.npsClient') },
      ],
    },
    {
      key: 'network',
      icon: <ApartmentOutlined />,
      label: t('menu.network'),
      children: [
        { key: 'easytier/client', icon: <ApiOutlined />, label: t('menu.easytierClient') },
        { key: 'easytier/server', icon: <CloudServerOutlined />, label: t('menu.easytierServer') },
      ],
    },
    {
      key: 'ddns',
      icon: <GlobalOutlined />,
      label: t('menu.ddns'),
    },
    {
      key: 'caddy',
      icon: <LinkOutlined />,
      label: t('menu.caddy'),
    },
    {
      key: 'wol',
      icon: <ThunderboltOutlined />,
      label: t('menu.wol'),
    },
    {
      key: 'domain',
      icon: <KeyOutlined />,
      label: t('menu.domain'),
      children: [
        { key: 'domain/account', icon: <UserOutlined />, label: t('menu.domainAccount') },
        { key: 'domain/cert', icon: <SafetyOutlined />, label: t('menu.domainCert') },
        { key: 'domain/record', icon: <DatabaseOutlined />, label: t('menu.domainRecord') },
      ],
    },
    {
      key: 'dnsmasq',
      icon: <ControlOutlined />,
      label: t('menu.dnsmasq'),
    },
    {
      key: 'cron',
      icon: <ClockCircleOutlined />,
      label: t('menu.cron'),
    },
    {
      key: 'storage',
      icon: <FolderOpenOutlined />,
      label: t('menu.storage'),
    },
    {
      key: 'ipdb',
      icon: <DatabaseOutlined />,
      label: t('menu.ipdb'),
    },
    {
      key: 'access',
      icon: <FilterOutlined />,
      label: t('menu.access'),
    },
    {
      key: 'callback',
      icon: <BellOutlined />,
      label: t('menu.callback'),
      children: [
        { key: 'callback/account', icon: <UserOutlined />, label: t('menu.callbackAccount') },
        { key: 'callback/task', icon: <ClockCircleOutlined />, label: t('menu.callbackTask') },
      ],
    },
    {
      key: 'settings',
      icon: <SettingOutlined />,
      label: t('menu.settings'),
    },
  ]

  const userMenuItems: MenuProps['items'] = [
    {
      key: 'settings',
      icon: <SettingOutlined />,
      label: t('menu.settings'),
      onClick: () => navigate('/settings'),
    },
    { type: 'divider' },
    {
      key: 'logout',
      icon: <LogoutOutlined />,
      label: t('common.logout'),
      danger: true,
      onClick: () => {
        logout()
        navigate('/login')
      },
    },
  ]

  // 侧边栏背景
  const siderBg = isDark
    ? '#141414'
    : isGlass
    ? 'rgba(10,20,50,0.75)'
    : '#001529'

  const logoBorderColor = isLight
    ? 'rgba(255,255,255,0.08)'
    : 'rgba(255,255,255,0.1)'

  // 顶部栏背景
  const headerBg = isDark
    ? '#1a1a1a'
    : isGlass
    ? 'rgba(255,255,255,0.06)'
    : token.colorBgContainer

  const headerBorder = isGlass
    ? '1px solid rgba(255,255,255,0.08)'
    : `1px solid ${token.colorBorderSecondary}`

  // 内容区背景
  const contentBg = isDark
    ? '#0d0d0d'
    : isGlass
    ? 'transparent'
    : '#f0f2f5'

  return (
    <>
      {/* 玻璃模式背景 */}
      {isGlass && <GlassBackground />}

      <Layout style={{ height: '100vh' }}>
        {/* 侧边栏 */}
        <Sider
          collapsible
          collapsed={collapsed}
          onCollapse={setCollapsed}
          trigger={null}
          width={220}
          style={{
            background: siderBg,
            backdropFilter: isGlass ? 'blur(20px)' : undefined,
            WebkitBackdropFilter: isGlass ? 'blur(20px)' : undefined,
            boxShadow: isDark
              ? '2px 0 12px rgba(0,0,0,0.5)'
              : isGlass
              ? '2px 0 20px rgba(0,0,0,0.3), inset -1px 0 0 rgba(255,255,255,0.06)'
              : '2px 0 8px rgba(0,0,0,0.12)',
            overflow: 'auto',
            height: '100vh',
            position: 'fixed',
            left: 0,
            top: 0,
            bottom: 0,
            zIndex: 100,
            borderRight: isGlass ? '1px solid rgba(255,255,255,0.08)' : 'none',
          }}
        >
          {/* Logo 区域 */}
          <div
            style={{
              height: 60,
              display: 'flex',
              alignItems: 'center',
              justifyContent: collapsed ? 'center' : 'flex-start',
              padding: collapsed ? '0' : '0 20px',
              borderBottom: `1px solid ${logoBorderColor}`,
              cursor: 'pointer',
              transition: 'all 0.2s',
              flexShrink: 0,
            }}
            onClick={() => navigate('/dashboard')}
          >
            <div style={{
              width: 34, height: 34, borderRadius: 10,
              background: 'linear-gradient(135deg, #1677ff 0%, #0958d9 100%)',
              display: 'flex', alignItems: 'center', justifyContent: 'center',
              flexShrink: 0,
              boxShadow: '0 4px 12px rgba(22,119,255,0.5)',
              transition: 'transform 0.2s',
            }}>
              <WifiOutlined style={{ color: '#fff', fontSize: 17 }} />
            </div>
            {!collapsed && (
              <div style={{ marginLeft: 12 }}>
                <Text style={{
                  color: '#fff', fontSize: 16, fontWeight: 700,
                  letterSpacing: '0.3px', whiteSpace: 'nowrap',
                  display: 'block', lineHeight: 1.2,
                }}>
                  NetPanel
                </Text>
                <Text style={{
                  color: 'rgba(255,255,255,0.35)', fontSize: 10,
                  letterSpacing: '1px', whiteSpace: 'nowrap',
                  display: 'block', lineHeight: 1,
                }}>
                  NETWORK MANAGER
                </Text>
              </div>
            )}
          </div>

          {/* 菜单 */}
          <div style={{ flex: 1, overflow: 'auto', paddingTop: 6 }}>
            <Menu
              theme="dark"
              mode="inline"
              selectedKeys={[selectedKey]}
              defaultOpenKeys={openKeys}
              items={menuItems}
              onClick={({ key }) => navigate(`/${key}`)}
              style={{
                borderRight: 0,
                background: 'transparent',
                fontSize: 13,
              }}
            />
          </div>

          {/* 底部版本号 */}
          {!collapsed && (
            <div style={{
              padding: '10px 20px',
              borderTop: `1px solid ${logoBorderColor}`,
              textAlign: 'center',
            }}>
              <Text style={{ color: 'rgba(255,255,255,0.2)', fontSize: 11, letterSpacing: '0.5px' }}>
                v0.1.0
              </Text>
            </div>
          )}
        </Sider>

        {/* 右侧主区域 */}
        <Layout style={{ marginLeft: collapsed ? 80 : 220, transition: 'margin-left 0.2s' }}>
          {/* 顶部栏 */}
          <Header style={{
            padding: '0 20px',
            background: headerBg,
            backdropFilter: isGlass ? 'blur(20px)' : undefined,
            WebkitBackdropFilter: isGlass ? 'blur(20px)' : undefined,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            borderBottom: headerBorder,
            height: 56,
            position: 'sticky',
            top: 0,
            zIndex: 99,
            boxShadow: isDark
              ? '0 1px 6px rgba(0,0,0,0.4)'
              : isGlass
              ? '0 4px 20px rgba(0,0,0,0.15)'
              : '0 1px 4px rgba(0,0,0,0.06)',
          }}>
            {/* 折叠按钮 */}
            <Tooltip title={collapsed ? t('common.expandMenu') : t('common.collapseMenu')}>
              <div
                onClick={() => setCollapsed(!collapsed)}
                style={{
                  cursor: 'pointer', fontSize: 17,
                  color: token.colorTextSecondary,
                  padding: '6px 10px', borderRadius: 8,
                  transition: 'all 0.2s',
                  display: 'flex', alignItems: 'center',
                }}
                onMouseEnter={e => (e.currentTarget.style.background = token.colorFillSecondary)}
                onMouseLeave={e => (e.currentTarget.style.background = 'transparent')}
              >
                {collapsed ? <MenuUnfoldOutlined /> : <MenuFoldOutlined />}
              </div>
            </Tooltip>

            {/* 右侧工具栏 */}
            <Space size={6}>
              {/* 语言切换 */}
              <Tooltip title={language === 'zh' ? 'Switch to English' : '切换为中文'}>
                <div
                  onClick={() => setLanguage(language === 'zh' ? 'en' : 'zh')}
                  style={{
                    cursor: 'pointer',
                    padding: '5px 10px',
                    borderRadius: 8,
                    display: 'flex', alignItems: 'center', gap: 5,
                    color: token.colorTextSecondary,
                    fontSize: 12, fontWeight: 500,
                    transition: 'all 0.2s',
                    border: `1px solid ${token.colorBorderSecondary}`,
                    letterSpacing: '0.3px',
                  }}
                  onMouseEnter={e => (e.currentTarget.style.background = token.colorFillSecondary)}
                  onMouseLeave={e => (e.currentTarget.style.background = 'transparent')}
                >
                  <TranslationOutlined style={{ fontSize: 13 }} />
                  <span>{language === 'zh' ? '中文' : 'EN'}</span>
                </div>
              </Tooltip>

              {/* 暗黑模式开关 */}
              <Tooltip title={isDark ? t('settings.lightTheme') : t('settings.darkTheme')}>
                <div
                  onClick={toggleDark}
                  style={{
                    cursor: 'pointer',
                    padding: '5px 10px',
                    borderRadius: 8,
                    display: 'flex', alignItems: 'center', gap: 5,
                    color: isDark ? '#fadb14' : token.colorTextSecondary,
                    fontSize: 12, fontWeight: 500,
                    transition: 'all 0.2s',
                    border: `1px solid ${isDark ? 'rgba(250,219,20,0.35)' : token.colorBorderSecondary}`,
                    background: isDark ? 'rgba(250,219,20,0.08)' : 'transparent',
                  }}
                  onMouseEnter={e => (e.currentTarget.style.background = isDark ? 'rgba(250,219,20,0.15)' : token.colorFillSecondary)}
                  onMouseLeave={e => (e.currentTarget.style.background = isDark ? 'rgba(250,219,20,0.08)' : 'transparent')}
                >
                  <span style={{ fontSize: 15 }}>{isDark ? '🌙' : '☀️'}</span>
                  <span style={{ fontSize: 11 }}>{isDark ? '暗黑' : '白天'}</span>
                </div>
              </Tooltip>

              {/* 透明模式开关 */}
              <Tooltip title={isGlass ? '关闭透明模式' : '开启透明模式'}>
                <div
                  onClick={toggleGlass}
                  style={{
                    cursor: 'pointer',
                    padding: '5px 10px',
                    borderRadius: 8,
                    display: 'flex', alignItems: 'center', gap: 5,
                    color: isGlass ? '#a78bfa' : token.colorTextSecondary,
                    fontSize: 12, fontWeight: 500,
                    transition: 'all 0.2s',
                    border: `1px solid ${isGlass ? 'rgba(167,139,250,0.4)' : token.colorBorderSecondary}`,
                    background: isGlass ? 'rgba(167,139,250,0.12)' : 'transparent',
                  }}
                  onMouseEnter={e => (e.currentTarget.style.background = isGlass ? 'rgba(167,139,250,0.2)' : token.colorFillSecondary)}
                  onMouseLeave={e => (e.currentTarget.style.background = isGlass ? 'rgba(167,139,250,0.12)' : 'transparent')}
                >
                  <span style={{ fontSize: 15 }}>✨</span>
                  <span style={{ fontSize: 11 }}>{isGlass ? '透明' : '不透明'}</span>
                </div>
              </Tooltip>

              {/* 用户菜单 */}
              <Dropdown menu={{ items: userMenuItems }} placement="bottomRight" arrow>
                <Space
                  style={{
                    cursor: 'pointer', padding: '4px 10px',
                    borderRadius: 8, transition: 'all 0.2s',
                  }}
                  onMouseEnter={e => (e.currentTarget.style.background = token.colorFillSecondary)}
                  onMouseLeave={e => (e.currentTarget.style.background = 'transparent')}
                >
                  <Avatar
                    size={28}
                    style={{
                      background: 'linear-gradient(135deg, #1677ff, #0958d9)',
                      flexShrink: 0,
                      boxShadow: '0 2px 8px rgba(22,119,255,0.4)',
                    }}
                    icon={<UserOutlined />}
                  />
                  <Text style={{ fontSize: 13, fontWeight: 500 }}>{username || 'admin'}</Text>
                </Space>
              </Dropdown>
            </Space>
          </Header>

          {/* 内容区 */}
          <Content style={{
            padding: 20,
            overflow: 'auto',
            height: 'calc(100vh - 56px)',
            background: contentBg,
          }}>
            <div className="page-enter">
              <Outlet />
            </div>
          </Content>
        </Layout>
      </Layout>
    </>
  )
}

function getOpenKeys(pathname: string): string[] {
  if (pathname.startsWith('/frp') || pathname.startsWith('/stun') || pathname.startsWith('/nps')) return ['tunnel']
  if (pathname.startsWith('/easytier')) return ['network']
  if (pathname.startsWith('/domain')) return ['domain']
  if (pathname.startsWith('/callback')) return ['callback']
  return []
}

export default MainLayout
