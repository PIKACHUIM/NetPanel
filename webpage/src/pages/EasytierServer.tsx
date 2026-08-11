import React, { useEffect, useRef, useState } from 'react'
import {
  Table, Button, Space, Switch, Modal, Form, Input,
  Popconfirm, message, Typography, Tag, Tooltip, Row, Col,
  Checkbox, Radio, Select, Tabs, InputNumber, Spin, Alert,
} from 'antd'
import {
  PlusOutlined, EditOutlined, DeleteOutlined,
  PlayCircleOutlined, StopOutlined, InfoCircleOutlined, MinusCircleOutlined,
  SettingOutlined, WifiOutlined, SafetyOutlined, ThunderboltOutlined,
  GlobalOutlined, ApiOutlined, FileTextOutlined,
  ReloadOutlined, NodeIndexOutlined, DownloadOutlined, LinkOutlined,
} from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { easytierServerApi } from '../api'
import { useTunnelApi } from '../contexts/TunnelApiContext'
import StatusTag from '../components/StatusTag'
import { useTableStyle } from '../hooks/useTableStyle'

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
// 随机生成 RPC 门户端口（15000~25000，避免与常用端口冲突）
const genRpcPort = () => String(Math.floor(Math.random() * 10000) + 15000)

const parseListenPorts = (s: string): string[] => {
  if (!s) return []
  return s.split(',').map(p => p.trim()).filter(Boolean)
}
const joinListenPorts = (ports: string[]): string => (ports || []).filter(Boolean).join(',')

const EasytierServer: React.FC = () => {
  const tunnelCtx = useTunnelApi()
  const tableStyle = useTableStyle()
  const api = tunnelCtx?.api || easytierServerApi
  const isRemote = tunnelCtx?.isRemoteMode || false
  const { t } = useTranslation()
  const [data, setData] = useState<any[]>([])
  const [loading, setLoading] = useState(false)
  const [modalOpen, setModalOpen] = useState(false)
  const [editRecord, setEditRecord] = useState<any>(null)
  const [serverMode, setServerMode] = useState<string>('standalone')
  const [quickModalOpen, setQuickModalOpen] = useState(false)
  const [quickForm] = Form.useForm()
  const [form] = Form.useForm()

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
    a.download = `easytier-server-${new Date().toISOString().slice(0, 10)}.json`
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

  const fetchData = async () => {
    setLoading(true)
    try {
      const res: any = await api.list()
      setData(res.data || [])
    } finally { setLoading(false) }
  }
  useEffect(() => { fetchData() }, [])

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
    await api.create(payload)
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
      rpc_portal: genRpcPort(),
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

  // 检测是否有二进制不存在的错误
  const hasBinaryError = !isRemote && data.some(d =>
    d.status === 'error' && d.last_error && d.last_error.includes('easytier-core')
  )

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
            extra={<span style={{ fontSize: 11 }}>格式：<code>tcp://host:port</code>，如 <code>tcp://1.2.3.4:11010</code>；使用官方服务器填 <code>tcp://public.easytier.cn:11010</code></span>}
          >
            <Input placeholder="tcp://config-server:11010" style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item
            name="config_server_token"
            label="Token（用户名）"
            rules={[{ required: true, message: '请填写 Token，不能为空' }]}
            extra={<span style={{ fontSize: 11 }}>Config-Server 认证 token（即用户名），将拼接到地址末尾，如 <code>tcp://host:port/<b>your_token</b></code></span>}
          >
            <Input placeholder="your_token" style={{ width: '100%' }} />
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

  // ===== Tab 4: 网络与安全 =====
  const tabNetworkSecurity = (
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
                  style={{ cursor: 'pointer', color: '#1677ff', fontSize: 12, userSelect: 'none' }}
                  onClick={() => {
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
                  style={{ cursor: 'pointer', color: '#1677ff', fontSize: 12, userSelect: 'none' }}
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
      {hasBinaryError && (
        <Alert
          message={t('easytier.binaryNotFound')}
          description={
            <span>
              {t('easytier.binaryNotFoundTip')}
              <a
                href="https://github.com/EasyTier/EasyTier/releases"
                target="_blank"
                rel="noopener noreferrer"
                style={{ marginLeft: 8 }}
              >
                <LinkOutlined /> {t('easytier.downloadFromGithub')}
              </a>
            </span>
          }
          type="warning"
          showIcon
          closable
          style={{ marginBottom: 16 }}
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
        <Typography.Title level={4} style={{ margin: 0 }}>{t('easytier.serverTitle')}</Typography.Title>
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
              { key: 'network',  label: <span><SafetyOutlined /> 网络与安全</span>, disabled: serverMode === 'config-server', children: tabNetworkSecurity },
              { key: 'other',    label: <span><FileTextOutlined />    日志与其他</span>, children: tabOther },
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

export default EasytierServer
