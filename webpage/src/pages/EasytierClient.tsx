import React, { useEffect, useRef, useState } from 'react'
import {
  Table, Button, Switch, Modal, Form, Input, InputNumber,
  Popconfirm, message, Typography, Tag, Tooltip, Row, Col,
  Checkbox, Select, Tabs, Alert, Space, Divider, Spin,
} from 'antd'
import {
  PlusOutlined, EditOutlined, DeleteOutlined,
  PlayCircleOutlined, StopOutlined, InfoCircleOutlined, MinusCircleOutlined,
  SettingOutlined, GlobalOutlined, LinkOutlined,
  SafetyOutlined, ApiOutlined, FileTextOutlined, ThunderboltOutlined,
  ReloadOutlined, NodeIndexOutlined, DownloadOutlined,
} from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { easytierClientApi } from '../api'
import { useTunnelApi } from '../contexts/TunnelApiContext'
import StatusTag from '../components/StatusTag'
import SectionTitle from '../components/SectionTitle'
import { CheckboxGrid, FieldRow } from '../components/ConfigFields'
import { SimpleList, AddrList, ProtoPortList } from '../components/FormListFields'
import { useTableStyle } from '../hooks/useTableStyle'
import {
  genRpcPort, parseAddrStr, serializeAddr, parseAddrList,
  parseListenPorts, parseSimpleList, parsePortForwards, randomStr,
} from '../utils/networkParse'

const { Text } = Typography

// 日志级别选项（控制台与文件日志共用）
const LOG_LEVEL_OPTIONS = [
  { label: 'trace', value: 'trace' },
  { label: 'debug', value: 'debug' },
  { label: 'info', value: 'info' },
  { label: 'warn', value: 'warn' },
  { label: 'error', value: 'error' },
  { label: 'off', value: 'off' },
]

const PROTOCOL_OPTIONS = [
  { label: 'TCP', value: 'tcp' },
  { label: 'UDP', value: 'udp' },
  { label: 'WS', value: 'ws' },
  { label: 'WSS', value: 'wss' },
  { label: 'WG', value: 'wg' },
  { label: 'QUIC', value: 'quic' },
]

