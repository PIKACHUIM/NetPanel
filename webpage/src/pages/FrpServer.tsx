import React, { useEffect, useState } from 'react'
import {
  Table, Button, Space, Switch, Modal, Form, Input, InputNumber,
  Popconfirm, message, Typography, Tag, Tooltip, Row, Col, Tabs, Select,
} from 'antd'
import {
  PlusOutlined, EditOutlined, DeleteOutlined,
  PlayCircleOutlined, StopOutlined, LinkOutlined,
  SettingOutlined, ApiOutlined,
  GlobalOutlined, SafetyOutlined, FileTextOutlined,
} from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { frpsApi } from '../api'
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
      log_max_days: 3,
      max_ports_per_client: 0,
      transport_max_pool_count: 5,
      transport_tls_force: false,
      udp_packet_size: 1500,
      detailed_errors_to_client: true,
      user_conn_timeout: 10,
      nathole_analysis_data_reserve_hours: 168,
      vhost_http_timeout: 60,
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
      title: '监听',
      render: (_: any, r: any) => (
        <Space size={4} direction="vertical" style={{ gap: 2 }}>
          <Text code style={{ fontSize: 12 }}>{r.bind_addr || '0.0.0.0'}:{r.bind_port || 7000}</Text>
      {r.kcp_bind_port > 0 && <Tag color="cyan" style={{ fontSize: 11 }}>KCP:{r.kcp_bind_port}</Tag>}
          {r.quic_bind_port > 0 && <Tag color="purple" style={{ fontSize: 11 }}>QUIC:{r.quic_bind_port}</Tag>}
        </Space>
      ),
    },
    {
      title: 'Token', dataIndex: 'token',
      render: (v: string) => v ? <Text type="secondary">••••••</Text> : <Tag>无认证</Tag>,
    },
    {
      title: '子域名', dataIndex: 'sub_domain_host',
      render: (v: string) => v ? <Text code style={{ fontSize: 12 }}>{v}</Text> : <Text type="secondary">-</Text>,
    },
    {
      title: 'Dashboard', width: 180,
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
      ) : <Text type="secondary">未启用</Text>,
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

  // ===== Tab 1: 基本配置 =====
  const tabBasic = (
    <>
      <Row gutter={16}>
        <Col span={18}>
          <Form.Item name="name" label="名称" rules={[{ required: true, message: '请填写名称' }]}>
            <Input placeholder="服务端名称" style={{ width: '100%' }} />
          </Form.Item>
        </Col>
        <Col span={6}>
          <Form.Item name="enable" label="启用" valuePropName="checked">
            <Switch />
          </Form.Item>
        </Col>
      </Row>

      <SectionTitle>监听配置</SectionTitle>
      <Row gutter={16}>
        <Col span={12}>
          <Form.Item
            name="bind_addr"
            label="监听地址"
            extra={<span style={{ fontSize: 11 }}>填 <code>0.0.0.0</code> 监听所有网卡</span>}
          >
            <Input placeholder="0.0.0.0" style={{ width: '100%' }} />
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item name="bind_port" label="TCP 监听端口" rules={[{ required: true, message: '请填写端口' }]}>
            <InputNumber min={1} max={65535} placeholder="7000" style={{ width: '100%' }} />
          </Form.Item>
        </Col>
      </Row>
      <Row gutter={16}>
        <Col span={24}>
          <Form.Item
            name="proxy_bind_addr"
            label="代理监听地址"
            extra={<span style={{ fontSize: 11 }}>代理监听在不同网卡地址，默认同监听地址</span>}
          >
            <Input placeholder="留空同监听地址" style={{ width: '100%' }} />
          </Form.Item>
        </Col>
      </Row>

      <SectionTitle>认证</SectionTitle>
      <Form.Item
        name="token"
        label="Token"
        extra={<span style={{ fontSize: 11 }}>客户端连接时需携带相同 Token，留空不启用认证</span>}
      >
        <Input.Password placeholder="留空不启用认证" style={{ width: '100%' }} />
      </Form.Item>

      <SectionTitle>限制</SectionTitle>
      <Row gutter={16}>
        <Col span={8}>
          <Form.Item
            name="max_ports_per_client"
            label="每客户端最大端口数"
            extra={<span style={{ fontSize: 11 }}>0 表示不限制</span>}
          >
            <InputNumber min={0} placeholder="0" style={{ width: '100%' }} />
          </Form.Item>
        </Col>
        <Col span={8}>
          <Form.Item
            name="udp_packet_size"
            label="UDP 包大小 (字节)"
            extra={<span style={{ fontSize: 11 }}>需与客户端一致，默认 1500</span>}
          >
            <InputNumber min={576} max={65535} placeholder="1500" style={{ width: '100%' }} />
          </Form.Item>
        </Col>
        <Col span={8}>
          <Form.Item
            name="user_conn_timeout"
            label="用户连接超时 (秒)"
            extra={<span style={{ fontSize: 11 }}>等待客户端响应超时，默认 10</span>}
          >
            <InputNumber min={1} placeholder="10" style={{ width: '100%' }} />
          </Form.Item>
        </Col>
      </Row>

      <Form.Item name="remark" label="备注">
        <Input.TextArea rows={2} placeholder="备注（可选）" style={{ width: '100%' }} />
      </Form.Item>
    </>
  )

  // ===== Tab 2: 传输与协议 =====
  const tabTransport = (
    <>
      <SectionTitle>高级协议</SectionTitle>
      <Row gutter={16}>
        <Col span={12}>
          <Form.Item
            name="kcp_bind_port"
            label="KCP 监听端口"
            extra={<span style={{ fontSize: 11 }}>UDP 端口，留空禁用 KCP。可与 TCP 端口相同</span>}
          >
            <InputNumber min={1} max={65535} placeholder="留空禁用" style={{ width: '100%' }} />
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item
            name="quic_bind_port"
            label="QUIC 监听端口"
            extra={<span style={{ fontSize: 11 }}>UDP 端口，留空禁用 QUIC</span>}
          >
            <InputNumber min={1} max={65535} placeholder="留空禁用" style={{ width: '100%' }} />
          </Form.Item>
        </Col>
      </Row>
      <Row gutter={16}>
        <Col span={12}>
          <Form.Item
            name="tcpmux_http_connect_port"
            label="TCP 多路复用端口"
            extra={<span style={{ fontSize: 11 }}>HTTP CONNECT 请求监听端口，0 表示禁用</span>}
          >
            <InputNumber min={0} max={65535} placeholder="0（禁用）" style={{ width: '100%' }} />
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item
            name="tcpmux_passthrough"
            label="透传 CONNECT 请求"
            valuePropName="checked"
            extra={<span style={{ fontSize: 11 }}>tcpmux 类型代理是否透传 CONNECT 请求</span>}
          >
            <Switch />
          </Form.Item>
        </Col>
      </Row>

      <SectionTitle>连接池与复用</SectionTitle>
      <Row gutter={16}>
        <Col span={8}>
          <Form.Item
            name="transport_max_pool_count"
            label="最大连接池数量"
            extra={<span style={{ fontSize: 11 }}>每个代理保持的预建连接数，默认 5</span>}
          >
            <InputNumber min={0} placeholder="5" style={{ width: '100%' }} />
          </Form.Item>
        </Col>
        <Col span={8}>
          <Form.Item
            name="transport_heartbeat_timeout"
            label="心跳超时 (秒)"
            extra={<span style={{ fontSize: 11 }}>默认 90，设为负数禁用</span>}
          >
            <InputNumber placeholder="90" style={{ width: '100%' }} />
          </Form.Item>
        </Col>
        <Col span={8}>
          <Form.Item
            name="transport_tcp_mux_keepalive"
            label="TCP Mux 心跳 (秒)"
            extra={<span style={{ fontSize: 11 }}>TCP 多路复用心跳间隔，0 使用默认</span>}
          >
            <InputNumber min={0} placeholder="0（默认）" style={{ width: '100%' }} />
          </Form.Item>
        </Col>
      </Row>
      <Row gutter={16}>
        <Col span={12}>
          <Form.Item
            name="transport_tcp_keepalive"
            label="TCP Keepalive (秒)"
            extra={<span style={{ fontSize: 11 }}>底层 TCP 连接保活间隔，负数禁用，0 使用默认</span>}
          >
            <InputNumber placeholder="0（默认）" style={{ width: '100%' }} />
          </Form.Item>
        </Col>
      </Row>

      <SectionTitle>TLS</SectionTitle>
      <Row gutter={16}>
        <Col span={12}>
          <Form.Item
            name="transport_tls_cert_file"
            label="TLS 证书文件"
            extra={<span style={{ fontSize: 11 }}>PEM 格式证书路径</span>}
          >
            <Input placeholder="server.crt" style={{ width: '100%' }} />
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item
            name="transport_tls_key_file"
            label="TLS 私钥文件"
          >
            <Input placeholder="server.key" style={{ width: '100%' }} />
          </Form.Item>
        </Col>
      </Row>
      <Row gutter={16}>
        <Col span={12}>
          <Form.Item
            name="transport_tls_trusted_ca_file"
            label="受信任 CA 文件"
            extra={<span style={{ fontSize: 11 }}>用于验证客户端证书（双向 TLS）</span>}
          >
            <Input placeholder="ca.crt（可选）" style={{ width: '100%' }} />
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item
              name="transport_tls_force"
              label="强制 TLS"
              valuePropName="checked"
              extra={<span style={{ fontSize: 11 }}>仅接受 TLS 加密连接</span>}
          >
            <Switch />
          </Form.Item>
        </Col>
      </Row>
    </>
  )

  // ===== Tab 3: HTTP/HTTPS + Dashboard =====
  const tabHttp = (
    <>
      <SectionTitle>虚拟主机</SectionTitle>
      <Row gutter={16}>
        <Col span={8}>
          <Form.Item
            name="vhost_http_port"
            label="HTTP 代理端口"
            extra={<span style={{ fontSize: 11 }}>HTTP 域名代理监听端口</span>}
          >
            <InputNumber min={1} max={65535} placeholder="80（留空禁用）" style={{ width: '100%' }} />
          </Form.Item>
        </Col>
        <Col span={8}>
          <Form.Item
            name="vhost_https_port"
            label="HTTPS 代理端口"
            extra={<span style={{ fontSize: 11 }}>HTTPS 域名代理监听端口</span>}
          >
            <InputNumber min={1} max={65535} placeholder="443（留空禁用）" style={{ width: '100%' }} />
          </Form.Item>
        </Col>
        <Col span={8}>
          <Form.Item
            name="vhost_http_timeout"
            label="HTTP 响应超时 (秒)"
            extra={<span style={{ fontSize: 11 }}>默认 60</span>}
          >
            <InputNumber min={1} placeholder="60" style={{ width: '100%' }} />
          </Form.Item>
        </Col>
      </Row>

      <SectionTitle>子域名</SectionTitle>
      <Form.Item
        name="sub_domain_host"
        label="子域名主机"
        extra={<span style={{ fontSize: 11 }}>配置后客户端可使用子域名，如填 <code>frps.com</code>，客户端 subdomain=test 则访问 <code>test.frps.com</code></span>}
      >
        <Input placeholder="frps.com（可选）" style={{ width: '100%' }} />
      </Form.Item>

      <SectionTitle>自定义页面</SectionTitle>
      <Form.Item
        name="custom_404_page"
        label="自定义 404 页面"
        extra={<span style={{ fontSize: 11 }}>自定义 404 错误页面文件路径</span>}
      >
        <Input placeholder="./404.html（可选）" style={{ width: '100%' }} />
      </Form.Item>

      <SectionTitle>Dashboard</SectionTitle>
      <Row gutter={16}>
        <Col span={16}>
          <Form.Item
            name="dashboard_addr"
            label="Dashboard 监听地址"
            extra={<span style={{ fontSize: 11 }}>默认 127.0.0.1，填 0.0.0.0 允许外部访问</span>}
          >
            <Input placeholder="127.0.0.1" style={{ width: '100%' }} />
          </Form.Item>
        </Col>
        <Col span={8}>
          <Form.Item
            name="dashboard_port"
            label="Dashboard 端口"
            extra={<span style={{ fontSize: 11 }}>留空则不启用</span>}
          >
            <InputNumber min={1} max={65535} placeholder="7500（留空不启用）" style={{ width: '100%' }} />
          </Form.Item>
        </Col>
      </Row>
      <Row gutter={16}>
        <Col span={12}>
          <Form.Item
            name="dashboard_user"
            label="登录用户名"
          >
            <Input placeholder="admin" style={{ width: '100%' }} />
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item
            name="dashboard_password"
            label="登录密码"
          >
            <Input.Password placeholder="Dashboard 登录密码" style={{ width: '100%' }} />
          </Form.Item>
        </Col>
      </Row>

      <SectionTitle>Prometheus 监控</SectionTitle>
      <Row gutter={16}>
        <Col span={8}>
          <Form.Item
            name="enable_prometheus"
            label="启用 Prometheus"
            valuePropName="checked"
            extra={<span style={{ fontSize: 11 }}>在 /metrics 暴露指标，需同时启用 Dashboard</span>}
          >
            <Switch />
          </Form.Item>
        </Col>
      </Row>
    </>
  )

  // ===== Tab 4: 日志与高级 =====
  const tabAdvanced = (
    <>
      <SectionTitle>日志</SectionTitle>
      <Row gutter={16}>
        <Col span={12}>
          <Form.Item name="log_level" label="日志级别">
            <Select style={{ width: '100%' }}>
              <Option value="trace">trace</Option>
              <Option value="debug">debug</Option>
              <Option value="info">info（推荐）</Option>
              <Option value="warn">warn</Option>
              <Option value="error">error</Option>
            </Select>
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item
            name="log_max_days"
            label="日志保留天数"
            extra={<span style={{ fontSize: 11 }}>默认 3 天</span>}
          >
            <InputNumber min={1} placeholder="3" style={{ width: '100%' }} />
          </Form.Item>
        </Col>
      </Row>
      <Row gutter={16}>
        <Col span={24}>
          <Form.Item
            name="log_file"
            label="日志文件路径"
            extra={<span style={{ fontSize: 11 }}>留空输出到控制台，填路径写入文件，如 <code>./frps.log</code></span>}
          >
            <Input placeholder="留空输出到控制台" style={{ width: '100%' }} />
          </Form.Item>
        </Col>
      </Row>

      <SectionTitle>SSH 隧道网关</SectionTitle>
      <Row gutter={16}>
        <Col span={12}>
          <Form.Item
            name="ssh_tunnel_gateway_bind_port"
            label="SSH 隧道监听端口"
            extra={<span style={{ fontSize: 11 }}>大于 0 时启用 SSH 隧道网关功能</span>}
          >
            <InputNumber min={0} max={65535} placeholder="2200（留空禁用）" style={{ width: '100%' }} />
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item
            name="ssh_tunnel_gateway_private_key_file"
            label="SSH 私钥文件"
            extra={<span style={{ fontSize: 11 }}>留空则使用自动生成路径</span>}
          >
            <Input placeholder="/home/user/.ssh/id_rsa（可选）" style={{ width: '100%' }} />
          </Form.Item>
        </Col>
      </Row>
      <Row gutter={16}>
        <Col span={12}>
          <Form.Item
            name="ssh_tunnel_gateway_auto_gen_key_path"
            label="自动生成私钥路径"
            extra={<span style={{ fontSize: 11 }}>默认 ./.autogen_ssh_key，私钥文件不存在时自动生成</span>}
          >
            <Input placeholder="./.autogen_ssh_key（可选）" style={{ width: '100%' }} />
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item
            name="ssh_tunnel_gateway_authorized_keys_file"
            label="授权公钥文件"
            extra={<span style={{ fontSize: 11 }}>SSH authorized_keys 文件路径，留空不鉴权</span>}
          >
            <Input placeholder="/home/user/.ssh/authorized_keys（可选）" style={{ width: '100%' }} />
          </Form.Item>
        </Col>
      </Row>

      <SectionTitle>其他</SectionTitle>
      <Row gutter={16}>
        <Col span={12}>
          <Form.Item
            name="detailed_errors_to_client"
            label="向客户端发送详细错误"
            valuePropName="checked"
            extra={<span style={{ fontSize: 11 }}>默认开启，调试时有用</span>}
          >
            <Switch />
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item
            name="nathole_analysis_data_reserve_hours"
            label="NAT 打洞数据保留 (小时)"
            extra={<span style={{ fontSize: 11 }}>默认 168 小时（7天）</span>}
          >
            <InputNumber min={1} placeholder="168" style={{ width: '100%' }} />
          </Form.Item>
        </Col>
      </Row>
    </>
  )

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
        width={660} destroyOnHidden
        styles={{ body: { padding: '4px 24px 0' } }}
      >
        <Form form={form} layout="vertical" style={{ paddingTop: 4 }}>
          <Tabs
            size="small"
            items={[
              { key: 'basic',     label: <span><SettingOutlined />  基本配置</span>,   children: tabBasic },
              { key: 'transport', label: <span><ApiOutlined />      传输协议</span>, children: tabTransport },
              { key: 'http',      label: <span><GlobalOutlined />   页面设置</span>, children: tabHttp },
              { key: 'advanced',  label: <span><FileTextOutlined /> 更多设置</span>, children: tabAdvanced },
            ]}
          />
        </Form>
      </Modal>
    </div>
  )
}

export default FrpServer
