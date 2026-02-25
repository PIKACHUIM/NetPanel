import React, { useEffect, useState, useCallback } from 'react'
import { Row, Col, Card, Progress, Statistic, Badge, Typography, Space, Spin, Tag, Tooltip, Button } from 'antd'
import {
  DashboardOutlined, ReloadOutlined, CloudServerOutlined,
  ApiOutlined, WifiOutlined, GlobalOutlined, LinkOutlined,
  ThunderboltOutlined, ClockCircleOutlined, FolderOpenOutlined,
  FilterOutlined, SwapOutlined, ApartmentOutlined,
} from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { systemApi } from '../api'

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
  if (val >= 90) return '#ff4d4f'
  if (val >= 70) return '#faad14'
  return '#52c41a'
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

const serviceColors: Record<string, string> = {
  running: 'success',
  stopped: 'default',
  error: 'error',
}

const Dashboard: React.FC = () => {
  const { t } = useTranslation()
  const [info, setInfo] = useState<SystemInfo | null>(null)
  const [loading, setLoading] = useState(true)

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

  return (
    <div>
      {/* 标题栏 */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 20 }}>
        <Space>
          <DashboardOutlined style={{ fontSize: 20, color: '#1677ff' }} />
          <Title level={4} style={{ margin: 0 }}>{t('dashboard.title')}</Title>
        </Space>
        <Button icon={<ReloadOutlined />} onClick={fetchInfo} size="small">
          {t('common.refresh')}
        </Button>
      </div>

      {/* 系统信息卡片 */}
      <Row gutter={[16, 16]}>
        {/* 基本信息 */}
        <Col xs={24} lg={8}>
          <Card
            title={
              <Space>
                <CloudServerOutlined style={{ color: '#1677ff' }} />
                <span>{t('dashboard.systemInfo')}</span>
              </Space>
            }
            size="small"
            style={{ height: '100%' }}
          >
            <Space direction="vertical" style={{ width: '100%' }} size={8}>
              {[
                { label: '主机名', value: info?.hostname || '-' },
                { label: t('dashboard.os'), value: info?.os || '-' },
                { label: t('dashboard.arch'), value: info?.arch || '-' },
                { label: t('dashboard.version'), value: info?.version || 'dev' },
                { label: 'Go 版本', value: info?.go_version || '-' },
                { label: '运行时间', value: info?.uptime ? formatUptime(info.uptime) : '-' },
              ].map(({ label, value }) => (
                <div key={label} style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                  <Text type="secondary" style={{ fontSize: 13 }}>{label}</Text>
                  <Text strong style={{ fontSize: 13 }}>{value}</Text>
                </div>
              ))}
            </Space>
          </Card>
        </Col>

        {/* 资源使用率 */}
        <Col xs={24} lg={16}>
          <Card
            title={
              <Space>
                <DashboardOutlined style={{ color: '#1677ff' }} />
                <span>资源使用率</span>
              </Space>
            }
            size="small"
          >
            <Row gutter={[24, 16]}>
              {/* CPU */}
              <Col xs={24} md={8}>
                <div style={{ textAlign: 'center' }}>
                  <Progress
                    type="dashboard"
                    percent={Math.round(cpuUsage)}
                    strokeColor={progressColor(cpuUsage)}
                    size={100}
                    format={p => <span style={{ fontSize: 16, fontWeight: 600 }}>{p}%</span>}
                  />
                  <div style={{ marginTop: 8 }}>
                    <Text strong>{t('dashboard.cpuUsage')}</Text>
                    {info?.load_avg && (
                      <div>
                        <Text type="secondary" style={{ fontSize: 12 }}>
                          负载: {info.load_avg.map(v => v.toFixed(2)).join(' / ')}
                        </Text>
                      </div>
                    )}
                  </div>
                </div>
              </Col>

              {/* 内存 */}
              <Col xs={24} md={8}>
                <div style={{ textAlign: 'center' }}>
                  <Progress
                    type="dashboard"
                    percent={Math.round(memUsage)}
                    strokeColor={progressColor(memUsage)}
                    size={100}
                    format={p => <span style={{ fontSize: 16, fontWeight: 600 }}>{p}%</span>}
                  />
                  <div style={{ marginTop: 8 }}>
                    <Text strong>{t('dashboard.memUsage')}</Text>
                    <div>
                      <Text type="secondary" style={{ fontSize: 12 }}>
                        {formatBytes(info?.mem_used ?? 0)} / {formatBytes(info?.mem_total ?? 0)}
                      </Text>
                    </div>
                  </div>
                </div>
              </Col>

              {/* 磁盘 */}
              <Col xs={24} md={8}>
                <div style={{ textAlign: 'center' }}>
                  <Progress
                    type="dashboard"
                    percent={Math.round(diskUsage)}
                    strokeColor={progressColor(diskUsage)}
                    size={100}
                    format={p => <span style={{ fontSize: 16, fontWeight: 600 }}>{p}%</span>}
                  />
                  <div style={{ marginTop: 8 }}>
                    <Text strong>{t('dashboard.diskUsage')}</Text>
                    <div>
                      <Text type="secondary" style={{ fontSize: 12 }}>
                        {formatBytes(info?.disk_used ?? 0)} / {formatBytes(info?.disk_total ?? 0)}
                      </Text>
                    </div>
                  </div>
                </div>
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
                <span>{t('dashboard.runningServices')}</span>
              </Space>
            }
            size="small"
          >
            <Row gutter={[12, 12]}>
              {(info?.services || defaultServices).map((svc) => (
                <Col key={svc.type} xs={12} sm={8} md={6} lg={4}>
                  <Tooltip title={`${svc.running}/${svc.count} 运行中`}>
                    <div style={{
                      padding: '12px 16px',
                      borderRadius: 8,
                      border: '1px solid #f0f0f0',
                      background: svc.running > 0 ? '#f6ffed' : '#fafafa',
                      cursor: 'default',
                      transition: 'all 0.2s',
                    }}>
                      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 6 }}>
                        <span style={{ color: svc.running > 0 ? '#52c41a' : '#8c8c8c', fontSize: 16 }}>
                          {serviceIcons[svc.type] || <ApiOutlined />}
                        </span>
                        <Badge
                          count={svc.running}
                          showZero
                          style={{
                            backgroundColor: svc.running > 0 ? '#52c41a' : '#d9d9d9',
                            fontSize: 11,
                          }}
                        />
                      </div>
                      <Text style={{ fontSize: 12, display: 'block' }}>{svc.name}</Text>
                      <Text type="secondary" style={{ fontSize: 11 }}>
                        {svc.count > 0 ? `${svc.running}/${svc.count}` : '未配置'}
                      </Text>
                    </div>
                  </Tooltip>
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
