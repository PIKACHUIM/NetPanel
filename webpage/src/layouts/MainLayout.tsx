import React, { useState, useEffect } from 'react'
import { Outlet, useNavigate, useLocation } from 'react-router-dom'
import {
  Layout, Menu, Avatar, Dropdown, Space, Typography, Tooltip,
  theme as antTheme, Switch as AntSwitch,
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

const MainLayout: React.FC = () => {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const location = useLocation()
  const { username, collapsed, setCollapsed, logout, language, setLanguage, theme, setTheme } = useAppStore()
  const { token } = antTheme.useToken()
  const isDark = theme === 'dark'

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

  const siderBg = isDark ? '#141414' : '#001529'
  const logoBorderColor = isDark ? 'rgba(255,255,255,0.06)' : 'rgba(255,255,255,0.08)'

  return (
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
          boxShadow: isDark ? '2px 0 8px rgba(0,0,0,0.4)' : '2px 0 8px rgba(0,0,0,0.15)',
          overflow: 'auto',
          height: '100vh',
          position: 'fixed',
          left: 0,
          top: 0,
          bottom: 0,
          zIndex: 100,
        }}
      >
        {/* Logo 区域 */}
        <div
          style={{
            height: 56,
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
            width: 32, height: 32, borderRadius: 8,
            background: 'linear-gradient(135deg, #1677ff 0%, #0958d9 100%)',
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            flexShrink: 0,
            boxShadow: '0 2px 8px rgba(22,119,255,0.4)',
          }}>
            <WifiOutlined style={{ color: '#fff', fontSize: 16 }} />
          </div>
          {!collapsed && (
            <Text style={{
              color: '#fff', fontSize: 16, fontWeight: 700,
              marginLeft: 10, letterSpacing: '0.5px', whiteSpace: 'nowrap',
            }}>
              NetPanel
            </Text>
          )}
        </div>

        {/* 菜单 */}
        <div style={{ flex: 1, overflow: 'auto' }}>
          <Menu
            theme="dark"
            mode="inline"
            selectedKeys={[selectedKey]}
            defaultOpenKeys={openKeys}
            items={menuItems}
            onClick={({ key }) => navigate(`/${key}`)}
            style={{
              borderRight: 0,
              marginTop: 4,
              background: siderBg,
            }}
          />
        </div>

        {/* 底部版本号 */}
        {!collapsed && (
          <div style={{
            padding: '12px 20px',
            borderTop: `1px solid ${logoBorderColor}`,
            textAlign: 'center',
          }}>
            <Text style={{ color: 'rgba(255,255,255,0.25)', fontSize: 11 }}>
              NetPanel v0.1.0
            </Text>
          </div>
        )}
      </Sider>

      {/* 右侧主区域 */}
      <Layout style={{ marginLeft: collapsed ? 80 : 220, transition: 'margin-left 0.2s' }}>
        {/* 顶部栏 */}
        <Header style={{
          padding: '0 20px',
          background: token.colorBgContainer,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          borderBottom: `1px solid ${token.colorBorderSecondary}`,
          height: 56,
          position: 'sticky',
          top: 0,
          zIndex: 99,
          boxShadow: isDark ? '0 1px 4px rgba(0,0,0,0.3)' : '0 1px 4px rgba(0,0,0,0.06)',
        }}>
          {/* 折叠按钮 */}
          <Tooltip title={collapsed ? t('common.expandMenu') : t('common.collapseMenu')}>
            <div
              onClick={() => setCollapsed(!collapsed)}
              style={{
                cursor: 'pointer', fontSize: 18,
                color: token.colorTextSecondary,
                padding: '4px 8px', borderRadius: 6,
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
          <Space size={8}>
            {/* 语言切换 */}
            <Tooltip title={language === 'zh' ? 'Switch to English' : '切换为中文'}>
              <div
                onClick={() => setLanguage(language === 'zh' ? 'en' : 'zh')}
                style={{
                  cursor: 'pointer',
                  padding: '4px 10px',
                  borderRadius: 6,
                  display: 'flex', alignItems: 'center', gap: 4,
                  color: token.colorTextSecondary,
                  fontSize: 13, fontWeight: 500,
                  transition: 'all 0.2s',
                  border: `1px solid ${token.colorBorderSecondary}`,
                }}
                onMouseEnter={e => (e.currentTarget.style.background = token.colorFillSecondary)}
                onMouseLeave={e => (e.currentTarget.style.background = 'transparent')}
              >
                <TranslationOutlined style={{ fontSize: 14 }} />
                <span>{language === 'zh' ? '中文' : 'EN'}</span>
              </div>
            </Tooltip>

            {/* 主题切换 */}
            <Tooltip title={isDark ? t('settings.lightTheme') : t('settings.darkTheme')}>
              <div
                onClick={() => setTheme(isDark ? 'light' : 'dark')}
                style={{
                  cursor: 'pointer',
                  padding: '4px 8px',
                  borderRadius: 6,
                  display: 'flex', alignItems: 'center',
                  color: token.colorTextSecondary,
                  fontSize: 18,
                  transition: 'all 0.2s',
                }}
                onMouseEnter={e => (e.currentTarget.style.background = token.colorFillSecondary)}
                onMouseLeave={e => (e.currentTarget.style.background = 'transparent')}
              >
                <BulbOutlined style={{ color: isDark ? '#faad14' : token.colorTextSecondary }} />
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
                  style={{ background: 'linear-gradient(135deg, #1677ff, #0958d9)', flexShrink: 0 }}
                  icon={<UserOutlined />}
                />
                <Text style={{ fontSize: 13, fontWeight: 500 }}>{username || 'admin'}</Text>
              </Space>
            </Dropdown>
          </Space>
        </Header>

        {/* 内容区 */}
        <Content style={{
          padding: 24,
          overflow: 'auto',
          height: 'calc(100vh - 56px)',
          background: isDark ? '#0d0d0d' : '#f0f2f5',
        }}>
          <Outlet />
        </Content>
      </Layout>
    </Layout>
  )
}

function getOpenKeys(pathname: string): string[] {
  if (pathname.startsWith('/frp') || pathname.startsWith('/stun')) return ['tunnel']
  if (pathname.startsWith('/easytier')) return ['network']
  if (pathname.startsWith('/domain')) return ['domain']
  if (pathname.startsWith('/callback')) return ['callback']
  return []
}

export default MainLayout
