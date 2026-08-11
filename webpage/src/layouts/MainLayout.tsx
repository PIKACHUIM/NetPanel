import React, {useEffect} from 'react'
import {Outlet, useLocation, useNavigate} from 'react-router-dom'
import type {MenuProps} from 'antd'
import {Avatar, Dropdown, Layout, Menu, Popover, Space, theme as antTheme, Tooltip, Typography,} from 'antd'
import {
    ApartmentOutlined,
    ApiOutlined,
    BellOutlined,
    BugOutlined,
    ClockCircleOutlined,
    CloudServerOutlined,
    ClusterOutlined,
    ControlOutlined,
    DashboardOutlined,
    DatabaseOutlined,
    FilterOutlined,
    FireOutlined,
    FolderOpenOutlined,
    GlobalOutlined,
    HistoryOutlined,
    KeyOutlined,
    LinkOutlined,
    LogoutOutlined,
    MenuFoldOutlined,
    MenuUnfoldOutlined,
    NodeIndexOutlined,
    SafetyOutlined,
    SettingOutlined,
    SwapOutlined,
    TeamOutlined,
    ThunderboltOutlined,
    TranslationOutlined,
    UserOutlined,
    WifiOutlined,
    FileTextOutlined,
    RobotOutlined,
    MessageOutlined,
    MonitorOutlined,
    DesktopOutlined,
    EyeOutlined,
    CodeOutlined,
    NotificationOutlined,
} from '@ant-design/icons'
import {useTranslation} from 'react-i18next'
import {useAppStore, wallpaperList, hasWallpaper, getWallpaperBg} from '../store/appStore'
import type {WallpaperKey} from '../store/appStore'
import i18n from '../i18n'

const {Sider, Header, Content} = Layout
const {Text} = Typography


