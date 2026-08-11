import React, { useEffect, useState, useCallback } from 'react'
import {
  Table, Button, Space, Select, Typography, Tag, Popconfirm, message,
  InputNumber,
} from 'antd'
import {
  SyncOutlined, HistoryOutlined, DeleteOutlined,
  CheckCircleOutlined, CloseCircleOutlined, PlusCircleOutlined,
  EditOutlined, MinusCircleOutlined,
} from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { meshNodeApi } from '../api'
import dayjs from 'dayjs'
import { useTableStyle } from '../hooks/useTableStyle'

const { Text } = Typography

// 事件类型配置
const EVENT_TYPE_CONFIG: Record<string, { color: string; icon: React.ReactNode; label: string }> = {
  online: { color: 'success', icon: <CheckCircleOutlined />, label: '上线' },
  offline: { color: 'error', icon: <CloseCircleOutlined />, label: '离线' },
  created: { color: 'processing', icon: <PlusCircleOutlined />, label: '新增' },
  updated: { color: 'warning', icon: <EditOutlined />, label: '修改' },
  deleted: { color: 'default', icon: <MinusCircleOutlined />, label: '删除' },
}

const MeshEvents: React.FC = () => {
  const { t } = useTranslation()
  const tableStyle = useTableStyle()
  const [data, setData] = useState<any[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(false)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(50)
  const [filterNodeId, setFilterNodeId] = useState<number | undefined>()
  const [filterType, setFilterType] = useState<string | undefined>()
  const [nodes, setNodes] = useState<any[]>([])
  const [cleanDays, setCleanDays] = useState(30)

  // 加载节点列表（用于过滤）
  useEffect(() => {
    meshNodeApi.listNodes().then((res: any) => {
      setNodes(res.data || [])
    })
  }, [])

  const fetchData = useCallback(async () => {
    setLoading(true)
    try {
      const params: any = { page, page_size: pageSize }
      if (filterNodeId) params.node_id = filterNodeId
      if (filterType) params.event_type = filterType
      const res: any = await meshNodeApi.listEvents(params)
      const d = res.data || {}
      setData(d.list || [])
      setTotal(d.total || 0)
    } finally {
      setLoading(false)
    }
  }, [page, pageSize, filterNodeId, filterType])

  useEffect(() => { fetchData() }, [fetchData])

  const handleClean = async () => {
    try {
      await meshNodeApi.cleanEvents(cleanDays)
      message.success('清理成功')
      fetchData()
    } catch (err: any) {
      message.error(`清理失败: ${err.message}`)
    }
  }

  const columns = [
    {
      title: '时间', dataIndex: 'event_time', width: 180,
      render: (v: string) => (
        <Text style={{ fontSize: 13 }}>
          {v ? dayjs(v).format('YYYY-MM-DD HH:mm:ss') : '-'}
        </Text>
      ),
    },
    {
      title: '事件类型', dataIndex: 'event_type', width: 100,
      render: (v: string) => {
        const config = EVENT_TYPE_CONFIG[v] || { color: 'default', icon: null, label: v }
        return (
          <Tag color={config.color} icon={config.icon}>
            {config.label}
          </Tag>
        )
      },
    },
    {
      title: '节点', dataIndex: 'node_name', width: 150,
      render: (name: string, r: any) => (
        <Space>
          <Text strong>{name || '-'}</Text>
          {r.node_id > 0 && <Text type="secondary" style={{ fontSize: 11 }}>#{r.node_id}</Text>}
        </Space>
      ),
    },
    {
      title: '事件描述', dataIndex: 'message',
      render: (v: string) => <Text style={{ fontSize: 13 }}>{v}</Text>,
    },
  ]

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Typography.Title level={4} style={{ margin: 0 }}>
          <HistoryOutlined style={{ marginRight: 8 }} />
          组网节点事件
        </Typography.Title>
        <Space>
          <Button icon={<SyncOutlined />} onClick={fetchData}>刷新</Button>
        </Space>
      </div>

      {/* 过滤器 */}
      <div style={{ marginBottom: 16, display: 'flex', alignItems: 'center', gap: 12, flexWrap: 'wrap' }}>
        <Space>
          <Text>节点：</Text>
          <Select
            value={filterNodeId}
            onChange={v => { setFilterNodeId(v); setPage(1) }}
            allowClear
            placeholder="全部节点"
            style={{ minWidth: 160 }}
          >
            {nodes.map((n: any) => (
              <Select.Option key={n.id} value={n.id}>{n.name}</Select.Option>
            ))}
          </Select>
        </Space>
        <Space>
          <Text>类型：</Text>
          <Select
            value={filterType}
            onChange={v => { setFilterType(v); setPage(1) }}
            allowClear
            placeholder="全部类型"
            style={{ minWidth: 120 }}
          >
            {Object.entries(EVENT_TYPE_CONFIG).map(([key, config]) => (
              <Select.Option key={key} value={key}>
                <Space>{config.icon}{config.label}</Space>
              </Select.Option>
            ))}
          </Select>
        </Space>
        <div style={{ flex: 1 }} />
        <Space>
          <Text type="secondary">清理</Text>
          <InputNumber min={1} max={365} value={cleanDays} onChange={v => setCleanDays(v || 30)} style={{ width: 80 }} />
          <Text type="secondary">天前的事件</Text>
          <Popconfirm title={`确定清理 ${cleanDays} 天前的事件？`} onConfirm={handleClean}>
            <Button danger icon={<DeleteOutlined />} size="small">清理</Button>
          </Popconfirm>
        </Space>
      </div>

      <Table
        dataSource={data}
        columns={columns}
        rowKey="id"
        loading={loading}
        size="middle"
        style={{ borderRadius: 8 }}
        pagination={{
          current: page,
          pageSize,
          total,
          showSizeChanger: true,
          showTotal: (total) => `共 ${total} 条`,
          onChange: (p, ps) => { setPage(p); setPageSize(ps) },
        }}
      />
    </div>
  )
}

export default MeshEvents
