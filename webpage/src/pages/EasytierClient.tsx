import React, { useEffect, useState } from 'react'
import {
  Table, Button, Switch, Modal, Form, Input, InputNumber,
  Popconfirm, message, Typography, Tag, Tooltip, Row, Col,
  Checkbox, Select, Tabs, Alert, Space, Divider,
} from 'antd'
import {
  PlusOutlined, EditOutlined, DeleteOutlined,
  PlayCircleOutlined, StopOutlined, InfoCircleOutlined, MinusCircleOutlined,
  SettingOutlined, GlobalOutlined, LinkOutlined,
  SafetyOutlined, ApiOutlined, FileTextOutlined, ThunderboltOutlined,
} from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { easytierClientApi } from '../api'
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

// ---- 数据解析工具 ----
const parseAddrStr = (s: string): { proto: string; host: string; port: string } => {
  s = s.trim()
  const m = s.match(/^(\w+):\/\/(.+):(\d+)$/)
  if (m) return { proto: m[1], host: m[2], port: m[3] }
  const parts = s.split(':')
  if (parts.length === 3) return { proto: parts[0], host: parts[1], port: parts[2] }
  if (parts.length === 2) return { proto: 'tcp', host: parts[0], port: parts[1] }
  return { proto: 'tcp', host: s, port: '' }
}
const serializeAddr = (item: { proto: string; host: string; port: string }): string => {
  if (!item?.host) return ''
  return `${item.proto || 'tcp'}://${item.host}:${item.port || ''}`
}
const parseAddrList = (str: string) => {
  if (!str) return [{ proto: 'tcp', host: '', port: '' }]
  return str.split(',').map(s => parseAddrStr(s)).filter(i => i.host)
}
const parseListenPorts = (s: string) => {
  if (!s) return []
  return s.split(',').map(p => {
    p = p.trim()
    if (p.includes(':')) { const [proto, port] = p.split(':'); return { proto, port } }
    return { proto: 'tcp', port: p }
  }).filter(Boolean)
}
const parseSimpleList = (str: string): Array<{ value: string }> => {
  if (!str) return []
  return str.split(',').map(s => s.trim()).filter(Boolean).map(s => ({ value: s }))
}
const parsePortForwards = (str: string) => {
  if (!str) return []
  return str.split('\n').map(s => s.trim()).filter(Boolean).map(s => {
    const p = s.split(':')
    if (p.length >= 5) return { proto: p[0], listen_ip: p[1], listen_port: p[2], target_ip: p[3], target_port: p[4] }
    return { proto: 'tcp', listen_ip: '0.0.0.0', listen_port: '', target_ip: '', target_port: '' }
  })
}

// ---- 通用子组件 ----
const SimpleList = ({ fieldName, placeholder, addText }: { fieldName: string; placeholder: string; addText: string }) => (
  <Form.List name={fieldName}>
    {(fields, { add, remove }) => (
      <>
        {fields.map(({ key, name, ...rest }) => (
          <Row key={key} gutter={8} align="middle" style={{ marginBottom: 8 }}>
            <Col flex="auto">
              <Form.Item {...rest} name={[name, 'value']} style={{ marginBottom: 0 }} rules={[{ required: true, message: '请填写' }]}>
                <Input placeholder={placeholder} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
            <Col flex="none" style={{ display: 'flex', alignItems: 'center' }}>
              <MinusCircleOutlined onClick={() => remove(name)} style={{ color: '#ff4d4f', fontSize: 16 }} />
            </Col>
          </Row>
        ))}
        <Button type="dashed" onClick={() => add({ value: '' })} icon={<PlusOutlined />} block>{addText}</Button>
      </>
    )}
  </Form.List>
)

