import React, { useEffect, useState } from 'react'
import {
  Table, Button, Space, Switch, Modal, Form, Input, InputNumber,
  Popconfirm, message, Typography, Tabs, Select, Tag, Tooltip,
  Drawer, Row, Col, Divider, Badge,
} from 'antd'
import {
  PlusOutlined, EditOutlined, DeleteOutlined,
  PlayCircleOutlined, StopOutlined, ReloadOutlined, UnorderedListOutlined,
} from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { frpcApi } from '../api'
import StatusTag from '../components/StatusTag'
import request from '../api/request'

const { Option } = Select
const { Text } = Typography

// 代理类型颜色
const proxyTypeColor: Record<string, string> = {
  tcp: 'blue', udp: 'green', http: 'orange', https: 'gold',
  stcp: 'purple', xtcp: 'magenta',
}

const FrpClient: React.FC = () => {
  const { t } = useTranslation()
  const [data, setData] = useState<any[]>([])
  const [loading, setLoading] = useState(false)
  const [modalOpen, setModalOpen] = useState(false)
  const [editRecord, setEditRecord] = useState<any>(null)
  const [form] = Form.useForm()

  // 代理管理
  const [proxyDrawerOpen, setProxyDrawerOpen] = useState(false)
  const [currentFrpc, setCurrentFrpc] = useState<any>(null)
  const [proxies, setProxies] = useState<any[]>([])
  const [proxyModalOpen, setProxyModalOpen] = useState(false)
  const [editProxy, setEditProxy] = useState<any>(null)
  const [proxyForm] = Form.useForm()
  const [proxyType, setProxyType] = useState('tcp')

  const fetchData = async () => {
    setLoading(true)
    try {
      const res: any = await frpcApi.list()
      setData(res.data || [])
    } finally { setLoading(false) }
  }

  useEffect(() => { fetchData() }, [])

  const fetchProxies = async (frpcId: number) => {
    const res: any = await request.get(`/v1/frpc/${frpcId}/proxies`)
    setProxies(res.data || [])
  }

  const openProxyDrawer = (record: any) => {
    setCurrentFrpc(record)
    fetchProxies(record.id)
    setProxyDrawerOpen(true)
  }

  const handleCreate = () => {
    setEditRecord(null)
    form.resetFields()
    form.setFieldsValue({ enable: true, server_port: 7000, protocol: 'tcp', log_level: 'info' })
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
      await frpcApi.update(editRecord.id, values)
    } else {
      await frpcApi.create(values)
    }
    message.success(t('common.success'))
    setModalOpen(false)
    fetchData()
  }

  const handleToggle = async (record: any, checked: boolean) => {
    await frpcApi.update(record.id, { ...record, enable: checked })
    checked ? await frpcApi.start(record.id) : await frpcApi.stop(record.id)
    fetchData()
  }

  // 代理操作
  const handleCreateProxy = () => {
    setEditProxy(null)
    proxyForm.resetFields()
    proxyForm.setFieldsValue({ type: 'tcp', local_ip: '127.0.0.1', enable: true })
    setProxyType('tcp')
    setProxyModalOpen(true)
  }

  const handleEditProxy = (proxy: any) => {
    setEditProxy(proxy)
    proxyForm.setFieldsValue(proxy)
    setProxyType(proxy.type)
    setProxyModalOpen(true)
  }

  const handleSubmitProxy = async () => {
    const values = await proxyForm.validateFields()
    if (editProxy) {
      await request.put(`/v1/frpc/${currentFrpc.id}/proxies/${editProxy.id}`, values)
    } else {
      await request.post(`/v1/frpc/${currentFrpc.id}/proxies`, values)
    }
    message.success(t('common.success'))
    setProxyModalOpen(false)
    fetchProxies(currentFrpc.id)
  }

  const handleDeleteProxy = async (proxyId: number) => {
    await request.delete(`/v1/frpc/${currentFrpc.id}/proxies/${proxyId}`)
    fetchProxies(currentFrpc.id)
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
      title: t('frp.serverAddr'),
      render: (_: any, r: any) => (
        <Text code style={{ fontSize: 12 }}>{r.server_addr}:{r.server_port}</Text>
      ),
    },
    {
      title: 'Token', dataIndex: 'token',
      render: (v: string) => v ? <Text type="secondary">••••••</Text> : '-',
    },
    {
      title: 'TLS', dataIndex: 'tls_enable', width: 60,
      render: (v: boolean) => v ? <Tag color="green" style={{ fontSize: 11 }}>TLS</Tag> : '-',
    },
    {
      title: '代理', width: 80,
      render: (_: any, r: any) => (
        <Button
          size="small" type="link" icon={<UnorderedListOutlined />}
          onClick={() => openProxyDrawer(r)}
          style={{ padding: 0 }}
        >
          管理
        </Button>
      ),
    },
    {
      title: t('common.action'), width: 160,
      render: (_: any, r: any) => (
        <Space size={4}>
          {r.status === 'running'
            ? <Tooltip title={t('common.stop')}><Button size="small" icon={<StopOutlined />} onClick={async () => { await frpcApi.stop(r.id); fetchData() }} /></Tooltip>
            : <Tooltip title={t('common.start')}><Button size="small" type="primary" icon={<PlayCircleOutlined />} onClick={async () => { await frpcApi.start(r.id); fetchData() }} /></Tooltip>
          }
          <Tooltip title={t('common.restart')}>
            <Button size="small" icon={<ReloadOutlined />} onClick={async () => { await frpcApi.restart(r.id); fetchData() }} />
          </Tooltip>
          <Tooltip title={t('common.edit')}>
            <Button size="small" icon={<EditOutlined />} onClick={() => handleEdit(r)} />
          </Tooltip>
          <Popconfirm title={t('common.deleteConfirm')} onConfirm={async () => { await frpcApi.delete(r.id); fetchData() }}>
            <Tooltip title={t('common.delete')}>
              <Button size="small" danger icon={<DeleteOutlined />} />
            </Tooltip>
          </Popconfirm>
        </Space>
      ),
    },
  ]

  // 代理列表列
  const proxyColumns = [
    {
      title: '代理名称', dataIndex: 'name',
      render: (name: string, r: any) => (
        <Space>
          <Tag color={proxyTypeColor[r.type] || 'default'}>{r.type?.toUpperCase()}</Tag>
          <Text>{name}</Text>
        </Space>
      ),
    },
    {
      title: '本地地址',
      render: (_: any, r: any) => (
        <Text code style={{ fontSize: 12 }}>{r.local_ip}:{r.local_port}</Text>
      ),
    },
    {
      title: '远程端口/域名',
      render: (_: any, r: any) => {
        if (r.type === 'http' || r.type === 'https') {
          return <Text code style={{ fontSize: 12 }}>{r.custom_domains || r.subdomain || '-'}</Text>
        }
        return r.remote_port ? <Text code style={{ fontSize: 12 }}>:{r.remote_port}</Text> : '-'
      },
    },
    {
      title: t('common.enable'), dataIndex: 'enable', width: 70,
      render: (v: boolean) => <Badge status={v ? 'success' : 'default'} text={v ? '启用' : '禁用'} />,
    },
    {
      title: t('common.action'), width: 100,
      render: (_: any, r: any) => (
        <Space size={4}>
          <Button size="small" icon={<EditOutlined />} onClick={() => handleEditProxy(r)} />
          <Popconfirm title={t('common.deleteConfirm')} onConfirm={() => handleDeleteProxy(r.id)}>
            <Button size="small" danger icon={<DeleteOutlined />} />
          </Popconfirm>
        </Space>
      ),
    },
  ]

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Typography.Title level={4} style={{ margin: 0 }}>{t('frp.clientTitle')}</Typography.Title>
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
        width={600} destroyOnClose
      >
        <Form form={form} layout="vertical" style={{ marginTop: 16 }}>
          <Tabs items={[
            {
              key: 'basic', label: '基本配置',
              children: (
                <>
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
                    <Col span={16}>
                      <Form.Item name="server_addr" label={t('frp.serverAddr')} rules={[{ required: true }]}>
                        <Input placeholder="FRP 服务器地址" />
                      </Form.Item>
                    </Col>
                    <Col span={8}>
                      <Form.Item name="server_port" label={t('frp.serverPort')} rules={[{ required: true }]}>
                        <InputNumber min={1} max={65535} style={{ width: '100%' }} />
                      </Form.Item>
                    </Col>
                  </Row>
                  <Form.Item name="token" label={t('frp.token')}>
                    <Input.Password placeholder="认证 Token" />
                  </Form.Item>
                  <Row gutter={16}>
                    <Col span={12}>
                      <Form.Item name="tls_enable" label={t('frp.tlsEnable')} valuePropName="checked">
                        <Switch />
                      </Form.Item>
                    </Col>
                    <Col span={12}>
                      <Form.Item name="log_level" label="日志级别">
                        <Select>
                          <Option value="trace">trace</Option>
                          <Option value="debug">debug</Option>
                          <Option value="info">info</Option>
                          <Option value="warn">warn</Option>
                          <Option value="error">error</Option>
                        </Select>
                      </Form.Item>
                    </Col>
                  </Row>
                  <Form.Item name="remark" label={t('common.remark')}>
                    <Input.TextArea rows={2} placeholder="备注（可选）" />
                  </Form.Item>
                </>
              ),
            },
          ]} />
        </Form>
      </Modal>

      {/* 代理管理抽屉 */}
      <Drawer
        title={`代理管理 - ${currentFrpc?.name || ''}`}
        open={proxyDrawerOpen}
        onClose={() => setProxyDrawerOpen(false)}
        width={700}
        extra={
          <Button type="primary" icon={<PlusOutlined />} onClick={handleCreateProxy}>
            {t('frp.addProxy')}
          </Button>
        }
      >
        <Table
          dataSource={proxies} columns={proxyColumns} rowKey="id"
          size="small" pagination={false}
        />
      </Drawer>

      {/* 代理编辑弹窗 */}
      <Modal
        title={editProxy ? '编辑代理' : '添加代理'}
        open={proxyModalOpen} onOk={handleSubmitProxy}
        onCancel={() => setProxyModalOpen(false)} width={520} destroyOnClose
      >
        <Form form={proxyForm} layout="vertical" style={{ marginTop: 16 }}>
          <Row gutter={16}>
            <Col span={16}>
              <Form.Item name="name" label={t('frp.proxyName')} rules={[{ required: true }]}>
                <Input placeholder="代理名称（全局唯一）" />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="type" label={t('frp.proxyType')} rules={[{ required: true }]}>
                <Select onChange={(v) => setProxyType(v)}>
                  <Option value="tcp">TCP</Option>
                  <Option value="udp">UDP</Option>
                  <Option value="http">HTTP</Option>
                  <Option value="https">HTTPS</Option>
                  <Option value="stcp">STCP</Option>
                  <Option value="xtcp">XTCP</Option>
                </Select>
              </Form.Item>
            </Col>
          </Row>

          <Row gutter={16}>
            <Col span={16}>
              <Form.Item name="local_ip" label={t('frp.localIP')} rules={[{ required: true }]}>
                <Input placeholder="127.0.0.1" />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="local_port" label={t('frp.localPort')} rules={[{ required: true }]}>
                <InputNumber min={1} max={65535} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
          </Row>

          {(proxyType === 'tcp' || proxyType === 'udp') && (
            <Form.Item name="remote_port" label={t('frp.remotePort')} rules={[{ required: true }]}>
              <InputNumber min={1} max={65535} style={{ width: '100%' }} placeholder="服务端映射端口" />
            </Form.Item>
          )}

          {(proxyType === 'http' || proxyType === 'https') && (
            <>
              <Form.Item name="custom_domains" label={t('frp.customDomains')}>
                <Input placeholder="example.com（多个用逗号分隔）" />
              </Form.Item>
              <Form.Item name="subdomain" label={t('frp.subdomain')}>
                <Input placeholder="子域名（需服务端配置 subdomain_host）" />
              </Form.Item>
            </>
          )}

          <Divider plain style={{ fontSize: 13 }}>高级选项</Divider>
          <Row gutter={16}>
            <Col span={8}>
              <Form.Item name="enable" label={t('common.enable')} valuePropName="checked">
                <Switch />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="use_encryption" label="加密" valuePropName="checked">
                <Switch />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="use_compression" label="压缩" valuePropName="checked">
                <Switch />
              </Form.Item>
            </Col>
          </Row>
        </Form>
      </Modal>
    </div>
  )
}

export default FrpClient
