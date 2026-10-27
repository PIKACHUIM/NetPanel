import React, { useEffect, useState } from 'react'
import {
  Table, Button, Space, Tag, Typography, Tooltip, message,
  Card, Descriptions,
} from 'antd'
import {
  PlusOutlined, DeleteOutlined, SyncOutlined,
  ClusterOutlined, CheckCircleOutlined, CloseCircleOutlined,
  WarningOutlined,
} from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { frpmasterApi } from '../api'
import { useTableStyle } from '../hooks/useTableStyle'

const { Text, Title } = Typography

// 节点 row（与后端 Manager.List 对齐）
type MasterNode = {
  id: number
  name: string
  region: string
  server_addr: string
  server_port: number
  status: 'online' | 'offline'
  last_seen: string
  remark: string
}

// 候选线路 row（由在线节点派生，前端展示用的简化视图）
type CandidateLine = {
  id: string          // fnode:<nodeId>
  nodeId: number
  name: string
  region: string
  address: string
  status: 'online' | 'offline'
  lastSeen: string
}

const Pool: React.FC = () => {
  const { t } = useTranslation()
  const tableStyle = useTableStyle()
  const [nodes, setNodes] = useState<MasterNode[]>([])
  const [candidates, setCandidates] = useState<CandidateLine[]>([])
  const [loading, setLoading] = useState(false)
  const [checking, setChecking] = useState<Set<number>>(new Set())

  // 计算候选线路：仅在线节点提供一条线路 fnode:<id>。
  const rebuildCandidates = (ns: MasterNode[]) => {
    const out: CandidateLine[] = []
    for (const n of ns) {
      if (n.status !== 'online') continue
      out.push({
        id: `fnode:${n.id}`,
        nodeId: n.id,
        name: n.name,
        region: n.region,
        address: n.server_addr ? `${n.server_addr}:${n.server_port}` : '',
        status: n.status,
        lastSeen: n.last_seen,
      })
    }
    return out
  }

  const fetchData = async () => {
    setLoading(true)
    try {
      const res: any = await frpmasterApi.listNodes()
      const raw = (res?.data ?? []) as MasterNode[]
      const now = Date.now()
      for (const n of raw) {
        // 本地兜底：last_seen 为空或过期的视为离线（与后端窗口一致）。
        if (n.status === 'online') {
          const ts = new Date(n.last_seen).getTime()
          if (!ts || now - ts > 100_000) n.status = 'offline'
        }
      }
      setNodes(raw)
      setCandidates(rebuildCandidates(raw))
    } catch {
      message.error(t('frpmaster.fetchFailed'))
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { fetchData() }, [])

  const handleDelete = async (n: MasterNode) => {
    try {
      await frpmasterApi.deleteNode(n.id)
      message.success(t('frpmaster.removed'))
      await fetchData()
    } catch (e: any) {
      message.error(e?.message || t('frpmaster.deleteFailed'))
    }
  }

  const handleRecheck = async (n: MasterNode) => {
    setChecking((prev) => new Set(prev).add(n.id))
    try {
      await fetchData()
      message.success(t('common.refresh'))
    } catch {
      message.error(t('frpmaster.fetchFailed'))
    } finally {
      setChecking((prev) => {
        const next = new Set(prev)
        next.delete(n.id)
        return next
      })
    }
  }

  const nodeColumns = [
    { title: t('common.name'), dataIndex: 'name', key: 'name' },
    { title: t('frpmaster.region'), dataIndex: 'region', key: 'region', width: 110 },
    { title: t('frpmaster.serverAddr'), dataIndex: 'server_addr', key: 'serverAddr',
      render: (v: string, r: MasterNode) => v ? `${v}:${r.server_port}` : null },
    { title: t('common.status'), dataIndex: 'status', key: 'status', width: 90,
      render: (v: string) => (
        <Tag color={v === 'online' ? 'green' : 'default'} icon={v === 'online' ? <CheckCircleOutlined /> : <CloseCircleOutlined />}>
          {v === 'online' ? t('common.online') : t('common.offline')}
        </Tag>
      ) },
    { title: t('frpmaster.lastSeen'), dataIndex: 'last_seen', key: 'lastSeen', width: 170,
      render: (v: string) => v ? new Date(v).toLocaleTimeString() : <Text type="secondary">—</Text> },
    { title: t('common.remark'), dataIndex: 'remark', key: 'remark', ellipsis: true },
    { title: t('common.action'), key: 'action', width: 120,
      render: (_: any, r: MasterNode) => (
        <Space size={8}>
          <Tooltip title={t('frpmaster.recheck')}>
            <Button
              type="text"
              icon={<SyncOutlined spin={checking.has(r.id)} />}
              loading={checking.has(r.id)}
              onClick={() => handleRecheck(r)}
            />
          </Tooltip>
          <Tooltip title={t('frpmaster.remove')}>
            <Button type="text" danger icon={<DeleteOutlined />} onClick={() => handleDelete(r)} />
          </Tooltip>
        </Space>
      ) },
  ]

  const lineColumns = [
    { title: t('common.name'), dataIndex: 'name', key: 'name' },
    { title: t('frpmaster.region'), dataIndex: 'region', key: 'region', width: 110 },
    { title: t('frpmaster.serverAddr'), dataIndex: 'address', key: 'address', width: 200 },
    { title: t('common.status'), dataIndex: 'status', key: 'status', width: 90,
      render: (v: string) => (
        <Tag color={v === 'online' ? 'green' : 'default'} icon={
          v === 'online' ? <CheckCircleOutlined /> : <CloseCircleOutlined />
        }>{v === 'online' ? t('common.online') : t('common.offline')}</Tag>
      ) },
    { title: t('frpmaster.lastSeen'), dataIndex: 'lastSeen', key: 'lastSeen', width: 170,
      render: (v: string) => v ? new Date(v).toLocaleTimeString() : null },
  ]

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <Title level={4} style={{ margin: 0 }}>
          <ClusterOutlined style={{ marginRight: 8 }} />{t('frpmaster.poolTitle')}
        </Title>
        <Space>
          <Button icon={<SyncOutlined />} onClick={fetchData} loading={loading}>
            {t('common.refresh')}
          </Button>
          <Button
            type="primary"
            icon={<PlusOutlined />}
            onClick={() => window.open('/frp/master/nodes', '_blank')}
          >
            {t('frpmaster.registerNode')}
          </Button>
        </Space>
      </div>

      <Card size="small" title={
        <Space>
          <span>{t('frpmaster.nodes')}</span>
          <Tag color="blue">{nodes.length}</Tag>
          <Tag color={nodes.some((n) => n.status === 'online') ? 'green' : 'default'}>
            {nodes.filter((n) => n.status === 'online').length} {t('common.online')}
          </Tag>
        </Space>
      }>
        <Table<MasterNode>
          rowKey="id"
          columns={nodeColumns}
          dataSource={nodes}
          loading={loading}
          size="small"
          scroll={{ x: 800 }}
          pagination={{ pageSize: 15, showTotal: (total) => `${t('common.total')} ${total}` }}
          locale={{ emptyText: t('frpmaster.empty') }}
          {...tableStyle}
        />
      </Card>

      <Card size="small" title={
        <Space>
          <span>{t('frpmaster.candidates')}</span>
          <Tag color="geekblue">{candidates.length}</Tag>
        </Space>
      }>
        <Table<CandidateLine>
          rowKey="id"
          columns={lineColumns}
          dataSource={candidates}
          loading={loading}
          size="small"
          scroll={{ x: 700 }}
          pagination={{ pageSize: 15, showTotal: (total) => `${t('common.total')} ${total}` }}
          locale={{ emptyText: t('frpmaster.emptyCandidates') }}
          {...tableStyle}
        />
      </Card>
    </div>
  )
}

export default Pool
