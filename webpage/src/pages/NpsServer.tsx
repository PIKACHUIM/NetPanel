import React, { useEffect, useState } from 'react'
import {
  Table, Button, Space, Switch, Modal, Form, Input, InputNumber,
  Popconfirm, message, Typography, Tooltip, Row, Col, Tag, Tabs, Select,
} from 'antd'
import {
  PlusOutlined, EditOutlined, DeleteOutlined,
  PlayCircleOutlined, StopOutlined, LinkOutlined,
  SettingOutlined, ApiOutlined, AppstoreOutlined,
  SafetyOutlined, DatabaseOutlined, BugOutlined,
} from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { npsServerApi } from '../api'
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
      bridge_ip: '0.0.0.0',
      bridge_tcp_port: 8024,
      bridge_kcp_port: 8024,
      bridge_tls_port: 8025,
      bridge_quic_port: 8025,
      bridge_ws_port: 8026,
      bridge_wss_port: 8027,
      bridge_path: '/ws',
      bridge_select_mode: 'Primary',
      http_proxy_ip: '0.0.0.0',
      http_proxy_port: 80,
      https_proxy_port: 443,
      http_proxy_response_timeout: 100,
      http_add_origin_header: true,
      p2p_ip: '0.0.0.0',
      p2p_port: 6000,
      disconnect_timeout: 60,
      web_ip: '0.0.0.0',
      web_port: 8081,
      web_username: 'admin',
      web_password: '123456',
      web_open_ssl: false,
      open_captcha: true,
      allow_user_login: false,
      allow_user_register: false,
      secure_mode: true,
      log: 'stdout',
      log_level: 'info',
      log_max_files: 10,
      log_max_days: 7,
      log_max_size: 2,
      log_compress: false,
      flow_store_interval: 1,
      allow_flow_limit: true,
      allow_rate_limit: true,
      allow_time_limit: true,
      allow_tunnel_num_limit: true,
      allow_connection_num_limit: true,
      allow_multi_ip: true,
      system_info_display: true,
      allow_local_proxy: false,
      allow_secret_link: false,
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
      title: '客户端连接端口',
      render: (_: any, r: any) => (
        <Space size={4} wrap>
          {r.bridge_tcp_port && <Tag style={{ fontSize: 11 }}>TCP:{r.bridge_tcp_port}</Tag>}
          {r.bridge_kcp_port && <Tag color="blue" style={{ fontSize: 11 }}>KCP:{r.bridge_kcp_port}</Tag>}
          {r.bridge_tls_port && <Tag color="green" style={{ fontSize: 11 }}>TLS:{r.bridge_tls_port}</Tag>}
          {r.bridge_quic_port && <Tag color="purple" style={{ fontSize: 11 }}>QUIC:{r.bridge_quic_port}</Tag>}
        </Space>
      ),
    },
    {
      title: 'HTTP/HTTPS',
      render: (_: any, r: any) => (
        <Space size={4}>
          {r.http_proxy_port ? <Tag style={{ fontSize: 11 }}>HTTP:{r.http_proxy_port}</Tag> : null}
          {r.https_proxy_port ? <Tag color="green" style={{ fontSize: 11 }}>HTTPS:{r.https_proxy_port}</Tag> : null}
        </Space>
      ),
    },
    {
      title: t('nps.webPanel'), width: 180,
      render: (_: any, r: any) => r.web_port ? (
        <Space size={4}>
          <Text code style={{ fontSize: 12 }}>{r.web_ip || '0.0.0.0'}:{r.web_port}</Text>
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

  // ===== Tab 1: 基本配置 =====
  const tabBasic = (
    <>
      <Row gutter={16}>
        <Col span={18}>
          <Form.Item name="name" label="名称" rules={[{ required: true, message: '请填写名称' }]}>
            <Input placeholder="服务端实例名称" />
          </Form.Item>
        </Col>
        <Col span={6}>
          <Form.Item name="enable" label="启用" valuePropName="checked">
            <Switch />
          </Form.Item>
        </Col>
      </Row>

      <SectionTitle>安全设置</SectionTitle>
      <Row gutter={16}>
        <Col span={12}>
          <Form.Item name="secure_mode" label="安全模式" valuePropName="checked"
            extra={<span style={{ fontSize: 11 }}>开启后提高安全性，不再兼容旧版客户端</span>}>
            <Switch />
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item name="public_vkey" label="公共密钥"
            extra={<span style={{ fontSize: 11 }}>客户端可使用此密钥连接，留空禁用</span>}>
            <Input.Password placeholder="留空禁用" />
          </Form.Item>
        </Col>
      </Row>
      <Row gutter={16}>
        <Col span={12}>
          <Form.Item name="auth_key" label="API 认证密钥"
            extra={<span style={{ fontSize: 11 }}>用于 API 访问的认证密钥</span>}>
            <Input.Password placeholder="建议设置长且复杂的密钥" />
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item name="auth_crypt_key" label="AES 加密密钥"
            extra={<span style={{ fontSize: 11 }}>16位，用于获取服务端 authKey 时的 AES 加密</span>}>
            <Input placeholder="16位随机字符串" maxLength={16} />
          </Form.Item>
        </Col>
      </Row>

      <SectionTitle>DNS / 时区</SectionTitle>
      <Row gutter={16}>
        <Col span={12}>
          <Form.Item name="dns_server" label="DNS 服务器">
            <Input placeholder="8.8.8.8" />
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item name="disconnect_timeout" label="客户端断线超时(s)"
            extra={<span style={{ fontSize: 11 }}>客户端断开连接超时时间（秒）</span>}>
            <InputNumber min={1} style={{ width: '100%' }} placeholder="1800" />
          </Form.Item>
        </Col>
      </Row>

      <Form.Item name="remark" label="备注">
        <Input.TextArea rows={2} placeholder="备注（可选）" />
      </Form.Item>
    </>
  )

  // ===== Tab 2: 客户端连接 =====
  const tabBridge = (
    <>
      <SectionTitle>监听地址</SectionTitle>
      <Row gutter={16}>
        <Col span={12}>
          <Form.Item name="bridge_ip" label="绑定地址"
            extra={<span style={{ fontSize: 11 }}><code>0.0.0.0</code> 表示所有网卡</span>}>
            <Input placeholder="0.0.0.0" />
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item name="bridge_host" label="端口复用域名"
            extra={<span style={{ fontSize: 11 }}>端口复用时需要配置此域名</span>}>
            <Input placeholder="xxx.com" />
          </Form.Item>
        </Col>
      </Row>

      <SectionTitle>协议端口（填 0 表示禁用）</SectionTitle>
      <Row gutter={16}>
        <Col span={8}>
          <Form.Item name="bridge_tcp_port" label="TCP 端口">
            <InputNumber min={0} max={65535} style={{ width: '100%' }} placeholder="8024" />
          </Form.Item>
        </Col>
        <Col span={8}>
          <Form.Item name="bridge_kcp_port" label="KCP 端口"
            extra={<span style={{ fontSize: 11 }}>UDP 加速传输</span>}>
            <InputNumber min={0} max={65535} style={{ width: '100%' }} placeholder="8024" />
          </Form.Item>
        </Col>
        <Col span={8}>
          <Form.Item name="bridge_tls_port" label="TLS 端口">
            <InputNumber min={0} max={65535} style={{ width: '100%' }} placeholder="8025" />
          </Form.Item>
        </Col>
      </Row>
      <Row gutter={16}>
        <Col span={8}>
          <Form.Item name="bridge_quic_port" label="QUIC 端口">
            <InputNumber min={0} max={65535} style={{ width: '100%' }} placeholder="8025" />
          </Form.Item>
        </Col>
        <Col span={8}>
          <Form.Item name="bridge_ws_port" label="WebSocket 端口">
            <InputNumber min={0} max={65535} style={{ width: '100%' }} placeholder="8026" />
          </Form.Item>
        </Col>
        <Col span={8}>
          <Form.Item name="bridge_wss_port" label="WebSocket TLS 端口">
            <InputNumber min={0} max={65535} style={{ width: '100%' }} placeholder="8027" />
          </Form.Item>
        </Col>
      </Row>

      <SectionTitle>WebSocket 设置</SectionTitle>
      <Row gutter={16}>
        <Col span={12}>
          <Form.Item name="bridge_path" label="WS 连接路径">
            <Input placeholder="/ws" />
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item name="bridge_select_mode" label="多客户端选取策略"
            extra={<span style={{ fontSize: 11 }}>相同 vkey 多客户端时的负载策略</span>}>
            <Select style={{ width: '100%' }}>
              <Option value="Primary">主备 (Primary)</Option>
              <Option value="RoundRobin">轮询 (RoundRobin)</Option>
              <Option value="Random">随机 (Random)</Option>
            </Select>
          </Form.Item>
        </Col>
      </Row>
      <Row gutter={16}>
        <Col span={12}>
          <Form.Item name="bridge_addr" label="显示连接地址"
            extra={<span style={{ fontSize: 11 }}>在网页命令行显示的连接地址，留空使用网页地址</span>}>
            <Input placeholder="留空自动" />
          </Form.Item>
        </Col>
      </Row>

      <SectionTitle>P2P 穿透</SectionTitle>
      <Row gutter={16}>
        <Col span={12}>
          <Form.Item name="p2p_ip" label="P2P 监听地址"
            extra={<span style={{ fontSize: 11 }}><code>0.0.0.0</code> 自动识别，<code>::</code> 自动识别 IPv6</span>}>
            <Input placeholder="0.0.0.0" />
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item name="p2p_port" label="P2P 端口">
            <InputNumber min={1} max={65535} style={{ width: '100%' }} placeholder="6000" />
          </Form.Item>
        </Col>
      </Row>

      <SectionTitle>允许端口范围</SectionTitle>
      <Form.Item name="allow_ports" label="允许的端口范围"
        extra={<span style={{ fontSize: 11 }}>格式：9001-9009,10001,11000-12000，留空不限制</span>}>
        <Input placeholder="留空不限制，例如：9001-9009,10001" />
      </Form.Item>
    </>
  )

  // ===== Tab 3: 域名转发 =====
  const tabDomain = (
    <>
      <SectionTitle>HTTP/HTTPS 代理</SectionTitle>
      <Row gutter={16}>
        <Col span={8}>
          <Form.Item name="http_proxy_ip" label="代理监听地址">
            <Input placeholder="0.0.0.0" />
          </Form.Item>
        </Col>
        <Col span={8}>
          <Form.Item name="http_proxy_port" label="HTTP 端口"
            extra={<span style={{ fontSize: 11 }}>0 表示禁用</span>}>
            <InputNumber min={0} max={65535} style={{ width: '100%' }} placeholder="80" />
          </Form.Item>
        </Col>
        <Col span={8}>
          <Form.Item name="https_proxy_port" label="HTTPS 端口"
            extra={<span style={{ fontSize: 11 }}>0 表示禁用</span>}>
            <InputNumber min={0} max={65535} style={{ width: '100%' }} placeholder="443" />
          </Form.Item>
        </Col>
      </Row>
      <Row gutter={16}>
        <Col span={12}>
          <Form.Item name="http_proxy_response_timeout" label="后端响应超时(s)">
            <InputNumber min={1} style={{ width: '100%' }} placeholder="100" />
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item name="http_add_origin_header" label="获取客户端真实IP" valuePropName="checked">
            <Switch />
          </Form.Item>
        </Col>
      </Row>

      <SectionTitle>HTTPS 默认证书</SectionTitle>
      <Row gutter={16}>
        <Col span={12}>
          <Form.Item name="https_default_cert_file" label="证书文件路径">
            <Input placeholder="conf/server.pem" />
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item name="https_default_key_file" label="私钥文件路径">
            <Input placeholder="conf/server.key" />
          </Form.Item>
        </Col>
      </Row>

      <SectionTitle>自动申请 SSL 证书</SectionTitle>
      <Row gutter={16}>
        <Col span={8}>
          <Form.Item name="force_auto_ssl" label="自动申请证书" valuePropName="checked">
            <Switch />
          </Form.Item>
        </Col>
        <Col span={8}>
          <Form.Item name="ssl_ca" label="证书 CA">
            <Select style={{ width: '100%' }}>
              <Option value="LetsEncrypt">Let's Encrypt</Option>
              <Option value="ZeroSSL">ZeroSSL</Option>
              <Option value="GoogleTrust">Google Trust</Option>
            </Select>
          </Form.Item>
        </Col>
        <Col span={8}>
          <Form.Item name="ssl_email" label="申请邮箱">
            <Input placeholder="you@yours.com" />
          </Form.Item>
        </Col>
      </Row>
      <Row gutter={16}>
        <Col span={12}>
          <Form.Item name="ssl_path" label="证书保存目录">
            <Input placeholder="conf/ssl" />
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item name="ssl_cache_max" label="证书缓存最大数"
            extra={<span style={{ fontSize: 11 }}>0 表示不限制</span>}>
            <InputNumber min={0} style={{ width: '100%' }} placeholder="0" />
          </Form.Item>
        </Col>
      </Row>
    </>
  )

  // ===== Tab 4: 管理面板 =====
  const tabWebPanel = (
    <>
      <SectionTitle>监听设置</SectionTitle>
      <Row gutter={16}>
        <Col span={8}>
          <Form.Item name="web_ip" label="监听地址">
            <Input placeholder="0.0.0.0" />
          </Form.Item>
        </Col>
        <Col span={8}>
          <Form.Item name="web_port" label="管理面板端口" rules={[{ required: true, message: '请填写端口' }]}>
            <InputNumber min={1} max={65535} style={{ width: '100%' }} placeholder="8081" />
          </Form.Item>
        </Col>
        <Col span={8}>
          <Form.Item name="web_host" label="面板域名"
            extra={<span style={{ fontSize: 11 }}>通过域名访问管理面板时配置</span>}>
            <Input placeholder="a.o.com" />
          </Form.Item>
        </Col>
      </Row>
      <Row gutter={16}>
        <Col span={8}>
          <Form.Item name="web_open_ssl" label="启用 HTTPS" valuePropName="checked">
            <Switch />
          </Form.Item>
        </Col>
        <Col span={8}>
          <Form.Item name="web_cert_file" label="证书文件">
            <Input placeholder="conf/server.pem" />
          </Form.Item>
        </Col>
        <Col span={8}>
          <Form.Item name="web_key_file" label="私钥文件">
            <Input placeholder="conf/server.key" />
          </Form.Item>
        </Col>
      </Row>

      <SectionTitle>登录账号</SectionTitle>
      <Row gutter={16}>
        <Col span={8}>
          <Form.Item name="web_username" label="管理员用户名">
            <Input placeholder="admin" />
          </Form.Item>
        </Col>
        <Col span={8}>
          <Form.Item name="web_password" label="管理员密码">
            <Input.Password placeholder="登录密码" />
          </Form.Item>
        </Col>
        <Col span={8}>
          <Form.Item name="totp_secret" label="2FA 密钥"
            extra={<span style={{ fontSize: 11 }}>TOTP 双因素认证密钥，留空禁用</span>}>
            <Input placeholder="留空禁用" />
          </Form.Item>
        </Col>
      </Row>

      <SectionTitle>安全设置</SectionTitle>
      <Row gutter={16}>
        <Col span={8}>
          <Form.Item name="open_captcha" label="启用验证码" valuePropName="checked">
            <Switch />
          </Form.Item>
        </Col>
        <Col span={8}>
          <Form.Item name="allow_x_real_ip" label="允许 X-Real-IP" valuePropName="checked">
            <Switch />
          </Form.Item>
        </Col>
        <Col span={8}>
          <Form.Item name="trusted_proxy_ips" label="受信任代理IP"
            extra={<span style={{ fontSize: 11 }}>多个用逗号分隔</span>}>
            <Input placeholder="127.0.0.1" />
          </Form.Item>
        </Col>
      </Row>

      <SectionTitle>多用户设置</SectionTitle>
      <Row gutter={16}>
        <Col span={8}>
          <Form.Item name="allow_user_login" label="允许用户登录" valuePropName="checked">
            <Switch />
          </Form.Item>
        </Col>
        <Col span={8}>
          <Form.Item name="allow_user_register" label="允许用户注册" valuePropName="checked">
            <Switch />
          </Form.Item>
        </Col>
        <Col span={8}>
          <Form.Item name="allow_user_change_username" label="允许修改用户名" valuePropName="checked">
            <Switch />
          </Form.Item>
        </Col>
      </Row>
    </>
  )

  // ===== Tab 5: 功能限制 =====
  const tabLimits = (
    <>
      <SectionTitle>流量与资源限制</SectionTitle>
      <Row gutter={16}>
        <Col span={8}>
          <Form.Item name="flow_store_interval" label="流量持久化间隔(min)"
            extra={<span style={{ fontSize: 11 }}>留空不持久化，使用限制功能需开启</span>}>
            <InputNumber min={0} style={{ width: '100%' }} placeholder="1" />
          </Form.Item>
        </Col>
        <Col span={8}>
          <Form.Item name="allow_flow_limit" label="流量限制" valuePropName="checked">
            <Switch />
          </Form.Item>
        </Col>
        <Col span={8}>
          <Form.Item name="allow_rate_limit" label="带宽限制" valuePropName="checked">
            <Switch />
          </Form.Item>
        </Col>
      </Row>
      <Row gutter={16}>
        <Col span={8}>
          <Form.Item name="allow_time_limit" label="时间限制" valuePropName="checked">
            <Switch />
          </Form.Item>
        </Col>
        <Col span={8}>
          <Form.Item name="allow_tunnel_num_limit" label="隧道数量限制" valuePropName="checked">
            <Switch />
          </Form.Item>
        </Col>
        <Col span={8}>
          <Form.Item name="allow_connection_num_limit" label="连接数限制" valuePropName="checked">
            <Switch />
          </Form.Item>
        </Col>
      </Row>

      <SectionTitle>代理权限</SectionTitle>
      <Row gutter={16}>
        <Col span={8}>
          <Form.Item name="allow_local_proxy" label="允许本地代理" valuePropName="checked"
            extra={<span style={{ fontSize: 11 }}>允许 NPS 本地代理连接</span>}>
            <Switch />
          </Form.Item>
        </Col>
        <Col span={8}>
          <Form.Item name="allow_multi_ip" label="允许多IP监听" valuePropName="checked"
            extra={<span style={{ fontSize: 11 }}>允许配置隧道监听 IP</span>}>
            <Switch />
          </Form.Item>
        </Col>
        <Col span={8}>
          <Form.Item name="allow_secret_link" label="私密代理任意地址" valuePropName="checked"
            extra={<span style={{ fontSize: 11 }}>允许私密代理客户端连接任意地址</span>}>
            <Switch />
          </Form.Item>
        </Col>
      </Row>
      <Row gutter={16}>
        <Col span={8}>
          <Form.Item name="system_info_display" label="显示系统负载" valuePropName="checked">
            <Switch />
          </Form.Item>
        </Col>
      </Row>
    </>
  )

  // ===== Tab 6: 日志配置 =====
  const tabLog = (
    <>
      <SectionTitle>日志输出</SectionTitle>
      <Row gutter={16}>
        <Col span={12}>
          <Form.Item name="log" label="日志模式">
            <Select style={{ width: '100%' }}>
              <Option value="stdout">stdout（控制台）</Option>
              <Option value="file">file（文件）</Option>
              <Option value="both">both（控制台+文件）</Option>
              <Option value="off">off（关闭）</Option>
            </Select>
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item name="log_level" label="日志级别">
            <Select style={{ width: '100%' }}>
              {['trace', 'debug', 'info', 'warn', 'error', 'fatal', 'panic', 'off'].map(l => (
                <Option key={l} value={l}>{l}</Option>
              ))}
            </Select>
          </Form.Item>
        </Col>
      </Row>
      <Row gutter={16}>
        <Col span={12}>
          <Form.Item name="log_path" label="日志文件路径">
            <Input placeholder="conf/nps.log" />
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item name="log_compress" label="启用日志压缩" valuePropName="checked">
            <Switch />
          </Form.Item>
        </Col>
      </Row>

      <SectionTitle>日志轮转</SectionTitle>
      <Row gutter={16}>
        <Col span={8}>
          <Form.Item name="log_max_files" label="最大文件数">
            <InputNumber min={1} style={{ width: '100%' }} placeholder="10" />
          </Form.Item>
        </Col>
        <Col span={8}>
          <Form.Item name="log_max_days" label="最大保留天数">
            <InputNumber min={1} style={{ width: '100%' }} placeholder="7" />
          </Form.Item>
        </Col>
        <Col span={8}>
          <Form.Item name="log_max_size" label="单文件最大(MB)">
            <InputNumber min={1} style={{ width: '100%' }} placeholder="2" />
          </Form.Item>
        </Col>
      </Row>
    </>
  )

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
        width={680} destroyOnHidden
        styles={{ body: { padding: '4px 24px 0' } }}
      >
        <Form form={form} layout="vertical" style={{ paddingTop: 4 }}>
          <Tabs
            size="small"
            items={[
              { key: 'basic',    label: <span><SettingOutlined /> 基本配置</span>,   children: tabBasic },
              { key: 'bridge',   label: <span><ApiOutlined /> 客户端连接</span>,     children: tabBridge },
              { key: 'domain',   label: <span><LinkOutlined /> 域名转发</span>,      children: tabDomain },
              { key: 'webpanel', label: <span><AppstoreOutlined /> 管理面板</span>,  children: tabWebPanel },
              { key: 'limits',   label: <span><SafetyOutlined /> 功能限制</span>,    children: tabLimits },
              { key: 'log',      label: <span><DatabaseOutlined /> 日志配置</span>,  children: tabLog },
            ]}
          />
        </Form>
      </Modal>
    </div>
  )
}

export default NpsServer
