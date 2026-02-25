import React, { useEffect, useState } from 'react'
import {
  Table, Button, Space, Switch, Modal, Form, Input, InputNumber,
  Popconfirm, message, Typography, Tag, Tooltip, Divider, Row, Col, Card,
} from 'antd'
import {
  PlusOutlined, EditOutlined, DeleteOutlined,
  PlayCircleOutlined, StopOutlined, InfoCircleOutlined,
} from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { stunApi } from '../api'
import StatusTag from '../components/StatusTag'

const { Text } = Typography

const Stun: React.FC = () => {
  const { t } = useTranslation()
  const [data, setData] = useState<any[]>([])
  const [loading, setLoading] = useState(false)
  const [modalOpen, setModalOpen] = useState(false)
  const [detailOpen, setDetailOpen] = useState(false)
  const [detailRecord, setDetailRecord] = useState<any>(null)
  const [editRecord, setEditRecord] = useState<any>(null)
  const [form] = Form.useForm()

  const fetchData = async () => {
    setLoading(true)
    try {
      const res: any = await stunApi.list()
      setData(res.data || [])
    } finally { setLoading(false) }
  }

  useEffect(() => { fetchData() }, [])

  const handleCreate = () => {
    setEditRecord(null)
    form.resetFields()
    form.setFieldsValue({
      enable: true,
      stun_server: 'stun.l.google.com:19302',
      use_upnp: false,
      use_natmap: false,
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
      await stunApi.update(editRecord.id, values)
    } else {
      await stunApi.create(values)
    }
    message.success(t('common.success'))
    setModalOpen(false)
    fetchData()
  }

  const handleToggle = async (record: any, checked: boolean) => {
    await stunApi.update(record.id, { ...record, enable: checked })
    checked ? await stunApi.start(record.id) : await stunApi.stop(record.id)
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
      title: t('stun.stunServer'), dataIndex: 'stun_server',
      render: (v: string) => <Text code style={{ fontSize: 12 }}>{v}</Text>,
    },
    {
      title: t('stun.currentIP'),
      render: (_: any, r: any) => r.current_ip
        ? <Text code style={{ color: '#52c41a' }}>{r.current_ip}:{r.current_port}</Text>
        : <Text type="secondary">-</Text>,
    },
    {
      title: t('stun.natType'), dataIndex: 'nat_type',
      render: (v: string) => v ? <Tag color="blue">{v}</Tag> : '-',
    },
    {
      title: '选项', width: 100,
      render: (_: any, r: any) => (
        <Space size={4}>
          {r.use_upnp && <Tag color="purple" style={{ fontSize: 11 }}>UPnP</Tag>}
          {r.use_natmap && <Tag color="orange" style={{ fontSize: 11 }}>NATMAP</Tag>}
        </Space>
      ),
    },
    {
      title: t('common.action'), width: 180,
      render: (_: any, r: any) => (
        <Space size={4}>
          {r.status === 'running'
            ? <Tooltip title={t('common.stop')}><Button size="small" icon={<StopOutlined />} onClick={async () => { await stunApi.stop(r.id); fetchData() }} /></Tooltip>
            : <Tooltip title={t('common.start')}><Button size="small" type="primary" icon={<PlayCircleOutlined />} onClick={async () => { await stunApi.start(r.id); fetchData() }} /></Tooltip>
          }
          <Tooltip title="详情">
            <Button size="small" icon={<InfoCircleOutlined />} onClick={() => { setDetailRecord(r); setDetailOpen(true) }} />
          </Tooltip>
          <Tooltip title={t('common.edit')}>
            <Button size="small" icon={<EditOutlined />} onClick={() => handleEdit(r)} />
          </Tooltip>
          <Popconfirm title={t('common.deleteConfirm')} onConfirm={async () => { await stunApi.delete(r.id); fetchData() }}>
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
        <Typography.Title level={4} style={{ margin: 0 }}>{t('stun.title')}</Typography.Title>
        <Button type="primary" icon={<PlusOutlined />} onClick={handleCreate}>{t('common.create')}</Button>
      </div>

      <Table
        dataSource={data} columns={columns} rowKey="id" loading={loading}
        size="middle" style={{ background: '#fff', borderRadius: 8 }}
        pagination={{ pageSize: 20, showSizeChanger: true }}
      />

      {/* 编辑弹窗 */}
      <Modal
        title={editRecord ? t('common.edit') : t('common.create')}
        open={modalOpen} onOk={handleSubmit} onCancel={() => setModalOpen(false)}
        width={560} destroyOnClose
      >
        <Form form={form} layout="vertical" style={{ marginTop: 16 }}>
          <Row gutter={16}>
            <Col span={16}>
              <Form.Item name="name" label={t('common.name')} rules={[{ required: true }]}>
                <Input placeholder="规则名称" />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="enable" label={t('common.enable')} valuePropName="checked">
                <Switch />
              </Form.Item>
            </Col>
          </Row>

          <Form.Item name="stun_server" label={t('stun.stunServer')}>
            <Input placeholder="stun.l.google.com:19302" />
          </Form.Item>

          <Divider orientation="left" plain style={{ fontSize: 13 }}>转发目标</Divider>
          <Row gutter={16}>
            <Col span={16}>
              <Form.Item name="target_address" label={t('stun.targetAddress')}>
                <Input placeholder="转发目标IP/域名（可选）" />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="target_port" label={t('stun.targetPort')}>
                <InputNumber min={1} max={65535} style={{ width: '100%' }} placeholder="端口" />
              </Form.Item>
            </Col>
          </Row>

          <Divider orientation="left" plain style={{ fontSize: 13 }}>高级选项</Divider>
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item name="use_upnp" label={t('stun.useUpnp')} valuePropName="checked">
                <Switch />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="use_natmap" label={t('stun.useNatmap')} valuePropName="checked">
                <Switch />
              </Form.Item>
            </Col>
          </Row>

          <Form.Item name="callback_task_id" label={t('stun.callbackTask')}>
            <InputNumber min={0} style={{ width: '100%' }} placeholder="回调任务ID（可选）" />
          </Form.Item>

          <Form.Item name="remark" label={t('common.remark')}>
            <Input.TextArea rows={2} placeholder="备注（可选）" />
          </Form.Item>
        </Form>
      </Modal>

      {/* 详情弹窗 */}
      <Modal
        title="STUN 穿透详情"
        open={detailOpen} onCancel={() => setDetailOpen(false)} footer={null} width={480}
      >
        {detailRecord && (
          <div style={{ padding: '8px 0' }}>
            <Card size="small" style={{ marginBottom: 12, background: '#f6ffed', border: '1px solid #b7eb8f' }}>
              <Row gutter={16}>
                <Col span={12}>
                  <Text type="secondary">当前公网IP</Text>
                  <div><Text strong style={{ color: '#52c41a' }}>{detailRecord.current_ip || '-'}</Text></div>
                </Col>
                <Col span={12}>
                  <Text type="secondary">当前端口</Text>
                  <div><Text strong style={{ color: '#52c41a' }}>{detailRecord.current_port || '-'}</Text></div>
                </Col>
              </Row>
            </Card>
            <Row gutter={[16, 8]}>
              <Col span={12}><Text type="secondary">NAT 类型</Text><div><Tag color="blue">{detailRecord.nat_type || '未知'}</Tag></div></Col>
              <Col span={12}><Text type="secondary">STUN 服务器</Text><div><Text code style={{ fontSize: 12 }}>{detailRecord.stun_server}</Text></div></Col>
              <Col span={12}><Text type="secondary">UPnP</Text><div><Tag color={detailRecord.use_upnp ? 'purple' : 'default'}>{detailRecord.use_upnp ? '已启用' : '未启用'}</Tag></div></Col>
              <Col span={12}><Text type="secondary">NATMAP</Text><div><Tag color={detailRecord.use_natmap ? 'orange' : 'default'}>{detailRecord.use_natmap ? '已启用' : '未启用'}</Tag></div></Col>
              {detailRecord.last_error && (
                <Col span={24}><Text type="secondary">最后错误</Text><div><Text type="danger" style={{ fontSize: 12 }}>{detailRecord.last_error}</Text></div></Col>
              )}
            </Row>
          </div>
        )}
      </Modal>
    </div>
  )
}

export default Stun
