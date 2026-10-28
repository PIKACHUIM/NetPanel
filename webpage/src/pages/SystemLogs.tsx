import React, { useState, useEffect, useCallback } from 'react'
import {
  Card, Table, Tag, Select, Input, Button, Space, DatePicker,
  Typography, Row, Col, Tooltip, Badge, Popconfirm, message,
  Statistic, Segmented,
} from 'antd'
import {
  SearchOutlined, ReloadOutlined, DeleteOutlined,
  InfoCircleOutlined, WarningOutlined, CloseCircleOutlined,
  BugOutlined, FileTextOutlined, ClearOutlined,
} from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import dayjs from 'dayjs'
import { useTranslation } from 'react-i18next'
import { adminApi } from '../api'
import { useTableStyle } from '../hooks/useTableStyle'

const { RangePicker } = DatePicker
const { Text } = Typography
const { Option } = Select

// 日志级别配置
const LEVEL_CONFIG: Record<string, { color: string; icon: React.ReactNode; label: string }> = {
  info:    { color: 'blue',    icon: <InfoCircleOutlined />,  label: 'INFO'  },
  warn:    { color: 'orange',  icon: <WarningOutlined />,     label: 'WARN'  },
  warning: { color: 'orange',  icon: <WarningOutlined />,     label: 'WARN'  },
  error:   { color: 'red',     icon: <CloseCircleOutlined />, label: 'ERROR' },
  debug:   { color: 'default', icon: <BugOutlined />,         label: 'DEBUG' },
}

// 服务标签颜色
const SERVICE_COLORS: Record<string, string> = {
  system:      'geekblue',
  frp:         'purple',
  nps:         'cyan',
  easytier:    'magenta',
  ddns:        'gold',
  caddy:       'lime',
  portforward: 'blue',
  stun:        'volcano',
  dnsmasq:     'green',
  storage:     'orange',
  cron:        'geekblue',
  waf:         'red',
  firewall:    'red',
  access:      'purple',
  cert:        'cyan',
  callback:    'gold',
}

interface LogItem {
  id: number
  level: string
  service: string
  message: string
  log_time: string
}

interface QueryResult {
  total: number
  items: LogItem[]
}

