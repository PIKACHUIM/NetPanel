import React, { useEffect, useState, useCallback } from 'react'
import { Row, Col, Typography, Spin, Tooltip, Button, theme as antTheme, Tag } from 'antd'
import {
  ReloadOutlined, CloudServerOutlined,
  ApiOutlined, WifiOutlined, GlobalOutlined, LinkOutlined,
  ThunderboltOutlined, ClockCircleOutlined, FolderOpenOutlined,
  FilterOutlined, SwapOutlined, ApartmentOutlined, CheckCircleFilled,
  MinusCircleFilled, DesktopOutlined, HddOutlined, DatabaseOutlined,
  RocketOutlined, PlayCircleOutlined,
} from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import {
  systemApi,
  portForwardApi, stunApi,
  frpcApi, frpsApi,
  npsClientApi, npsServerApi,
  easytierClientApi, easytierServerApi,
  ddnsApi, caddyApi,
  cronApi, storageApi, accessApi, wolApi,
} from '../api'
import { useAppStore } from '../store/appStore'

const { Text } = Typography

interface SystemInfo {
  hostname: string
  os: string
  arch: string
  version: string
  go_version: string
  uptime: number
}

interface NetInterface {
  name: string
  ips: string
}

interface SystemStats {
  cpu_usage: number
  cpu_cores: number
  mem_total: number
  mem_used: number
  mem_free: number
  mem_percent: number
  swap_total: number
  swap_used: number
  swap_percent: number
  disk_total: number
  disk_used: number
  disk_free: number
  disk_percent: number
}

interface ServiceStatus {
  name: string
  type: string
  status: string
  count: number
  running: number
  names: string[]  // 已有配置的名称列表
}

// 格式化字节
const formatBytes = (bytes: number) => {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return `${(bytes / Math.pow(k, i)).toFixed(1)} ${sizes[i]}`
}

