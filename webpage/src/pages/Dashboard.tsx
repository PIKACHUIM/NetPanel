import React, { useEffect, useState, useCallback } from 'react'
import { Row, Col, Card, Progress, Typography, Space, Spin, Tag, Tooltip, Button, theme as antTheme } from 'antd'
import {
  DashboardOutlined, ReloadOutlined, CloudServerOutlined,
  ApiOutlined, WifiOutlined, GlobalOutlined, LinkOutlined,
  ThunderboltOutlined, ClockCircleOutlined, FolderOpenOutlined,
  FilterOutlined, SwapOutlined, ApartmentOutlined, CheckCircleFilled,
  CloseCircleFilled, MinusCircleFilled,
} from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { systemApi } from '../api'
import { useAppStore } from '../store/appStore'

const { Title, Text } = Typography

interface SystemInfo {
  hostname: string
  os: string
  arch: string
  version: string
  build_time: string
  go_version: string
  uptime: number
  cpu_usage: number
  mem_total: number
  mem_used: number
  mem_usage: number
  disk_total: number
  disk_used: number
  disk_usage: number
  load_avg: number[]
  services: ServiceStatus[]
}

interface ServiceStatus {
  name: string
  type: string
  status: string
  count: number
  running: number
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

// 进度条颜色
const progressColor = (val: number) => {
  if (val >= 90) return { '0%': '#ff4d4f', '100%': '#ff7875' }
  if (val >= 70) return { '0%': '#faad14', '100%': '#ffc53d' }
  return { '0%': '#52c41a', '100%': '#73d13d' }
}

// 服务图标映射
const serviceIcons: Record<string, React.ReactNode> = {
  port_forward: <SwapOutlined />,
  stun: <WifiOutlined />,
  frpc: <ApiOutlined />,
  frps: <CloudServerOutlined />,
  easytier_client: <ApartmentOutlined />,
  easytier_server: <ApartmentOutlined />,
  ddns: <GlobalOutlined />,
  caddy: <LinkOutlined />,
  wol: <ThunderboltOutlined />,
  cron: <ClockCircleOutlined />,
  storage: <FolderOpenOutlined />,
  access: <FilterOutlined />,
}

// 资源仪表盘卡片
const ResourceGauge: React.FC<{
  label: string
  value: number
  subtitle?: string
  icon?: React.ReactNode
}> = ({ label, value, subtitle, icon }) => {
  const color = progressColor(value)
  const strokeColor = value >= 90 ? '#ff4d4f' : value >= 70 ? '#faad14' : '#52c41a'

  return (
    <div style={{ textAlign: 'center', padding: '8px 0' }}>
      <Progress
        type="dashboard"
        percent={Math.round(value)}
        strokeColor={color}
        trailColor="rgba(128,128,128,0.15)"
        size={110}
        strokeWidth={8}
        format={p => (
          <div>
            <div style={{ fontSize: 20, fontWeight: 700, color: strokeColor, lineHeight: 1 }}>{p}%</div>
          </div>
        )}
      />
      <div style={{ marginTop: 10 }}>
        <Text strong style={{ fontSize: 13, display: 'block' }}>{label}</Text>
        {subtitle && (
          <Text type="secondary" style={{ fontSize: 11, display: 'block', marginTop: 2 }}>
            {subtitle}
          </Text>
        )}
      </div>
    </div>
  )
}

// 信息行组件
const InfoRow: React.FC<{ label: string; value: string }> = ({ label, value }) => (
  <div style={{
    display: 'flex',
    justifyContent: 'space-between',
    alignItems: 'center',
    padding: '7px 0',
    borderBottom: '1px solid rgba(128,128,128,0.08)',
  }}>
    <Text type="secondary" style={{ fontSize: 12 }}>{label}</Text>
    <Text style={{ fontSize: 12, fontWeight: 500, maxWidth: '60%', textAlign: 'right' }}>{value}</Text>
  </div>
)

// 服务卡片
const ServiceCard: React.FC<{ svc: ServiceStatus }> = ({ svc }) => {
  const isRunning = svc.running > 0
  const hasConfig = svc.count > 0

  return (
    <Tooltip title={hasConfig ? `${svc.running}/${svc.count} 运行中` : '未配置'}>
      <div style={{
        padding: '14px 16px',
        borderRadius: 10,
        border: `1px solid ${isRunning ? 'rgba(82,196,26,0.25)' : 'rgba(128,128,128,0.12)'}`,
        background: isRunning
          ? 'linear-gradient(135deg, rgba(82,196,26,0.06) 0%, rgba(82,196,26,0.02) 100%)'
          : 'rgba(128,128,128,0.04)',
        cursor: 'default',
        transition: 'all 0.25s',
        position: 'relative',
        overflow: 'hidden',
      }}
        onMouseEnter={e => {
          e.currentTarget.style.transform = 'translateY(-2px)'
          e.currentTarget.style.boxShadow = isRunning
            ? '0 6px 20px rgba(82,196,26,0.15)'
            : '0 6px 20px rgba(0,0,0,0.08)'
        }}
        onMouseLeave={e => {
          e.currentTarget.style.transform = 'translateY(0)'
          e.currentTarget.style.boxShadow = 'none'
        }}
      >
        {/* 运行状态指示条 */}
        {isRunning && (
          <div style={{
            position: 'absolute', top: 0, left: 0, right: 0, height: 2,
            background: 'linear-gradient(90deg, #52c41a, #73d13d)',
            borderRadius: '10px 10px 0 0',
          }} />
        )}
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 8 }}>
          <span style={{
            color: isRunning ? '#52c41a' : 'rgba(128,128,128,0.5)',
            fontSize: 18,
            transition: 'color 0.2s',
          }}>
            {serviceIcons[svc.type] || <ApiOutlined />}
          </span>
          {hasConfig ? (
            isRunning
              ? <CheckCircleFilled style={{ color: '#52c41a', fontSize: 13 }} />
              : <MinusCircleFilled style={{ color: 'rgba(128,128,128,0.4)', fontSize: 13 }} />
          ) : null}
        </div>
        <Text style={{ fontSize: 12, display: 'block', fontWeight: 500 }}>{svc.name}</Text>
        <Text type="secondary" style={{ fontSize: 11, display: 'block', marginTop: 2 }}>
          {hasConfig ? `${svc.running}/${svc.count} 运行` : '未配置'}
        </Text>
      </div>
    </Tooltip>
  )
}

