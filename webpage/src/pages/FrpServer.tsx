import React, { useEffect, useState } from 'react'
import {
  Table, Button, Space, Switch, Modal, Form, Input, InputNumber,
  Popconfirm, message, Typography, Tag, Tooltip, Row, Col,
} from 'antd'
import {
  PlusOutlined, EditOutlined, DeleteOutlined,
  PlayCircleOutlined, StopOutlined, LinkOutlined,
} from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { frpsApi } from '../api'
import StatusTag from '../components/StatusTag'

const { Text } = Typography

const FrpServer: React.FC = () => {
  const { t } = useTranslation()
  const [data, setData] = useState<any[]>([])
  const [loading, setLoading] = useState(false)
  const [modalOpen, setModalOpen] = useState(false)
  const [editRecord, setEditRecord] = useState<any>(null)
  const [form] = Form.useForm()

  const fetchData = async () => {
    setLoading(true)
    try {
      const res: any = await frpsApi.list()
      setData(res.data || [])
    } finally { setLoading(false) }
  }

  useEffect(() => { fetchData() }, [])

  const handleCreate = () => {
    setEditRecord(null)
    form.resetFields()
    form.setFieldsValue({
      enable: true,
      bind_addr: '0.0.0.0',
      bind_port: 7000,
      log_level: 'info',
      max_ports_per_client: 0,
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
      await frpsApi.update(editRecord.id, values)
    } else {
      await frpsApi.create(values)
    }
    message.success(t('common.success'))
    setModalOpen(false)
    fetchData()
  }

  const handleToggle = async (record: any, checked: boolean) => {
    await frpsApi.update(record.id, { ...record, enable: checked })
    checked ? await frpsApi.start(record.id) : await frpsApi.stop(record.id)
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
      title: '监听地址',
      render: (_: any, r: any) => (
        <Text code style={{ fontSize: 12 }}>{r.bind_addr || '0.0.0.0'}:{r.bind_port || 7000}</Text>
      ),
    },
    {
      title: 'Token', dataIndex: 'token',
      render: (v: string) => v ? <Text type="secondary">••••••</Text> : <Tag>无认证</Tag>,
    },
    {
      title: 'Dashboard', width: 160,
      render: (_: any, r: any) => r.dashboard_port ? (
        <Space size={4}>
          <Text code style={{ fontSize: 12 }}>{r.dashboard_addr || '0.0.0.0'}:{r.dashboard_port}</Text>
          {r.status === 'running' && (
            <Tooltip title="打开 Dashboard">
              <Button
                size="small" type="link" icon={<LinkOutlined />}
                onClick={() => window.open(`http://${r.dashboard_addr || location.hostname}:${r.dashboard_port}`, '_blank')}
                style={{ padding: 0 }}
              />
            </Tooltip>
          )}
        </Space>
      ) : '-',
    },
    {
      title: t('common.action'), width: 160,
      render: (_: any, r: any) => (
        <Space size={4}>
          {r.status === 'running'
            ? <Tooltip title={t('common.stop')}><Button size="small" icon={<StopOutlined />} onClick={async () => { await frpsApi.stop(r.id); fetchData() }} /></Tooltip>
            : <Tooltip title={t('common.start')}><Button size="small" type="primary" icon={<PlayCircleOutlined />} onClick={async () => { await frpsApi.start(r.id); fetchData() }} /></Tooltip>
          }
          <Tooltip title={t('common.edit')}>
            <Button size="small" icon={<EditOutlined />} onClick={() => handleEdit(r)} />
          </Tooltip>
          <Popconfirm title={t('common.deleteConfirm')} onConfirm={async () => { await frpsApi.delete(r.id); fetchData() }}>
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
        <Typography.Title level={4} style={{ margin: 0 }}>{t('frp.serverTitle')}</Typography.Title>
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
        width={560} destroyOnClose
      >
        <Form form={form} layout="vertical" style={{ marginTop: 16 }}>
          <Row gutter={16}>
            <Col span={16}>
              <Form.Item name="name" label={t('common.name')} rules={[{ required: true }]}>
                <Input placeholder="服务端名称" />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="enable" label={t('common.enable')} valuePropName="checked">
                <Switch />
              </Form.Item>
            </Col>
          </Row>

          <Row gutter={16}>
            <Col span={16}>
              <Form.Item name="bind_addr" label={t('frp.bindAddr')}>
                <Input placeholder="0.0.0.0" />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="bind_port" label={t('frp.bindPort')} rules={[{ required: true }]}>
                <InputNumber min={1} max={65535} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
          </Row>

          <Form.Item name="token" label={t('frp.token')}>
            <Input.Password placeholder="认证 Token（留空不启用认证）" />
          </Form.Item>

          <Form.Item
            label="Dashboard 配置"
            style={{ marginBottom: 0 }}
          >
            <Row gutter={16}>
              <Col span={8}>
                <Form.Item name="dashboard_port" label={t('frp.dashboardPort')}>
                  <InputNumber min={1} max={65535} style={{ width: '100%' }} placeholder="端口" />
                </Form.Item>
              </Col>
              <Col span={8}>
                <Form.Item name="dashboard_user" label={t('frp.dashboardUser')}>
                  <Input placeholder="用户名" />
                </Form.Item>
              </Col>
              <Col span={8}>
                <Form.Item name="dashboard_password" label={t('frp.dashboardPassword')}>
                  <Input.Password placeholder="密码" />
                </Form.Item>
              </Col>
            </Row>
          </Form.Item>

          <Form.Item name="max_ports_per_client" label="每客户端最大端口数">
            <InputNumber min={0} style={{ width: '100%' }} placeholder="0 表示不限制" />
          </Form.Item>

          <Form.Item name="remark" label={t('common.remark')}>
            <Input.TextArea rows={2} placeholder="备注（可选）" />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

export default FrpServer
