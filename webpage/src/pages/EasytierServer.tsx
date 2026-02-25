import React, { useEffect, useState } from 'react'
import {
  Table, Button, Space, Switch, Modal, Form, Input,
  Popconfirm, message, Typography, Tag, Tooltip, Row, Col, Divider,
} from 'antd'
import {
  PlusOutlined, EditOutlined, DeleteOutlined,
  PlayCircleOutlined, StopOutlined, InfoCircleOutlined,
} from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { easytierServerApi } from '../api'
import StatusTag from '../components/StatusTag'

const { Text } = Typography

// 默认监听端口配置（参考 EasyTier 默认端口）
const DEFAULT_LISTEN_PORTS = 'tcp:11010,udp:11010'

// 解析 listen_ports 字符串为可读标签
const parseListenPorts = (listenPorts: string): string[] => {
  if (!listenPorts) return []
  return listenPorts.split(',').map(p => p.trim()).filter(Boolean)
}

const EasytierServer: React.FC = () => {
  const { t } = useTranslation()
  const [data, setData] = useState<any[]>([])
  const [loading, setLoading] = useState(false)
  const [modalOpen, setModalOpen] = useState(false)
  const [editRecord, setEditRecord] = useState<any>(null)
  const [form] = Form.useForm()

  const fetchData = async () => {
    setLoading(true)
    try {
      const res: any = await easytierServerApi.list()
      setData(res.data || [])
    } finally { setLoading(false) }
  }

  useEffect(() => { fetchData() }, [])

  const handleCreate = () => {
    setEditRecord(null)
    form.resetFields()
    form.setFieldsValue({
      enable: true,
      listen_addr: '0.0.0.0',
      listen_ports: DEFAULT_LISTEN_PORTS,
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
      await easytierServerApi.update(editRecord.id, values)
    } else {
      await easytierServerApi.create(values)
    }
    message.success(t('common.success'))
    setModalOpen(false)
    fetchData()
  }

  const handleToggle = async (record: any, checked: boolean) => {
    await easytierServerApi.update(record.id, { ...record, enable: checked })
    checked ? await easytierServerApi.start(record.id) : await easytierServerApi.stop(record.id)
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
      title: '监听端口',
      render: (_: any, r: any) => {
        const ports = parseListenPorts(r.listen_ports)
        if (ports.length === 0) return <Text type="secondary">未配置</Text>
        return (
          <Space size={4} wrap>
            {ports.map((p, i) => (
              <Tag key={i} color="geekblue" style={{ fontSize: 11 }}>{p}</Tag>
            ))}
          </Space>
        )
      },
    },
    {
      title: t('easytier.networkName'), dataIndex: 'network_name',
      render: (v: string) => v
        ? <Tag color="blue">{v}</Tag>
        : <Tag color="default">公开服务器</Tag>,
    },
    {
      title: '密码', dataIndex: 'network_password',
      render: (v: string) => v ? <Text type="secondary">••••••</Text> : <Text type="secondary">无</Text>,
    },
    {
      title: t('common.action'), width: 160,
      render: (_: any, r: any) => (
        <Space size={4}>
          {r.status === 'running'
            ? <Tooltip title={t('common.stop')}><Button size="small" icon={<StopOutlined />} onClick={async () => { await easytierServerApi.stop(r.id); fetchData() }} /></Tooltip>
            : <Tooltip title={t('common.start')}><Button size="small" type="primary" icon={<PlayCircleOutlined />} onClick={async () => { await easytierServerApi.start(r.id); fetchData() }} /></Tooltip>
          }
          {r.last_error && (
            <Tooltip title={r.last_error}>
              <Button size="small" icon={<InfoCircleOutlined />} danger />
            </Tooltip>
          )}
          <Tooltip title={t('common.edit')}>
            <Button size="small" icon={<EditOutlined />} onClick={() => handleEdit(r)} />
          </Tooltip>
          <Popconfirm title={t('common.deleteConfirm')} onConfirm={async () => { await easytierServerApi.delete(r.id); fetchData() }}>
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
        <Typography.Title level={4} style={{ margin: 0 }}>{t('easytier.serverTitle')}</Typography.Title>
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
            <Col span={10}>
              <Form.Item name="listen_addr" label="监听地址">
                <Input placeholder="0.0.0.0" />
              </Form.Item>
            </Col>
            <Col span={14}>
              <Form.Item
                name="listen_ports"
                label="监听端口"
                rules={[{ required: true, message: '请填写监听端口' }]}
                extra={
                  <span style={{ fontSize: 11 }}>
                    多个用逗号分隔，如：<code>tcp:11010,udp:11010</code> 或 <code>12345</code>（基准端口）
                  </span>
                }
              >
                <Input placeholder="tcp:11010,udp:11010" />
              </Form.Item>
            </Col>
          </Row>

          <Divider orientation="left" plain style={{ fontSize: 13 }}>
            支持的协议及默认端口
          </Divider>
          <div style={{ fontSize: 12, color: '#888', marginBottom: 12, lineHeight: '22px' }}>
            <code>tcp:11010</code> · <code>udp:11010</code> · <code>ws:11011</code> · <code>wss:11012</code> · <code>wg:11011</code> · <code>quic:11012</code>
          </div>

          <Divider orientation="left" plain style={{ fontSize: 13 }}>网络配置（可选）</Divider>
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item
                name="network_name"
                label={t('easytier.networkName')}
                extra="留空为公开服务器"
              >
                <Input placeholder="留空为公开服务器" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="network_password" label={t('easytier.networkPassword')}>
                <Input.Password placeholder="网络密码（可选）" />
              </Form.Item>
            </Col>
          </Row>

          <Form.Item
            name="extra_args"
            label={t('easytier.extraArgs')}
            extra="额外的命令行参数"
          >
            <Input.TextArea rows={2} placeholder="额外命令行参数（可选）" />
          </Form.Item>

          <Form.Item name="remark" label={t('common.remark')}>
            <Input.TextArea rows={2} placeholder="备注（可选）" />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

export default EasytierServer