// 格式化运行时间
const formatUptime = (seconds: number) => {
  const d = Math.floor(seconds / 86400)
  const h = Math.floor((seconds % 86400) / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  if (d > 0) return `${d}天 ${h}小时`
  if (h > 0) return `${h}小时 ${m}分钟`
  return `${m}分钟`
}

// 获取资源颜色
const getResourceColor = (val: number) => {
  if (val >= 90) return { main: '#ff4d4f', glow: 'rgba(255,77,79,0.3)', track: 'rgba(255,77,79,0.12)' }
  if (val >= 70) return { main: '#faad14', glow: 'rgba(250,173,20,0.3)', track: 'rgba(250,173,20,0.12)' }
  return { main: '#52c41a', glow: 'rgba(82,196,26,0.3)', track: 'rgba(82,196,26,0.12)' }
}

// 服务图标映射
const serviceIcons: Record<string, React.ReactNode> = {
  port_forward: <SwapOutlined />,
  stun: <WifiOutlined />,
  frpc: <ApiOutlined />,
  frps: <CloudServerOutlined />,
  nps_client: <ApiOutlined />,
  nps_server: <CloudServerOutlined />,
  easytier_client: <ApartmentOutlined />,
  easytier_server: <ApartmentOutlined />,
  ddns: <GlobalOutlined />,
  caddy: <LinkOutlined />,
  wol: <ThunderboltOutlined />,
  cron: <ClockCircleOutlined />,
  storage: <FolderOpenOutlined />,
  access: <FilterOutlined />,
}

// 服务分组颜色
const serviceGroupColor: Record<string, { main: string; bg: string; border: string }> = {
  port_forward: { main: '#1677ff', bg: 'rgba(22,119,255,0.08)', border: 'rgba(22,119,255,0.2)' },
  stun:         { main: '#1677ff', bg: 'rgba(22,119,255,0.08)', border: 'rgba(22,119,255,0.2)' },
  frpc:         { main: '#722ed1', bg: 'rgba(114,46,209,0.08)', border: 'rgba(114,46,209,0.2)' },
  frps:         { main: '#722ed1', bg: 'rgba(114,46,209,0.08)', border: 'rgba(114,46,209,0.2)' },
  nps_client:   { main: '#eb2f96', bg: 'rgba(235,47,150,0.08)', border: 'rgba(235,47,150,0.2)' },
  nps_server:   { main: '#eb2f96', bg: 'rgba(235,47,150,0.08)', border: 'rgba(235,47,150,0.2)' },
  easytier_client: { main: '#13c2c2', bg: 'rgba(19,194,194,0.08)', border: 'rgba(19,194,194,0.2)' },
  easytier_server: { main: '#13c2c2', bg: 'rgba(19,194,194,0.08)', border: 'rgba(19,194,194,0.2)' },
  ddns:    { main: '#fa8c16', bg: 'rgba(250,140,22,0.08)', border: 'rgba(250,140,22,0.2)' },
  caddy:   { main: '#fa8c16', bg: 'rgba(250,140,22,0.08)', border: 'rgba(250,140,22,0.2)' },
  wol:     { main: '#52c41a', bg: 'rgba(82,196,26,0.08)', border: 'rgba(82,196,26,0.2)' },
  cron:    { main: '#52c41a', bg: 'rgba(82,196,26,0.08)', border: 'rgba(82,196,26,0.2)' },
  storage: { main: '#52c41a', bg: 'rgba(82,196,26,0.08)', border: 'rgba(82,196,26,0.2)' },
  access:  { main: '#52c41a', bg: 'rgba(82,196,26,0.08)', border: 'rgba(82,196,26,0.2)' },
}

// 环形进度条（SVG实现，更精致）
const RingProgress: React.FC<{
  value: number
  size?: number
  strokeWidth?: number
  label: string
  subtitle?: string
  icon: React.ReactNode
}> = ({ value, size = 120, strokeWidth = 8, label, subtitle, icon }) => {
  const { main, glow, track } = getResourceColor(value)
  const radius = (size - strokeWidth) / 2
  const circumference = 2 * Math.PI * radius
  const dashOffset = circumference * (1 - Math.min(value, 100) / 100)

  return (
    <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 10 }}>
      <div style={{ position: 'relative', width: size, height: size }}>
        <svg width={size} height={size} style={{ transform: 'rotate(-90deg)' }}>
          {/* 背景轨道 */}
          <circle
            cx={size / 2} cy={size / 2} r={radius}
            fill="none" stroke={track} strokeWidth={strokeWidth}
          />
          {/* 进度弧 */}
          <circle
            cx={size / 2} cy={size / 2} r={radius}
            fill="none"
            stroke={main}
            strokeWidth={strokeWidth}
            strokeLinecap="round"
            strokeDasharray={circumference}
            strokeDashoffset={dashOffset}
            style={{
              transition: 'stroke-dashoffset 0.8s cubic-bezier(0.4,0,0.2,1)',
              filter: `drop-shadow(0 0 6px ${glow})`,
            }}
          />
        </svg>
        {/* 中心内容 */}
        <div style={{
          position: 'absolute', inset: 0,
          display: 'flex', flexDirection: 'column',
          alignItems: 'center', justifyContent: 'center',
          gap: 2,
        }}>
          <span style={{ color: main, fontSize: 16, lineHeight: 1 }}>{icon}</span>
          <span style={{ fontSize: 18, fontWeight: 700, color: main, lineHeight: 1 }}>
            {Math.round(value)}%
          </span>
        </div>
      </div>
      <div style={{ textAlign: 'center' }}>
        <div style={{ fontSize: 13, fontWeight: 600, lineHeight: 1.3 }}>{label}</div>
        {subtitle && (
          <div style={{ fontSize: 11, color: 'rgba(128,128,128,0.8)', marginTop: 3, lineHeight: 1.3 }}>
            {subtitle}
          </div>
        )}
      </div>
    </div>
  )
}

