import React, { useEffect, useState } from 'react'
import {
  Table, Button, Space, Switch, Modal, Form, Input,
  Popconfirm, message, Typography, Tag, Tooltip, Row, Col,
  Divider, Alert,
} from 'antd'
import {
  PlusOutlined, EditOutlined, DeleteOutlined,
  PlayCircleOutlined, StopOutlined, InfoCircleOutlined,
} from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { easytierClientApi } from '../api'
import StatusTag from '../components/StatusTag'

const { Text } = Typography

// 解析 listen_ports 字符串为可读标签
const parseListenPorts = (listenPorts: string): string[] => {
  if (!listenPorts) return []
  return listenPorts.split(',').map(p => p.trim()).filter(Boolean)
}

const EasytierClient: React.FC = () => {
  const { t } = useTranslation()
  const [data, setData] = useState<any[]>([])
  const [loading, setLoading] = useState(false)
  const [modalOpen, setModalOpen] = useState(false)
  const [editRecord, setEditRecord] = useState<any>(null)
  const [form] = Form.useForm()

  const fetchData = async () => {
    setLoading(true)
    try {
      const res: any = await easytierClientApi.list()
      setData(res.data || [])
    } finally { setLoading(false) }
  }

  useEffect(() => { fetchData() }, [])

  const handleCreate = () => {
    setEditRecord(null)
    form.resetFields()
    form.setFieldsValue({
      enable: true,
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
      await easytierClientApi.update(editRecord.id, values)
    } else {
      await easytierClientApi.create(values)
    }
    message.success(t('common.success'))
    setModalOpen(false)
    fetchData()
  }

  const handleToggle = async (record: any, checked: boolean) => {
    await easytierClientApi.update(record.id, { ...record, enable: checked })
    checked ? await easytierClientApi.start(record.id) : await easytierClientApi.stop(record.id)
    fetchData()
  }

  // 检查是否有运行中的实例（用于判断二进制是否存在）
  const hasError = data.some(d => d.status === 'error' && d.last_error?.includes('not found'))

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
      title: t('easytier.serverAddr'), dataIndex: 'server_addr',
      render: (v: string) => (
        <Text code style={{ fontSize: 12 }}>{v}</Text>
      ),
    },
    {
      title: t('easytier.networkName'), dataIndex: 'network_name',
      render: (v: string) => <Tag color="blue">{v}</Tag>,
    },
    {
      title: t('easytier.virtualIP'), dataIndex: 'virtual_ip',
      render: (v: string) => v
        ? <Text code style={{ color: '#52c41a', fontSize: 12 }}>{v}</Text>
        : <Text type="secondary">自动分配</Text>,
    },
    {
      title: '本地监听',
      render: (_: any, r: any) => {
        const ports = parseListenPorts(r.listen_ports)
        if (ports.length === 0) return <Text type="secondary">-</Text>
        return (
          <Space size={4} wrap>
            {ports.map((p, i) => (
              <Tag key={i} color="cyan" style={{ fontSize: 11 }}>{p}</Tag>
            ))}
          </Space>
        )
      },
    },
    {
      title: t('common.action'), width: 160,
      render: (_: any, r: any) => (
        <Space size={4}>
          {r.status === 'running'
            ? <Tooltip title={t('common.stop')}><Button size="small" icon={<StopOutlined />} onClick={async () => { await easytierClientApi.stop(r.id); fetchData() }} /></Tooltip>
            : <Tooltip title={t('common.start')}><Button size="small" type="primary" icon={<PlayCircleOutlined />} onClick={async () => { await easytierClientApi.start(r.id); fetchData() }} /></Tooltip>
          }
          {r.last_error && (
            <Tooltip title={r.last_error}>
              <Button size="small" icon={<InfoCircleOutlined />} danger />
            </Tooltip>
          )}
          <Tooltip title={t('common.edit')}>
            <Button size="small" icon={<EditOutlined />} onClick={() => handleEdit(r)} />
          </Tooltip>
          <Popconfirm title={t('common.deleteConfirm')} onConfirm={async () => { await easytierClientApi.delete(r.id); fetchData() }}>
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
      {hasError && (
        <Alert
          message={t('easytier.binaryNotFound')}
          description="请前往 GitHub Releases 下载对应平台的 easytier-core 二进制文件，放置到程序目录的 bin/ 文件夹下。"
          type="warning"
          showIcon
          closable
          style={{ marginBottom: 16 }}
        />
      )}

      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Typography.Title level={4} style={{ margin: 0 }}>{t('easytier.clientTitle')}</Typography.Title>
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
        width={600} destroyOnClose
      >
        <Form form={form} layout="vertical" style={{ marginTop: 16 }}>
          <Row gutter={16}>
            <Col span={16}>
              <Form.Item name="name" label={t('common.name')} rules={[{ required: true }]}>
                <Input placeholder="节点名称" />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="enable" label={t('common.enable')} valuePropName="checked">
                <Switch />
              </Form.Item>
            </Col>
          </Row>

          <Form.Item
            name="server_addr"
            label={t('easytier.serverAddr')}
            rules={[{ required: true }]}
            extra={
              <span style={{ fontSize: 11 }}>
                支持多个地址，逗号分隔。格式：<code>协议://IP:端口</code>，如 <code>tcp://1.2.3.4:11010</code>，<code>udp://1.2.3.4:11010</code>
              </span>
            }
          >
            <Input placeholder="tcp://server:11010" />
          </Form.Item>

          <Row gutter={16}>
            <Col span={12}>
              <Form.Item name="network_name" label={t('easytier.networkName')} rules={[{ required: true }]}>
                <Input placeholder="网络名称" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="network_password" label={t('easytier.networkPassword')}>
                <Input.Password placeholder="网络密码（可选）" />
              </Form.Item>
            </Col>
          </Row>

          <Form.Item
            name="virtual_ip"
            label={t('easytier.virtualIP')}
            extra="留空自动分配，指定格式如：10.144.144.1/24"
          >
            <Input placeholder="留空自动分配" />
          </Form.Item>

          <Divider orientation="left" plain style={{ fontSize: 13 }}>本地监听端口（可选）</Divider>
          <Form.Item
            name="listen_ports"
            label="监听端口"
            extra={
              <span style={{ fontSize: 11 }}>
                多个用逗号分隔，如：<code>tcp:11010,udp:11010</code> 或 <code>12345</code>（基准端口）。
                支持协议：<code>tcp</code> · <code>udp</code> · <code>ws</code> · <code>wss</code> · <code>wg</code> · <code>quic</code>
              </span>
            }
          >
            <Input placeholder="tcp:11010,udp:11010（留空不监听）" />
          </Form.Item>

          <Form.Item
            name="extra_args"
            label={t('easytier.extraArgs')}
            extra="额外的命令行参数，如：--no-tun --relay-network-whitelist '*'"
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

export default EasytierClient