const SystemLogs: React.FC = () => {
  const { t } = useTranslation()
  const tableStyle = useTableStyle()
  const [loading, setLoading] = useState(false)
  const [data, setData] = useState<QueryResult>({ total: 0, items: [] })
  const [services, setServices] = useState<string[]>([])

  // 筛选条件
  const [service, setService] = useState<string>('')
  const [level, setLevel] = useState<string>('')
  const [keyword, setKeyword] = useState<string>('')
  const [dateRange, setDateRange] = useState<[dayjs.Dayjs | null, dayjs.Dayjs | null] | null>(null)
  const [order, setOrder] = useState<'desc' | 'asc'>('desc')
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(50)

  // 统计数据
  const [stats, setStats] = useState({ info: 0, warn: 0, error: 0 })

  const fetchLogs = useCallback(async (p = page, ps = pageSize) => {
    setLoading(true)
    try {
      const params: Record<string, any> = { page: p, page_size: ps, order }
      if (service) params.service = service
      if (level) params.level = level
      if (keyword) params.keyword = keyword
      if (dateRange?.[0]) params.start_at = dateRange[0].toISOString()
      if (dateRange?.[1]) params.end_at = dateRange[1].endOf('day').toISOString()

      const res = await adminApi.queryLogs(params)
      setData(res.data ?? { total: 0, items: [] })
    } finally {
      setLoading(false)
    }
  }, [service, level, keyword, dateRange, order, page, pageSize])

  const fetchServices = useCallback(async () => {
    try {
      const res = await adminApi.getLogServices()
      setServices(res.data || [])
    } catch {}
  }, [])

  const fetchStats = useCallback(async () => {
    try {
      const [infoRes, warnRes, errorRes] = await Promise.all([
        adminApi.queryLogs({ level: 'info', page: 1, page_size: 1 }),
        adminApi.queryLogs({ level: 'warn', page: 1, page_size: 1 }),
        adminApi.queryLogs({ level: 'error', page: 1, page_size: 1 }),
      ])
      setStats({
        info: infoRes.data?.total ?? 0,
        warn: warnRes.data?.total ?? 0,
        error: errorRes.data?.total ?? 0,
      })
    } catch {}
  }, [])

  useEffect(() => {
    fetchServices()
    fetchStats()
  }, [fetchServices, fetchStats])

  useEffect(() => {
    fetchLogs(page, pageSize)
  }, [fetchLogs, page, pageSize])

  const handleSearch = () => {
    setPage(1)
    fetchLogs(1, pageSize)
  }

  const handleReset = () => {
    setService('')
    setLevel('')
    setKeyword('')
    setDateRange(null)
    setOrder('desc')
    setPage(1)
    setTimeout(() => fetchLogs(1, pageSize), 0)
  }

  const handleCleanup = async (days: number) => {
    try {
      const res = await adminApi.cleanupLogs(days)
      message.success(t('systemLogs.cleanupSuccess', { total: res.data?.deleted ?? 0 }))
      fetchLogs(1, pageSize)
      fetchStats()
    } catch (e: any) {
      message.error(t('systemLogs.cleanupFailed', { message: e?.response?.data?.message || e.message }))
    }
  }

  const columns: ColumnsType<LogItem> = [
    {
      title: t('systemLogs.time'),
      dataIndex: 'log_time',
      key: 'log_time',
      width: 170,
      render: (v: string) => (
<Text style={{ fontSize: 12, fontFamily: "'MapleMono', monospace", whiteSpace: 'nowrap' }}>
          {dayjs(v).format('YYYY-MM-DD HH:mm:ss')}
        </Text>
      ),
    },
    {
      title: t('systemLogs.level'),
      dataIndex: 'level',
      key: 'level',
      width: 80,
      render: (v: string) => {
        const cfg = LEVEL_CONFIG[v] || LEVEL_CONFIG.info
        return (
          <Tag color={cfg.color} icon={cfg.icon} style={{ fontSize: 11, fontWeight: 600 }}>
            {cfg.label}
          </Tag>
        )
      },
    },
    {
      title: t('systemLogs.service'),
      dataIndex: 'service',
      key: 'service',
      width: 110,
      render: (v: string) => (
        <Tag color={SERVICE_COLORS[v] || 'default'} style={{ fontSize: 11 }}>
          {v}
        </Tag>
      ),
    },
    {
      title: t('systemLogs.content'),
      dataIndex: 'message',
      key: 'message',
      ellipsis: { showTitle: false },
      render: (v: string, record: LogItem) => {
        const isError = record.level === 'error'
        return (
          <Tooltip title={v} placement="topLeft">
            <Text
              style={{
                fontSize: 13,
                color: isError ? '#ff4d4f' : undefined,
                fontFamily: "'MapleMono', monospace",
              }}
            >
              {v}
            </Text>
          </Tooltip>
        )
      },
    },
  ]

  return (
    <div>
      {/* 统计卡片 */}
      <Row gutter={[12, 12]} style={{ marginBottom: 16 }}>
        <Col xs={8}>
          <Card size="small" style={{ borderRadius: 8, borderLeft: '3px solid #0071e3' }}>
            <Statistic
              title={<span style={{ fontSize: 12 }}>{t('systemLogs.info')}</span>}
              value={stats.info}
              valueStyle={{ color: '#0071e3', fontSize: 20 }}
              prefix={<InfoCircleOutlined />}
            />
          </Card>
        </Col>
        <Col xs={8}>
          <Card size="small" style={{ borderRadius: 8, borderLeft: '3px solid #fa8c16' }}>
            <Statistic
              title={<span style={{ fontSize: 12 }}>{t('systemLogs.warn')}</span>}
              value={stats.warn}
              valueStyle={{ color: '#fa8c16', fontSize: 20 }}
              prefix={<WarningOutlined />}
            />
          </Card>
        </Col>
        <Col xs={8}>
          <Card size="small" style={{ borderRadius: 8, borderLeft: '3px solid #ff4d4f' }}>
            <Statistic
              title={<span style={{ fontSize: 12 }}>{t('systemLogs.error')}</span>}
              value={stats.error}
              valueStyle={{ color: '#ff4d4f', fontSize: 20 }}
              prefix={<CloseCircleOutlined />}
            />
          </Card>
        </Col>
      </Row>

      {/* 主内容卡片 */}
      <Card
        title={
          <Space>
            <FileTextOutlined style={{ color: '#0071e3' }} />
            <span>{t('systemLogs.title')}</span>
            <Badge count={data.total} overflowCount={99999} style={{ backgroundColor: '#0071e3' }} />
          </Space>
        }
        extra={
          <Space>
            <Popconfirm
              title={t('systemLogs.cleanupTitle')}
              description={t('systemLogs.cleanupDescription')}
              onConfirm={() => handleCleanup(30)}
              okText={t('systemLogs.cleanupConfirm')}
              cancelText={t('common.cancel')}
            >
              <Button icon={<ClearOutlined />} size="small" danger>
                {t('systemLogs.cleanup')}
              </Button>
            </Popconfirm>
            <Button icon={<ReloadOutlined />} size="small" onClick={() => fetchLogs(page, pageSize)}>
              {t('systemLogs.refresh')}
            </Button>
          </Space>
        }
        style={{ borderRadius: 8 }}
      >
        {/* 筛选栏 */}
        <Row gutter={[8, 8]} style={{ marginBottom: 12 }}>
          <Col xs={24} sm={12} md={5}>
            <Select
              placeholder={t('systemLogs.serviceType')}
              value={service || undefined}
              onChange={v => setService(v || '')}
              allowClear
              style={{ width: '100%' }}
              showSearch
            >
              {services.map(s => (
                <Option key={s} value={s}>
                  <Tag color={SERVICE_COLORS[s] || 'default'} style={{ fontSize: 11 }}>{s}</Tag>
                </Option>
              ))}
            </Select>
          </Col>
          <Col xs={24} sm={12} md={4}>
            <Select
              placeholder={t('systemLogs.level')}
              value={level || undefined}
              onChange={v => setLevel(v || '')}
              allowClear
              style={{ width: '100%' }}
            >
              <Option value="info"><Tag color="blue">INFO</Tag></Option>
              <Option value="warn"><Tag color="orange">WARN</Tag></Option>
              <Option value="error"><Tag color="red">ERROR</Tag></Option>
              <Option value="debug"><Tag>DEBUG</Tag></Option>
            </Select>
          </Col>
          <Col xs={24} sm={24} md={6}>
            <Input
              placeholder={t('systemLogs.searchPlaceholder')}
              value={keyword}
              onChange={e => setKeyword(e.target.value)}
              onPressEnter={handleSearch}
              prefix={<SearchOutlined style={{ color: '#bbb' }} />}
              allowClear
            />
          </Col>
          <Col xs={24} sm={24} md={6}>
            <RangePicker
              value={dateRange}
              onChange={v => setDateRange(v as any)}
              style={{ width: '100%' }}
              placeholder={[t('systemLogs.startDate'), t('systemLogs.endDate')]}
            />
          </Col>
          <Col xs={24} sm={24} md={3}>
            <div style={{ display: 'flex', gap: 4 }}>
              <Button type="primary" icon={<SearchOutlined />} onClick={handleSearch} style={{ flex: 1, minWidth: 0 }}>
                {t('systemLogs.search')}
              </Button>
              <Button icon={<ReloadOutlined />} onClick={handleReset} style={{ flex: 1, minWidth: 0 }}>
                {t('systemLogs.reset')}
              </Button>
            </div>
          </Col>
        </Row>

        {/* 排序切换 */}
        <div style={{ marginBottom: 8 }}>
          <Space>
            <Text type="secondary" style={{ fontSize: 12 }}>{t('systemLogs.timeOrder')}:</Text>
            <Segmented
              size="small"
              value={order}
              onChange={v => { setOrder(v as 'desc' | 'asc'); setPage(1) }}
              options={[
                { label: t('systemLogs.newest'), value: 'desc' },
                { label: t('systemLogs.oldest'), value: 'asc' },
              ]}
            />
          </Space>
        </div>

        <Table<LogItem>
          columns={columns}
          dataSource={data.items}
          rowKey="id"
          loading={loading}
          size="small"
          scroll={{ x: 800 }}
          rowClassName={(record) => {
            if (record.level === 'error') return 'log-row-error'
            if (record.level === 'warn' || record.level === 'warning') return 'log-row-warn'
            return ''
          }}
          pagination={{
            current: page,
            pageSize,
            total: data.total,
            showSizeChanger: true,
            showQuickJumper: true,
            pageSizeOptions: ['20', '50', '100', '200'],
            showTotal: (total) => t('systemLogs.totalItems', { total }),
            onChange: (p, ps) => {
              setPage(p)
              setPageSize(ps)
            },
          }}
        />
      </Card>

      <style>{`
        .log-row-error td { background: rgba(255, 77, 79, 0.04) !important; }
        .log-row-warn td { background: rgba(250, 140, 22, 0.04) !important; }
      `}</style>
    </div>
  )
}

export default SystemLogs
