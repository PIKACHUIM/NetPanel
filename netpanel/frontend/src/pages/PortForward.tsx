import React, { useEffect, useState } from 'react'
import {
  Table, Button, Space, Switch, Modal, Form, Input, Select, InputNumber,
  Popconfirm, message, Tag, Tooltip, Typography,
} from 'antd'
import { PlusOutlined, EditOutlined, DeleteOutlined, PlayCircleOutlined, StopOutlined, FileTextOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { portForwardApi } from '../api'
import StatusTag from '../components/StatusTag'

const { Text } = Typography
const { Option } = Select

interface PortForwardRule {
  id: number
  name: string
  enable: boolean
  listen_ip: string
  listen_port: number
  target_address: string
  target_port: number
  protocol: string
  max_connections: number
  status: string
  last_error: string
  remark: string
}

const PortForward: React.FC = () => {
  const { t } = useTranslation()
  const [data, setData] = useState<PortForwardRule[]>([])
  const [loading, setLoading] = useState(false)
  const [modalOpen, setModalOpen] = useState(false)
  const [editRecord, setEditRecord] = useState<PortForwardRule | null>(null)
  const [logModalOpen, setLogModalOpen] = useState(false)
  const [logs, setLogs] = useState<string[]>([])
  const [form] = Form.useForm()

  const fetchData = async () => {
    setLoading(true)
    try {
      const res: any = await portForwardApi.list()
      setData(res.data || [])
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { fetchData() }, [])

  const handleCreate = () => {
    setEditRecord(null)
    form.resetFields()
    form.setFieldsValue({ protocol: 'tcp', listen_ip: '0.0.0.0', max_connections: 256, enable: true })
    setModalOpen(true)
  }

  const handleEdit = (record: PortForwardRule) => {
    setEditRecord(record)
    form.setFieldsValue(record)
    setModalOpen(true)
  }

  const handleDelete = async (id: number) => {
    await portForwardApi.delete(id)
    message.success(t('common.success'))
    fetchData()
  }

  const handleToggle = async (record: PortForwardRule, checked: boolean) => {
    await portForwardApi.update(record.id, { ...record, enable: checked })
    if (checked) {
      await portForwardApi.start(record.id)
    } else {
      await portForwardApi.stop(record.id)
    }
    fetchData()
  }

  const handleStart = async (id: number) => {
    await portForwardApi.start(id)
    message.success('已启动')
    fetchData()
  }

  const handleStop = async (id: number) => {
    await portForwardApi.stop(id)
    message.success('已停止')
    fetchData()
  }

  const handleViewLogs = async (id: number) => {
    const res: any = await portForwardApi.getLogs(id)
    setLogs(res.data || [])
    setLogModalOpen(true)
  }

  const handleSubmit = async () => {
    const values = await form.validateFields()
    if (editRecord) {
      await portForwardApi.update(editRecord.id, values)
    } else {
      await portForwardApi.create(values)
    }
    message.success(t('common.success'))
    setModalOpen(false)
    fetchData()
  }

  const columns = [
    {
      title: t('common.status'),
      dataIndex: 'status',
      width: 100,
      render: (status: string) => <StatusTag status={status} />,
    },
    {
      title: t('common.enable'),
      dataIndex: 'enable',
      width: 80,
      render: (enable: boolean, record: PortForwardRule) => (
        <Switch
          size="small"
          checked={enable}
          onChange={(checked) => handleToggle(record, checked)}
        />
      ),
    },
    {
      title: t('common.name'),
      dataIndex: 'name',
      render: (name: string, record: PortForwardRule) => (
        <div>
          <Text strong>{name}</Text>
          {record.remark && <div><Text type="secondary" style={{ fontSize: 12 }}>{record.remark}</Text></div>}
        </div>
      ),
    },
    {
      title: '监听',
      render: (_: any, record: PortForwardRule) => (
        <Text code>{record.listen_ip}:{record.listen_port}</Text>
      ),
    },
    {
      title: '目标',
      render: (_: any, record: PortForwardRule) => (
        <Text code>{record.target_address}:{record.target_port}</Text>
      ),
    },
    {
      title: t('common.protocol'),
      dataIndex: 'protocol',
      width: 100,
      render: (p: string) => <Tag>{p?.toUpperCase()}</Tag>,
    },
    {
      title: t('common.action'),
      width: 180,
      render: (_: any, record: PortForwardRule) => (
        <Space size={4}>
          {record.status === 'running' ? (
            <Tooltip title={t('common.stop')}>
              <Button size="small" icon={<StopOutlined />} onClick={() => handleStop(record.id)} />
            </Tooltip>
          ) : (
            <Tooltip title={t('common.start')}>
              <Button size="small" type="primary" icon={<PlayCircleOutlined />} onClick={() => handleStart(record.id)} />
            </Tooltip>
          )}
          <Tooltip title={t('common.viewLogs')}>
            <Button size="small" icon={<FileTextOutlined />} onClick={() => handleViewLogs(record.id)} />
          </Tooltip>
          <Tooltip title={t('common.edit')}>
            <Button size="small" icon={<EditOutlined />} onClick={() => handleEdit(record)} />
          </Tooltip>
          <Popconfirm title={t('common.deleteConfirm')} onConfirm={() => handleDelete(record.id)}>
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
        <Typography.Title level={4} style={{ margin: 0 }}>{t('portForward.title')}</Typography.Title>
        <Button type="primary" icon={<PlusOutlined />} onClick={handleCreate}>
          {t('common.create')}
        </Button>
      </div>

      <Table
        dataSource={data}
        columns={columns}
        rowKey="id"
        loading={loading}
        size="middle"
        style={{ background: '#fff', borderRadius: 8 }}
        pagination={{ pageSize: 20, showSizeChanger: true }}
      />

      {/* 编辑弹窗 */}
      <Modal
        title={editRecord ? t('common.edit') : t('common.create')}
        open={modalOpen}
        onOk={handleSubmit}
        onCancel={() => setModalOpen(false)}
        width={560}
        destroyOnClose
      >
        <Form form={form} layout="vertical" style={{ marginTop: 16 }}>
          <Form.Item name="name" label={t('common.name')} rules={[{ required: true }]}>
            <Input placeholder="规则名称" />
          </Form.Item>
          <Form.Item name="enable" label={t('common.enable')} valuePropName="checked">
            <Switch />
          </Form.Item>
          <Form.Item label="监听设置">
            <Space.Compact style={{ width: '100%' }}>
              <Form.Item name="listen_ip" noStyle>
                <Input style={{ width: '60%' }} placeholder="监听IP (0.0.0.0)" />
              </Form.Item>
              <Form.Item name="listen_port" noStyle rules={[{ required: true }]}>
                <InputNumber style={{ width: '40%' }} placeholder="端口" min={1} max={65535} />
              </Form.Item>
            </Space.Compact>
          </Form.Item>
          <Form.Item label="目标设置">
            <Space.Compact style={{ width: '100%' }}>
              <Form.Item name="target_address" noStyle rules={[{ required: true }]}>
                <Input style={{ width: '60%' }} placeholder="目标IP/域名" />
              </Form.Item>
              <Form.Item name="target_port" noStyle rules={[{ required: true }]}>
                <InputNumber style={{ width: '40%' }} placeholder="端口" min={1} max={65535} />
              </Form.Item>
            </Space.Compact>
          </Form.Item>
          <Form.Item name="protocol" label={t('common.protocol')} rules={[{ required: true }]}>
            <Select>
              <Option value="tcp">TCP</Option>
              <Option value="udp">UDP</Option>
              <Option value="tcp+udp">TCP+UDP</Option>
            </Select>
          </Form.Item>
          <Form.Item name="max_connections" label={t('portForward.maxConnections')}>
            <InputNumber min={1} max={10000} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="remark" label={t('common.remark')}>
            <Input.TextArea rows={2} placeholder="备注（可选）" />
          </Form.Item>
        </Form>
      </Modal>

      {/* 日志弹窗 */}
      <Modal
        title={t('common.logs')}
        open={logModalOpen}
        onCancel={() => setLogModalOpen(false)}
        footer={null}
        width={700}
      >
        <div style={{
          background: '#1a1a1a',
          borderRadius: 6,
          padding: 16,
          maxHeight: 400,
          overflow: 'auto',
          fontFamily: 'monospace',
          fontSize: 12,
          color: '#d4d4d4',
        }}>
          {logs.length > 0 ? logs.map((log, i) => (
            <div key={i} style={{ marginBottom: 2 }}>{log}</div>
          )) : <Text style={{ color: '#666' }}>暂无日志</Text>}
        </div>
      </Modal>
    </div>
  )
}

export default PortForward