const MainLayout: React.FC = () => {
    const {t} = useTranslation()
    const navigate = useNavigate()
    const location = useLocation()
    const {username, collapsed, setCollapsed, logout, language, setLanguage, uiMode, setUIMode, wallpaper, setWallpaper} = useAppStore()
    const {token} = antTheme.useToken()
    const hasWp = hasWallpaper(wallpaper)
    const isDark = uiMode === 'dark'
    const isLight = !isDark
    const animeBgUrl = getWallpaperBg(wallpaper)

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
            icon: <DashboardOutlined/>,
            label: t('menu.dashboard'),
        },
        {
            key: 'port-mapping',
            icon: <SwapOutlined/>,
            label: t('menu.portMapping'),
            children: [
                {key: 'port-forward', icon: <NodeIndexOutlined/>, label: t('menu.portForward')},
                {key: 'stun', icon: <WifiOutlined/>, label: t('menu.stun')},
                {key: 'frp/client', icon: <ApiOutlined/>, label: t('menu.frpc')},
                {key: 'frp/server', icon: <CloudServerOutlined/>, label: t('menu.frps')},
                {key: 'cftunnel', icon: <GlobalOutlined/>, label: t('menu.cftunnel')},
                {key: 'tunservice', icon: <ApartmentOutlined/>, label: t('menu.tunservice')},
            ],
        },
        {
            key: 'network',
            icon: <ApartmentOutlined/>,
            label: t('menu.networkGroup'),
            children: [
                {key: 'nps/client', icon: <ApiOutlined/>, label: t('menu.npsClient')},
                {key: 'nps/server', icon: <CloudServerOutlined/>, label: t('menu.npsServer')},
                {key: 'easytier/client', icon: <ApiOutlined/>, label: t('menu.easytierClient')},
                {key: 'easytier/server', icon: <CloudServerOutlined/>, label: t('menu.easytierServer')},
                {key: 'wireguard', icon: <SafetyOutlined/>, label: t('menu.wireguard')},
            ],
        },
        {
            key: 'mesh',
            icon: <ClusterOutlined/>,
            label: t('menu.meshManagement'),
            children: [
                {key: 'mesh/nodes', icon: <ClusterOutlined/>, label: t('menu.meshNodes')},
                {key: 'mesh/tunnels', icon: <SwapOutlined/>, label: t('menu.meshTunnels')},
                {key: 'mesh/topology', icon: <ApartmentOutlined/>, label: t('menu.meshTopology')},
                {key: 'mesh/events', icon: <HistoryOutlined/>, label: t('menu.meshEvents')},
            ],
        },
        {
            key: 'web-service',
            icon: <GlobalOutlined/>,
            label: t('menu.webService'),
            children: [
                {key: 'ddns', icon: <GlobalOutlined/>, label: t('menu.ddns')},
                {key: 'caddy', icon: <LinkOutlined/>, label: t('menu.caddy')},
            ],
        },
        {
            key: 'security',
            icon: <SafetyOutlined/>,
            label: t('menu.security'),
            children: [
                {key: 'ipdb', icon: <DatabaseOutlined/>, label: t('menu.ipdb')},
                {key: 'security/firewall', icon: <FireOutlined/>, label: t('menu.firewall')},
                {key: 'access', icon: <FilterOutlined/>, label: t('menu.access')},
                {key: 'security/waf', icon: <BugOutlined/>, label: t('menu.waf')},
            ],
        },
        {
            key: 'intranet',
            icon: <ControlOutlined/>,
            label: t('menu.localTools'),
            children: [
                {key: 'dnsmasq', icon: <ControlOutlined/>, label: t('menu.dnsmasq')},
                {key: 'wol', icon: <ThunderboltOutlined/>, label: t('menu.wol')},
                {key: 'storage', icon: <FolderOpenOutlined/>, label: t('menu.storage')},
                {key: 'cron', icon: <ClockCircleOutlined/>, label: t('menu.cron')},
            ],
        },
        {
            key: 'domain',
            icon: <KeyOutlined/>,
            label: t('menu.domain'),
            children: [

                {key: 'domain/account', icon: <UserOutlined/>, label: t('menu.domainAccount')},
                {key: 'domain/info', icon: <DatabaseOutlined/>, label: t('menu.domainInfo')},
                {key: 'domain/cert-account', icon: <SafetyOutlined/>, label: t('menu.certAccount')},
                {key: 'domain/cert', icon: <KeyOutlined/>, label: t('menu.domainCert')},

            ],
        },
        {
            key: 'callback',
            icon: <BellOutlined/>,
            label: t('menu.callback'),
            children: [
                {key: 'callback/account', icon: <UserOutlined/>, label: t('menu.callbackAccount')},
                {key: 'callback/task', icon: <ClockCircleOutlined/>, label: t('menu.callbackTask')},
            ],
        },
        {
            key: 'ai',
            icon: <RobotOutlined/>,
            label: t('menu.aiManagement'),
            children: [
                {key: 'ai/chat', icon: <MessageOutlined/>, label: t('menu.aiChat')},
                {key: 'ai/assistant', icon: <RobotOutlined/>, label: t('menu.aiAssistant')},
                {key: 'ai/cron-task', icon: <ClockCircleOutlined/>, label: t('menu.aiCronTask')},
                {key: 'ai/plugin', icon: <ApiOutlined/>, label: t('menu.aiPlugin')},
                {key: 'ai/provider', icon: <CloudServerOutlined/>, label: t('menu.aiProvider')},
            ],
        },
        {
            key: 'monitor',
            icon: <MonitorOutlined/>,
            label: t('menu.serverMonitor'),
            children: [
                {key: 'monitor/dashboard', icon: <DashboardOutlined/>, label: t('menu.monitorDashboard')},
                {key: 'monitor/servers', icon: <DesktopOutlined/>, label: t('menu.monitorServers')},
                {key: 'monitor/probes', icon: <EyeOutlined/>, label: t('menu.monitorProbes')},
                {key: 'monitor/tasks', icon: <ClockCircleOutlined/>, label: t('menu.monitorTasks')},
                {key: 'monitor/alerts', icon: <BellOutlined/>, label: t('menu.monitorAlerts')},
                {key: 'monitor/ddns', icon: <GlobalOutlined/>, label: t('menu.monitorDDNS')},
                {key: 'monitor/notifications', icon: <NotificationOutlined/>, label: t('menu.monitorNotifications')},
                {key: 'monitor/tunnels', icon: <ApiOutlined/>, label: t('menu.monitorTunnels')},
                {key: 'monitor/terminal', icon: <CodeOutlined/>, label: t('menu.monitorTerminal')},
            ],
        },
        {
            key: 'admin',
            icon: <SettingOutlined/>,
            label: t('menu.admin'),
            children: [
                {key: 'admin/logs', icon: <FileTextOutlined/>, label: t('menu.adminLogs')},
                {key: 'admin/users', icon: <TeamOutlined/>, label: t('menu.adminUsers')},
                {key: 'admin/oauth-providers', icon: <KeyOutlined/>, label: t('menu.oauthProviders') || 'OAuth 登录'},
                {key: 'settings', icon: <SettingOutlined/>, label: t('menu.settings')},
            ],
        },
    ]

    const userMenuItems: MenuProps['items'] = [
        {
            key: 'settings',
            icon: <SettingOutlined/>,
            label: t('menu.settings'),
            onClick: () => navigate('/settings'),
        },
        {type: 'divider'},
        {
            key: 'logout',
            icon: <LogoutOutlined/>,
            label: t('common.logout'),
            danger: true,
            onClick: () => {
                logout()
                navigate('/login')
            },
        },
    ]

    // 侧边栏背景
    const siderBg = hasWp
        ? (isDark ? 'rgba(10,15,30,0.8)' : 'rgba(255,255,255,0.7)')
        : isDark
            ? '#0a1028'
            : '#001529'

    const logoBorderColor = hasWp && !isDark
        ? 'rgba(0,0,0,0.08)'
        : isLight
            ? 'rgba(255,255,255,0.08)'
            : 'rgba(255,255,255,0.1)'

    // 顶部栏背景
    const headerBg = hasWp
        ? (isDark ? 'rgba(10,15,30,0.6)' : 'rgba(255,255,255,0.4)')
        : isDark
            ? '#0f1838'
            : token.colorBgContainer

    const headerBorder = hasWp
        ? '1px solid rgba(255,255,255,0.08)'
        : `1px solid ${token.colorBorderSecondary}`

    // 内容区背景
    const contentBg = hasWp
        ? 'transparent'
        : isDark
            ? '#0a1028'
            : '#f0f2f5'

    return (
        <>
            {/* 壁纸背景由 index.css 的 body::before 渲染，避免 React hydration 延迟 */}

            <Layout style={{height: '100vh'}}>
                {/* 侧边栏 */}
                <Sider
                    collapsible
                    collapsed={collapsed}
                    onCollapse={setCollapsed}
                    trigger={null}
                    width={220}
                    style={{
                        background: siderBg,
                        backdropFilter: hasWp ? 'blur(20px)' : undefined,
                        WebkitBackdropFilter: hasWp ? 'blur(20px)' : undefined,
                        boxShadow: isDark
                            ? '2px 0 12px rgba(0,0,0,0.5)'
                            : hasWp
                                ? '2px 0 20px rgba(0,0,0,0.1), inset -1px 0 0 rgba(255,255,255,0.5)'
                                : '2px 0 8px rgba(0,0,0,0.12)',
                        overflow: 'auto',
                        height: '100vh',
                        position: 'fixed',
                        left: 0,
                        top: 0,
                        bottom: 0,
                        zIndex: 100,
                        borderRight: hasWp ? (isDark ? '1px solid rgba(255,255,255,0.08)' : '1px solid rgba(0,0,0,0.06)') : 'none',
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
                            <WifiOutlined style={{color: '#fff', fontSize: 17}}/>
                        </div>
                        {!collapsed && (
                            <div style={{marginLeft: 12}}>
                                <Text style={{
                                    color: hasWp && !isDark ? 'rgba(0,0,0,0.88)' : '#fff', fontSize: 16, fontWeight: 700,
                                    letterSpacing: '0.3px', whiteSpace: 'nowrap',
                                    display: 'block', lineHeight: 1.2,
                                }}>
                                    NetPanel
                                </Text>
                                <Text style={{
                                    color: hasWp && !isDark ? 'rgba(0,0,0,0.4)' : 'rgba(255,255,255,0.35)', fontSize: 10,
                                    letterSpacing: '1px', whiteSpace: 'nowrap',
                                    display: 'block', lineHeight: 1,
                                }}>
                                    NETWORK MANAGER
                                </Text>
                            </div>
                        )}
                    </div>

                    {/* 菜单 */}
                    <div style={{flex: 1, overflow: 'auto', paddingTop: 6}}>
                        <Menu
                            theme={hasWp && !isDark ? 'light' : 'dark'}
                            mode="inline"
                            selectedKeys={[selectedKey]}
                            defaultOpenKeys={openKeys}
                            items={menuItems}
                            onClick={({key}) => navigate(`/${key}`)}
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
                            <Text style={{color: hasWp && !isDark ? 'rgba(0,0,0,0.3)' : 'rgba(255,255,255,0.2)', fontSize: 11, letterSpacing: '0.5px'}}>
                                v0.2.0
                            </Text>
                        </div>
                    )}
                </Sider>

                {/* 右侧主区域 */}
                <Layout style={{marginLeft: collapsed ? 80 : 220, transition: 'margin-left 0.2s'}}>
                    {/* 顶部栏 */}
                    <Header style={{
                        padding: '0 20px',
                        background: headerBg,
                        backdropFilter: hasWp ? 'blur(20px)' : undefined,
                        WebkitBackdropFilter: hasWp ? 'blur(20px)' : undefined,
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
                            : hasWp
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
                                {collapsed ? <MenuUnfoldOutlined/> : <MenuFoldOutlined/>}
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
                                        letterSpacing: '0.3px',
                                    }}
                                    onMouseEnter={e => (e.currentTarget.style.background = token.colorFillSecondary)}
                                    onMouseLeave={e => (e.currentTarget.style.background = 'transparent')}
                                >
                                    <TranslationOutlined style={{fontSize: 13}}/>
                                    <span>{language === 'zh' ? '中文' : 'EN'}</span>
                                </div>
                            </Tooltip>

                            {/* 白天/黑夜模式切换 */}
                            <div
                                onClick={() => setUIMode(isDark ? 'light' : 'dark')}
                                style={{
                                    cursor: 'pointer',
                                    padding: '5px 10px',
                                    borderRadius: 8,
                                    display: 'flex', alignItems: 'center', gap: 5,
                                    color: token.colorTextSecondary,
                                    fontSize: 12, fontWeight: 500,
                                }}
                            >
                                <span style={{fontSize: 14}}>{isDark ? '🌙' : '☀️'}</span>
                                <span style={{fontSize: 11}}>{isDark ? '黑夜' : '白天'}</span>
                            </div>

                            {/* 主题/壁纸选择 */}
                            <Dropdown
                                menu={{
                                    items: [
                                        {
                                            key: 'theme-grid',
                                            type: 'group',
                                            label: <div style={{
                                                display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)',
                                                gap: 8, padding: '8px 0', minWidth: 240,
                                            }}>
                                                {wallpaperList.map(w => (
                                                    <div key={w.key} onClick={(e) => { e.stopPropagation(); setWallpaper(w.key); }}
                                                        style={{
                                                            display: 'flex', flexDirection: 'column', alignItems: 'center',
                                                            gap: 4, padding: '8px 4px', borderRadius: 8, cursor: 'pointer',
                                                            background: wallpaper === w.key ? 'rgba(22,119,255,0.15)' : 'transparent',
                                                            border: wallpaper === w.key ? '1px solid rgba(22,119,255,0.4)' : '1px solid transparent',
                                                            transition: 'all 0.2s',
                                                        }}
                                                    >
                                                        <span style={{fontSize: 20}}>{w.icon}</span>
                                                        <span style={{fontSize: 10, opacity: 0.9}}>{w.name}</span>
                                                    </div>
                                                ))}
                                            </div>,
                                        },
                                    ],
                                }}
                                placement="bottomRight"
                                trigger={['click']}
                            >
                                <div style={{
                                    cursor: 'pointer',
                                    padding: '5px 10px',
                                    borderRadius: 8,
                                    display: 'flex', alignItems: 'center', gap: 5,
                                    color: token.colorTextSecondary,
                                    fontSize: 12, fontWeight: 500,
                                }}>
                                    <span style={{fontSize: 14}}>
                                        {wallpaperList.find(w => w.key === wallpaper)?.icon || '🎯'}
                                    </span>
                                    <span style={{fontSize: 11}}>主题</span>
                                </div>
                            </Dropdown>

                            {/* 用户菜单 */}
                            <Dropdown menu={{items: userMenuItems}} placement="bottomRight" arrow>
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
                                        icon={<UserOutlined/>}
                                    />
                                    <Text style={{fontSize: 13, fontWeight: 500}}>{username || 'admin'}</Text>
                                </Space>
                            </Dropdown>
                        </Space>
                    </Header>

                    {/* 内容区（背景由 index.css 根据 data-wallpaper 控制，避免 hydration 闪烁） */}
                    {/* 关键：滚动放在内部 div，Content 本身不滚动，否则会破坏子 card 的 backdrop-filter */}
                    <Content style={{
                        height: 'calc(100vh - 56px)',
                        overflow: 'hidden',
                    }}>
                        <div className="page-enter" style={{
                            height: '100%',
                            overflow: 'auto',
                            padding: 20,
                        }}>
                            <Outlet/>
                        </div>
                    </Content>
                </Layout>
            </Layout>
        </>
    )
}

function getOpenKeys(pathname: string): string[] {
    if (pathname.startsWith('/port-forward') || pathname.startsWith('/stun') || pathname.startsWith('/frp')) return ['port-mapping']
    if (pathname.startsWith('/nps') || pathname.startsWith('/easytier') || pathname.startsWith('/wireguard')) return ['network']
    if (pathname.startsWith('/mesh')) return ['mesh']
    if (pathname.startsWith('/ddns') || pathname.startsWith('/caddy')) return ['web-service']
    if (pathname.startsWith('/ipdb') || pathname.startsWith('/access') || pathname.startsWith('/security')) return ['security']
    if (pathname.startsWith('/dnsmasq') || pathname.startsWith('/wol') || pathname.startsWith('/storage') || pathname.startsWith('/cron')) return ['intranet']
    if (pathname.startsWith('/domain')) return ['domain']
    if (pathname.startsWith('/callback')) return ['callback']
    if (pathname.startsWith('/ai')) return ['ai']
    if (pathname.startsWith('/admin') || pathname.startsWith('/settings')) return ['admin']
    return []
}

export default MainLayout
