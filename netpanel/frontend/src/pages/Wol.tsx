import React, { useEffect, useState } from 'react'
import {
  Table, Button, Space, Modal, Form, Input, InputNumber,
  Popconfirm, message, Typography, Tooltip, Tag, Card, Row, Col,
} from 'antd'
import {
  PlusOutlined, EditOutlined, DeleteOutlined, ThunderboltOutlined,
} from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { wolApi } from '../api'

const { Text } = Typography

const Wol: React.FC = () => {
  const { t } = useTranslation()
  const [data, setData] = useState<any[]>([])
  const [loading, setLoading] = useState(false)
  const [modalOpen, setModalOpen] = useState(false)
  const [editRecord, setEditRecord] = useState<any>(null)
  const [wakingIds, setWakingIds] = useState<Set<number>>(new Set())
  const [form] = Form.useForm()

  const fetchData = async () => {
    setLoading(true)
    try { const res: any = await wolApi.list(); setData(res.data || []) }
    finally { setLoading(false) }
  }
  useEffect(() => { fetchData() }, [])

  const handleOpen = (record?: any) => {
    if (record) {
      setEditRecord(record)
      form.setFieldsValue(record)
    } else {
      setEditRecord(null)
      form.resetFields()
      form.setFieldsValue({ broadcast_ip: '255.255.255.255', port: 9 })
    }
    setModalOpen(true)
  }

  const handleSubmit = async () => {
    const values = await form.validateFields()
    if (editRecord) {
      await wolApi.update(editRecord.id, values)
    } else {
      await wolApi.create(values)
    }
    message.success(t('common.success'))
    setModalOpen(false)
    fetchData()
  }

  const handleWake = async (id: number) => {
    setWakingIds(prev => new Set(prev).add(id))
    try {
      await wolApi.wake(id)
      message.success(t('wol.wakeSuccess'))
    } finally {
      setWakingIds(prev => { const s = new Set(prev); s.delete(id); return s })
    }
  }

  const columns = [
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
      title: t('wol.macAddress'), dataIndex: 'mac_address',
      render: (v: string) => <Text code style={{ fontSize: 13 }}>{v}</Text>,
    },
    {
      title: '广播地址',
      render: (_: any, r: any) => (
        <Text type="secondary">{r.broadcast_ip || '255.255.255.255'}:{r.port || 9}</Text>
      ),
    },
    {
      title: t('wol.netInterface'), dataIndex: 'net_interface',
      render: (v: string) => v ? <Tag>{v}</Tag> : <Text type="secondary">默认</Text>,
    },
    {
      title: t('common.action'), width: 160,
      render: (_: any, r: any) => (
        <Space size={4}>
          <Button
            size="small" type="primary" icon={<ThunderboltOutlined />}
            loading={wakingIds.has(r.id)}
            onClick={() => handleWake(r.id)}
          >
            {t('wol.wake')}
          </Button>
          <Tooltip title={t('common.edit')}>
            <Button size="small" icon={<EditOutlined />} onClick={() => handleOpen(r)} />
          </Tooltip>
          <Popconfirm title={t('common.deleteConfirm')} onConfirm={async () => { await wolApi.delete(r.id); fetchData() }}>
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
        <Typography.Title level={4} style={{ margin: 0 }}>{t('wol.title')}</Typography.Title>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => handleOpen()}>
          {t('common.create')}
        </Button>
      </div>

      {/* 说明卡片 */}
      <Card size="small" style={{ marginBottom: 16, background: '#f6ffed', border: '1px solid #b7eb8f' }}>
        <Text type="secondary" style={{ fontSize: 13 }}>
          💡 网络唤醒（WOL）通过发送 Magic Packet 唤醒局域网内的设备。
          目标设备需在 BIOS 中启用 WOL 功能，且与本机在同一网段或可路由。
        </Text>
      </Card>

      <Table
        dataSource={data} columns={columns} rowKey="id" loading={loading}
        size="middle" style={{ background: '#fff', borderRadius: 8 }}
        pagination={{ pageSize: 20, showSizeChanger: true }}
      />

      <Modal
        title={editRecord ? t('common.edit') : t('common.create')}
        open={modalOpen} onOk={handleSubmit} onCancel={() => setModalOpen(false)}
        width={480} destroyOnClose
      >
        <Form form={form} layout="vertical" style={{ marginTop: 16 }}>
          <Form.Item name="name" label={t('common.name')} rules={[{ required: true }]}>
            <Input placeholder="设备名称，如：家里的台式机" />
          </Form.Item>

          <Form.Item
            name="mac_address"
            label={t('wol.macAddress')}
            rules={[{
              required: true,
              pattern: /^([0-9a-fA-F]{2}[:\-]){5}[0-9a-fA-F]{2}$/,
              message: '请输入有效的MAC地址，如：AA:BB:CC:DD:EE:FF',
            }]}
          >
            <Input placeholder="AA:BB:CC:DD:EE:FF" style={{ fontFamily: 'monospace' }} />
          </Form.Item>

          <Row gutter={16}>
            <Col span={16}>
              <Form.Item name="broadcast_ip" label={t('wol.broadcastIP')}
                extra="局域网广播：255.255.255.255，定向广播：192.168.1.255">
                <Input placeholder="255.255.255.255" />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="port" label="端口">
                <InputNumber min={1} max={65535} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
          </Row>

          <Form.Item name="net_interface" label={t('wol.netInterface')}
            extra="留空使用系统默认网卡">
            <Input placeholder="如：eth0、ens33（可选）" />
          </Form.Item>

          <Form.Item name="remark" label={t('common.remark')}>
            <Input.TextArea rows={2} placeholder="备注（可选）" />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

export default Wol
