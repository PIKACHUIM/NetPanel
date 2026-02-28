import React, { useEffect, useState } from 'react'
import {
  Table, Button, Space, Switch, Modal, Form, Input,
  Popconfirm, message, Typography, Tag, Tooltip, Row, Col,
  Checkbox, Radio, Select, Tabs, InputNumber,
} from 'antd'
import {
  PlusOutlined, EditOutlined, DeleteOutlined,
  PlayCircleOutlined, StopOutlined, InfoCircleOutlined, MinusCircleOutlined,
  SettingOutlined, WifiOutlined, SafetyOutlined, ThunderboltOutlined,
  GlobalOutlined, ApiOutlined, FileTextOutlined,
} from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { easytierServerApi } from '../api'
import StatusTag from '../components/StatusTag'

const { Text } = Typography

// 分组标题
const SectionTitle = ({ children }: { children: React.ReactNode }) => (
  <div style={{ display: 'flex', alignItems: 'center', gap: 8, margin: '12px 0 8px' }}>
    <div style={{ width: 3, height: 14, background: '#1677ff', borderRadius: 2, flexShrink: 0 }} />
    <span style={{ fontSize: 12, fontWeight: 600, color: '#595959', letterSpacing: '0.02em' }}>{children}</span>
    <div style={{ flex: 1, height: 1, background: '#f0f0f0' }} />
  </div>
)

const PROTOCOL_OPTIONS = [
  { label: 'TCP', value: 'tcp' },
  { label: 'UDP', value: 'udp' },
  { label: 'WS', value: 'ws' },
  { label: 'WSS', value: 'wss' },
  { label: 'WG', value: 'wg' },
  { label: 'QUIC', value: 'quic' },
]

// 随机生成网络名称（8位字母数字）
const genNetworkName = () => Math.random().toString(36).slice(2, 10)
// 随机生成网络密码（16位字母数字）
const genNetworkPassword = () => Math.random().toString(36).slice(2, 10) + Math.random().toString(36).slice(2, 10)

const parseListenPorts = (s: string): string[] => {
  if (!s) return []
  return s.split(',').map(p => p.trim()).filter(Boolean)
}
const joinListenPorts = (ports: string[]): string => (ports || []).filter(Boolean).join(',')

