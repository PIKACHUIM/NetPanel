import React, { useState, useEffect, useCallback } from 'react'
import {
  Card, Table, Tag, Select, Input, Button, Space, Typography,
  DatePicker, Tooltip, message,
} from 'antd'
import {
  SearchOutlined, ReloadOutlined, FileTextOutlined,
} from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import dayjs from 'dayjs'
import { useTranslation } from 'react-i18next'
import { adminApi } from '../api'
import { useTableStyle } from '../hooks/useTableStyle'

const { RangePicker } = DatePicker
const { Text } = Typography
const { Option } = Select

const ACTION_COLORS: Record<string, string> = {
  CREATE: 'green', UPDATE: 'blue', DELETE: 'red',
  ENABLE: 'green', DISABLE: 'orange', UNKNOWN: 'default',
}
const RESOURCE_COLORS: Record<string, string> = {
  portforward: 'blue', frp_proxy: 'purple', caddy_site: 'lime',
  easytier_client: 'magenta', stun: 'volcano', ddns: 'gold',
}

interface AuditItem {
  id: number
  level: string
  service: string
  message: string
  actor: string
  action_type: string
  resource_type: string
  resource_id: number
  diff?: string
  log_time: string
}

interface QueryResult {
  total: number
  items: AuditItem[]
}

const SystemAudit: React.FC = () => {
  const { t } = useTranslation()
  const tableStyle = useTableStyle()
  const [loading, setLoading] = useState(false)
  const [data, setData] = useState<QueryResult>({ total: 0, items: [] })
  const [actors, setActors] = useState<string[]>([])
  const [resourceTypes, setResourceTypes] = useState<string[]>([])
  const [actionTypes, setActionTypes] = useState<string[]>([])

  const [service, setService] = useState('audit')
  const [level, setLevel] = useState('audit')
  const [actor, setActor] = useState('')
  const [actionType, setActionType] = useState('')
  const [resourceType, setResourceType] = useState('')
  const [keyword, setKeyword] = useState('')
  const [dateRange, setDateRange] = useState<[dayjs.Dayjs | null, dayjs.Dayjs | null] | null>(null)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(50)

  const fetchMeta = useCallback(async () => {
    try {
      const [svc, act, res] = await Promise.all([
        adminApi.getLogServices(),
        adminApi.getAuditActors(),
        adminApi.getAuditResourceTypes(),
      ])
      setActors((act as any)?.data || [])
      setResourceTypes((res as any)?.data || [])
    } catch { /* ignore */ }
  }, [])

  const fetchData = useCallback(async () => {
    setLoading(true)
    try {
      const params: Record<string, any> = {
        service, level, actor, action_type: actionType,
        resource_type: resourceType, keyword, page, page_size: pageSize,
      }
      if (dateRange && dateRange[0]) params.start_at = dateRange[0].toISOString()
      if (dateRange && dateRange[1]) params.end_at = dateRange[1].toISOString()
      const res: any = await adminApi.queryAuditLogs(params)
      setData({ total: res.data?.total || 0, items: res.data?.items || [] })
    } catch (e: any) {
      message.error(e?.response?.data?.message || t('common.operationFailed'))
    } finally {
      setLoading(false)
    }
  }, [service, level, actor, actionType, resourceType, keyword, dateRange, page, pageSize, t])

  useEffect(() => { fetchData() }, [fetchData])
  useEffect(() => { fetchMeta() }, [fetchMeta])

  const columns: ColumnsType<AuditItem> = [
    { title: t('systemAudit.time'), dataIndex: 'log_time', width: 160, render: v => <Text style={{ fontSize: 12 }}>{dayjs(v).format('YYYY-MM-DD HH:mm:ss')}</Text> },
    { title: t('systemAudit.actor'), dataIndex: 'actor', width: 110 },
    {
      title: t('systemAudit.action'), key: 'action', width: 90,
      render: (_, r) => <Tag color={ACTION_COLORS[r.action_type] || 'default'}>{t(`systemAudit.actions.${r.action_type}`, { defaultValue: r.action_type })}</Tag>,
    },
    {
      title: t('systemAudit.resource'), key: 'resource', width: 160,
      render: (_, r) => (
        <Space size={4}>
          <Tag color={RESOURCE_COLORS[r.resource_type] || 'default'}>{r.resource_type}</Tag>
          <Text type="secondary" style={{ fontSize: 11 }}>#{r.resource_id}</Text>
        </Space>
      ),
    },
    { title: t('systemAudit.message'), dataIndex: 'message', ellipsis: true },
    {
      title: t('systemAudit.changes'), key: 'diff', width: 80,
      render: (_, r) => r.diff ? (
        <Tooltip title={r.diff}>
          <Button type="link" size="small" style={{ padding: 0 }}>{t('systemAudit.view')}</Button>
        </Tooltip>
      ) : '-',
    },
  ]

  return (
    <div>
      <Card title={<Space><FileTextOutlined />{t('systemAudit.title')}</Space>} style={{ marginBottom: 16 }}>
        <Space wrap>
          <Select value={actor} onChange={setActor} placeholder={t('systemAudit.actor')} allowClear style={{ width: 120 }}>
            {actors.map(v => <Option key={v} value={v}>{v}</Option>)}
          </Select>
          <Select value={actionType} onChange={setActionType} placeholder={t('systemAudit.action')} allowClear style={{ width: 110 }}>
            {Object.keys(ACTION_COLORS).map(v => <Option key={v} value={v}>{t(`systemAudit.actions.${v}`)}</Option>)}
          </Select>
          <Select value={resourceType} onChange={setResourceType} placeholder={t('systemAudit.resourceType')} allowClear style={{ width: 140 }}>
            {resourceTypes.map(v => <Option key={v} value={v}>{v}</Option>)}
          </Select>
          <Input.Search value={keyword} onChange={e => setKeyword(e.target.value)} onSearch={fetchData} placeholder={t('systemAudit.searchPlaceholder')} allowClear style={{ width: 220 }} />
          <RangePicker showTime value={dateRange} onChange={v => setDateRange(v as any)} />
          <Button icon={<ReloadOutlined />} onClick={fetchData}>{t('systemAudit.refresh')}</Button>
        </Space>
      </Card>
      <Card>
        <Table<AuditItem>
          columns={columns} dataSource={data.items} rowKey="id" loading={loading}
          size="small" scroll={{ x: 900 }}
          pagination={{ current: page, total: data.total, pageSize, showSizeChanger: true, showTotal: total => t('systemAudit.totalItems', { total }) }}
          onChange={p => { setPage(p.current || 1); setPageSize(p.pageSize || 50) }}
          {...tableStyle}
        />
      </Card>
    </div>
  )
}

export default SystemAudit