const Dashboard: React.FC = () => {
  const { t } = useTranslation()
  const { theme } = useAppStore()
  const { token } = antTheme.useToken()
  const [info, setInfo] = useState<SystemInfo | null>(null)
  const [loading, setLoading] = useState(true)
const isGlass = theme === 'glass-light' || theme === 'glass-dark'

  const fetchInfo = useCallback(async () => {
    try {
      const res: any = await systemApi.getInfo()
      setInfo(res.data)
    } catch {
      // ignore
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchInfo()
    const timer = setInterval(fetchInfo, 10000)
    return () => clearInterval(timer)
  }, [fetchInfo])

  if (loading) {
    return (
      <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: 400 }}>
        <Spin size="large" />
      </div>
    )
  }

  const cpuUsage = info?.cpu_usage ?? 0
  const memUsage = info?.mem_usage ?? 0
  const diskUsage = info?.disk_usage ?? 0

  // 玻璃模式下的卡片样式
  const cardStyle = isGlass ? {
    background: 'rgba(255,255,255,0.06)',
    backdropFilter: 'blur(20px)',
    WebkitBackdropFilter: 'blur(20px)',
    border: '1px solid rgba(255,255,255,0.1)',
    boxShadow: '0 8px 32px rgba(0,0,0,0.2)',
  } : {}

  return (
    <div>
      {/* 标题栏 */}
      <div style={{
        display: 'flex',
        justifyContent: 'space-between',
        alignItems: 'center',
        marginBottom: 20,
      }}>
        <Space align="center">
          <div style={{
            width: 36, height: 36, borderRadius: 10,
            background: 'linear-gradient(135deg, #1677ff 0%, #0958d9 100%)',
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            boxShadow: '0 4px 12px rgba(22,119,255,0.35)',
          }}>
            <DashboardOutlined style={{ color: '#fff', fontSize: 17 }} />
          </div>
          <div>
            <Title level={4} style={{ margin: 0, lineHeight: 1.2 }}>{t('dashboard.title')}</Title>
            <Text type="secondary" style={{ fontSize: 12 }}>系统运行状态监控</Text>
          </div>
        </Space>
        <Button
          icon={<ReloadOutlined />}
          onClick={fetchInfo}
          size="small"
          style={{ borderRadius: 8 }}
        >
          {t('common.refresh')}
        </Button>
      </div>

      <Row gutter={[16, 16]}>
        {/* 系统信息卡片 */}
        <Col xs={24} lg={8}>
          <Card
            title={
              <Space>
                <CloudServerOutlined style={{ color: '#1677ff' }} />
                <span style={{ fontSize: 14 }}>{t('dashboard.systemInfo')}</span>
              </Space>
            }
            size="small"
            style={{ height: '100%', borderRadius: 12, ...cardStyle }}
            styles={{ body: { padding: '12px 16px' } }}
          >
            <div>
              {[
                { label: '主机名', value: info?.hostname || '-' },
                { label: t('dashboard.os'), value: info?.os || '-' },
                { label: t('dashboard.arch'), value: info?.arch || '-' },
                { label: t('dashboard.version'), value: info?.version || 'dev' },
                { label: 'Go 版本', value: info?.go_version || '-' },
                { label: '运行时间', value: info?.uptime ? formatUptime(info.uptime) : '-' },
              ].map(({ label, value }) => (
                <InfoRow key={label} label={label} value={value} />
              ))}
            </div>
          </Card>
        </Col>

        {/* 资源使用率 */}
        <Col xs={24} lg={16}>
          <Card
            title={
              <Space>
                <DashboardOutlined style={{ color: '#1677ff' }} />
                <span style={{ fontSize: 14 }}>资源使用率</span>
              </Space>
            }
            size="small"
            style={{ borderRadius: 12, ...cardStyle }}
          >
            <Row gutter={[16, 16]}>
              <Col xs={24} md={8}>
                <ResourceGauge
                  label={t('dashboard.cpuUsage')}
                  value={cpuUsage}
                  subtitle={info?.load_avg
                    ? `负载: ${info.load_avg.map(v => v.toFixed(2)).join(' / ')}`
                    : undefined}
                />
              </Col>
              <Col xs={24} md={8}>
                <ResourceGauge
                  label={t('dashboard.memUsage')}
                  value={memUsage}
                  subtitle={`${formatBytes(info?.mem_used ?? 0)} / ${formatBytes(info?.mem_total ?? 0)}`}
                />
              </Col>
              <Col xs={24} md={8}>
                <ResourceGauge
                  label={t('dashboard.diskUsage')}
                  value={diskUsage}
                  subtitle={`${formatBytes(info?.disk_used ?? 0)} / ${formatBytes(info?.disk_total ?? 0)}`}
                />
              </Col>
            </Row>
          </Card>
        </Col>

        {/* 服务状态 */}
        <Col xs={24}>
          <Card
            title={
              <Space>
                <ApiOutlined style={{ color: '#1677ff' }} />
                <span style={{ fontSize: 14 }}>{t('dashboard.runningServices')}</span>
                <Tag
                  color="blue"
                  style={{ marginLeft: 4, fontSize: 11, borderRadius: 10 }}
                >
                  {(info?.services || defaultServices).filter(s => s.running > 0).length} 运行中
                </Tag>
              </Space>
            }
            size="small"
            style={{ borderRadius: 12, ...cardStyle }}
          >
            <Row gutter={[10, 10]}>
              {(info?.services || defaultServices).map((svc) => (
                <Col key={svc.type} xs={12} sm={8} md={6} lg={4}>
                  <ServiceCard svc={svc} />
                </Col>
              ))}
            </Row>
          </Card>
        </Col>
      </Row>
    </div>
  )
}

