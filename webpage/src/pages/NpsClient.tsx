import React, { useEffect, useState } from 'react'
import {
  Table, Button, Space, Switch, Modal, Form, Input, InputNumber,
  Popconfirm, message, Typography, Tooltip, Row, Col, Select,
} from 'antd'
import {
  PlusOutlined, EditOutlined, DeleteOutlined,
  PlayCircleOutlined, StopOutlined,
} from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { npsClientApi } from '../api'
import StatusTag from '../components/StatusTag'

const { Text } = Typography

const NpsClient: React.FC = () => {
  const { t } = useTranslation()
  const [data, setData] = useState<any[]>([])
  const [loading, setLoading] = useState(false)
  const [modalOpen, setModalOpen] = useState(false)
  const [editRecord, setEditRecord] = useState<any>(null)
  const [form] = Form.useForm()

  const fetchData = async () => {
    setLoading(true)
    try {
      const res: any = await npsClientApi.list()
      setData(res.data || [])
    } finally { setLoading(false) }
  }

  useEffect(() => { fetchData() }, [])

  const handleCreate = () => {
    setEditRecord(null)
    form.resetFields()
    form.setFieldsValue({
      enable: true,
      server_port: 8024,
      conn_type: 'tcp',
      log_level: 'info',
    })
    setModalOpen(true)
  }

  const handleEdit = (record: any) => {
    setEditRecord(record)
    form.setFieldsValue(record)
    setModalOpen(true)
  }

  const handleSubmit = async () => {
    const values = await form.validateFields()
    if (editRecord) {
      await npsClientApi.update(editRecord.id, values)
    } else {
      await npsClientApi.create(values)
    }
    message.success(t('common.success'))
    setModalOpen(false)
    fetchData()
  }

  const handleToggle = async (record: any, checked: boolean) => {
    await npsClientApi.update(record.id, { ...record, enable: checked })
    checked ? await npsClientApi.start(record.id) : await npsClientApi.stop(record.id)
    fetchData()
  }

  const columns = [
    {
      title: t('common.status'), dataIndex: 'status', width: 100,
      render: (s: string) => <StatusTag status={s} />,
    },
    {
      title: t('common.enable'), dataIndex: 'enable', width: 80,
      render: (v: boolean, r: any) => (
        <Switch size="small" checked={v} onChange={(c) => handleToggle(r, c)} />
      ),
    },
    {
      title: t('common.name'), dataIndex: 'name',
      render: (name: string, r: any) => (
        <div>
          <Text strong>{name}</Text>
          {r.remark && <div><Text type="secondary" style={{ fontSize: 12 }}>{r.remark}</Text></div>}
        </div>
      ),
    },
    {
      title: t('nps.serverAddr'),
      render: (_: any, r: any) => (
        <Text code style={{ fontSize: 12 }}>{r.server_addr}:{r.server_port || 8024} ({r.conn_type || 'tcp'})</Text>
      ),
    },
    {
      title: t('nps.vkeyOrId'), dataIndex: 'vkey_or_id',
      render: (v: string) => v ? <Text code style={{ fontSize: 12 }}>{v}</Text> : '-',
    },
    {
      title: t('nps.authKey'), dataIndex: 'auth_key',
      render: (v: string) => v ? <Text type="secondary">••••••</Text> : '-',
    },
    {
      title: t('common.action'), width: 140,
      render: (_: any, r: any) => (
        <Space size={4}>
          {r.status === 'running'
            ? <Tooltip title={t('common.stop')}><Button size="small" icon={<StopOutlined />} onClick={async () => { await npsClientApi.stop(r.id); fetchData() }} /></Tooltip>
            : <Tooltip title={t('common.start')}><Button size="small" type="primary" icon={<PlayCircleOutlined />} onClick={async () => { await npsClientApi.start(r.id); fetchData() }} /></Tooltip>
          }
          <Tooltip title={t('common.edit')}>
            <Button size="small" icon={<EditOutlined />} onClick={() => handleEdit(r)} />
          </Tooltip>
          <Popconfirm title={t('common.deleteConfirm')} onConfirm={async () => { await npsClientApi.delete(r.id); fetchData() }}>
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
        <Typography.Title level={4} style={{ margin: 0 }}>{t('nps.clientTitle')}</Typography.Title>
        <Button type="primary" icon={<PlusOutlined />} onClick={handleCreate}>{t('common.create')}</Button>
      </div>

      <Table
        dataSource={data} columns={columns} rowKey="id" loading={loading}
        size="middle" style={{ background: '#fff', borderRadius: 8 }}
        pagination={{ pageSize: 20, showSizeChanger: true }}
      />

      <Modal
        title={editRecord ? t('common.edit') : t('common.create')}
        open={modalOpen} onOk={handleSubmit} onCancel={() => setModalOpen(false)}
        width={520} destroyOnClose
      >
        <Form form={form} layout="vertical" style={{ marginTop: 16 }}>
          <Row gutter={16}>
            <Col span={16}>
              <Form.Item name="name" label={t('common.name')} rules={[{ required: true }]}>
                <Input placeholder="客户端名称" />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="enable" label={t('common.enable')} valuePropName="checked">
                <Switch />
              </Form.Item>
            </Col>
          </Row>

          <Row gutter={16}>
            <Col span={12}>
              <Form.Item name="server_addr" label={t('nps.serverAddr')} rules={[{ required: true }]}>
                <Input placeholder="NPS 服务器地址" />
              </Form.Item>
            </Col>
            <Col span={6}>
              <Form.Item name="server_port" label={t('nps.serverPort')} rules={[{ required: true }]}>
                <InputNumber min={1} max={65535} style={{ width: '100%' }} placeholder="8024" />
              </Form.Item>
            </Col>
            <Col span={6}>
              <Form.Item name="conn_type" label={t('nps.connType')}>
                <Select>
                  <Select.Option value="tcp">TCP</Select.Option>
                  <Select.Option value="tls">TLS</Select.Option>
                  <Select.Option value="kcp">KCP</Select.Option>
                  <Select.Option value="quic">QUIC</Select.Option>
                  <Select.Option value="ws">WS</Select.Option>
                  <Select.Option value="wss">WSS</Select.Option>
                </Select>
              </Form.Item>
            </Col>
          </Row>

          <Form.Item
            name="vkey_or_id"
            label={t('nps.vkeyOrId')}
            tooltip={t('nps.vkeyOrIdTip')}
          >
            <Input placeholder="客户端唯一标识 vkey（在NPS管理面板中获取）" />
          </Form.Item>

          <Form.Item name="auth_key" label={t('nps.authKey')}>
            <Input.Password placeholder="全局认证密钥（与服务端 auth_key 一致）" />
          </Form.Item>

          <Form.Item name="remark" label={t('common.remark')}>
            <Input.TextArea rows={2} placeholder="备注（可选）" />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

export default NpsClient