// 服务卡片
const ServiceCard: React.FC<{ svc: ServiceStatus; isDark: boolean }> = ({ svc, isDark }) => {
  const isRunning = svc.running > 0
  const hasConfig = svc.count > 0
  const colors = serviceGroupColor[svc.type] || { main: '#1677ff', bg: 'rgba(22,119,255,0.08)', border: 'rgba(22,119,255,0.2)' }
  const percent = hasConfig ? Math.round((svc.running / svc.count) * 100) : 0

  return (
    <div style={{
      padding: '12px 14px',
      borderRadius: 12,
      border: `1px solid ${isRunning ? colors.border : isDark ? 'rgba(255,255,255,0.06)' : 'rgba(0,0,0,0.06)'}`,
      background: isRunning ? colors.bg : isDark ? 'rgba(255,255,255,0.03)' : 'rgba(0,0,0,0.02)',
      cursor: 'default',
      transition: 'all 0.2s ease',
      position: 'relative',
      overflow: 'hidden',
      height: '100%',
      boxSizing: 'border-box',
    }}
      onMouseEnter={e => {
        e.currentTarget.style.transform = 'translateY(-3px)'
        e.currentTarget.style.boxShadow = isRunning
          ? `0 8px 24px ${colors.bg.replace('0.08', '0.25')}`
          : isDark ? '0 8px 24px rgba(0,0,0,0.3)' : '0 8px 24px rgba(0,0,0,0.1)'
      }}
      onMouseLeave={e => {
        e.currentTarget.style.transform = 'translateY(0)'
        e.currentTarget.style.boxShadow = 'none'
      }}
    >
      {/* 顶部彩色指示条 */}
      {isRunning && (
        <div style={{
          position: 'absolute', top: 0, left: 0, right: 0, height: 2,
          background: `linear-gradient(90deg, ${colors.main}, ${colors.main}88)`,
          borderRadius: '12px 12px 0 0',
        }} />
      )}

      {/* 标题行 */}
      <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', marginBottom: 6 }}>
        <span style={{
          color: isRunning ? colors.main : isDark ? 'rgba(255,255,255,0.25)' : 'rgba(0,0,0,0.2)',
          fontSize: 18,
        }}>
          {serviceIcons[svc.type] || <ApiOutlined />}
        </span>
        {hasConfig ? (
          isRunning
            ? <CheckCircleFilled style={{ color: colors.main, fontSize: 12 }} />
            : <MinusCircleFilled style={{ color: isDark ? 'rgba(255,255,255,0.2)' : 'rgba(0,0,0,0.2)', fontSize: 12 }} />
        ) : null}
      </div>

      {/* 服务名 + 运行数 */}
      <div style={{ fontSize: 12, fontWeight: 600, lineHeight: 1.3, marginBottom: 2 }}>{svc.name}</div>
      <div style={{
        fontSize: 11,
        color: isRunning ? colors.main : isDark ? 'rgba(255,255,255,0.3)' : 'rgba(0,0,0,0.3)',
        fontWeight: isRunning ? 500 : 400,
        marginBottom: 8,
      }}>
        {hasConfig ? `${svc.running}/${svc.count} 运行` : '未配置'}
      </div>

      {/* 进度条（有配置时显示实际进度，未配置时显示带纹理的空状态） */}
      <div style={{
        height: 4, borderRadius: 2,
        background: isDark ? 'rgba(255,255,255,0.08)' : 'rgba(0,0,0,0.07)',
        marginBottom: 8, overflow: 'hidden',
        position: 'relative',
      }}>
        {!hasConfig && (
          <div style={{
            position: 'absolute', inset: 0,
            backgroundImage: `repeating-linear-gradient(90deg, transparent, transparent 4px, ${isDark ? 'rgba(255,255,255,0.06)' : 'rgba(0,0,0,0.06)'} 4px, ${isDark ? 'rgba(255,255,255,0.06)' : 'rgba(0,0,0,0.06)'} 8px)`,
          }} />
        )}
        <div style={{
          height: '100%',
          width: hasConfig ? `${percent}%` : '0%',
          borderRadius: 2,
          background: isRunning
            ? `linear-gradient(90deg, ${colors.main}, ${colors.main}cc)`
            : isDark ? 'rgba(255,255,255,0.15)' : 'rgba(0,0,0,0.12)',
          transition: 'width 0.6s cubic-bezier(0.4,0,0.2,1)',
        }} />
      </div>

      {/* 配置名称 tag 列表 */}
      {svc.names.length > 0 && (
        <div style={{ display: 'flex', flexWrap: 'wrap', gap: 3 }}>
          {svc.names.slice(0, 3).map((n, i) => (
            <Tag
              key={i}
              style={{
                margin: 0,
                fontSize: 10,
                lineHeight: '16px',
                padding: '0 5px',
                borderRadius: 4,
                border: `1px solid ${isRunning ? colors.border : isDark ? 'rgba(255,255,255,0.1)' : 'rgba(0,0,0,0.1)'}`,
                background: isRunning ? colors.bg : isDark ? 'rgba(255,255,255,0.04)' : 'rgba(0,0,0,0.03)',
                color: isRunning ? colors.main : isDark ? 'rgba(255,255,255,0.45)' : 'rgba(0,0,0,0.45)',
                maxWidth: 80,
                overflow: 'hidden',
                textOverflow: 'ellipsis',
                whiteSpace: 'nowrap',
              }}
            >
              {n}
            </Tag>
          ))}
          {svc.names.length > 3 && (
            <Tooltip title={svc.names.slice(3).join('、')}>
              <Tag style={{
                margin: 0, fontSize: 10, lineHeight: '16px', padding: '0 5px', borderRadius: 4,
                border: `1px solid ${isDark ? 'rgba(255,255,255,0.1)' : 'rgba(0,0,0,0.1)'}`,
                background: 'transparent',
                color: isDark ? 'rgba(255,255,255,0.35)' : 'rgba(0,0,0,0.35)',
                cursor: 'pointer',
              }}>
                +{svc.names.length - 3}
              </Tag>
            </Tooltip>
          )}
        </div>
      )}
    </div>
  )
}