// 默认服务列表（API 未返回时显示）
const defaultServices: ServiceStatus[] = [
  { name: '端口转发', type: 'port_forward', status: 'stopped', count: 0, running: 0 },
  { name: 'STUN穿透', type: 'stun', status: 'stopped', count: 0, running: 0 },
  { name: 'FRP客户端', type: 'frpc', status: 'stopped', count: 0, running: 0 },
  { name: 'FRP服务端', type: 'frps', status: 'stopped', count: 0, running: 0 },
  { name: 'ET客户端', type: 'easytier_client', status: 'stopped', count: 0, running: 0 },
  { name: 'ET服务端', type: 'easytier_server', status: 'stopped', count: 0, running: 0 },
  { name: '动态域名', type: 'ddns', status: 'stopped', count: 0, running: 0 },
  { name: '网站服务', type: 'caddy', status: 'stopped', count: 0, running: 0 },
  { name: '计划任务', type: 'cron', status: 'stopped', count: 0, running: 0 },
  { name: '网络存储', type: 'storage', status: 'stopped', count: 0, running: 0 },
  { name: '访问控制', type: 'access', status: 'stopped', count: 0, running: 0 },
  { name: '网络唤醒', type: 'wol', status: 'stopped', count: 0, running: 0 },
]

export default Dashboard
