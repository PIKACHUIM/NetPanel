import React, { useEffect, useState } from 'react'
import {
  Table, Button, Space, Form, Input, Switch,
  Popconfirm, message, Typography, Tooltip, Tag, Badge,
} from 'antd'
import {
  PlusOutlined, EditOutlined, DeleteOutlined, CheckCircleOutlined,
  CloseCircleOutlined, SyncOutlined, EyeOutlined, ClusterOutlined,
} from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { meshNodeApi } from '../api'
import FormModal, { FormSection } from '../components/FormModal'
import { useTableStyle } from '../hooks/useTableStyle'

const { Text } = Typography

const MeshNodes: React.FC = () => {
  const { t } = useTranslation()
  const tableStyle = useTableStyle()
  const [data, setData] = useState<any[]>([])
  const [loading, setLoading] = useState(false)
  const [modalOpen, setModalOpen] = useState(false)
  const [editRecord, setEditRecord] = useState<any>(null)
  const [confirmLoading, setConfirmLoading] = useState(false)
  const [checkingIds, setCheckingIds] = useState<Set<number>>(new Set())
  const [viewRecord, setViewRecord] = useState<any>(null)
  const [form] = Form.useForm()

  const fetchData = async () => {
    setLoading(true)
    try {
      const res: any = await meshNodeApi.listNodes()
      setData(res.data || [])
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { fetchData() }, [])

  const handleOpen = (record?: any) => {
    if (record) {
      setEditRecord(record)
      form.setFieldsValue(record)
    } else {
      setEditRecord(null)
      form.resetFields()
      form.setFieldsValue({ enable: true })
    }
    setModalOpen(true)
  }

  const handleSubmit = async () => {
    const values = await form.validateFields()
    setConfirmLoading(true)
    try {
      if (editRecord) {
        await meshNodeApi.updateNode(editRecord.id, values)
      } else {
        await meshNodeApi.createNode(values)
      }
      message.success(t('common.success'))
      setModalOpen(false)
      fetchData()
    } finally {
      setConfirmLoading(false)
    }
  }

  const handleCheck = async (id: number) => {
    setCheckingIds(prev => new Set(prev).add(id))
    try {
      const res: any = await meshNodeApi.checkNode(id)
      const d = res.data
      if (d.is_online) {
        message.success(`节点在线，延迟 ${d.latency}ms`)
      } else {
        message.warning(`节点不可达: ${d.error || '未知错误'}`)
      }
      fetchData()
    } finally {
      setCheckingIds(prev => { const s = new Set(prev); s.delete(id); return s })
    }
  }

  // 解析节点间延迟
  const parsePeerLatencies = (raw: string): Record<string, number> => {
    if (!raw) return {}
    try { return JSON.parse(raw) } catch { return {} }
  }

  const columns = [
    {
      title: '节点名称', dataIndex: 'name', width: 200,
      render: (name: string, r: any) => (
        <div>
          <Space>
            <ClusterOutlined style={{ color: '#1677ff' }} />
            <Text strong>{name}</Text>
          </Space>
          {r.remark && <div><Text type="secondary" style={{ fontSize: 12 }}>{r.remark}</Text></div>}
        </div>
      ),
    },
    {
      title: '节点URL', dataIndex: 'url', width: 250,
      render: (v: string) => <Text code style={{ fontSize: 12 }}>{v}</Text>,
    },
    {
      title: '节点IP', dataIndex: 'node_ip', width: 130,
      render: (v: string) => v ? <Text code>{v}</Text> : <Text type="secondary">-</Text>,
    },
    {
      title: '状态', width: 100,
      render: (_: any, r: any) => (
        <Space>
          {r.is_online ? (
            <Badge status="success" text={<Text style={{ color: '#52c41a' }}>在线</Text>} />
          ) : (
            <Badge status="error" text={<Text type="secondary">离线</Text>} />
          )}
        </Space>
      ),
    },
    {
      title: '延迟', dataIndex: 'latency', width: 80,
      render: (v: number) => {
        if (v < 0) return <Text type="secondary">-</Text>
        const color = v < 50 ? '#52c41a' : v < 200 ? '#faad14' : '#ff4d4f'
        return <Tag color={color}>{v}ms</Tag>
      },
    },
    {
      title: '启用', dataIndex: 'enable', width: 70,
      render: (v: boolean) => v ? <CheckCircleOutlined style={{ color: '#52c41a' }} /> : <CloseCircleOutlined style={{ color: '#d9d9d9' }} />,
    },
    {
      title: t('common.action'), width: 200,
      render: (_: any, r: any) => (
        <Space size={4}>
          <Tooltip title="检测连通性">
            <Button size="small" icon={<SyncOutlined spin={checkingIds.has(r.id)} />}
              loading={checkingIds.has(r.id)} onClick={() => handleCheck(r.id)} />
          </Tooltip>
          <Tooltip title="查看详情">
            <Button size="small" icon={<EyeOutlined />} onClick={() => setViewRecord(r)} />
          </Tooltip>
          <Tooltip title={t('common.edit')}>
            <Button size="small" icon={<EditOutlined />} onClick={() => handleOpen(r)} />
          </Tooltip>
          <Popconfirm title={t('common.deleteConfirm')} onConfirm={async () => { await meshNodeApi.deleteNode(r.id); fetchData() }}>
            <Tooltip title={t('common.delete')}>
              <Button size="small" danger icon={<DeleteOutlined />} />
            </Tooltip>
          </Popconfirm>
        </Space>
      ),
    },
  ]

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Typography.Title level={4} style={{ margin: 0 }}>
          <ClusterOutlined style={{ marginRight: 8 }} />
          组网节点管理
        </Typography.Title>
        <Space>
          <Button icon={<SyncOutlined />} onClick={fetchData}>刷新</Button>
          <Button type="primary" icon={<PlusOutlined />} onClick={() => handleOpen()}>
            {t('common.create')}
          </Button>
        </Space>
      </div>

      <Table
        dataSource={data} columns={columns} rowKey="id" loading={loading}
        size="middle" style={{ borderRadius: 8 }}
        pagination={{ pageSize: 20, showSizeChanger: true }}
      />

      {/* 新建/编辑模态框 */}
      <FormModal
        open={modalOpen}
        title={editRecord ? '编辑节点' : '新建节点'}
        icon={<ClusterOutlined />}
        isEdit={!!editRecord}
        confirmLoading={confirmLoading}
        onOk={handleSubmit}
        onCancel={() => setModalOpen(false)}
        width={560}
      >
        <Form form={form} layout="vertical">
          <FormSection title="基本信息" icon={<ClusterOutlined />} color="blue">
            <Form.Item name="name" label="节点名称" rules={[{ required: true, message: '请输入节点名称' }]}>
              <Input placeholder="如：北京节点、家里的服务器" />
            </Form.Item>
            <Form.Item name="url" label="节点URL" rules={[{ required: true, message: '请输入节点URL' }]}
              extra="远程 NetPanel 的访问地址，如 http://192.168.1.100:8080">
              <Input placeholder="http://IP:端口" />
            </Form.Item>
          </FormSection>

          <FormSection title="认证信息" icon={<EditOutlined />} color="orange">
            <Form.Item name="admin_user" label="管理员用户" rules={[{ required: true, message: '请输入管理员用户名' }]}>
              <Input placeholder="远程节点的登录用户名" />
            </Form.Item>
            <Form.Item name="admin_password" label="管理员密码" rules={[{ required: true, message: '请输入管理员密码' }]}>
              <Input.Password placeholder="远程节点的登录密码" />
            </Form.Item>
          </FormSection>

          <FormSection title="其他设置" icon={<EditOutlined />} color="green">
            <Form.Item name="enable" label="是否启用" valuePropName="checked">
              <Switch checkedChildren="启用" unCheckedChildren="禁用" />
            </Form.Item>
            <Form.Item name="remark" label="节点备注">
              <Input.TextArea rows={2} placeholder="备注信息（可选）" />
            </Form.Item>
          </FormSection>
        </Form>
      </FormModal>

      {/* 查看详情模态框 */}
      <FormModal
        open={!!viewRecord}
        title="节点详情"
        icon={<EyeOutlined />}
        onOk={() => setViewRecord(null)}
        onCancel={() => setViewRecord(null)}
        okText="关闭"
        width={600}
      >
        {viewRecord && (
          <div>
            <FormSection title="基本信息" icon={<ClusterOutlined />} color="blue">
              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '8px 16px' }}>
                <div><Text type="secondary">名称：</Text><Text strong>{viewRecord.name}</Text></div>
                <div><Text type="secondary">URL：</Text><Text code>{viewRecord.url}</Text></div>
                <div><Text type="secondary">IP：</Text><Text code>{viewRecord.node_ip || '-'}</Text></div>
                <div><Text type="secondary">状态：</Text>
                  {viewRecord.is_online ? <Tag color="success">在线</Tag> : <Tag color="error">离线</Tag>}
                </div>
                <div><Text type="secondary">延迟：</Text>
                  {viewRecord.latency >= 0 ? <Tag color="blue">{viewRecord.latency}ms</Tag> : <Text type="secondary">-</Text>}
                </div>
                <div><Text type="secondary">启用：</Text>
                  {viewRecord.enable ? <Tag color="success">是</Tag> : <Tag color="default">否</Tag>}
                </div>
                <div><Text type="secondary">最后心跳：</Text>
                  <Text>{viewRecord.last_heartbeat || '-'}</Text>
                </div>
              </div>
            </FormSection>

            {viewRecord.peer_latencies && (
              <FormSection title="节点间连通性" icon={<SyncOutlined />} color="purple">
                {(() => {
                  const peers = parsePeerLatencies(viewRecord.peer_latencies)
                  const entries = Object.entries(peers)
                  if (entries.length === 0) return <Text type="secondary">暂无数据</Text>
                  return (
                    <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8 }}>
                      {entries.map(([nodeId, latency]) => {
                        const targetNode = data.find((n: any) => String(n.id) === nodeId)
                        const name = targetNode?.name || `节点#${nodeId}`
                        return (
                          <Tag key={nodeId} color={latency >= 0 ? (latency < 100 ? 'green' : 'orange') : 'red'}>
                            {name}: {latency >= 0 ? `${latency}ms` : '不可达'}
                          </Tag>
                        )
                      })}
                    </div>
                  )
                })()}
              </FormSection>
            )}

            {viewRecord.remark && (
              <FormSection title="备注" icon={<EditOutlined />} color="green">
                <Text>{viewRecord.remark}</Text>
              </FormSection>
            )}
          </div>
        )}
      </FormModal>
    </div>
  )
}

export default MeshNodes