// 信息条目
const InfoItem: React.FC<{
  icon: React.ReactNode
  label: string
  value: string
  accent?: string
  isDark: boolean
}> = ({ icon, label, value, accent, isDark }) => (
  <div style={{
    display: 'flex',
    alignItems: 'center',
    gap: 10,
    padding: '9px 12px',
    borderRadius: 8,
    background: isDark ? 'rgba(255,255,255,0.03)' : 'rgba(0,0,0,0.02)',
    border: `1px solid ${isDark ? 'rgba(255,255,255,0.05)' : 'rgba(0,0,0,0.04)'}`,
  }}>
    <span style={{ color: accent || '#1677ff', fontSize: 15, flexShrink: 0 }}>{icon}</span>
    <div style={{ flex: 1, minWidth: 0 }}>
      <div style={{ fontSize: 11, color: isDark ? 'rgba(255,255,255,0.4)' : 'rgba(0,0,0,0.4)', lineHeight: 1.2 }}>{label}</div>
      <div style={{
        fontSize: 12, fontWeight: 600, lineHeight: 1.4,
        overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
      }}>{value}</div>
    </div>
  </div>
)

// 顶部迷你资源卡片
const MiniResourceCard: React.FC<{
  icon: React.ReactNode
  label: string
  value: number
  subtitle: string
  color: string
  isDark: boolean
}> = ({ icon, label, value, subtitle, color, isDark }) => {
  const { main, track } = getResourceColor(value)
  const finalColor = color || main
  return (
    <div style={{
      flex: 1, minWidth: 120,
      padding: '14px 16px',
      borderRadius: 14,
      background: isDark ? 'rgba(255,255,255,0.04)' : '#fff',
      border: `1px solid ${isDark ? 'rgba(255,255,255,0.08)' : 'rgba(0,0,0,0.07)'}`,
      boxShadow: isDark ? '0 2px 12px rgba(0,0,0,0.25)' : '0 2px 12px rgba(0,0,0,0.05)',
      position: 'relative', overflow: 'hidden',
    }}>
      {/* 左侧彩色竖条 */}
      <div style={{
        position: 'absolute', left: 0, top: 8, bottom: 8, width: 3,
        borderRadius: '0 3px 3px 0',
        background: finalColor,
      }} />
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 8 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
          <span style={{ color: finalColor, fontSize: 14 }}>{icon}</span>
          <span style={{ fontSize: 12, fontWeight: 600 }}>{label}</span>
        </div>
        <span style={{ fontSize: 18, fontWeight: 700, color: finalColor, lineHeight: 1 }}>
          {Math.round(value)}%
        </span>
      </div>
      {/* 进度条 */}
      <div style={{
        height: 5, borderRadius: 3,
        background: track,
        overflow: 'hidden',
        marginBottom: 5,
      }}>
        <div style={{
          height: '100%', width: `${Math.min(value, 100)}%`, borderRadius: 3,
          background: `linear-gradient(90deg, ${finalColor}cc, ${finalColor})`,
          transition: 'width 0.8s cubic-bezier(0.4,0,0.2,1)',
        }} />
      </div>
      <div style={{ fontSize: 10, color: isDark ? 'rgba(255,255,255,0.4)' : 'rgba(0,0,0,0.4)' }}>{subtitle}</div>
    </div>
  )
}