const AddrList = ({ fieldName, addText, defaultPort }: { fieldName: string; addText: string; defaultPort?: string }) => (
  <Form.List name={fieldName}>
    {(fields, { add, remove }) => (
      <>
        {fields.map(({ key, name, ...rest }) => (
          <Row key={key} gutter={8} align="middle" style={{ marginBottom: 8 }}>
            <Col span={5}>
              <Form.Item {...rest} name={[name, 'proto']} style={{ marginBottom: 0 }}>
                <Select options={PROTOCOL_OPTIONS} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
            <Col span={13}>
              <Form.Item {...rest} name={[name, 'host']} style={{ marginBottom: 0 }} rules={[{ required: true, message: '请填写地址' }]}>
                <Input placeholder="IP 或域名" style={{ width: '100%' }} />
              </Form.Item>
            </Col>
            <Col span={5}>
              <Form.Item {...rest} name={[name, 'port']} style={{ marginBottom: 0 }} rules={[{ required: true, message: '端口' }]}>
                <Input placeholder="端口" style={{ width: '100%' }} />
              </Form.Item>
            </Col>
            <Col span={1} style={{ display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
              <MinusCircleOutlined onClick={() => remove(name)} style={{ color: '#ff4d4f', fontSize: 16 }} />
            </Col>
          </Row>
        ))}
        <Button type="dashed" onClick={() => add({ proto: 'tcp', host: '', port: defaultPort || '' })} icon={<PlusOutlined />} block>{addText}</Button>
      </>
    )}
  </Form.List>
)

// ---- 随机字符串工具 ----
const randomStr = (len: number, chars = 'abcdefghijklmnopqrstuvwxyz0123456789') =>
  Array.from({ length: len }, () => chars[Math.floor(Math.random() * chars.length)]).join('')

// ---- 主组件 ----
const EasytierClient: React.FC = () => {
  const { t } = useTranslation()
  const [data, setData] = useState<any[]>([])
  const [loading, setLoading] = useState(false)
  const [modalOpen, setModalOpen] = useState(false)
  const [editRecord, setEditRecord] = useState<any>(null)
  const [form] = Form.useForm()

  // 快速创建
  const [quickModalOpen, setQuickModalOpen] = useState(false)
  const [quickForm] = Form.useForm()

  const handleQuickCreate = () => {
    quickForm.resetFields()
    quickForm.setFieldsValue({
      proto: 'tcp',
      port: '11010',
    })
    setQuickModalOpen(true)
  }

  const handleQuickSubmit = async () => {
    const values = await quickForm.validateFields()
    const { proto, host, port, network_name, network_password, remark } = values
    const payload: any = {
      name: network_name,
      enable: true,
      network_name,
      network_password: network_password || '',
      server_addr: `${proto}://${host}:${port}`,
      remark: remark || '',
    }
    await easytierClientApi.create(payload)
    message.success('创建成功')
    setQuickModalOpen(false)
    fetchData()
  }

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
      virtual_ip_addr: '',
      virtual_ip_prefix: 24,
      server_addr_list: [{ proto: 'tcp', host: '', port: '11010' }],
      listen_ports_list: [],
      mapped_listeners_list: [],
      exit_nodes_list: [],
      external_nodes_list: [],
      proxy_cidrs_list: [],
      manual_routes_list: [],
      port_forwards_list: [],
      stun_servers_list: [],
      stun_servers_v6_list: [],
    })
    setModalOpen(true)
  }

  const handleEdit = (record: any) => {
    setEditRecord(record)
    // 拆分 virtual_ip（如 10.144.144.1/24）
    const [vipAddr, vipPrefix] = (record.virtual_ip || '').split('/')
    form.setFieldsValue({
      ...record,
      virtual_ip_addr: vipAddr || '',
      virtual_ip_prefix: vipPrefix ? parseInt(vipPrefix) : 24,
      server_addr_list: parseAddrList(record.server_addr),
      listen_ports_list: parseListenPorts(record.listen_ports),
      mapped_listeners_list: parseAddrList(record.mapped_listeners || '').filter((i: any) => i.host),
      exit_nodes_list: parseSimpleList(record.exit_nodes),
      external_nodes_list: parseSimpleList(record.external_nodes),
      proxy_cidrs_list: parseSimpleList(record.proxy_cidrs),
      manual_routes_list: parseSimpleList(record.manual_routes),
      port_forwards_list: parsePortForwards(record.port_forwards),
      stun_servers_list: parseSimpleList(record.stun_servers),
      stun_servers_v6_list: parseSimpleList(record.stun_servers_v6),
    })
    setModalOpen(true)
  }

  const handleSubmit = async () => {
    const values = await form.validateFields()
    // 合并 virtual_ip
    const vipAddr = (values.virtual_ip_addr || '').trim()
    const vipPrefix = values.virtual_ip_prefix ?? 24
    values.virtual_ip = vipAddr ? `${vipAddr}/${vipPrefix}` : ''
    delete values.virtual_ip_addr
    delete values.virtual_ip_prefix
    values.server_addr = (values.server_addr_list || []).map(serializeAddr).filter(Boolean).join(',')
    delete values.server_addr_list
    values.listen_ports = (values.listen_ports_list || []).map((i: any) => i.proto && i.port ? `${i.proto}:${i.port}` : '').filter(Boolean).join(',')
    delete values.listen_ports_list
    values.mapped_listeners = (values.mapped_listeners_list || []).map(serializeAddr).filter(Boolean).join(',')
    delete values.mapped_listeners_list
    values.exit_nodes = (values.exit_nodes_list || []).map((i: any) => i.value).filter(Boolean).join(',')
    delete values.exit_nodes_list
    values.external_nodes = (values.external_nodes_list || []).map((i: any) => i.value).filter(Boolean).join(',')
    delete values.external_nodes_list
    values.proxy_cidrs = (values.proxy_cidrs_list || []).map((i: any) => i.value).filter(Boolean).join(',')
    delete values.proxy_cidrs_list
    values.manual_routes = (values.manual_routes_list || []).map((i: any) => i.value).filter(Boolean).join(',')
    delete values.manual_routes_list
    values.port_forwards = (values.port_forwards_list || [])
      .filter((i: any) => i.listen_port && i.target_ip && i.target_port)
      .map((i: any) => `${i.proto}:${i.listen_ip}:${i.listen_port}:${i.target_ip}:${i.target_port}`)
      .join('\n')
    delete values.port_forwards_list
    values.stun_servers = (values.stun_servers_list || []).map((i: any) => i.value).filter(Boolean).join(',')
    delete values.stun_servers_list
    values.stun_servers_v6 = (values.stun_servers_v6_list || []).map((i: any) => i.value).filter(Boolean).join(',')
    delete values.stun_servers_v6_list
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

  const hasError = data.some(d => d.status === 'error' && d.last_error?.includes('not found'))

  const columns = [
    { title: t('common.status'), dataIndex: 'status', width: 80, render: (s: string) => <StatusTag status={s} /> },
    {
      title: t('common.enable'), dataIndex: 'enable', width: 70,
      render: (v: boolean, r: any) => <Switch size="small" checked={v} onChange={c => handleToggle(r, c)} />,
    },
    {
      title: t('common.name'), dataIndex: 'name', width: 160,
      render: (name: string, r: any) => (
        <div>
          <Text strong>{name}</Text>
          {r.remark && <div><Text type="secondary" style={{ fontSize: 11 }}>{r.remark}</Text></div>}
        </div>
      ),
    },
    {
      title: '主机名', dataIndex: 'hostname', width: 130,
      render: (v: string) => v ? <Text code style={{ fontSize: 12 }}>{v}</Text> : <Text type="secondary" style={{ fontSize: 11 }}>系统默认</Text>,
    },
    {
      title: '虚拟 IP', dataIndex: 'virtual_ip', width: 150,
      render: (v: string, r: any) => {
        if (r.enable_dhcp) return <Tag color="purple" style={{ margin: 0 }}>DHCP 自动</Tag>
        return v
          ? <Text code style={{ color: '#52c41a', fontSize: 12 }}>{v}</Text>
          : <Text type="secondary" style={{ fontSize: 11 }}>未设置</Text>
      },
    },
    {
      title: '网络', dataIndex: 'network_name', width: 120,
      render: (v: string) => <Tag color="blue">{v}</Tag>,
    },
    {
      title: t('easytier.serverAddr'), dataIndex: 'server_addr',
      render: (v: string) => <Text code style={{ fontSize: 11 }}>{v}</Text>,
    },
    {
      title: '选项', width: 200,
      render: (_: any, r: any) => (
        <Space size={4} wrap>
          {r.no_tun && <Tag color="orange">no-tun</Tag>}
          {r.disable_p2p && <Tag color="red">no-p2p</Tag>}
          {r.p2p_only && <Tag color="red">p2p-only</Tag>}
          {r.latency_first && <Tag color="gold">延迟优先</Tag>}
          {r.enable_exit_node && <Tag color="volcano">出口节点</Tag>}
          {r.enable_vpn_portal && <Tag color="purple">VPN门户</Tag>}
          {r.enable_socks5 && <Tag color="cyan">SOCKS5</Tag>}
        </Space>
      ),
    },
    {
      title: t('common.action'), width: 160,
      render: (_: any, r: any) => (
        <Space size={4}>
          {r.status === 'running'
            ? <Tooltip title={t('common.stop')}><Button size="small" icon={<StopOutlined />} onClick={async () => { await easytierClientApi.stop(r.id); fetchData() }} /></Tooltip>
            : <Tooltip title={t('common.start')}><Button size="small" type="primary" icon={<PlayCircleOutlined />} onClick={async () => { await easytierClientApi.start(r.id); fetchData() }} /></Tooltip>
          }
          {r.last_error && <Tooltip title={r.last_error}><Button size="small" icon={<InfoCircleOutlined />} danger /></Tooltip>}
          <Tooltip title={t('common.edit')}><Button size="small" icon={<EditOutlined />} onClick={() => handleEdit(r)} /></Tooltip>
          <Popconfirm title={t('common.deleteConfirm')} onConfirm={async () => { await easytierClientApi.delete(r.id); fetchData() }}>
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
        <Col span={20}>
          <Form.Item name="name" label="名称" rules={[{ required: true, message: '请填写名称' }]}>
            <Input placeholder="节点名称" style={{ width: '100%' }} />
          </Form.Item>
        </Col>
        <Col span={4}>
          <Form.Item name="enable" label="启用" valuePropName="checked">
            <Switch />
          </Form.Item>
        </Col>
      </Row>
      <Row gutter={16}>
        <Col span={12}>
          <Form.Item name="network_name" label="网络名称" rules={[{ required: true, message: '请填写网络名称' }]}>
            <Input placeholder="my-network" style={{ width: '100%' }} />
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item name="network_password" label="网络密码">
            <Input.Password placeholder="留空不设密码" style={{ width: '100%' }} />
          </Form.Item>
        </Col>
      </Row>
      <Row gutter={16}>
        <Col span={10}>
          <Form.Item
            name="virtual_ip_addr"
            label="虚拟 IPv4"
            extra={<span style={{ fontSize: 11 }}>如 <code>10.144.144.1</code>，DHCP 时无效</span>}
          >
            <Input placeholder="10.144.144.1" style={{ width: '100%' }} />
          </Form.Item>
        </Col>
        <Col span={6}>
          <Form.Item
            name="virtual_ip_prefix"
            label="前缀长度"
            extra={<span style={{ fontSize: 11 }}>子网掩码位数</span>}
          >
            <InputNumber min={0} max={32} placeholder="24" style={{ width: '100%' }} />
          </Form.Item>
        </Col>
        <Col span={8}>
          <Form.Item name="enable_dhcp" label="DHCP 自动分配" valuePropName="checked" extra={<span style={{ fontSize: 11 }}>自动分配虚拟 IP，忽略上方 IP</span>}>
            <Switch />
          </Form.Item>
        </Col>
      </Row>
      <Row gutter={16}>
        <Col span={12}>
          <Form.Item name="ipv6" label="虚拟 IPv6" extra={<span style={{ fontSize: 11 }}>可与 IPv4 同时使用（双栈）</span>}>
            <Input placeholder="可选，如 fd00::1" style={{ width: '100%' }} />
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item name="hostname" label="主机名" extra={<span style={{ fontSize: 11 }}>留空使用系统主机名</span>}>
            <Input placeholder="自定义主机名（可选）" style={{ width: '100%' }} />
          </Form.Item>
        </Col>
      </Row>
      <Row gutter={16}>
        <Col span={12}>
          <Form.Item name="instance_name" label="实例名称" extra={<span style={{ fontSize: 11 }}>同机多节点时用于区分，留空使用默认</span>}>
            <Input placeholder="可选，如 node1" style={{ width: '100%' }} />
          </Form.Item>
        </Col>
      </Row>
      <Form.Item name="remark" label="备注">
        <Input.TextArea rows={2} placeholder="备注（可选）" style={{ width: '100%' }} />
      </Form.Item>
    </>
  )

  // ===== Tab 2: 连接设置 =====
  const tabConnection = (
    <>
      <Form.Item
        label="服务器地址"
        required
        extra={<span style={{ fontSize: 11 }}>连接到 EasyTier 服务端或公共节点（--peers），可添加多个</span>}
      >
        <AddrList fieldName="server_addr_list" addText="添加服务器地址" defaultPort="11010" />
      </Form.Item>

      <Form.Item
        label="公共共享节点"
        extra={<span style={{ fontSize: 11 }}>使用公共共享节点发现对等节点（--external-node），可添加多个</span>}
      >
        <SimpleList fieldName="external_nodes_list" placeholder="tcp://public.easytier.top:11010" addText="添加公共节点" />
      </Form.Item>

      <SectionTitle>本地监听（可选）</SectionTitle>
      <Row gutter={16}>
        <Col span={24}>
          <Form.Item name="no_listener" valuePropName="checked" style={{ marginBottom: 8 }}
            extra={<span style={{ fontSize: 11 }}>启用后不监听任何端口，只主动连接对等节点</span>}
          >
            <Checkbox>不监听端口（--no-listener）</Checkbox>
          </Form.Item>
        </Col>
      </Row>
      <Form.Item
        label="监听端口"
        extra={<span style={{ fontSize: 11 }}>本节点对外监听，让其他节点主动连接到本节点</span>}
      >
        <Form.List name="listen_ports_list">
          {(fields, { add, remove }) => (
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
                    <MinusCircleOutlined onClick={() => remove(name)} style={{ color: '#ff4d4f', fontSize: 16 }} />
                  </Col>
                </Row>
              ))}
              <Button type="dashed" onClick={() => add({ proto: 'tcp', port: '' })} icon={<PlusOutlined />} block>添加监听端口</Button>
            </>
          )}
        </Form.List>
      </Form.Item>

      <Form.Item
        label="映射监听器"
        extra={<span style={{ fontSize: 11 }}>NAT 后公告外部地址，让其他节点知道如何连接到本节点（--mapped-listeners）</span>}
      >
        <AddrList fieldName="mapped_listeners_list" addText="添加映射地址" defaultPort="11010" />
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

  // ===== Tab 3: 路由与代理 =====
  const tabRouting = (
    <>
      <Form.Item
        name="relay_network_whitelist"
        label="中继网络白名单"
        extra={<span style={{ fontSize: 11 }}>允许为哪些网络提供中继，填 <code>*</code> 允许所有，留空不提供中继</span>}
      >
        <Input placeholder="留空不提供中继，填 * 允许所有" style={{ width: '100%' }} />
      </Form.Item>

      <SectionTitle>出口节点</SectionTitle>
      <Form.Item extra={<span style={{ fontSize: 11 }}>使用指定节点的 IP 作为出口，如 <code>10.0.0.1</code></span>}>
        <SimpleList fieldName="exit_nodes_list" placeholder="10.0.0.1" addText="添加出口节点" />
      </Form.Item>

      <SectionTitle>子网代理</SectionTitle>
      <Form.Item extra={<span style={{ fontSize: 11 }}>将本机子网共享给虚拟网络，格式：<code>192.168.1.0/24</code></span>}>
        <SimpleList fieldName="proxy_cidrs_list" placeholder="192.168.1.0/24" addText="添加子网" />
      </Form.Item>

      <SectionTitle>手动路由</SectionTitle>
      <Row gutter={[16, 0]}>
        <Col span={24}>
          <Form.Item name="enable_manual_routes" valuePropName="checked">
            <Checkbox>启用手动路由 <Text type="secondary" style={{ fontSize: 11 }}>（--manual-routes，覆盖自动路由）</Text></Checkbox>
          </Form.Item>
        </Col>
      </Row>
      <Form.Item extra={<span style={{ fontSize: 11 }}>每条一个 CIDR，如 <code>10.0.0.0/24</code></span>}>
        <SimpleList fieldName="manual_routes_list" placeholder="10.0.0.0/24" addText="添加路由" />
      </Form.Item>

      <SectionTitle>端口转发</SectionTitle>
      <Form.List name="port_forwards_list">
        {(fields, { add, remove }) => (
          <>
            {fields.map(({ key, name, ...rest }) => (
              <Row key={key} gutter={6} align="middle" style={{ marginBottom: 8 }}>
                <Col span={4}>
                  <Form.Item {...rest} name={[name, 'proto']} style={{ marginBottom: 0 }}>
                    <Select options={[{ label: 'TCP', value: 'tcp' }, { label: 'UDP', value: 'udp' }]} style={{ width: '100%' }} />
                  </Form.Item>
                </Col>
                <Col span={6}>
                  <Form.Item {...rest} name={[name, 'listen_ip']} style={{ marginBottom: 0 }}>
                    <Input placeholder="监听IP" style={{ width: '100%' }} />
                  </Form.Item>
                </Col>
                <Col span={3}>
                  <Form.Item {...rest} name={[name, 'listen_port']} style={{ marginBottom: 0 }} rules={[{ required: true, message: '端口' }]}>
                    <Input placeholder="端口" style={{ width: '100%' }} />
                  </Form.Item>
                </Col>
                <Col span={1} style={{ textAlign: 'center' }}>
                  <Text type="secondary" style={{ fontSize: 12 }}>→</Text>
                </Col>
                <Col span={6}>
                  <Form.Item {...rest} name={[name, 'target_ip']} style={{ marginBottom: 0 }} rules={[{ required: true, message: '目标IP' }]}>
                    <Input placeholder="目标IP" style={{ width: '100%' }} />
                  </Form.Item>
                </Col>
                <Col span={3}>
                  <Form.Item {...rest} name={[name, 'target_port']} style={{ marginBottom: 0 }} rules={[{ required: true, message: '端口' }]}>
                    <Input placeholder="端口" style={{ width: '100%' }} />
                  </Form.Item>
                </Col>
                <Col span={1} style={{ display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                  <MinusCircleOutlined onClick={() => remove(name)} style={{ color: '#ff4d4f', fontSize: 16 }} />
                </Col>
              </Row>
            ))}
            <Button type="dashed" onClick={() => add({ proto: 'tcp', listen_ip: '0.0.0.0', listen_port: '', target_ip: '', target_port: '' })} icon={<PlusOutlined />} block>
              添加转发规则
            </Button>
          </>
        )}
      </Form.List>

      <SectionTitle>TUN / 网卡</SectionTitle>
      <Row gutter={16}>
        <Col span={12}>
          <Form.Item name="dev_name" label="TUN 设备名" extra={<span style={{ fontSize: 11 }}>留空使用默认（如 tun0）</span>}>
            <Input placeholder="tun0" style={{ width: '100%' }} />
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item name="mtu" label="MTU">
            <InputNumber min={576} max={9000} placeholder="默认 1380" style={{ width: '100%' }} />
          </Form.Item>
        </Col>
      </Row>
      <Row gutter={16}>
        <Col span={12}>
          <Form.Item name="bind_device" label="绑定物理设备"
            extra={<span style={{ fontSize: 11 }}>将套接字绑定到指定物理网卡，避免路由问题</span>}
          >
            <Input placeholder="如 eth0（可选）" style={{ width: '100%' }} />
          </Form.Item>
        </Col>
      </Row>
    </>
  )

  // ===== Tab 功能开关 =====
  const tabFeatures = (
    <>
      <SectionTitle>网络模式</SectionTitle>
      <Row gutter={[0, 4]}>
        <Col span={12}>
          <Form.Item name="latency_first" valuePropName="checked" style={{ marginBottom: 4 }}>
            <Checkbox>延迟优先模式 <Text type="secondary" style={{ fontSize: 11 }}>（--latency-first）</Text></Checkbox>
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item name="use_smoltcp" valuePropName="checked" style={{ marginBottom: 4 }}>
            <Checkbox>用户态协议栈 <Text type="secondary" style={{ fontSize: 11 }}>（smoltcp）</Text></Checkbox>
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item name="disable_ipv6" valuePropName="checked" style={{ marginBottom: 4 }}>
            <Checkbox>禁用 IPv6</Checkbox>
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item name="accept_dns" valuePropName="checked" style={{ marginBottom: 4 }}>
            <Checkbox>启用 Magic DNS <Text type="secondary" style={{ fontSize: 11 }}>（--accept-dns）</Text></Checkbox>
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item name="proxy_forward_by_system" valuePropName="checked" style={{ marginBottom: 4 }}>
            <Checkbox>系统内核转发子网代理 <Text type="secondary" style={{ fontSize: 11 }}>（--proxy-forward-by-system）</Text></Checkbox>
          </Form.Item>
        </Col>
      </Row>
      <Row gutter={16} style={{ marginTop: 4 }}>
        <Col span={12}>
          <Form.Item name="tld_dns_zone" label="Magic DNS 顶级域名"
            extra={<span style={{ fontSize: 11 }}>仅 accept-dns 启用时有效，默认 et.net.</span>}
          >
            <Input placeholder="et.net." style={{ width: '100%' }} />
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item name="default_protocol" label="默认连接协议"
            extra={<span style={{ fontSize: 11 }}>连接对等节点时使用的默认协议</span>}
          >
            <Select allowClear placeholder="默认自动" options={PROTOCOL_OPTIONS} style={{ width: '100%' }} />
          </Form.Item>
        </Col>
      </Row>

      <SectionTitle>P2P 与打洞</SectionTitle>
      <Row gutter={[0, 4]}>
        <Col span={12}>
          <Form.Item name="disable_p2p" valuePropName="checked" style={{ marginBottom: 4 }}>
            <Checkbox>禁用 P2P（强制中继）</Checkbox>
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item name="p2p_only" valuePropName="checked" style={{ marginBottom: 4 }}>
            <Checkbox>仅 P2P（禁用中继）</Checkbox>
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item name="relay_all_peer_rpc" valuePropName="checked" style={{ marginBottom: 4 }}>
            <Checkbox>中继所有对等 RPC</Checkbox>
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item name="disable_tcp_hole_punching" valuePropName="checked" style={{ marginBottom: 4 }}>
            <Checkbox>禁用 TCP 打洞</Checkbox>
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item name="disable_udp_hole_punching" valuePropName="checked" style={{ marginBottom: 4 }}>
            <Checkbox>禁用 UDP 打洞</Checkbox>
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item name="disable_sym_hole_punching" valuePropName="checked" style={{ marginBottom: 4 }}>
            <Checkbox>禁用对称 NAT 打洞 <Text type="secondary" style={{ fontSize: 11 }}>（防运营商封锁）</Text></Checkbox>
          </Form.Item>
        </Col>
      </Row>

      <SectionTitle>协议加速</SectionTitle>
      <Row gutter={[0, 4]}>
        <Col span={12}>
          <Form.Item name="enable_kcp_proxy" valuePropName="checked" style={{ marginBottom: 4 }}>
            <Checkbox>启用 KCP 加速 <Text type="secondary" style={{ fontSize: 11 }}>（提升 UDP 丢包网络性能）</Text></Checkbox>
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item name="disable_kcp_input" valuePropName="checked" style={{ marginBottom: 4 }}>
            <Checkbox>禁止其他节点 KCP 代理到本节点</Checkbox>
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item name="enable_quic_proxy" valuePropName="checked" style={{ marginBottom: 4 }}>
            <Checkbox>启用 QUIC 加速 <Text type="secondary" style={{ fontSize: 11 }}>（提升 UDP 丢包网络性能）</Text></Checkbox>
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item name="disable_quic_input" valuePropName="checked" style={{ marginBottom: 4 }}>
            <Checkbox>禁止其他节点 QUIC 代理到本节点</Checkbox>
          </Form.Item>
        </Col>
      </Row>
      <Row gutter={16} style={{ marginTop: 4 }}>
        <Col span={12}>
          <Form.Item name="quic_listen_port" label="QUIC 监听端口"
            extra={<span style={{ fontSize: 11 }}>0 为随机端口</span>}
          >
            <InputNumber min={0} max={65535} placeholder="0（随机）" style={{ width: '100%' }} />
          </Form.Item>
        </Col>
      </Row>

      <SectionTitle>节点行为</SectionTitle>
      <Row gutter={[0, 4]}>
        <Col span={12}>
          <Form.Item name="no_tun" valuePropName="checked" style={{ marginBottom: 4 }}>
            <Checkbox>No TUN 模式 <Text type="secondary" style={{ fontSize: 11 }}>（无需 Npcap）</Text></Checkbox>
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item name="enable_exit_node" valuePropName="checked" style={{ marginBottom: 4 }}>
            <Checkbox>允许作为出口节点</Checkbox>
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item name="multi_thread" valuePropName="checked" style={{ marginBottom: 4 }}>
            <Checkbox>多线程模式</Checkbox>
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item name="disable_relay_kcp" valuePropName="checked" style={{ marginBottom: 4 }}>
            <Checkbox>禁止转发 KCP 数据包 <Text type="secondary" style={{ fontSize: 11 }}>（防过度消耗流量）</Text></Checkbox>
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item name="enable_relay_foreign_network_kcp" valuePropName="checked" style={{ marginBottom: 4 }}>
            <Checkbox>作为共享节点时转发其他网络 KCP</Checkbox>
          </Form.Item>
        </Col>
      </Row>
      <Row gutter={16} style={{ marginTop: 4 }}>
        <Col span={12}>
          <Form.Item name="multi_thread_count" label="多线程数量"
            extra={<span style={{ fontSize: 11 }}>仅多线程模式有效，需大于 2，0 使用默认值 2</span>}
          >
            <InputNumber min={0} max={64} placeholder="0（默认2）" style={{ width: '100%' }} />
          </Form.Item>
        </Col>
      </Row>
    </>
  )

  // ===== Tab 6: 其他（含安全与隐私） =====
  const tabOther = (
    <>
      <SectionTitle>安全</SectionTitle>
      <Row gutter={[0, 4]}>
        <Col span={12}>
          <Form.Item name="disable_encryption" valuePropName="checked" style={{ marginBottom: 4 }}>
            <Checkbox><Text type="danger">禁用加密</Text> <Text type="secondary" style={{ fontSize: 11 }}>（不推荐）</Text></Checkbox>
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item name="private_mode" valuePropName="checked" style={{ marginBottom: 4 }}>
            <Checkbox>私有模式 <Text type="secondary" style={{ fontSize: 11 }}>（仅允许已知节点握手/中转）</Text></Checkbox>
          </Form.Item>
        </Col>
      </Row>
      <Row gutter={16} style={{ marginTop: 4 }}>
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

      <SectionTitle>中继流量控制</SectionTitle>
      <Row gutter={16}>
        <Col span={12}>
          <Form.Item name="relay_network_whitelist" label="中继网络白名单"
            extra={<span style={{ fontSize: 11 }}>允许为哪些网络提供中继，填 <code>*</code> 允许所有，留空不提供中继</span>}
          >
            <Input placeholder="留空不提供中继，填 * 允许所有" style={{ width: '100%' }} />
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item name="foreign_relay_bps_limit" label="转发带宽限制（bps）"
            extra={<span style={{ fontSize: 11 }}>限制转发流量带宽，0 不限制</span>}
          >
            <InputNumber min={0} placeholder="0（不限制）" style={{ width: '100%' }} />
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
      <Row gutter={16}>
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

      <SectionTitle>STUN 服务器</SectionTitle>
      <Form.Item
        label="IPv4 STUN 服务器"
        extra={<span style={{ fontSize: 11 }}>覆盖内置默认 STUN 列表，留空使用默认，填写后为空则不使用 STUN</span>}
      >
        <SimpleList fieldName="stun_servers_list" placeholder="stun.l.google.com:19302" addText="添加 STUN 服务器" />
      </Form.Item>
      <Form.Item
        label="IPv6 STUN 服务器"
        extra={<span style={{ fontSize: 11 }}>覆盖内置默认 IPv6 STUN 列表</span>}
      >
        <SimpleList fieldName="stun_servers_v6_list" placeholder="stun.l.google.com:19302" addText="添加 IPv6 STUN 服务器" />
      </Form.Item>

      <SectionTitle>WireGuard VPN 门户</SectionTitle>
      <Row gutter={[16, 0]}>
        <Col span={24}>
          <Form.Item name="enable_vpn_portal" valuePropName="checked">
            <Checkbox>启用 VPN 门户 <Text type="secondary" style={{ fontSize: 11 }}>（允许 WireGuard 客户端接入虚拟网络）</Text></Checkbox>
          </Form.Item>
        </Col>
      </Row>
      <Row gutter={16}>
        <Col span={8}>
          <Form.Item name="vpn_portal_listen_port" label="WG 监听端口">
            <InputNumber min={1} max={65535} placeholder="11013" style={{ width: '100%' }} />
          </Form.Item>
        </Col>
        <Col span={16}>
          <Form.Item
            name="vpn_portal_client_network"
            label="VPN 客户端网段"
            extra={<span style={{ fontSize: 11 }}>分配给 WireGuard 客户端的网段，如 <code>10.14.14.0/24</code></span>}
          >
            <Input placeholder="10.14.14.0/24" style={{ width: '100%' }} />
          </Form.Item>
        </Col>
      </Row>

      <SectionTitle>SOCKS5 代理</SectionTitle>
      <Row gutter={[16, 0]}>
        <Col span={24}>
          <Form.Item name="enable_socks5" valuePropName="checked">
            <Checkbox>启用 SOCKS5 代理 <Text type="secondary" style={{ fontSize: 11 }}>（通过虚拟网络代理流量）</Text></Checkbox>
          </Form.Item>
        </Col>
      </Row>
      <Row gutter={16}>
        <Col span={8}>
          <Form.Item name="socks5_port" label="SOCKS5 端口">
            <InputNumber min={1} max={65535} placeholder="1080" style={{ width: '100%' }} />
          </Form.Item>
        </Col>
      </Row>

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
      {hasError && (
        <Alert
          message={t('easytier.binaryNotFound')}
          description="请前往 GitHub Releases 下载对应平台的 easytier-core 二进制文件，放置到程序目录的 bin/ 文件夹下。"
          type="warning" showIcon closable style={{ marginBottom: 16 }}
        />
      )}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Typography.Title level={4} style={{ margin: 0 }}>{t('easytier.clientTitle')}</Typography.Title>
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
        title={<Space><ThunderboltOutlined style={{ color: '#52c41a' }} />快速创建客户端</Space>}
        open={quickModalOpen}
        onOk={handleQuickSubmit}
        onCancel={() => setQuickModalOpen(false)}
        okText="创建"
        cancelText="取消"
        okButtonProps={{ style: { background: '#52c41a', borderColor: '#52c41a' } }}
        width={480}
        destroyOnHidden
      >
        <Form form={quickForm} layout="vertical" style={{ paddingTop: 8 }}>
          <Form.Item
            name="network_name"
            label="网络名称"
            rules={[{ required: true, message: '请填写网络名称' }]}
            extra={<span style={{ fontSize: 11 }}>需与服务端网络名称一致</span>}
          >
            <Input placeholder="my-network" style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item
            name="network_password"
            label="网络密码"
            extra={<span style={{ fontSize: 11 }}>需与服务端网络密码一致，留空不设密码</span>}
          >
            <Input.Password placeholder="留空不设密码" style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item label="服务器地址" required style={{ marginBottom: 0 }}>
            <Space.Compact style={{ width: '100%' }}>
              <Form.Item name="proto" noStyle>
                <Select style={{ width: 90 }} options={PROTOCOL_OPTIONS} />
              </Form.Item>
              <Form.Item
                name="host"
                noStyle
                rules={[{ required: true, message: '请填写服务器地址' }]}
              >
                <Input placeholder="服务器 IP 或域名" style={{ flex: 1 }} />
              </Form.Item>
              <Form.Item
                name="port"
                noStyle
                rules={[{ required: true, message: '端口' }]}
              >
                <Input placeholder="端口" style={{ width: 90 }} />
              </Form.Item>
            </Space.Compact>
          </Form.Item>
          <Form.Item name="remark" label="备注" style={{ marginTop: 16 }}>
            <Input placeholder="备注（可选）" style={{ width: '100%' }} />
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
              { key: 'basic',      label: <span><SettingOutlined />  基本配置</span>, children: tabBasic },
              { key: 'connection', label: <span><LinkOutlined />     连接设置</span>, children: tabConnection },
              { key: 'routing',    label: <span><GlobalOutlined />   路由与代理</span>, children: tabRouting },
              { key: 'features',   label: <span><ApiOutlined />      功能开关</span>, children: tabFeatures },
              { key: 'other',      label: <span><SafetyOutlined />   安全与其他</span>, children: tabOther },
            ]}
          />
        </Form>
      </Modal>
    </div>
  )
}

export default EasytierClient