const EasytierServer: React.FC = () => {
  const { t } = useTranslation()
  const [data, setData] = useState<any[]>([])
  const [loading, setLoading] = useState(false)
  const [modalOpen, setModalOpen] = useState(false)
  const [editRecord, setEditRecord] = useState<any>(null)
  const [serverMode, setServerMode] = useState<string>('standalone')
  const [quickModalOpen, setQuickModalOpen] = useState(false)
  const [quickForm] = Form.useForm()
  const [form] = Form.useForm()

  const fetchData = async () => {
    setLoading(true)
    try {
      const res: any = await easytierServerApi.list()
      setData(res.data || [])
    } finally { setLoading(false) }
  }
  useEffect(() => { fetchData() }, [])

  const handleQuickCreate = () => {
    quickForm.resetFields()
    quickForm.setFieldsValue({
      listen_addr: '0.0.0.0',
      listeners: [{ proto: 'tcp', port: '11010' }, { proto: 'udp', port: '11010' }],
      relay_network_whitelist: '',
      network_name: genNetworkName(),
      network_password: genNetworkPassword(),
      dhcp: false,
      ipv4: '',
    })
    setQuickModalOpen(true)
  }

  const handleQuickSubmit = async () => {
    const values = await quickForm.validateFields()
    const listeners: Array<{ proto: string; port: string }> = values.listeners || []
    const listenPorts = listeners
      .filter(l => l.proto && l.port)
      .map(l => `${l.proto}:${l.port}`)
      .join(',')
    const firstPort = listeners[0]?.port || '11010'
    const payload = {
      name: values.remark || `ET服务端-${firstPort}`,
      enable: true,
      server_mode: 'standalone',
      listen_addr: values.listen_addr || '0.0.0.0',
      listen_ports: listenPorts,
      network_name: values.network_name,
      network_password: values.network_password,
      relay_network_whitelist: values.enable_relay ? '*' : '',
      multi_thread: true,
      remark: values.remark || '',
      dhcp: values.dhcp || false,
      ipv4: values.dhcp ? '' : (values.ipv4 || ''),
    }
    await easytierServerApi.create(payload)
    message.success(t('common.success'))
    setQuickModalOpen(false)
    fetchData()
  }

  const handleCreate = () => {
    setEditRecord(null)
    setServerMode('standalone')
    form.resetFields()
    form.setFieldsValue({
      enable: true,
      server_mode: 'standalone',
      listen_addr: '0.0.0.0',
      listen_ports_list: [{ proto: 'tcp', port: '11010' }, { proto: 'udp', port: '11010' }],
      multi_thread: true,
    })
    setModalOpen(true)
  }

  const handleEdit = (record: any) => {
    setEditRecord(record)
    const mode = record.server_mode || 'standalone'
    setServerMode(mode)
    const portsList = parseListenPorts(record.listen_ports).map(p => {
      if (p.includes(':')) { const [proto, port] = p.split(':'); return { proto, port } }
      return { proto: 'tcp', port: p }
    })
    form.setFieldsValue({
      ...record,
      server_mode: mode,
      listen_ports_list: portsList.length > 0 ? portsList : [{ proto: 'tcp', port: '11010' }],
    })
    setModalOpen(true)
  }

  const handleSubmit = async () => {
    const values = await form.validateFields()
    const portsList: Array<{ proto: string; port: string }> = values.listen_ports_list || []
    values.listen_ports = joinListenPorts(
      portsList.map(item => item.proto && item.port ? `${item.proto}:${item.port}` : item.port).filter(Boolean)
    )
    delete values.listen_ports_list
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
    { title: t('common.status'), dataIndex: 'status', width: 100, render: (s: string) => <StatusTag status={s} /> },
    {
      title: t('common.enable'), dataIndex: 'enable', width: 80,
      render: (v: boolean, r: any) => <Switch size="small" checked={v} onChange={c => handleToggle(r, c)} />,
    },
    {
      title: t('common.name'), dataIndex: 'name',
      render: (name: string, r: any) => (
        <div>
          <Text strong>{name}</Text>
          {r.hostname && <Text type="secondary" style={{ fontSize: 11 }}> ({r.hostname})</Text>}
          {r.remark && <div><Text type="secondary" style={{ fontSize: 12 }}>{r.remark}</Text></div>}
        </div>
      ),
    },
    {
      title: '模式', dataIndex: 'server_mode', width: 110,
      render: (mode: string, r: any) => mode === 'config-server'
        ? <Tooltip title={r.config_server_addr || '未配置地址'}><Tag color="purple">节点模式</Tag></Tooltip>
        : <Tag color="blue">独立模式</Tag>,
    },
    {
      title: '监听端口',
      render: (_: any, r: any) => {
        if (r.server_mode === 'config-server') return <Text type="secondary" style={{ fontSize: 11 }}>{r.config_server_addr || '-'}</Text>
        const ports = parseListenPorts(r.listen_ports)
        if (ports.length === 0) return <Text type="secondary">未配置</Text>
        return <Space size={4} wrap>{ports.map((p, i) => <Tag key={i} color="geekblue" style={{ fontSize: 11 }}>{p}</Tag>)}</Space>
      },
    },
    {
      title: '选项',
      render: (_: any, r: any) => (
        <Space size={4} wrap>
          {r.no_tun && <Tag color="orange">no-tun</Tag>}
          {r.disable_p2p && <Tag color="red">no-p2p</Tag>}
          {r.enable_exit_node && <Tag color="volcano">出口节点</Tag>}
          {r.enable_kcp_proxy && <Tag color="cyan">KCP</Tag>}
          {r.enable_quic_proxy && <Tag color="cyan">QUIC</Tag>}
          {r.multi_thread && <Tag color="geekblue">多线程</Tag>}
        </Space>
      ),
    },
    {
      title: t('easytier.networkName'), dataIndex: 'network_name',
      render: (v: string) => v ? <Tag color="blue">{v}</Tag> : <Tag color="default">公开服务器</Tag>,
    },
    {
      title: t('common.action'), width: 160,
      render: (_: any, r: any) => (
        <Space size={4}>
          {r.status === 'running'
            ? <Tooltip title={t('common.stop')}><Button size="small" icon={<StopOutlined />} onClick={async () => { await easytierServerApi.stop(r.id); fetchData() }} /></Tooltip>
            : <Tooltip title={t('common.start')}><Button size="small" type="primary" icon={<PlayCircleOutlined />} onClick={async () => { await easytierServerApi.start(r.id); fetchData() }} /></Tooltip>
          }
          {r.last_error && <Tooltip title={r.last_error}><Button size="small" icon={<InfoCircleOutlined />} danger /></Tooltip>}
          <Tooltip title={t('common.edit')}><Button size="small" icon={<EditOutlined />} onClick={() => handleEdit(r)} /></Tooltip>
          <Popconfirm title={t('common.deleteConfirm')} onConfirm={async () => { await easytierServerApi.delete(r.id); fetchData() }}>
            <Tooltip title={t('common.delete')}><Button size="small" danger icon={<DeleteOutlined />} /></Tooltip>
          </Popconfirm>
        </Space>
      ),
    },
  ]

  // ===== Tab 1: 基本配置 =====
  const tabBasic = (
    <>
      <Row gutter={16}>
        <Col span={16}>
          <Form.Item name="name" label="名称" rules={[{ required: true, message: '请填写名称' }]}>
            <Input placeholder="服务端名称" style={{ width: '100%' }} />
          </Form.Item>
        </Col>
        <Col span={4}>
          <Form.Item name="enable" label="启用" valuePropName="checked">
            <Switch />
          </Form.Item>
        </Col>
        <Col span={4}>
          <Form.Item name="multi_thread" label="多线程" valuePropName="checked">
            <Switch />
          </Form.Item>
        </Col>
      </Row>

      <Form.Item
        name="server_mode"
        label="运行模式"
        extra={
          serverMode === 'config-server'
            ? <span style={{ fontSize: 11 }}>节点模式：连接到 config-server，配置由服务端下发，无需手动配置网络参数</span>
            : <span style={{ fontSize: 11 }}>独立模式：自主管理网络，可配置所有参数</span>
        }
      >
        <Radio.Group
          onChange={e => setServerMode(e.target.value)}
          optionType="button"
          buttonStyle="solid"
          options={[
            { label: '独立模式（Standalone）', value: 'standalone' },
            { label: '节点模式（Config-Server）', value: 'config-server' },
          ]}
        />
      </Form.Item>

      {serverMode === 'config-server' && (
        <>
          <Form.Item
            name="config_server_addr"
            label="Config-Server 地址"
            rules={[{ required: true, message: '请填写 config-server 地址' }]}
            extra={<span style={{ fontSize: 11 }}>格式：<code>tcp://host:port</code>，如 <code>tcp://1.2.3.4:11010</code></span>}
          >
            <Input placeholder="tcp://config-server:11010" style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item
            name="machine_id"
            label="Machine ID"
            extra={<span style={{ fontSize: 11 }}>Web 配置服务器用于识别机器，断线重连后恢复配置，需唯一且固定，留空自动获取</span>}
          >
            <Input placeholder="留空自动获取" style={{ width: '100%' }} />
          </Form.Item>
        </>
      )}

      {serverMode === 'standalone' && (
        <>
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item
                name="network_name"
                label="网络名称"
                extra={<span style={{ fontSize: 11 }}>留空为公开服务器（允许任意网络接入）</span>}
              >
                <Input placeholder="留空为公开服务器" style={{ width: '100%' }} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="network_password" label="网络密码">
                <Input.Password placeholder="网络密码（可选）" style={{ width: '100%' }} />
              </Form.Item>
            </Col>
          </Row>
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item name="hostname" label="主机名" extra={<span style={{ fontSize: 11 }}>留空使用系统主机名</span>}>
                <Input placeholder="自定义主机名（可选）" style={{ width: '100%' }} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="instance_name" label="实例名称" extra={<span style={{ fontSize: 11 }}>同机多节点时用于区分，留空使用默认</span>}>
                <Input placeholder="可选，如 node1" style={{ width: '100%' }} />
              </Form.Item>
            </Col>
          </Row>

          <Form.Item
            label="节点 IP"
            extra={<span style={{ fontSize: 11 }}>本节点在虚拟网络中的 IP 地址，留空则不创建 TUN 网卡（仅转发）</span>}
          >
            <Form.Item name="dhcp" valuePropName="checked" style={{ marginBottom: 6 }}>
              <Checkbox onChange={e => {
                if (e.target.checked) form.setFieldValue('ipv4', '')
              }}>DHCP 自动分配（从 10.0.0.1 开始）</Checkbox>
            </Form.Item>
            <Form.Item noStyle shouldUpdate={(prev, cur) => prev.dhcp !== cur.dhcp}>
              {() => !form.getFieldValue('dhcp') && (
                <Form.Item
                  name="ipv4"
                  style={{ marginBottom: 0 }}
                  rules={[{
                    pattern: /^(\d{1,3}\.){3}\d{1,3}(\/\d{1,2})?$/,
                    message: '格式如 10.0.0.1 或 10.0.0.1/24'
                  }]}
                >
                  <Input placeholder="如 10.0.0.1（可选，留空仅转发）" style={{ width: 280 }} />
                </Form.Item>
              )}
            </Form.Item>
          </Form.Item>
        </>
      )}

      <Form.Item name="remark" label="备注">
        <Input.TextArea rows={2} placeholder="备注（可选）" style={{ width: '100%' }} />
      </Form.Item>
    </>
  )

  // ===== Tab 2: 监听端口 =====
  const tabListen = (
    <>
      <Row gutter={16}>
        <Col span={10}>
          <Form.Item name="listen_addr" label="监听地址" extra={<span style={{ fontSize: 11 }}>监听的网卡地址，0.0.0.0 表示所有网卡</span>}>
            <Input placeholder="0.0.0.0" style={{ width: '100%' }} />
          </Form.Item>
        </Col>
      </Row>

      <Form.Item
        label="监听端口"
        required
        extra={<span style={{ fontSize: 11 }}>支持协议：<code>tcp</code> · <code>udp</code> · <code>ws</code> · <code>wss</code> · <code>wg</code> · <code>quic</code></span>}
      >
        <Form.List name="listen_ports_list" rules={[{
          validator: async (_, items) => {
            if (!items || items.length === 0) throw new Error('至少添加一个监听端口')
          }
        }]}>
          {(fields, { add, remove }, { errors }) => (
            <>
              {fields.map(({ key, name, ...rest }) => (
                <Row key={key} gutter={8} align="middle" style={{ marginBottom: 8 }}>
                  <Col span={7}>
                    <Form.Item {...rest} name={[name, 'proto']} style={{ marginBottom: 0 }}>
                      <Select options={PROTOCOL_OPTIONS} style={{ width: '100%' }} />
                    </Form.Item>
                  </Col>
                  <Col span={15}>
                    <Form.Item {...rest} name={[name, 'port']} style={{ marginBottom: 0 }} rules={[{ required: true, message: '请填写端口' }]}>
                      <Input placeholder="11010" style={{ width: '100%' }} />
                    </Form.Item>
                  </Col>
                  <Col span={2} style={{ display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                    {fields.length > 1 && (
                      <MinusCircleOutlined onClick={() => remove(name)} style={{ color: '#ff4d4f', fontSize: 16 }} />
                    )}
                  </Col>
                </Row>
              ))}
              <Form.ErrorList errors={errors} />
              <Button type="dashed" onClick={() => add({ proto: 'tcp', port: '' })} icon={<PlusOutlined />} block>添加端口</Button>
            </>
          )}
        </Form.List>
      </Form.Item>

      <SectionTitle>RPC 管理</SectionTitle>
      <Row gutter={16}>
        <Col span={12}>
          <Form.Item name="rpc_portal" label="RPC 门户地址"
            extra={<span style={{ fontSize: 11 }}>如 <code>0</code>（随机）、<code>15888</code>、<code>0.0.0.0:15888</code></span>}
          >
            <Input placeholder="0（随机端口）" style={{ width: '100%' }} />
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item name="rpc_portal_whitelist" label="RPC 白名单"
            extra={<span style={{ fontSize: 11 }}>如 <code>127.0.0.1/32,127.0.0.0/8</code></span>}
          >
            <Input placeholder="127.0.0.1/32,::1/128" style={{ width: '100%' }} />
          </Form.Item>
        </Col>
      </Row>
    </>
  )

  // ===== Tab 3: 路由与转发 =====
  const tabRouting = (
    <>
      <SectionTitle>手动路由</SectionTitle>
      <Row gutter={[16, 0]}>
        <Col span={24}>
          <Form.Item name="enable_manual_routes" valuePropName="checked">
            <Checkbox>启用手动路由 <Text type="secondary" style={{ fontSize: 11 }}>（--manual-routes，覆盖自动路由）</Text></Checkbox>
          </Form.Item>
        </Col>
      </Row>
      <Form.Item
        name="manual_routes"
        label="路由列表"
        extra={<span style={{ fontSize: 11 }}>逗号分隔，如 <code>10.0.0.0/24,192.168.1.0/24</code></span>}
      >
        <Input placeholder="10.0.0.0/24,192.168.1.0/24（可选）" style={{ width: '100%' }} />
      </Form.Item>

      <SectionTitle>端口转发</SectionTitle>
      <Form.Item
        name="port_forwards"
        label="转发规则"
        extra={<span style={{ fontSize: 11 }}>每行一条，格式：<code>协议:监听IP:监听端口:目标IP:目标端口</code><br />示例：<code>tcp:0.0.0.0:8080:192.168.1.1:80</code></span>}
      >
        <Input.TextArea rows={4} placeholder={'tcp:0.0.0.0:8080:192.168.1.1:80\nudp:0.0.0.0:5353:10.0.0.1:53'} style={{ width: '100%' }} />
      </Form.Item>
    </>
  )

  // ===== Tab 4: 网络行为 =====
  const tabNetwork = (
    <>
      <SectionTitle>基础行为</SectionTitle>
      <Row gutter={[16, 0]}>
        <Col span={12}>
          <Form.Item name="no_tun" valuePropName="checked">
            <Checkbox>不创建 TUN 网卡 <Text type="secondary" style={{ fontSize: 11 }}>（--no-tun，无需 Npcap）</Text></Checkbox>
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item name="disable_p2p" valuePropName="checked">
            <Checkbox>禁用 P2P 直连 <Text type="secondary" style={{ fontSize: 11 }}>（强制走中继）</Text></Checkbox>
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item name="enable_exit_node" valuePropName="checked">
            <Checkbox>允许作为出口节点 <Text type="secondary" style={{ fontSize: 11 }}>（--enable-exit-node）</Text></Checkbox>
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item name="relay_all_peer_rpc" valuePropName="checked">
            <Checkbox>中继所有对等 RPC <Text type="secondary" style={{ fontSize: 11 }}>（--relay-all-peer-rpc）</Text></Checkbox>
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item name="proxy_forward_by_system" valuePropName="checked">
            <Checkbox>系统内核转发子网代理 <Text type="secondary" style={{ fontSize: 11 }}>（--proxy-forward-by-system）</Text></Checkbox>
          </Form.Item>
        </Col>
      </Row>
      <Row gutter={16}>
        <Col span={12}>
          <Form.Item name="default_protocol" label="默认连接协议"
            extra={<span style={{ fontSize: 11 }}>连接对等节点时使用的默认协议</span>}
          >
            <Select allowClear placeholder="默认自动" options={PROTOCOL_OPTIONS} style={{ width: '100%' }} />
          </Form.Item>
        </Col>
      </Row>

      <Form.Item
        name="relay_network_whitelist"
        label="中继网络白名单"
        extra={<span style={{ fontSize: 11 }}>允许为哪些网络提供中继，填 <code>*</code> 允许所有，留空不限制</span>}
      >
        <Input placeholder="留空不限制，填 * 允许所有网络" style={{ width: '100%' }} />
      </Form.Item>

      <SectionTitle>协议加速</SectionTitle>
      <Row gutter={[16, 0]}>
        <Col span={12}>
          <Form.Item name="enable_kcp_proxy" valuePropName="checked">
            <Checkbox>启用 KCP 加速 <Text type="secondary" style={{ fontSize: 11 }}>（--enable-kcp-proxy）</Text></Checkbox>
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item name="disable_kcp_input" valuePropName="checked">
            <Checkbox>禁止其他节点 KCP 代理到本节点 <Text type="secondary" style={{ fontSize: 11 }}>（--disable-kcp-input）</Text></Checkbox>
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item name="enable_quic_proxy" valuePropName="checked">
            <Checkbox>启用 QUIC 加速 <Text type="secondary" style={{ fontSize: 11 }}>（--enable-quic-proxy）</Text></Checkbox>
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item name="disable_quic_input" valuePropName="checked">
            <Checkbox>禁止其他节点 QUIC 代理到本节点 <Text type="secondary" style={{ fontSize: 11 }}>（--disable-quic-input）</Text></Checkbox>
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item name="disable_relay_kcp" valuePropName="checked">
            <Checkbox>禁止转发 KCP 数据包 <Text type="secondary" style={{ fontSize: 11 }}>（防过度消耗流量）</Text></Checkbox>
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item name="enable_relay_foreign_network_kcp" valuePropName="checked">
            <Checkbox>作为共享节点时转发其他网络 KCP</Checkbox>
          </Form.Item>
        </Col>
      </Row>
      <Row gutter={16}>
        <Col span={12}>
          <Form.Item name="quic_listen_port" label="QUIC 监听端口"
            extra={<span style={{ fontSize: 11 }}>0 为随机端口</span>}
          >
            <InputNumber min={0} max={65535} placeholder="0（随机）" style={{ width: '100%' }} />
          </Form.Item>
        </Col>
      </Row>
    </>
  )

  // ===== Tab 5: 安全 =====
  const tabSecurity = (
    <>
      <SectionTitle>安全选项</SectionTitle>
      <Row gutter={[16, 0]}>
        <Col span={12}>
          <Form.Item name="disable_encryption" valuePropName="checked">
            <Checkbox>
              <Text type="danger">禁用加密</Text>
              <Text type="secondary" style={{ fontSize: 11 }}> （不推荐）</Text>
            </Checkbox>
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item name="private_mode" valuePropName="checked">
            <Checkbox>私有模式 <Text type="secondary" style={{ fontSize: 11 }}>（仅允许已知节点握手/中转）</Text></Checkbox>
          </Form.Item>
        </Col>
      </Row>
      <Row gutter={16}>
        <Col span={12}>
          <Form.Item name="encryption_algorithm" label="加密算法"
            extra={<span style={{ fontSize: 11 }}>留空使用默认（aes-gcm）</span>}
          >
            <Select allowClear placeholder="默认 aes-gcm" style={{ width: '100%' }} options={[
              { label: '默认（aes-gcm）', value: '' },
              { label: 'AES-GCM', value: 'aes-gcm' },
              { label: 'AES-GCM-256', value: 'aes-gcm-256' },
              { label: 'ChaCha20', value: 'chacha20' },
              { label: 'XOR（最快，无安全性）', value: 'xor' },
              { label: 'OpenSSL AES-128-GCM', value: 'openssl-aes128-gcm' },
              { label: 'OpenSSL AES-256-GCM', value: 'openssl-aes256-gcm' },
              { label: 'OpenSSL ChaCha20', value: 'openssl-chacha20' },
            ]} />
          </Form.Item>
        </Col>
      </Row>

      <SectionTitle>流量控制</SectionTitle>
      <Row gutter={16}>
        <Col span={12}>
          <Form.Item name="foreign_relay_bps_limit" label="转发带宽限制（bps）"
            extra={<span style={{ fontSize: 11 }}>限制转发流量带宽，0 不限制</span>}
          >
            <InputNumber min={0} placeholder="0（不限制）" style={{ width: '100%' }} />
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item name="compression" label="压缩算法"
            extra={<span style={{ fontSize: 11 }}>默认不压缩</span>}
          >
            <Select allowClear placeholder="默认 none" style={{ width: '100%' }} options={[
              { label: '不压缩（none）', value: 'none' },
              { label: 'Zstd', value: 'zstd' },
            ]} />
          </Form.Item>
        </Col>
      </Row>
      <Row gutter={16}>
        <Col span={12}>
          <Form.Item name="tcp_whitelist" label="TCP 端口白名单"
            extra={<span style={{ fontSize: 11 }}>如 <code>80,8000-9000</code></span>}
          >
            <Input placeholder="80,443,8000-9000" style={{ width: '100%' }} />
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item name="udp_whitelist" label="UDP 端口白名单"
            extra={<span style={{ fontSize: 11 }}>如 <code>53,5000-6000</code></span>}
          >
            <Input placeholder="53,5000-6000" style={{ width: '100%' }} />
          </Form.Item>
        </Col>
      </Row>
    </>
  )

  // ===== Tab 6: 其他 =====
  const tabOther = (
    <>
      <SectionTitle>STUN 服务器</SectionTitle>
      <Form.Item
        name="stun_servers"
        label="IPv4 STUN 服务器"
        extra={<span style={{ fontSize: 11 }}>覆盖内置默认 STUN 列表，逗号分隔；留空使用默认，填写后为空则不使用 STUN</span>}
      >
        <Input placeholder="stun.l.google.com:19302,stun1.l.google.com:19302" style={{ width: '100%' }} />
      </Form.Item>
      <Form.Item
        name="stun_servers_v6"
        label="IPv6 STUN 服务器"
        extra={<span style={{ fontSize: 11 }}>覆盖内置默认 IPv6 STUN 列表，逗号分隔</span>}
      >
        <Input placeholder="留空使用默认" style={{ width: '100%' }} />
      </Form.Item>

      <SectionTitle>日志设置</SectionTitle>
      <Row gutter={16}>
        <Col span={12}>
          <Form.Item name="console_log_level" label="控制台日志级别">
            <Select allowClear placeholder="默认" style={{ width: '100%' }} options={[
              { label: 'trace', value: 'trace' },
              { label: 'debug', value: 'debug' },
              { label: 'info', value: 'info' },
              { label: 'warn', value: 'warn' },
              { label: 'error', value: 'error' },
              { label: 'off', value: 'off' },
            ]} />
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item name="file_log_level" label="文件日志级别">
            <Select allowClear placeholder="默认" style={{ width: '100%' }} options={[
              { label: 'trace', value: 'trace' },
              { label: 'debug', value: 'debug' },
              { label: 'info', value: 'info' },
              { label: 'warn', value: 'warn' },
              { label: 'error', value: 'error' },
              { label: 'off', value: 'off' },
            ]} />
          </Form.Item>
        </Col>
      </Row>
      <Row gutter={16}>
        <Col span={12}>
          <Form.Item name="file_log_dir" label="日志文件目录"
            extra={<span style={{ fontSize: 11 }}>留空不写入文件日志</span>}
          >
            <Input placeholder="如 /var/log/easytier" style={{ width: '100%' }} />
          </Form.Item>
        </Col>
        <Col span={6}>
          <Form.Item name="file_log_size" label="单文件大小（MB）"
            extra={<span style={{ fontSize: 11 }}>0 使用默认 100MB</span>}
          >
            <InputNumber min={0} placeholder="100" style={{ width: '100%' }} />
          </Form.Item>
        </Col>
        <Col span={6}>
          <Form.Item name="file_log_count" label="最大文件数量"
            extra={<span style={{ fontSize: 11 }}>0 使用默认 10</span>}
          >
            <InputNumber min={0} placeholder="10" style={{ width: '100%' }} />
          </Form.Item>
        </Col>
      </Row>

      <SectionTitle>运行时</SectionTitle>
      <Row gutter={16}>
        <Col span={12}>
          <Form.Item name="multi_thread_count" label="多线程数量"
            extra={<span style={{ fontSize: 11 }}>仅多线程模式有效，需大于 2，0 使用默认值 2</span>}
          >
            <InputNumber min={0} max={64} placeholder="0（默认2）" style={{ width: '100%' }} />
          </Form.Item>
        </Col>
      </Row>

      <SectionTitle>其他参数</SectionTitle>
      <Form.Item
        name="extra_args"
        label="额外命令行参数"
        extra={<span style={{ fontSize: 11 }}>其他不常用的参数，直接追加到命令行（兜底用）</span>}
      >
        <Input.TextArea rows={3} placeholder="--some-flag value" style={{ width: '100%' }} />
      </Form.Item>
    </>
  )

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Typography.Title level={4} style={{ margin: 0 }}>{t('easytier.serverTitle')}</Typography.Title>
        <Space>
          <Button icon={<ThunderboltOutlined />} onClick={handleQuickCreate} style={{ background: '#52c41a', borderColor: '#52c41a', color: '#fff' }}>快速创建</Button>
          <Button type="primary" icon={<PlusOutlined />} onClick={handleCreate}>{t('common.create')}</Button>
        </Space>
      </div>

      <Table
        dataSource={data} columns={columns} rowKey="id" loading={loading}
        size="middle" style={{ background: '#fff', borderRadius: 8 }}
        pagination={{ pageSize: 20, showSizeChanger: true }}
      />

      {/* 快速创建 Modal */}
      <Modal
        title={<Space><ThunderboltOutlined style={{ color: '#52c41a' }} />快速创建服务端</Space>}
        open={quickModalOpen}
        onOk={handleQuickSubmit}
        onCancel={() => setQuickModalOpen(false)}
        width={520}
        okText="创建"
        okButtonProps={{ style: { background: '#52c41a', borderColor: '#52c41a' } }}
        destroyOnHidden
      >
        <Form form={quickForm} layout="vertical" style={{ paddingTop: 8 }}>
          <Form.Item name="remark" label="备注（作为名称）">
            <Input placeholder="如：家庭服务器、公司节点（可选）" />
          </Form.Item>

          <Form.Item
            name="listen_addr"
            label="监听 IP"
            rules={[{ required: true, message: '请填写监听IP' }]}
            extra={<span style={{ fontSize: 11 }}>监听的网卡地址，0.0.0.0 表示所有网卡</span>}
          >
            <Input placeholder="0.0.0.0" style={{ width: 200 }} />
          </Form.Item>

          <Form.Item
            label="监听端口"
            required
            extra={<span style={{ fontSize: 11 }}>可添加多个协议/端口组合</span>}
          >
            <Form.List name="listeners" rules={[{
              validator: async (_, items) => {
                if (!items || items.length === 0) throw new Error('至少添加一个监听端口')
              }
            }]}>
              {(fields, { add, remove }, { errors }) => (
                <>
                  {fields.map(({ key, name, ...rest }) => (
                    <Row key={key} gutter={8} align="middle" style={{ marginBottom: 8 }}>
                      <Col span={8}>
                        <Form.Item {...rest} name={[name, 'proto']} style={{ marginBottom: 0 }}>
                          <Select options={PROTOCOL_OPTIONS} style={{ width: '100%' }} />
                        </Form.Item>
                      </Col>
                      <Col span={14}>
                        <Form.Item {...rest} name={[name, 'port']} style={{ marginBottom: 0 }} rules={[{ required: true, message: '请填写端口' }]}>
                          <Input placeholder="11010" style={{ width: '100%' }} />
                        </Form.Item>
                      </Col>
                      <Col span={2} style={{ display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                        {fields.length > 1 && (
                          <MinusCircleOutlined onClick={() => remove(name)} style={{ color: '#ff4d4f', fontSize: 16 }} />
                        )}
                      </Col>
                    </Row>
                  ))}
                  <Form.ErrorList errors={errors} />
                  <Button type="dashed" onClick={() => add({ proto: 'tcp', port: '' })} icon={<PlusOutlined />} block>添加端口</Button>
                </>
              )}
            </Form.List>
          </Form.Item>

          <Form.Item
            label="节点 IP"
            extra={<span style={{ fontSize: 11 }}>本节点在虚拟网络中的 IP 地址，留空则不创建 TUN 网卡（仅转发）</span>}
          >
            <Form.Item name="dhcp" valuePropName="checked" style={{ marginBottom: 6 }}>
              <Checkbox onChange={e => {
                if (e.target.checked) quickForm.setFieldValue('ipv4', '')
              }}>DHCP 自动分配（从 10.0.0.1 开始）</Checkbox>
            </Form.Item>
            <Form.Item noStyle shouldUpdate={(prev, cur) => prev.dhcp !== cur.dhcp}>
              {() => !quickForm.getFieldValue('dhcp') && (
                <Form.Item
                  name="ipv4"
                  style={{ marginBottom: 0 }}
                  rules={[{
                    pattern: /^(\d{1,3}\.){3}\d{1,3}(\/\d{1,2})?$/,
                    message: '格式如 10.0.0.1 或 10.0.0.1/24'
                  }]}
                >
                  <Input placeholder="如 10.0.0.1（可选，留空仅转发）" style={{ width: 280 }} />
                </Form.Item>
              )}
            </Form.Item>
          </Form.Item>

          <Row gutter={12}>
            <Col span={12}>
              <Form.Item name="network_name" label="网络名称" rules={[{ required: true, message: '请填写网络名称' }]}>
                <Input
                  placeholder="随机生成"
                  addonAfter={
                    <span
                      style={{ cursor: 'pointer', color: '#1677ff', fontSize: 12 }}
                      onClick={() => quickForm.setFieldValue('network_name', genNetworkName())}
                    >刷新</span>
                  }
                />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="network_password" label="网络密码" rules={[{ required: true, message: '请填写网络密码' }]}>
                <Input
                  placeholder="随机生成"
                  addonAfter={
                    <span
                      style={{ cursor: 'pointer', color: '#1677ff', fontSize: 12 }}
                      onClick={() => quickForm.setFieldValue('network_password', genNetworkPassword())}
                    >刷新</span>
                  }
                />
              </Form.Item>
            </Col>
          </Row>
          <Form.Item name="enable_relay" valuePropName="checked">
            <Checkbox>启用中转（允许所有网络通过本节点中转）</Checkbox>
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title={editRecord ? t('common.edit') : t('common.create')}
        open={modalOpen} onOk={handleSubmit} onCancel={() => setModalOpen(false)}
        width={780} destroyOnHidden
        styles={{ body: { padding: '4px 24px 0' } }}
      >
        <Form form={form} layout="vertical" style={{ paddingTop: 4 }}>
          <Tabs
            size="small"
            items={[
              { key: 'basic',    label: <span><SettingOutlined />     基本配置</span>, children: tabBasic },
              { key: 'listen',   label: <span><WifiOutlined />        监听端口</span>, disabled: serverMode === 'config-server', children: tabListen },
              { key: 'routing',  label: <span><GlobalOutlined />      路由转发</span>, disabled: serverMode === 'config-server', children: tabRouting },
              { key: 'network',  label: <span><ThunderboltOutlined /> 网络行为</span>, disabled: serverMode === 'config-server', children: tabNetwork },
              { key: 'security', label: <span><SafetyOutlined />      安全与流控</span>, disabled: serverMode === 'config-server', children: tabSecurity },
              { key: 'other',    label: <span><FileTextOutlined />    日志与其他</span>, children: tabOther },
            ]}
          />
        </Form>
      </Modal>
    </div>
  )
}

export default EasytierServer
