import React, { useEffect, useState } from 'react'
import {
  Table, Button, Space, Switch, Modal, Form, Input, InputNumber,
  Popconfirm, message, Typography, Tooltip, Row, Col, Tag,
} from 'antd'
import {
  PlusOutlined, EditOutlined, DeleteOutlined,
  PlayCircleOutlined, StopOutlined, LinkOutlined,
} from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { npsServerApi } from '../api'
import StatusTag from '../components/StatusTag'

const { Text } = Typography

const NpsServer: React.FC = () => {
  const { t } = useTranslation()
  const [data, setData] = useState<any[]>([])
  const [loading, setLoading] = useState(false)
  const [modalOpen, setModalOpen] = useState(false)
  const [editRecord, setEditRecord] = useState<any>(null)
  const [form] = Form.useForm()

  const fetchData = async () => {
    setLoading(true)
    try {
      const res: any = await npsServerApi.list()
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
      bridge_port: 8024,
      http_port: 80,
      https_port: 443,
      web_port: 8080,
      web_username: 'admin',
      web_password: '123456',
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
      await npsServerApi.update(editRecord.id, values)
    } else {
      await npsServerApi.create(values)
    }
    message.success(t('common.success'))
    setModalOpen(false)
    fetchData()
  }

  const handleToggle = async (record: any, checked: boolean) => {
    await npsServerApi.update(record.id, { ...record, enable: checked })
    checked ? await npsServerApi.start(record.id) : await npsServerApi.stop(record.id)
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
      title: t('nps.bridgePort'),
      render: (_: any, r: any) => (
        <Text code style={{ fontSize: 12 }}>{r.bind_addr || '0.0.0.0'}:{r.bridge_port || 8024}</Text>
      ),
    },
    {
      title: t('nps.authKey'), dataIndex: 'auth_key',
      render: (v: string) => v ? <Text type="secondary">••••••</Text> : <Tag>无认证</Tag>,
    },
    {
      title: t('nps.webPanel'), width: 180,
      render: (_: any, r: any) => r.web_port ? (
        <Space size={4}>
          <Text code style={{ fontSize: 12 }}>{r.bind_addr || '0.0.0.0'}:{r.web_port}</Text>
          {r.status === 'running' && (
            <Tooltip title={t('nps.openWebPanel')}>
              <Button
                size="small" type="link" icon={<LinkOutlined />}
                onClick={() => window.open(`http://${location.hostname}:${r.web_port}`, '_blank')}
                style={{ padding: 0 }}
              />
            </Tooltip>
          )}
        </Space>
      ) : '-',
    },
    {
      title: t('common.action'), width: 140,
      render: (_: any, r: any) => (
        <Space size={4}>
          {r.status === 'running'
            ? <Tooltip title={t('common.stop')}><Button size="small" icon={<StopOutlined />} onClick={async () => { await npsServerApi.stop(r.id); fetchData() }} /></Tooltip>
            : <Tooltip title={t('common.start')}><Button size="small" type="primary" icon={<PlayCircleOutlined />} onClick={async () => { await npsServerApi.start(r.id); fetchData() }} /></Tooltip>
          }
          <Tooltip title={t('common.edit')}>
            <Button size="small" icon={<EditOutlined />} onClick={() => handleEdit(r)} />
          </Tooltip>
          <Popconfirm title={t('common.deleteConfirm')} onConfirm={async () => { await npsServerApi.delete(r.id); fetchData() }}>
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
        <Typography.Title level={4} style={{ margin: 0 }}>{t('nps.serverTitle')}</Typography.Title>
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
        width={580} destroyOnClose
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
            <Col span={12}>
              <Form.Item name="bind_addr" label={t('nps.bindAddr')}>
                <Input placeholder="0.0.0.0" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="bridge_port" label={t('nps.bridgePort')} rules={[{ required: true }]}>
                <InputNumber min={1} max={65535} style={{ width: '100%' }} placeholder="8024" />
              </Form.Item>
            </Col>
          </Row>

          <Row gutter={16}>
            <Col span={8}>
              <Form.Item name="http_port" label={t('nps.httpPort')}>
                <InputNumber min={0} max={65535} style={{ width: '100%' }} placeholder="80" />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="https_port" label={t('nps.httpsPort')}>
                <InputNumber min={0} max={65535} style={{ width: '100%' }} placeholder="443" />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="web_port" label={t('nps.webPort')} rules={[{ required: true }]}>
                <InputNumber min={1} max={65535} style={{ width: '100%' }} placeholder="8080" />
              </Form.Item>
            </Col>
          </Row>

          <Row gutter={16}>
            <Col span={12}>
              <Form.Item name="web_username" label={t('nps.webUsername')}>
                <Input placeholder="admin" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="web_password" label={t('nps.webPassword')}>
                <Input.Password placeholder="管理面板密码" />
              </Form.Item>
            </Col>
          </Row>

          <Form.Item name="auth_key" label={t('nps.authKey')}>
            <Input.Password placeholder="客户端连接认证密钥（留空不启用）" />
          </Form.Item>

          <Form.Item name="remark" label={t('common.remark')}>
            <Input.TextArea rows={2} placeholder="备注（可选）" />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

export default NpsServer
