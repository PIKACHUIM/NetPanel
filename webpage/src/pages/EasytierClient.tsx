import React, { useEffect, useState } from 'react'
import {
  Table, Button, Switch, Modal, Form, Input, InputNumber,
  Popconfirm, message, Typography, Tag, Tooltip, Row, Col,
  Checkbox, Select, Tabs, Alert, Space,
} from 'antd'
import {
  PlusOutlined, EditOutlined, DeleteOutlined,
  PlayCircleOutlined, StopOutlined, InfoCircleOutlined, MinusCircleOutlined,
  SettingOutlined, GlobalOutlined, ThunderboltOutlined, LinkOutlined,
  SafetyOutlined, ApiOutlined,
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

// ---- 主组件 ----
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
      server_addr_list: [{ proto: 'tcp', host: '', port: '11010' }],
      listen_ports_list: [],
      mapped_listeners_list: [],
      exit_nodes_list: [],
      proxy_cidrs_list: [],
      manual_routes_list: [],
      port_forwards_list: [],
    })
    setModalOpen(true)
  }

  const handleEdit = (record: any) => {
    setEditRecord(record)
    form.setFieldsValue({
      ...record,
      server_addr_list: parseAddrList(record.server_addr),
      listen_ports_list: parseListenPorts(record.listen_ports),
      mapped_listeners_list: parseAddrList(record.mapped_listeners || '').filter((i: any) => i.host),
      exit_nodes_list: parseSimpleList(record.exit_nodes),
      proxy_cidrs_list: parseSimpleList(record.proxy_cidrs),
      manual_routes_list: parseSimpleList(record.manual_routes),
      port_forwards_list: parsePortForwards(record.port_forwards),
    })
    setModalOpen(true)
  }

  const handleSubmit = async () => {
    const values = await form.validateFields()
    values.server_addr = (values.server_addr_list || []).map(serializeAddr).filter(Boolean).join(',')
    delete values.server_addr_list
    values.listen_ports = (values.listen_ports_list || []).map((i: any) => i.proto && i.port ? `${i.proto}:${i.port}` : '').filter(Boolean).join(',')
    delete values.listen_ports_list
    values.mapped_listeners = (values.mapped_listeners_list || []).map(serializeAddr).filter(Boolean).join(',')
    delete values.mapped_listeners_list
    values.exit_nodes = (values.exit_nodes_list || []).map((i: any) => i.value).filter(Boolean).join(',')
    delete values.exit_nodes_list
    values.proxy_cidrs = (values.proxy_cidrs_list || []).map((i: any) => i.value).filter(Boolean).join(',')
    delete values.proxy_cidrs_list
    values.manual_routes = (values.manual_routes_list || []).map((i: any) => i.value).filter(Boolean).join(',')
    delete values.manual_routes_list
    values.port_forwards = (values.port_forwards_list || [])
      .filter((i: any) => i.listen_port && i.target_ip && i.target_port)
      .map((i: any) => `${i.proto}:${i.listen_ip}:${i.listen_port}:${i.target_ip}:${i.target_port}`)
      .join('\n')
    delete values.port_forwards_list
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
      title: t('easytier.serverAddr'), dataIndex: 'server_addr',
      render: (v: string) => <Text code style={{ fontSize: 12 }}>{v}</Text>,
    },
    {
      title: t('easytier.networkName'), dataIndex: 'network_name',
      render: (v: string) => <Tag color="blue">{v}</Tag>,
    },
    {
      title: t('easytier.virtualIP'), dataIndex: 'virtual_ip',
      render: (v: string, r: any) => {
        if (r.enable_dhcp) return <Tag color="purple">DHCP</Tag>
        return v ? <Text code style={{ color: '#52c41a', fontSize: 12 }}>{v}</Text> : <Text type="secondary">自动</Text>
      },
    },
    {
      title: '选项',
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
        <Col span={16}>
          <Form.Item name="name" label="名称" rules={[{ required: true, message: '请填写名称' }]}>
            <Input placeholder="节点名称" style={{ width: '100%' }} />
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
        <Col span={14}>
          <Form.Item
            name="virtual_ip"
            label="虚拟 IP"
            extra={<span style={{ fontSize: 11 }}>格式：<code>10.144.144.1/24</code>，启用 DHCP 时此项无效</span>}
          >
            <Input placeholder="留空自动分配" style={{ width: '100%' }} />
          </Form.Item>
        </Col>
        <Col span={10}>
          <Form.Item name="enable_dhcp" label="DHCP" valuePropName="checked" extra={<span style={{ fontSize: 11 }}>自动分配虚拟 IP</span>}>
            <Switch />
          </Form.Item>
        </Col>
      </Row>
      <Row gutter={16}>
        <Col span={12}>
          <Form.Item name="hostname" label="主机名" extra={<span style={{ fontSize: 11 }}>留空使用系统主机名</span>}>
            <Input placeholder="自定义主机名（可选）" style={{ width: '100%' }} />
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
        extra={<span style={{ fontSize: 11 }}>连接到 EasyTier 服务端或公共节点，可添加多个</span>}
      >
        <AddrList fieldName="server_addr_list" addText="添加服务器地址" defaultPort="11010" />
      </Form.Item>

      <SectionTitle>本地监听（可选）</SectionTitle>
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
        extra={<span style={{ fontSize: 11 }}>NAT 后公告外部地址，让其他节点知道如何连接到本节点</span>}
      >
        <AddrList fieldName="mapped_listeners_list" addText="添加映射地址" defaultPort="11010" />
      </Form.Item>
    </>
  )

  // ===== Tab 3: 路由与代理 =====
  const tabRouting = (
    <>
      <SectionTitle>路由行为</SectionTitle>
      <Row gutter={[16, 0]}>
        <Col span={12}>
          <Form.Item name="latency_first" valuePropName="checked">
            <Checkbox>延迟优先路由 <Text type="secondary" style={{ fontSize: 11 }}>（--latency-first）</Text></Checkbox>
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item name="enable_exit_node" valuePropName="checked">
            <Checkbox>允许作为出口 <Text type="secondary" style={{ fontSize: 11 }}>（--enable-exit-node）</Text></Checkbox>
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item name="disable_p2p" valuePropName="checked">
            <Checkbox>强制中继模式 <Text type="secondary" style={{ fontSize: 11 }}></Text></Checkbox>
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item name="p2p_only" valuePropName="checked">
            <Checkbox>禁用中转模式<Text type="secondary" style={{ fontSize: 11 }}>（--p2p-only，禁用中继）</Text></Checkbox>
          </Form.Item>
        </Col>

        <Col span={12}>
          <Form.Item name="relay_all_peer_rpc" valuePropName="checked">
            <Checkbox>中继所有对等 RPC <Text type="secondary" style={{ fontSize: 11 }}>（--relay-all-peer-rpc）</Text></Checkbox>
          </Form.Item>
        </Col>
      </Row>

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
    </>
  )

  // ===== Tab 4: 打洞与加速 =====
  const tabTunnel = (
    <>
      <SectionTitle>打洞选项</SectionTitle>
      <Row gutter={[16, 0]}>
        <Col span={12}>
          <Form.Item name="disable_udp_hole_punching" valuePropName="checked">
            <Checkbox>禁用 UDP 打洞</Checkbox>
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item name="disable_tcp_hole_punching" valuePropName="checked">
            <Checkbox>禁用 TCP 打洞</Checkbox>
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item name="disable_sym_hole_punching" valuePropName="checked">
            <Checkbox>禁用对称 NAT 打洞</Checkbox>
          </Form.Item>
        </Col>
      </Row>

      <SectionTitle>协议加速</SectionTitle>
      <Row gutter={[16, 0]}>
        <Col span={12}>
          <Form.Item name="enable_kcp_proxy" valuePropName="checked">
            <Checkbox>启用 KCP 加速 <Text type="secondary" style={{ fontSize: 11 }}>（--enable-kcp-proxy）</Text></Checkbox>
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item name="enable_quic_proxy" valuePropName="checked">
            <Checkbox>启用 QUIC 加速 <Text type="secondary" style={{ fontSize: 11 }}>（--enable-quic-proxy）</Text></Checkbox>
          </Form.Item>
        </Col>
      </Row>

      <SectionTitle>TUN / 网卡</SectionTitle>
      <Row gutter={16}>
        <Col span={12}>
          <Form.Item name="no_tun" valuePropName="checked">
            <Checkbox>不创建 TUN 网卡 <Text type="secondary" style={{ fontSize: 11 }}>（--no-tun，无需 Npcap）</Text></Checkbox>
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item name="use_smoltcp" valuePropName="checked">
            <Checkbox>使用 smoltcp 协议栈 <Text type="secondary" style={{ fontSize: 11 }}>（用户态网络）</Text></Checkbox>
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item name="disable_ipv6" valuePropName="checked">
            <Checkbox>禁用 IPv6 <Text type="secondary" style={{ fontSize: 11 }}>（--disable-ipv6）</Text></Checkbox>
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item name="enable_magic_dns" valuePropName="checked">
            <Checkbox>启用 Magic DNS</Checkbox>
          </Form.Item>
        </Col>
      </Row>
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
    </>
  )

  // ===== Tab 5: 安全与隐私 =====
  const tabSecurity = (
    <>
      <SectionTitle>安全选项</SectionTitle>
      <Row gutter={[16, 0]}>
        <Col span={12}>
          <Form.Item name="disable_encryption" valuePropName="checked">
            <Checkbox><Text type="danger">禁用加密</Text> <Text type="secondary" style={{ fontSize: 11 }}>（不推荐）</Text></Checkbox>
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item name="enable_private_mode" valuePropName="checked">
            <Checkbox>私有模式 <Text type="secondary" style={{ fontSize: 11 }}>（仅允许已知节点）</Text></Checkbox>
          </Form.Item>
        </Col>
      </Row>

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
    </>
  )

  // ===== Tab 6: 其他 =====
  const tabOther = (
    <>
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
        width={780} destroyOnClose
        styles={{ body: { padding: '4px 24px 0' } }}
      >
        <Form form={form} layout="vertical" style={{ paddingTop: 4 }}>
          <Tabs
            size="small"
            items={[
              { key: 'basic',      label: <span><SettingOutlined />  基本配置</span>, children: tabBasic },
              { key: 'connection', label: <span><LinkOutlined />     连接设置</span>, children: tabConnection },
              { key: 'routing',    label: <span><GlobalOutlined />   路由与代理</span>, children: tabRouting },
              { key: 'tunnel',     label: <span><ThunderboltOutlined /> 打洞与加速</span>, children: tabTunnel },
              { key: 'security',   label: <span><SafetyOutlined />   安全与隐私</span>, children: tabSecurity },
              { key: 'other',      label: <span><ApiOutlined />      其他</span>, children: tabOther },
            ]}
          />
        </Form>
      </Modal>
    </div>
  )
}

export default EasytierClient