// 顶部信息卡片（非百分比）
const MiniInfoCard: React.FC<{
  icon: React.ReactNode
  label: string
  value: string
  sub?: string
  color: string
  isDark: boolean
}> = ({ icon, label, value, sub, color, isDark }) => (
  <div style={{
    flex: 1, minWidth: 120,
    padding: '14px 16px',
    borderRadius: 14,
    background: isDark ? 'rgba(255,255,255,0.04)' : '#fff',
    border: `1px solid ${isDark ? 'rgba(255,255,255,0.08)' : 'rgba(0,0,0,0.07)'}`,
    boxShadow: isDark ? '0 2px 12px rgba(0,0,0,0.25)' : '0 2px 12px rgba(0,0,0,0.05)',
    position: 'relative', overflow: 'hidden',
  }}>
    <div style={{
      position: 'absolute', left: 0, top: 8, bottom: 8, width: 3,
      borderRadius: '0 3px 3px 0',
      background: color,
    }} />
    <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginBottom: 6 }}>
      <span style={{ color, fontSize: 14 }}>{icon}</span>
      <span style={{ fontSize: 12, fontWeight: 600 }}>{label}</span>
    </div>
    <div style={{ fontSize: 18, fontWeight: 700, lineHeight: 1.2, color }}>{value}</div>
    {sub && <div style={{ fontSize: 10, color: isDark ? 'rgba(255,255,255,0.4)' : 'rgba(0,0,0,0.4)', marginTop: 3 }}>{sub}</div>}
  </div>
)

