import React, { useEffect, useState } from 'react'
import {
  Table, Button, Space, Switch, Modal, Form, Input, InputNumber,
  Popconfirm, message, Typography, Tooltip, Row, Col, Select, Tabs, Divider,
} from 'antd'
import {
  PlusOutlined, EditOutlined, DeleteOutlined,
  PlayCircleOutlined, StopOutlined, SettingOutlined, LinkOutlined,
  SafetyOutlined, ThunderboltOutlined,
} from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { npsClientApi } from '../api'
import StatusTag from '../components/StatusTag'

const { Text } = Typography
const { Option } = Select

// 分组标题
const SectionTitle = ({ children }: { children: React.ReactNode }) => (
  <div style={{ display: 'flex', alignItems: 'center', gap: 8, margin: '12px 0 8px' }}>
    <div style={{ width: 3, height: 14, background: '#1677ff', borderRadius: 2, flexShrink: 0 }} />
    <span style={{ fontSize: 12, fontWeight: 600, color: '#595959', letterSpacing: '0.02em' }}>{children}</span>
    <div style={{ flex: 1, height: 1, background: '#f0f0f0' }} />
  </div>
)

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
      auto_reconnection: true,
      dns_server: '8.8.8.8',
      crypt: false,
      compress: false,
      disconnect_timeout: 60,
      max_conn: 1000,
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
      title: '服务器地址',
      render: (_: any, r: any) => (
        <Text code style={{ fontSize: 12 }}>{r.server_addr}:{r.server_port || 8024}</Text>
      ),
    },
    {
      title: '协议',
      dataIndex: 'conn_type',
      width: 80,
      render: (v: string) => <Text code style={{ fontSize: 12 }}>{(v || 'tcp').toUpperCase()}</Text>,
    },
    {
      title: '加密/压缩',
      width: 100,
      render: (_: any, r: any) => (
        <Space size={4}>
          {r.crypt && <Text type="success" style={{ fontSize: 11 }}>加密</Text>}
          {r.compress && <Text type="warning" style={{ fontSize: 11 }}>压缩</Text>}
          {!r.crypt && !r.compress && <Text type="secondary" style={{ fontSize: 11 }}>-</Text>}
        </Space>
      ),
    },
    {
      title: t('nps.vkeyOrId'), dataIndex: 'vkey_or_id',
      render: (v: string) => v ? <Text code style={{ fontSize: 12 }}>{v}</Text> : '-',
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

  // ===== Tab 1: 基本配置 =====
  const tabBasic = (
    <>
      <Row gutter={16}>
        <Col span={18}>
          <Form.Item name="name" label="名称" rules={[{ required: true, message: '请填写名称' }]}>
            <Input placeholder="客户端名称" />
          </Form.Item>
        </Col>
        <Col span={6}>
          <Form.Item name="enable" label="启用" valuePropName="checked">
            <Switch />
          </Form.Item>
        </Col>
      </Row>
      <Form.Item name="remark" label="备注">
        <Input.TextArea rows={2} placeholder="备注（可选）" />
      </Form.Item>
      <SectionTitle>日志与调试</SectionTitle>
      <Row gutter={16}>
        <Col span={12}>
          <Form.Item name="log_level" label="日志级别">
            <Select>
              <Option value="debug">Debug</Option>
              <Option value="info">Info</Option>
              <Option value="warn">Warn</Option>
              <Option value="error">Error</Option>
            </Select>
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item name="pprof_addr" label="pprof 地址" extra={<span style={{ fontSize: 11 }}>性能分析监听地址，如 0.0.0.0:9999</span>}>
            <Input placeholder="留空不启用" />
          </Form.Item>
        </Col>
      </Row>
    </>
  )

  // ===== Tab 2: 连接设置 =====
  const tabConnection = (
    <>
      <SectionTitle>服务器地址</SectionTitle>
      <Row gutter={16}>
        <Col span={14}>
          <Form.Item
            name="server_addr"
            label="服务器地址"
            rules={[{ required: true, message: '请填写服务器地址' }]}
            extra={<span style={{ fontSize: 11 }}>NPS 服务端的 IP 或域名</span>}
          >
            <Input placeholder="192.168.1.1 或 nps.example.com" />
          </Form.Item>
        </Col>
        <Col span={5}>
          <Form.Item
            name="server_port"
            label="端口"
            rules={[{ required: true, message: '请填写端口' }]}
            extra={<span style={{ fontSize: 11 }}>默认 8024</span>}
          >
            <InputNumber min={1} max={65535} style={{ width: '100%' }} placeholder="8024" />
          </Form.Item>
        </Col>
        <Col span={5}>
          <Form.Item name="conn_type" label="协议">
            <Select>
              <Option value="tcp">TCP</Option>
              <Option value="tls">TLS</Option>
              <Option value="kcp">KCP</Option>
              <Option value="quic">QUIC</Option>
              <Option value="ws">WS</Option>
              <Option value="wss">WSS</Option>
            </Select>
          </Form.Item>
        </Col>
      </Row>

      <SectionTitle>连接行为</SectionTitle>
      <Row gutter={16}>
        <Col span={8}>
          <Form.Item name="auto_reconnection" label="自动重连" valuePropName="checked">
            <Switch />
          </Form.Item>
        </Col>
        <Col span={8}>
          <Form.Item
            name="disconnect_timeout"
            label="断线超时(秒)"
            extra={<span style={{ fontSize: 11 }}>默认 60 秒</span>}
          >
            <InputNumber min={1} style={{ width: '100%' }} placeholder="60" />
          </Form.Item>
        </Col>
        <Col span={8}>
          <Form.Item
            name="max_conn"
            label="最大连接数"
            extra={<span style={{ fontSize: 11 }}>默认 1000</span>}
          >
            <InputNumber min={1} style={{ width: '100%' }} placeholder="1000" />
          </Form.Item>
        </Col>
      </Row>

      <Row gutter={16}>
        <Col span={12}>
          <Form.Item
            name="local_ip"
            label="本地绑定 IP"
            extra={<span style={{ fontSize: 11 }}>指定出口网卡 IP，留空自动选择</span>}
          >
            <Input placeholder="如 192.168.1.100" />
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item
            name="dns_server"
            label="DNS 服务器"
            extra={<span style={{ fontSize: 11 }}>自定义 DNS，默认 8.8.8.8</span>}
          >
            <Input placeholder="8.8.8.8" />
          </Form.Item>
        </Col>
      </Row>

      <Row gutter={16}>
        <Col span={12}>
          <Form.Item
            name="ntp_server"
            label="NTP 服务器"
            extra={<span style={{ fontSize: 11 }}>时间同步服务器，如 pool.ntp.org</span>}
          >
            <Input placeholder="pool.ntp.org" />
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item
            name="ntp_interval"
            label="NTP 同步间隔(分钟)"
            extra={<span style={{ fontSize: 11 }}>默认 5 分钟</span>}
          >
            <InputNumber min={1} style={{ width: '100%' }} placeholder="5" />
          </Form.Item>
        </Col>
      </Row>
    </>
  )

  // ===== Tab 3: 认证与安全 =====
  const tabAuth = (
    <>
      <SectionTitle>客户端认证</SectionTitle>
      <Row gutter={16}>
        <Col span={12}>
          <Form.Item
            name="vkey_or_id"
            label="客户端 vkey / ID"
            extra={<span style={{ fontSize: 11 }}>在 NPS 管理面板中获取，唯一标识本客户端</span>}
          >
            <Input placeholder="客户端唯一标识" />
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item
            name="auth_key"
            label="全局认证密钥"
            extra={<span style={{ fontSize: 11 }}>需与服务端 auth_key 一致，留空不启用</span>}
          >
            <Input.Password placeholder="留空不启用" />
          </Form.Item>
        </Col>
      </Row>

      <SectionTitle>传输安全</SectionTitle>
      <Row gutter={16}>
        <Col span={8}>
          <Form.Item name="crypt" label="加密传输" valuePropName="checked"
            extra={<span style={{ fontSize: 11 }}>对传输数据加密</span>}
          >
            <Switch />
          </Form.Item>
        </Col>
        <Col span={8}>
          <Form.Item name="compress" label="压缩传输" valuePropName="checked"
            extra={<span style={{ fontSize: 11 }}>对传输数据压缩</span>}
          >
            <Switch />
          </Form.Item>
        </Col>
      </Row>

      <SectionTitle>Web 管理认证</SectionTitle>
      <Row gutter={16}>
        <Col span={12}>
          <Form.Item name="web_username" label="Web 用户名"
            extra={<span style={{ fontSize: 11 }}>NPC 本地 Web 管理用户名</span>}
          >
            <Input placeholder="user" />
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item name="web_password" label="Web 密码">
            <Input.Password placeholder="留空不启用" />
          </Form.Item>
        </Col>
      </Row>

      <SectionTitle>Basic 认证（代理用）</SectionTitle>
      <Row gutter={16}>
        <Col span={12}>
          <Form.Item name="basic_username" label="Basic 用户名"
            extra={<span style={{ fontSize: 11 }}>HTTP 代理 Basic 认证用户名</span>}
          >
            <Input placeholder="留空不启用" />
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item name="basic_password" label="Basic 密码">
            <Input.Password placeholder="留空不启用" />
          </Form.Item>
        </Col>
      </Row>
    </>
  )

  // ===== Tab 4: 限流设置 =====
  const tabLimit = (
    <>
      <SectionTitle>流量与速率限制</SectionTitle>
      <Row gutter={16}>
        <Col span={12}>
          <Form.Item
            name="flow_limit"
            label="流量限制 (KB)"
            extra={<span style={{ fontSize: 11 }}>总流量上限，单位 KB，0 表示不限制</span>}
          >
            <InputNumber min={0} style={{ width: '100%' }} placeholder="0（不限制）" />
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item
            name="rate_limit"
            label="速率限制 (KB/s)"
            extra={<span style={{ fontSize: 11 }}>带宽速率上限，单位 KB/s，0 表示不限制</span>}
          >
            <InputNumber min={0} style={{ width: '100%' }} placeholder="0（不限制）" />
          </Form.Item>
        </Col>
      </Row>
    </>
  )

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
        width={600} destroyOnHidden
        styles={{ body: { padding: '4px 24px 0' } }}
      >
        <Form form={form} layout="vertical" style={{ paddingTop: 4 }}>
          <Tabs
            size="small"
            items={[
              { key: 'basic',      label: <span><SettingOutlined /> 基本配置</span>,   children: tabBasic },
              { key: 'connection', label: <span><LinkOutlined />    连接设置</span>,   children: tabConnection },
              { key: 'auth',       label: <span><SafetyOutlined />  认证与安全</span>, children: tabAuth },
              { key: 'limit',      label: <span><ThunderboltOutlined /> 限流设置</span>, children: tabLimit },
            ]}
          />
        </Form>
      </Modal>
    </div>
  )
}

export default NpsClient
