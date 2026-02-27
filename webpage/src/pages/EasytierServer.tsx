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
  GlobalOutlined, ApiOutlined,
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
  const [form] = Form.useForm()

  const fetchData = async () => {
    setLoading(true)
    try {
      const res: any = await easytierServerApi.list()
      setData(res.data || [])
    } finally { setLoading(false) }
  }
  useEffect(() => { fetchData() }, [])

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
        <Form.Item
          name="config_server_addr"
          label="Config-Server 地址"
          rules={[{ required: true, message: '请填写 config-server 地址' }]}
          extra={<span style={{ fontSize: 11 }}>格式：<code>tcp://host:port</code>，如 <code>tcp://1.2.3.4:11010</code></span>}
        >
          <Input placeholder="tcp://config-server:11010" style={{ width: '100%' }} />
        </Form.Item>
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
          </Row>
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
          <Form.Item name="enable_quic_proxy" valuePropName="checked">
            <Checkbox>启用 QUIC 加速 <Text type="secondary" style={{ fontSize: 11 }}>（--enable-quic-proxy）</Text></Checkbox>
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
          <Form.Item name="enable_private_mode" valuePropName="checked">
            <Checkbox>私有模式 <Text type="secondary" style={{ fontSize: 11 }}>（仅允许已知节点）</Text></Checkbox>
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
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Typography.Title level={4} style={{ margin: 0 }}>{t('easytier.serverTitle')}</Typography.Title>
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
              { key: 'basic',    label: <span><SettingOutlined />     基本配置</span>, children: tabBasic },
              { key: 'listen',   label: <span><WifiOutlined />        监听端口</span>, disabled: serverMode === 'config-server', children: tabListen },
              { key: 'routing',  label: <span><GlobalOutlined />      路由转发</span>, disabled: serverMode === 'config-server', children: tabRouting },
              { key: 'network',  label: <span><ThunderboltOutlined /> 网络行为</span>, disabled: serverMode === 'config-server', children: tabNetwork },
              { key: 'security', label: <span><SafetyOutlined />      安全</span>,     disabled: serverMode === 'config-server', children: tabSecurity },
              { key: 'other',    label: <span><ApiOutlined />         其他</span>,     children: tabOther },
            ]}
          />
        </Form>
      </Modal>
    </div>
  )
}

export default EasytierServer