const Dashboard: React.FC = () => {
  const { t } = useTranslation()
  const { theme } = useAppStore()
  const { token } = antTheme.useToken()
  const [info, setInfo] = useState<SystemInfo | null>(null)
  const [stats, setStats] = useState<SystemStats | null>(null)
  const [services, setServices] = useState<ServiceStatus[]>(defaultServices)
  const [netInterfaces, setNetInterfaces] = useState<NetInterface[]>([])
  const [loading, setLoading] = useState(true)
  const [refreshing, setRefreshing] = useState(false)

  const isDark = theme?.includes('dark') || token.colorBgBase === '#000'

  const fetchInfo = useCallback(async (isManual = false) => {
    if (isManual) setRefreshing(true)
    try {
      // 并行请求所有数据
      const [infoRes, statsRes, ...svcResults] = await Promise.allSettled([
        systemApi.getInfo(),
        systemApi.getStats(),
        portForwardApi.list(),
        stunApi.list(),
        frpcApi.list(),
        frpsApi.list(),
        npsClientApi.list(),
        npsServerApi.list(),
        easytierClientApi.list(),
        easytierServerApi.list(),
        ddnsApi.list(),
        caddyApi.list(),
        cronApi.list(),
        storageApi.list(),
        accessApi.list(),
        wolApi.list(),
      ])

      if (infoRes.status === 'fulfilled') {
        setInfo((infoRes.value as any).data)
      }
      if (statsRes.status === 'fulfilled') {
        setStats((statsRes.value as any).data)
      }
      // 获取网络接口
      try {
        const ifaceRes = await systemApi.getInterfaces()
        setNetInterfaces((ifaceRes as any).data || [])
      } catch { /* ignore */ }

      // 服务列表顺序与上面 svcResults 对应
      const svcDefs = [
        { name: '端口转发', type: 'port_forward' },
        { name: 'STUN穿透', type: 'stun' },
        { name: 'FRP客户端', type: 'frpc' },
        { name: 'FRP服务端', type: 'frps' },
        { name: 'NPS客户端', type: 'nps_client' },
        { name: 'NPS服务端', type: 'nps_server' },
        { name: 'ET客户端', type: 'easytier_client' },
        { name: 'ET服务端', type: 'easytier_server' },
        { name: '动态域名', type: 'ddns' },
        { name: '网站服务', type: 'caddy' },
        { name: '计划任务', type: 'cron' },
        { name: '网络存储', type: 'storage' },
        { name: '访问控制', type: 'access' },
        { name: '网络唤醒', type: 'wol' },
      ]

          const updatedServices: ServiceStatus[] = svcDefs.map((def, idx) => {
        const result = svcResults[idx]
        if (result.status === 'fulfilled') {
          const list: any[] = (result.value as any).data || []
          let running = 0
          if (def.type === 'access') {
            // 访问控制只有 enable 字段
            running = list.filter((item: any) => item.enable === true).length
          } else if (def.type === 'wol') {
            // WOL 是设备列表，不是运行服务，count 显示设备数，running 固定为 0
            running = 0
          } else {
            running = list.filter((item: any) => item.status === 'running').length
          }
          const names: string[] = list
            .map((item: any) => item.name || item.hostname || item.domain || '')
            .filter(Boolean)
          return { name: def.name, type: def.type, status: running > 0 ? 'running' : 'stopped', count: list.length, running, names }
        }
        return { name: def.name, type: def.type, status: 'stopped', count: 0, running: 0, names: [] }
      })
      setServices(updatedServices)
    } catch {
      // ignore
    } finally {
      setLoading(false)
      if (isManual) setRefreshing(false)
    }
  }, [])

  useEffect(() => {
    fetchInfo()
    const timer = setInterval(() => fetchInfo(), 10000)
    return () => clearInterval(timer)
  }, [fetchInfo])

  const runningCount = services.filter(s => s.running > 0).length
  const totalCount = services.length
  const cpuUsage = stats?.cpu_usage ?? 0
  const memUsage = stats?.mem_percent ?? 0
  const diskUsage = stats?.disk_percent ?? 0

  if (loading) {
    return (
      <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: 400 }}>
        <Spin size="large" />
      </div>
    )
  }

  // 卡片基础样式
  const cardBase: React.CSSProperties = {
    borderRadius: 16,
    border: `1px solid ${isDark ? 'rgba(255,255,255,0.08)' : 'rgba(0,0,0,0.07)'}`,
    background: isDark ? 'rgba(255,255,255,0.04)' : '#fff',
    boxShadow: isDark
      ? '0 4px 24px rgba(0,0,0,0.3)'
      : '0 2px 16px rgba(0,0,0,0.06)',
    padding: 20,
  }

  const sectionTitle = (text: string, accent: string) => (
    <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 14 }}>
      <div style={{ width: 3, height: 16, borderRadius: 2, background: accent }} />
      <span style={{ fontSize: 13, fontWeight: 700, letterSpacing: 0.3 }}>{text}</span>
    </div>
  )

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>

      {/* ── 顶部：标题 + 统计概览 ── */}
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', flexWrap: 'wrap', gap: 12 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
          <div style={{
            width: 42, height: 42, borderRadius: 12,
            background: 'linear-gradient(135deg, #1677ff 0%, #0958d9 100%)',
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            boxShadow: '0 4px 16px rgba(22,119,255,0.4)',
            flexShrink: 0,
          }}>
            <RocketOutlined style={{ color: '#fff', fontSize: 20 }} />
          </div>
          <div>
            <div style={{ fontSize: 18, fontWeight: 700, lineHeight: 1.2 }}>
              {info?.hostname || t('dashboard.title')}
            </div>
            <div style={{ fontSize: 12, color: isDark ? 'rgba(255,255,255,0.45)' : 'rgba(0,0,0,0.45)', marginTop: 2 }}>
              {info?.os} · {info?.arch} · 运行 {info?.uptime ? formatUptime(info.uptime) : '-'}
            </div>
          </div>
        </div>
        <Button
          icon={<ReloadOutlined spin={refreshing} />}
          onClick={() => fetchInfo(true)}
          size="small"
          style={{ borderRadius: 8 }}
        >
          刷新
        </Button>
      </div>

      {/* ── 顶部资源概览横条 ── */}
      <div style={{ display: 'flex', gap: 12, flexWrap: 'wrap' }}>
        {/* 服务总览 */}
        <MiniInfoCard
          icon={<PlayCircleOutlined />}
          label="运行服务"
          value={`${runningCount} / ${totalCount}`}
          sub={`${totalCount - runningCount} 个未运行`}
          color="#1677ff"
          isDark={isDark}
        />
        {/* CPU */}
        <MiniResourceCard
          icon={<DesktopOutlined />}
          label="CPU"
          value={cpuUsage}
          subtitle="处理器占用率"
          color=""
          isDark={isDark}
        />
        {/* 内存 */}
        <MiniResourceCard
          icon={<DatabaseOutlined />}
          label="内存"
          value={memUsage}
          subtitle={stats ? `${formatBytes(stats.mem_used)} / ${formatBytes(stats.mem_total)}` : '-'}
          color=""
          isDark={isDark}
        />
        {/* 磁盘 */}
        <MiniResourceCard
          icon={<HddOutlined />}
          label="磁盘"
          value={diskUsage}
          subtitle={stats ? `${formatBytes(stats.disk_used)} / ${formatBytes(stats.disk_total)}` : '-'}
          color=""
          isDark={isDark}
        />
        {/* 运行时间 */}
        <MiniInfoCard
          icon={<ClockCircleOutlined />}
          label="运行时间"
          value={info?.uptime ? formatUptime(info.uptime) : '-'}
          sub={`${info?.os || '-'} · ${info?.arch || '-'}`}
          color="#722ed1"
          isDark={isDark}
        />
        {/* 主机 */}
        <MiniInfoCard
          icon={<CloudServerOutlined />}
          label="主机"
          value={info?.hostname || '-'}
          sub={`Go ${info?.go_version?.replace('go', '') || '-'} · v${info?.version || 'dev'}`}
          color="#13c2c2"
          isDark={isDark}
        />
      </div>

      {/* ── 主体内容 ── */}
      <Row gutter={[16, 16]}>

        {/* 左列：系统信息 + 资源监控 */}
        <Col xs={24} lg={8}>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>

            {/* 系统信息 */}
            <div style={cardBase}>
            {sectionTitle('系统信息', '#1677ff')}
              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 8 }}>
                <InfoItem icon={<DesktopOutlined />} label="主机名" value={info?.hostname || '-'} isDark={isDark} />
                <InfoItem icon={<CloudServerOutlined />} label="操作系统" value={info?.os || '-'} accent="#722ed1" isDark={isDark} />
                <InfoItem icon={<ApiOutlined />} label="系统架构" value={info?.arch || '-'} accent="#13c2c2" isDark={isDark} />
                <InfoItem icon={<RocketOutlined />} label="程序版本" value={info?.version ? `v${info.version}` : 'dev'} accent="#fa8c16" isDark={isDark} />
                <InfoItem icon={<DatabaseOutlined />} label="Go 版本" value={info?.go_version || '-'} accent="#52c41a" isDark={isDark} />
                <InfoItem icon={<ClockCircleOutlined />} label="运行时间" value={info?.uptime ? formatUptime(info.uptime) : '-'} accent="#eb2f96" isDark={isDark} />
              </div>
              {/* 网络接口 */}
              {netInterfaces.length > 0 && (
                <>
                  <div style={{ height: 1, background: isDark ? 'rgba(255,255,255,0.06)' : 'rgba(0,0,0,0.05)', margin: '12px 0' }} />
                  <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginBottom: 8 }}>
                    <WifiOutlined style={{ color: '#1677ff', fontSize: 13 }} />
                    <span style={{ fontSize: 12, fontWeight: 600 }}>网络接口</span>
                  </div>
                  <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
                    {netInterfaces.slice(0, 5).map((iface, i) => {
                      const ipList = iface.ips ? iface.ips.split(',').filter(ip => !ip.startsWith('127.') && !ip.includes('::1') && ip.trim()) : []
                      if (ipList.length === 0) return null
                      return (
                        <div key={i} style={{
                          padding: '7px 10px',
                          borderRadius: 8,
                          background: isDark ? 'rgba(255,255,255,0.03)' : 'rgba(0,0,0,0.02)',
                          border: `1px solid ${isDark ? 'rgba(255,255,255,0.05)' : 'rgba(0,0,0,0.04)'}`,
                        }}>
                          <div style={{ fontSize: 11, color: isDark ? 'rgba(255,255,255,0.4)' : 'rgba(0,0,0,0.4)', marginBottom: 3 }}>{iface.name}</div>
                          <div style={{ display: 'flex', flexWrap: 'wrap', gap: 4 }}>
                            {ipList.slice(0, 3).map((ip, j) => (
                              <Tag key={j} style={{
                                margin: 0, fontSize: 10, lineHeight: '16px', padding: '0 6px',
                                borderRadius: 4, fontFamily: 'monospace',
                                background: isDark ? 'rgba(22,119,255,0.1)' : 'rgba(22,119,255,0.06)',
                                border: '1px solid rgba(22,119,255,0.2)',
                                color: '#1677ff',
                              }}>{ip.split('/')[0]}</Tag>
                            ))}
                          </div>
                        </div>
                      )
                    })}
                  </div>
                </>
              )}
            </div>

            {/* 资源监控 */}
            <div style={cardBase}>
              {sectionTitle('资源监控', '#52c41a')}
              <div style={{ display: 'flex', justifyContent: 'space-around', alignItems: 'flex-start', flexWrap: 'wrap', gap: 16 }}>
                <RingProgress
                  value={cpuUsage}
                  label="CPU"
                  subtitle={undefined}
                  icon={<DesktopOutlined />}
                />
                <RingProgress
                  value={memUsage}
                  label="内存"
                  subtitle={`${formatBytes(stats?.mem_used ?? 0)} / ${formatBytes(stats?.mem_total ?? 0)}`}
                  icon={<DatabaseOutlined />}
                />
                <RingProgress
                  value={diskUsage}
                  label="磁盘"
                  subtitle={`${formatBytes(stats?.disk_used ?? 0)} / ${formatBytes(stats?.disk_total ?? 0)}`}
                  icon={<HddOutlined />}
                />
              </div>
            </div>

          </div>
        </Col>

        {/* 右列：服务状态 */}
        <Col xs={24} lg={16}>
          <div style={{ ...cardBase, height: '100%' }}>
            {sectionTitle(`服务状态 · ${runningCount} 个运行中`, '#fa8c16')}
            <Row gutter={[10, 10]} style={{ height: 'calc(100% - 65px)' }}>
              {services.map((svc) => (
                <Col key={svc.type} xs={24} sm={12} md={8} style={{ height: '20%' }}>
                  <ServiceCard svc={svc} isDark={isDark} />
                </Col>
              ))}
            </Row>
          </div>
        </Col>

      </Row>
    </div>
  )
}