// ---- 主组件 ----
const EasytierClient: React.FC = () => {
  const tunnelCtx = useTunnelApi()
  const tableStyle = useTableStyle()
  const api = tunnelCtx?.api || easytierClientApi
  const isRemote = tunnelCtx?.isRemoteMode || false
  const { t } = useTranslation()
  const [data, setData] = useState<any[]>([])
  const [loading, setLoading] = useState(false)
  const [modalOpen, setModalOpen] = useState(false)
  const [editRecord, setEditRecord] = useState<any>(null)
  const [form] = Form.useForm()

  // 快速创建
  const [quickModalOpen, setQuickModalOpen] = useState(false)
  const [quickForm] = Form.useForm()

  // 导入
  const [importing, setImporting] = useState(false)
  const importInputRef = React.useRef<HTMLInputElement>(null)

  // 导出所有配置为 JSON 文件
  const handleExport = () => {
    if (data.length === 0) { message.warning('暂无配置可导出'); return }
    const exportData = data.map(({ id, status, last_error, created_at, updated_at, ...rest }: any) => rest)
    const blob = new Blob([JSON.stringify(exportData, null, 2)], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `easytier-client-${new Date().toISOString().slice(0, 10)}.json`
    a.click()
    URL.revokeObjectURL(url)
    message.success(`已导出 ${exportData.length} 条配置`)
  }

  // 导入配置
  const handleImportFile = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    e.target.value = ''
    setImporting(true)
    try {
      const text = await file.text()
      const list = JSON.parse(text)
      if (!Array.isArray(list)) { message.error('文件格式错误，需为 JSON 数组'); return }
      let ok = 0, fail = 0
      for (const item of list) {
        try {
          await api.create({ ...item, enable: false, status: 'stopped' })
          ok++
        } catch { fail++ }
      }
      message.success(`导入完成：成功 ${ok} 条${fail > 0 ? `，失败 ${fail} 条` : ''}`)
      fetchData()
    } catch {
      message.error('文件解析失败，请确认为有效的 JSON 文件')
    } finally {
      setImporting(false)
    }
  }

  // 日志弹窗
  const [logModalOpen, setLogModalOpen] = useState(false)
  const [logRecord, setLogRecord] = useState<any>(null)
  const [logs, setLogs] = useState<string[]>([])
  const [logLoading, setLogLoading] = useState(false)
  const logEndRef = useRef<HTMLDivElement>(null)

  // 节点信息弹窗
  const [peersModalOpen, setPeersModalOpen] = useState(false)
  const [peersRecord, setPeersRecord] = useState<any>(null)
  const [peersInfo, setPeersInfo] = useState<any>(null)
  const [peersLoading, setPeersLoading] = useState(false)

  const fetchLogs = async (record: any) => {
    setLogLoading(true)
    try {
      const res: any = await api.getLogs(record.id)
      setLogs(res.data || [])
      setTimeout(() => logEndRef.current?.scrollIntoView({ behavior: 'smooth' }), 100)
    } catch (e) {
      setLogs([])
    } finally {
      setLogLoading(false)
    }
  }

  const handleShowLogs = (record: any) => {
    setLogRecord(record)
    setLogs([])
    setLogModalOpen(true)
    fetchLogs(record)
  }

  const fetchPeers = async (record: any) => {
    setPeersLoading(true)
    try {
      const res: any = await api.getPeers(record.id)
      setPeersInfo(res.data || null)
    } catch (e) {
      setPeersInfo(null)
    } finally {
      setPeersLoading(false)
    }
  }

  const handleShowPeers = (record: any) => {
    setPeersRecord(record)
    setPeersInfo(null)
    setPeersModalOpen(true)
    fetchPeers(record)
  }

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
    await api.create(payload)
    message.success('创建成功')
    setQuickModalOpen(false)
    fetchData()
  }

  const fetchData = async () => {
    setLoading(true)
    try {
      const res: any = await api.list()
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
      rpc_portal: genRpcPort(),
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
      await api.update(editRecord.id, values)
    } else {
      await api.create(values)
    }
    message.success(t('common.success'))
    setModalOpen(false)
    fetchData()
    tunnelCtx?.onRefresh?.()
  }

  const handleToggle = async (record: any, checked: boolean) => {
    await api.update(record.id, { ...record, enable: checked })
    checked ? await api.start(record.id) : await api.stop(record.id)
    fetchData()
  }

  const hasError = data.some(d => d.status === 'error' && d.last_error?.includes('not found'))
  const hasBinaryError = !isRemote && data.some(d =>
    d.status === 'error' && d.last_error && d.last_error.includes('easytier-core')
  )

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
            ? <Tooltip title={t('common.stop')}><Button size="small" icon={<StopOutlined />} onClick={async () => { await api.stop(r.id); fetchData() }} /></Tooltip>
            : <Tooltip title={t('common.start')}><Button size="small" type="primary" icon={<PlayCircleOutlined />} onClick={async () => { await api.start(r.id); fetchData() }} /></Tooltip>
          }
          {r.last_error && <Tooltip title={r.last_error}><Button size="small" icon={<InfoCircleOutlined />} danger /></Tooltip>}
          <Tooltip title={t('common.edit')}><Button size="small" icon={<EditOutlined />} onClick={() => handleEdit(r)} /></Tooltip>
          <Tooltip title="查看日志"><Button size="small" icon={<FileTextOutlined />} onClick={() => handleShowLogs(r)} /></Tooltip>
          <Tooltip title="节点信息"><Button size="small" icon={<NodeIndexOutlined />} onClick={() => handleShowPeers(r)} /></Tooltip>
          <Popconfirm title={t('common.deleteConfirm')} onConfirm={async () => { await api.delete(r.id); fetchData() }}>
            <Tooltip title={t('common.delete')}><Button size="small" danger icon={<DeleteOutlined />} /></Tooltip>
          </Popconfirm>
        </Space>
      ),
    },
  ]

  // ===== Tab 1: 基本配置 =====
  const tabBasic = (
    <>
      <FieldRow
        fields={[
          { name: 'name', label: '名称', span: 20, placeholder: '节点名称', rules: [{ required: true, message: '请填写名称' }] },
          { name: 'enable', label: '启用', type: 'switch', span: 4 },
        ]}
      />
      <FieldRow
        fields={[
          { name: 'network_name', label: '网络名称', placeholder: 'my-network', rules: [{ required: true, message: '请填写网络名称' }] },
          { name: 'network_password', label: '网络密码', type: 'password', placeholder: '留空不设密码' },
        ]}
      />
      <FieldRow
        fields={[
          { name: 'virtual_ip_addr', label: '虚拟 IPv4', span: 10, placeholder: '10.144.144.1', hint: <>如 <code>10.144.144.1</code>，DHCP 时无效</> },
          { name: 'virtual_ip_prefix', label: '前缀长度', type: 'number', span: 6, min: 0, max: 32, placeholder: '24', hint: '子网掩码位数' },
          { name: 'enable_dhcp', label: 'DHCP 自动分配', type: 'switch', span: 8, hint: '自动分配虚拟 IP，忽略上方 IP' },
        ]}
      />
      <FieldRow
        fields={[
          { name: 'ipv6', label: '虚拟 IPv6', placeholder: '可选，如 fd00::1', hint: '可与 IPv4 同时使用（双栈）' },
          { name: 'hostname', label: '主机名', placeholder: '自定义主机名（可选）', hint: '留空使用系统主机名' },
        ]}
      />
      <FieldRow
        fields={[
          { name: 'instance_name', label: '实例名称', placeholder: '可选，如 node1', hint: '同机多节点时用于区分，留空使用默认' },
        ]}
      />
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
        <ProtoPortList fieldName="listen_ports_list" addText="添加监听端口" />
      </Form.Item>

      <Form.Item
        label="映射监听器"
        extra={<span style={{ fontSize: 11 }}>NAT 后公告外部地址，让其他节点知道如何连接到本节点（--mapped-listeners）</span>}
      >
        <AddrList fieldName="mapped_listeners_list" addText="添加映射地址" defaultPort="11010" />
      </Form.Item>

      <SectionTitle>RPC 管理</SectionTitle>
      <FieldRow
        fields={[
          {
            name: 'rpc_portal', label: 'RPC 门户地址', placeholder: '0（随机端口）',
            hint: <>如 <code>0</code>（随机）、<code>15888</code>、<code>0.0.0.0:15888</code></>,
          },
          {
            name: 'rpc_portal_whitelist', label: 'RPC 白名单', placeholder: '127.0.0.1/32,::1/128',
            hint: <>如 <code>127.0.0.1/32,127.0.0.0/8</code></>,
          },
        ]}
      />
    </>
  )

  // ===== Tab 3: 路由与代理 =====
  const tabRouting = (
    <>
      <FieldRow
        fields={[
          {
            name: 'relay_network_whitelist', label: '中继网络白名单',
            placeholder: '留空不提供中继，填 * 允许所有',
            hint: <>允许为哪些网络提供中继，填 <code>*</code> 允许所有，留空不提供中继</>,
          },
        ]}
      />

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
      <FieldRow
        fields={[
          { name: 'dev_name', label: 'TUN 设备名', placeholder: 'tun0', hint: '留空使用默认（如 tun0）' },
          { name: 'mtu', label: 'MTU', type: 'number', min: 576, max: 9000, placeholder: '默认 1380' },
        ]}
      />
      <FieldRow
        fields={[
          { name: 'bind_device', label: '绑定物理设备', placeholder: '如 eth0（可选）', hint: '将套接字绑定到指定物理网卡，避免路由问题' },
        ]}
      />
    </>
  )

  // ===== Tab 功能开关 =====
  const tabFeatures = (
    <>
      <SectionTitle>网络模式</SectionTitle>
      <CheckboxGrid
        items={[
          { name: 'latency_first', label: '延迟优先模式', hint: '（--latency-first）' },
          { name: 'use_smoltcp', label: '用户态协议栈', hint: '（smoltcp）' },
          { name: 'disable_ipv6', label: '禁用 IPv6' },
          { name: 'accept_dns', label: '启用 Magic DNS', hint: '（--accept-dns）' },
          { name: 'proxy_forward_by_system', label: '系统内核转发子网代理', hint: '（--proxy-forward-by-system）' },
        ]}
      />
      <FieldRow
        fields={[
          { name: 'tld_dns_zone', label: 'Magic DNS 顶级域名', placeholder: 'et.net.', hint: '仅 accept-dns 启用时有效，默认 et.net.' },
          { name: 'default_protocol', label: '默认连接协议', type: 'select', allowClear: true, placeholder: '默认自动', options: PROTOCOL_OPTIONS, hint: '连接对等节点时使用的默认协议' },
        ]}
      />

      <SectionTitle>P2P 与打洞</SectionTitle>
      <CheckboxGrid
        items={[
          { name: 'disable_p2p', label: '禁用 P2P（强制中继）' },
          { name: 'p2p_only', label: '仅 P2P（禁用中继）' },
          { name: 'relay_all_peer_rpc', label: '中继所有对等 RPC' },
          { name: 'disable_tcp_hole_punching', label: '禁用 TCP 打洞' },
          { name: 'disable_udp_hole_punching', label: '禁用 UDP 打洞' },
          { name: 'disable_sym_hole_punching', label: '禁用对称 NAT 打洞', hint: '（防运营商封锁）' },
        ]}
      />

      <SectionTitle>协议加速</SectionTitle>
      <CheckboxGrid
        items={[
          { name: 'enable_kcp_proxy', label: '启用 KCP 加速', hint: '（提升 UDP 丢包网络性能）' },
          { name: 'disable_kcp_input', label: '禁止其他节点 KCP 代理到本节点' },
          { name: 'enable_quic_proxy', label: '启用 QUIC 加速', hint: '（提升 UDP 丢包网络性能）' },
          { name: 'disable_quic_input', label: '禁止其他节点 QUIC 代理到本节点' },
        ]}
      />
      <FieldRow
        fields={[
          { name: 'quic_listen_port', label: 'QUIC 监听端口', type: 'number', min: 0, max: 65535, placeholder: '0（随机）', hint: '0 为随机端口' },
        ]}
      />

      <SectionTitle>节点行为</SectionTitle>
      <CheckboxGrid
        items={[
          { name: 'no_tun', label: 'No TUN 模式', hint: '（无需 Npcap）' },
          { name: 'enable_exit_node', label: '允许作为出口节点' },
          { name: 'multi_thread', label: '多线程模式' },
          { name: 'disable_relay_kcp', label: '禁止转发 KCP 数据包', hint: '（防过度消耗流量）' },
          { name: 'enable_relay_foreign_network_kcp', label: '作为共享节点时转发其他网络 KCP' },
        ]}
      />
      <FieldRow
        fields={[
          { name: 'multi_thread_count', label: '多线程数量', type: 'number', min: 0, max: 64, placeholder: '0（默认2）', hint: '仅多线程模式有效，需大于 2，0 使用默认值 2' },
        ]}
      />
    </>
  )

  // ===== Tab 6: 其他（含安全与隐私） =====
  const tabOther = (
    <>
      <SectionTitle>安全</SectionTitle>
      <CheckboxGrid
        items={[
          { name: 'disable_encryption', label: <Text type="danger">禁用加密</Text>, hint: '（不推荐）' },
          { name: 'private_mode', label: '私有模式', hint: '（仅允许已知节点握手/中转）' },
        ]}
      />
      <FieldRow
        fields={[
          {
            name: 'encryption_algorithm', label: '加密算法', type: 'select', allowClear: true,
            placeholder: '默认 aes-gcm', hint: '留空使用默认（aes-gcm）',
            options: [
              { label: '默认（aes-gcm）', value: '' },
              { label: 'AES-GCM', value: 'aes-gcm' },
              { label: 'AES-GCM-256', value: 'aes-gcm-256' },
              { label: 'ChaCha20', value: 'chacha20' },
              { label: 'XOR（最快，无安全性）', value: 'xor' },
              { label: 'OpenSSL AES-128-GCM', value: 'openssl-aes128-gcm' },
              { label: 'OpenSSL AES-256-GCM', value: 'openssl-aes256-gcm' },
              { label: 'OpenSSL ChaCha20', value: 'openssl-chacha20' },
            ],
          },
        ]}
      />

      <SectionTitle>节点密钥对</SectionTitle>
      <Row gutter={16}>
        <Col span={24}>
          <Form.Item
            name="private_key"
            label="节点私钥"
            extra={<span style={{ fontSize: 11 }}>Base64 编码的 Ed25519 私钥（--private-key），用于节点身份认证；留空自动生成</span>}
          >
            <Input.Password
              placeholder="留空自动生成，或点击右侧按钮生成"
              style={{ width: '100%' }}
              addonAfter={
                <span
                  style={{ cursor: 'pointer', color: '#0071e3', fontSize: 12, userSelect: 'none' }}
                  onClick={() => {
                    // 生成 32 字节随机私钥并 Base64 编码
                    const bytes = new Uint8Array(32)
                    crypto.getRandomValues(bytes)
                    const b64 = btoa(String.fromCharCode(...bytes))
                    form.setFieldsValue({ private_key: b64, public_key: '' })
                  }}
                >生成</span>
              }
            />
          </Form.Item>
        </Col>
      </Row>
      <Row gutter={16}>
        <Col span={24}>
          <Form.Item
            name="public_key"
            label="节点公钥（仅展示）"
            extra={<span style={{ fontSize: 11 }}>公钥由私钥派生，此处仅供展示和复制，不会写入配置</span>}
          >
            <Input
              readOnly
              placeholder="填写私钥后此处展示对应公钥（需服务端支持派生）"
              style={{ width: '100%', background: '#fafafa' }}
            />
          </Form.Item>
        </Col>
      </Row>

      <SectionTitle>预共享密钥</SectionTitle>
      <Row gutter={16}>
        <Col span={24}>
          <Form.Item
            name="pre_shared_key"
            label="预共享密钥（PSK）"
            extra={<span style={{ fontSize: 11 }}>Base64 编码的预共享密钥（--pre-shared-key），同一网络所有节点需保持一致；留空不启用</span>}
          >
            <Input.Password
              placeholder="留空不启用，或点击右侧按钮随机生成"
              style={{ width: '100%' }}
              addonAfter={
                <span
                  style={{ cursor: 'pointer', color: '#0071e3', fontSize: 12, userSelect: 'none' }}
                  onClick={() => {
                    const bytes = new Uint8Array(32)
                    crypto.getRandomValues(bytes)
                    const b64 = btoa(String.fromCharCode(...bytes))
                    form.setFieldsValue({ pre_shared_key: b64 })
                  }}
                >随机生成</span>
              }
            />
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
      <FieldRow
        fields={[
          { name: 'console_log_level', label: '控制台日志级别', type: 'select', allowClear: true, placeholder: '默认', options: LOG_LEVEL_OPTIONS },
          { name: 'file_log_level', label: '文件日志级别', type: 'select', allowClear: true, placeholder: '默认', options: LOG_LEVEL_OPTIONS },
        ]}
      />
      <FieldRow
        fields={[
          { name: 'file_log_dir', label: '日志文件目录', placeholder: '如 /var/log/easytier', hint: '留空不写入文件日志' },
          { name: 'file_log_size', label: '单文件大小（MB）', type: 'number', min: 0, placeholder: '100', hint: '0 使用默认 100MB' },
          { name: 'file_log_count', label: '最大文件数量', type: 'number', min: 0, placeholder: '10', hint: '0 使用默认 10' },
        ]}
      />

      <SectionTitle>其他参数</SectionTitle>
      <FieldRow
        fields={[
          { name: 'extra_args', label: '额外命令行参数', type: 'textarea', rows: 3, placeholder: '--some-flag value', hint: '其他不常用的参数，直接追加到命令行（兜底用）' },
        ]}
      />
    </>
  )

  return (
    <div>
      {(hasError || hasBinaryError) && !isRemote && (
        <Alert
          message={t('easytier.binaryNotFound')}
          description={
            <span>
              请先下载 <code>easytier-core</code> 二进制文件，放置到程序目录的 <code>bin/</code> 文件夹下。
              <a
                href="https://github.com/EasyTier/EasyTier/releases"
                target="_blank"
                rel="noopener noreferrer"
                style={{ marginLeft: 8 }}
              >
                <LinkOutlined /> 前往 GitHub Releases 下载
              </a>
            </span>
          }
          type="warning" showIcon closable style={{ marginBottom: 16 }}
        />
      )}
      {/* 隐藏的文件导入输入框 */}
      <input
        ref={importInputRef}
        type="file"
        accept=".json"
        style={{ display: 'none' }}
        onChange={handleImportFile}
      />

      {!isRemote && (
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Typography.Title level={4} style={{ margin: 0 }}>{t('easytier.clientTitle')}</Typography.Title>
        <Space>
          <Button
            icon={<LinkOutlined />}
            href="https://github.com/EasyTier/EasyTier/releases"
            target="_blank"
            rel="noopener noreferrer"
          >
            EasyTier {t('common.officialSite')}
          </Button>
          <Button icon={<DownloadOutlined />} onClick={handleExport}>{t('easytier.exportConfig')}</Button>
          <Button icon={<PlusOutlined />} loading={importing} onClick={() => importInputRef.current?.click()}>{t('easytier.importConfig')}</Button>
          <Button icon={<ThunderboltOutlined />} onClick={handleQuickCreate} style={{ background: '#52c41a', borderColor: '#52c41a', color: '#fff' }}>{t('easytier.quickCreate')}</Button>
          <Button type="primary" icon={<PlusOutlined />} onClick={handleCreate}>{t('common.create')}</Button>
        </Space>
      </div>
      )}
      {isRemote && (
      <div style={{ display: 'flex', justifyContent: 'flex-end', marginBottom: 12 }}>
        <Space>
          <Button icon={<DownloadOutlined />} onClick={handleExport}>导出配置</Button>
          <Button icon={<PlusOutlined />} loading={importing} onClick={() => importInputRef.current?.click()}>导入配置</Button>
          <Button icon={<ThunderboltOutlined />} onClick={handleQuickCreate} style={{ background: '#52c41a', borderColor: '#52c41a', color: '#fff' }}>快速创建</Button>
          <Button type="primary" icon={<PlusOutlined />} onClick={handleCreate}>{t('common.create')}</Button>
        </Space>
      </div>
      )}

      <Table
        dataSource={data} columns={columns} rowKey="id" loading={loading}
        size="middle" style={tableStyle}
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
              { key: 'basic',      label: <span><SettingOutlined />  {t('easytier.tabBasic')}</span>, children: tabBasic },
              { key: 'connection', label: <span><LinkOutlined />     {t('easytier.tabConnection')}</span>, children: tabConnection },
              { key: 'routing',    label: <span><GlobalOutlined />   {t('easytier.tabRouting')}</span>, children: tabRouting },
              { key: 'features',   label: <span><ApiOutlined />      {t('easytier.tabFeatures')}</span>, children: tabFeatures },
              { key: 'other',      label: <span><SafetyOutlined />   {t('easytier.tabOther')}</span>, children: tabOther },
            ]}
          />
        </Form>
      </Modal>

      {/* 日志 Modal */}
      <Modal
        title={<Space><FileTextOutlined />{t('common.realtimeLogs')} - {logRecord?.name}</Space>}
        open={logModalOpen}
        onCancel={() => setLogModalOpen(false)}
        footer={[
          <Button key="refresh" icon={<ReloadOutlined />} onClick={() => logRecord && fetchLogs(logRecord)} loading={logLoading}>刷新</Button>,
          <Button key="close" onClick={() => setLogModalOpen(false)}>关闭</Button>,
        ]}
        width={860}
        destroyOnHidden
      >
        <Spin spinning={logLoading}>
          <div style={{
            background: '#1e1e1e', borderRadius: 6, padding: '10px 14px',
            height: 420, overflowY: 'auto', fontFamily: 'monospace', fontSize: 12,
          }}>
            {logs.length === 0
              ? <span style={{ color: '#888' }}>{logLoading ? '加载中...' : '暂无日志（进程未运行或尚无输出）'}</span>
              : logs.map((line, i) => (
                <div key={i} style={{
                  color: line.startsWith('[stderr]') ? '#ff7875' : '#d4d4d4',
                  lineHeight: '1.6',
                  wordBreak: 'break-all',
                }}>{line}</div>
              ))
            }
            <div ref={logEndRef} />
          </div>
        </Spin>
      </Modal>

      {/* 节点信息 Modal */}
      <Modal
        title={<Space><NodeIndexOutlined />节点信息 - {peersRecord?.name}</Space>}
        open={peersModalOpen}
        onCancel={() => setPeersModalOpen(false)}
        footer={[
          <Button key="refresh" icon={<ReloadOutlined />} onClick={() => peersRecord && fetchPeers(peersRecord)} loading={peersLoading}>刷新</Button>,
          <Button key="close" onClick={() => setPeersModalOpen(false)}>关闭</Button>,
        ]}
        width={900}
        destroyOnHidden
      >
        <Spin spinning={peersLoading}>
          {!peersInfo && !peersLoading && (
            <Alert type="warning" message="无法获取节点信息，请确认已配置 RPC 门户地址且节点正在运行" style={{ marginBottom: 12 }} />
          )}
          {peersInfo && (
            <>
              <div style={{ marginBottom: 8, fontWeight: 600, color: '#595959' }}>对等节点（Peers）</div>
              <Table
                dataSource={peersInfo.peers || []}
                rowKey={(r: any, i: any) => r.id || i}
                size="small"
                pagination={false}
                style={{ marginBottom: 16 }}
                locale={{ emptyText: '暂无对等节点' }}
                columns={[
                  { title: '虚拟 IP', dataIndex: 'ipv4', width: 140, render: (v: string) => <Text code style={{ fontSize: 11 }}>{v || '-'}</Text> },
                  { title: '主机名', dataIndex: 'hostname', width: 130 },
                  { title: '延迟', dataIndex: 'latency', width: 90, render: (v: string) => v ? <Tag color="green">{v}</Tag> : '-' },
                  { title: '协议', dataIndex: 'tunnel_proto', width: 80, render: (v: string) => v ? <Tag color="blue">{v}</Tag> : '-' },
                  { title: 'NAT 类型', dataIndex: 'nat_type', width: 100 },
                  { title: '发送', dataIndex: 'tx_bytes', width: 90 },
                  { title: '接收', dataIndex: 'rx_bytes', width: 90 },
                  { title: '跳数', dataIndex: 'cost', width: 70 },
                ]}
              />
              <div style={{ marginBottom: 8, fontWeight: 600, color: '#595959' }}>路由表（Routes）</div>
              <Table
                dataSource={peersInfo.routes || []}
                rowKey={(r: any, i: any) => r.ipv4 || i}
                size="small"
                pagination={false}
                locale={{ emptyText: '暂无路由' }}
                columns={[
                  { title: '虚拟 IP', dataIndex: 'ipv4', width: 140, render: (v: string) => <Text code style={{ fontSize: 11 }}>{v || '-'}</Text> },
                  { title: '主机名', dataIndex: 'hostname', width: 130 },
                  { title: '代理网段', dataIndex: 'proxy_cidrs', width: 150 },
                  { title: '下一跳', dataIndex: 'next_hop_ipv4', width: 140 },
                  { title: '跳数', dataIndex: 'cost', width: 70 },
                ]}
              />
            </>
          )}
        </Spin>
      </Modal>
    </div>
  )
}

export default EasytierClient
