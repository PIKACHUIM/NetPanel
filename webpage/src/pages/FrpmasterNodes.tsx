import React, { useEffect, useState } from 'react'
import {
  Table, Button, Space, Tag, Typography, Tooltip, message,
} from 'antd'
import {
  PlusOutlined, DeleteOutlined, SyncOutlined, CopyOutlined,
  ClusterOutlined, CheckCircleOutlined, CloseCircleOutlined,
  WarningOutlined,
} from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { frpmasterApi } from '../api'
import FormModal, { FormSection } from '../components/FormModal'
import { useTableStyle } from '../hooks/useTableStyle'

const { Text } = Typography

// 节点在线状态：心跳超过阈值即离线。
// 与后端 DefaultOfflineAfter 保持同步（90s），这里取略宽松窗口方便 UI 刷新。
const OFFLINE_WINDOW_MS = 100 * 1000

type NodeRow = {
  id: number
  name: string
  region: string
  serverAddr: string
  serverPort: number
  status: 'online' | 'offline'
  lastSeen: string
  remark: string
  nodeToken?: string // 仅创建响应返回
}

const FrpMasterNodes: React.FC = () => {
  const { t } = useTranslation()
  const tableStyle = useTableStyle()
  const [data, setData] = useState<NodeRow[]>([])
  const [loading, setLoading] = useState(false)
  const [creating, setCreating] = useState(false)
  const [lastToken, setLastToken] = useState<string | null>(null)
  const [lastName, setLastName] = useState<string>('')
  const [modalOpen, setModalOpen] = useState(false)
  const [viewRecord, setViewRecord] = useState<NodeRow | null>(null)

  const fetchData = async () => {
    setLoading(true)
    try {
      const res: any = await frpmasterApi.listNodes()
      const raw = (res?.data ?? []) as Array<Record<string, any>>
      const now = Date.now()
      const rows: NodeRow[] = raw.map((n) => ({
        id: n.id,
        name: n.name || '',
        region: n.region || '',
        serverAddr: n.server_addr || '',
        serverPort: n.server_port || 0,
        status: n.status === 'online' ? 'online' : 'offline',
        lastSeen: n.last_seen || '',
        remark: n.remark || '',
        nodeToken: n.node_token,
      }))
      // 本地兜底：last_seen 为空或过期的视为 offline。
      for (const r of rows) {
        if (r.status === 'online') {
          const ts = new Date(r.lastSeen).getTime()
          if (!ts || now - ts > OFFLINE_WINDOW_MS) {
            r.status = 'offline'
          }
        }
      }
      setData(rows)
    } catch {
      message.error(t('frpmaster.fetchFailed'))
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { fetchData() }, [])

  const handleCreate = async (values: Record<string, any>) => {
    setCreating(true)
    try {
      const res: any = await frpmasterApi.createNode({
        name: values.name,
        region: values.region,
        server_addr: values.server_addr,
        server_port: Number(values.server_port),
        frps_token: values.frps_token || '',
        remark: values.remark || '',
      })
      const node: any = res?.data
      if (!node) throw new Error('response data missing')
      setLastToken(node.node_token ?? null)
      setLastName(node.name ?? values.name ?? '')
      message.success(t('frpmaster.created'))
      // 创建成功后立刻刷新列表（新节点初始离线）
      await fetchData()
    } catch (e: any) {
      message.error(e?.message || t('frpmaster.createFailed'))
    } finally {
      setCreating(false)
    }
  }

  const handleDelete = async (id: number, name: string) => {
    try {
      await frpmasterApi.deleteNode(id)
      message.success(t('frpmaster.deleted'))
      await fetchData()
    } catch (e: any) {
      message.error(e?.message || t('frpmaster.deleteFailed'))
    }
  }

  const columns = [
    {
      title: t('common.name'),
      dataIndex: 'name',
      key: 'name',
      render: (v: string, r: NodeRow) => (
        <Text strong>{v}</Text>
      ),
    },
    {
      title: t('frpmaster.region'),
      dataIndex: 'region',
      key: 'region',
      width: 110,
      render: (v: string) => v ? <Tag>{v}</Tag> : <Text type="secondary">—</Text>,
    },
    {
      title: t('frpmaster.serverAddr'),
      dataIndex: 'serverAddr',
      key: 'serverAddr',
      width: 180,
      render: (v: string, r: NodeRow) =>
        v ? `${v}:${r.serverPort}` : <Text type="secondary">—</Text>,
    },
    {
      title: t('common.status'),
      dataIndex: 'status',
      key: 'status',
      width: 90,
      render: (v: 'online' | 'offline') => (
        <Tag color={v === 'online' ? 'green' : 'default'} icon={
          v === 'online' ? <CheckCircleOutlined /> : <CloseCircleOutlined />
        }>
          {v === 'online' ? t('common.online') : t('common.offline')}
        </Tag>
      ),
    },
    {
      title: t('frpmaster.lastSeen'),
      dataIndex: 'lastSeen',
      key: 'lastSeen',
      width: 170,
      render: (v: string) => {
        if (!v) return <Text type="secondary">—</Text>
        const ts = new Date(v).getTime()
        const now = Date.now()
        const diffSec = Math.floor((now - ts) / 1000)
        let hint = ''
        let color: string | undefined
        if (diffSec < 60) {
          hint = t('frpmaster.justNow', { count: diffSec })
          color = 'green'
        } else if (diffSec < 180) {
          hint = t('frpmaster.recent', { count: Math.floor(diffSec / 60) })
          color = 'orange'
        } else {
          hint = t('frpmaster.stale')
          color = 'red'
        }
        return (
          <Tooltip title={v}>
            <Text style={{ color }}>
              {new Date(v).toLocaleTimeString()} · {hint}
            </Text>
          </Tooltip>
        )
      },
    },
    {
      title: t('common.remark'),
      dataIndex: 'remark',
      key: 'remark',
      ellipsis: true,
      render: (v: string) => v ? <Text type="secondary">{v}</Text> : null,
    },
    {
      title: t('common.action'),
      key: 'action',
      width: 100,
      render: (_: any, r: NodeRow) => (
        <Space size={8}>
          <Tooltip title={t('frpmaster.remove')}>
            <Button
              type="text"
              danger
              icon={<DeleteOutlined />}
              onClick={() => handleDelete(r.id, r.name)}
            />
          </Tooltip>
        </Space>
      ),
    },
  ]

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Text strong style={{ fontSize: 16 }}>{t('frpmaster.title')}</Text>
        <Space>
          <Button icon={<SyncOutlined />} onClick={fetchData} loading={loading}>
            {t('common.refresh')}
          </Button>
          <Button
            type="primary"
            icon={<PlusOutlined />}
            onClick={() => setModalOpen(true)}
          >
            {t('common.create')}
          </Button>
        </Space>
      </div>

      <Table<NodeRow>
        rowKey="id"
        columns={columns}
        dataSource={data}
        loading={loading}
        size="middle"
        scroll={{ x: 800 }}
        pagination={{ pageSize: 20, showTotal: (t, r) => `${t} / ${r}` }}
        locale={{ emptyText: t('frpmaster.empty') }}
        {...tableStyle}
      />

      {/* 创建成功提示：node_token 仅此一次可见 */}
      <FormModal
        open={!!lastToken}
        title={t('frpmaster.tokenSaved')}
        icon={<WarningOutlined style={{ color: '#faad14' }} />}
        onCancel={() => { setLastToken(null); setLastName('') }}
        onOk={() => { setLastToken(null); setLastName('') }}
        footerExtra={
          lastToken && (
            <Button
              icon={<CopyOutlined />}
              onClick={() => {
                navigator.clipboard.writeText(lastToken)
                message.success(t('common.copied'))
              }}
            >
              {t('common.copy')}
            </Button>
          )
        }
      >
        <div style={{ padding: '8px 0' }}>
          <Text style={{ display: 'block', marginBottom: 8 }}>{t('frpmaster.tokenDesc', { name: lastName })}</Text>
          <FormSection title={t('frpmaster.nodeToken')} icon={<ClusterOutlined />} color="orange">
            <Text
              copyable={{ text: lastToken || '' }}
              style={{ fontFamily: 'monospace', fontSize: 13, wordBreak: 'break-all' }}
            >
              {lastToken}
            </Text>
          </FormSection>
        </div>
      </FormModal>
    </div>
  )
}

export default FrpMasterNodes