// 默认服务列表（API 未返回时显示）
const defaultServices: ServiceStatus[] = [
  { name: '端口转发', type: 'port_forward', status: 'stopped', count: 0, running: 0, names: [] },
  { name: 'STUN穿透', type: 'stun', status: 'stopped', count: 0, running: 0, names: [] },
  { name: 'FRP客户端', type: 'frpc', status: 'stopped', count: 0, running: 0, names: [] },
  { name: 'FRP服务端', type: 'frps', status: 'stopped', count: 0, running: 0, names: [] },
  { name: 'NPS客户端', type: 'nps_client', status: 'stopped', count: 0, running: 0, names: [] },
  { name: 'NPS服务端', type: 'nps_server', status: 'stopped', count: 0, running: 0, names: [] },
  { name: 'ET客户端', type: 'easytier_client', status: 'stopped', count: 0, running: 0, names: [] },
  { name: 'ET服务端', type: 'easytier_server', status: 'stopped', count: 0, running: 0, names: [] },
  { name: '动态域名', type: 'ddns', status: 'stopped', count: 0, running: 0, names: [] },
  { name: '网站服务', type: 'caddy', status: 'stopped', count: 0, running: 0, names: [] },
  { name: '计划任务', type: 'cron', status: 'stopped', count: 0, running: 0, names: [] },
  { name: '网络存储', type: 'storage', status: 'stopped', count: 0, running: 0, names: [] },
  { name: '访问控制', type: 'access', status: 'stopped', count: 0, running: 0, names: [] },
  { name: '网络唤醒', type: 'wol', status: 'stopped', count: 0, running: 0, names: [] },
]

export default Dashboard
