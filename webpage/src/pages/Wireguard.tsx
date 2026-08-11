import React, { useEffect, useState } from 'react'
import {
  Table, Button, Space, Switch, Modal, Form, Input, InputNumber,
  Popconfirm, message, Typography, Tooltip, Row, Col, Tabs, Tag, Divider, Image,
} from 'antd'
import {
  PlusOutlined, EditOutlined, DeleteOutlined,
  PlayCircleOutlined, StopOutlined, SettingOutlined,
  SafetyOutlined, KeyOutlined, CopyOutlined, ReloadOutlined, TeamOutlined,
  DownloadOutlined, QrcodeOutlined,
} from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { wireguardApi } from '../api'
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

const Wireguard: React.FC = () => {
  const { t } = useTranslation()
  const tableStyle = useTableStyle()
  const [data, setData] = useState<any[]>([])
  const [loading, setLoading] = useState(false)
  const [modalOpen, setModalOpen] = useState(false)
  const [editRecord, setEditRecord] = useState<any>(null)
  const [form] = Form.useForm()

  // 对等节点管理
  const [peerModalOpen, setPeerModalOpen] = useState(false)
  const [peerEditRecord, setPeerEditRecord] = useState<any>(null)
  const [peerForm] = Form.useForm()
  const [peers, setPeers] = useState<any[]>([])
  const [peersLoading, setPeersLoading] = useState(false)
  const [currentWgId, setCurrentWgId] = useState<number | null>(null)
  const [peerListModalOpen, setPeerListModalOpen] = useState(false)
  const [currentWgName, setCurrentWgName] = useState('')

  // 二维码弹窗
  const [qrModalOpen, setQrModalOpen] = useState(false)
  const [qrImageUrl, setQrImageUrl] = useState('')
  const [qrPeerName, setQrPeerName] = useState('')

  const fetchData = async () => {
    setLoading(true)
    try {
      const res: any = await wireguardApi.list()
      setData(res.data || [])
    } finally { setLoading(false) }
  }

  useEffect(() => { fetchData() }, [])

  const handleCreate = async () => {
    setEditRecord(null)
    form.resetFields()
    // 自动生成密钥对
    try {
      const res: any = await wireguardApi.generateKeyPair()
      form.setFieldsValue({
        enable: true,
        listen_port: 51820,
        mtu: 1420,
        private_key: res.data?.private_key || '',
        public_key: res.data?.public_key || '',
      })
    } catch {
      form.setFieldsValue({
        enable: true,
        listen_port: 51820,
        mtu: 1420,
      })
    }
    setModalOpen(true)
  }

  const handleEdit = (record: any) => {
    setEditRecord(record)
    form.setFieldsValue(record)
    setCurrentWgId(record.id)
    setCurrentWgName(record.name)
    fetchPeers(record.id)
    setModalOpen(true)
  }

  const handleSubmit = async () => {
    const values = await form.validateFields()
    if (editRecord) {
      await wireguardApi.update(editRecord.id, values)
    } else {
      await wireguardApi.create(values)
    }
    message.success(t('common.success'))
    setModalOpen(false)
    fetchData()
  }

  const handleToggle = async (record: any, checked: boolean) => {
    await wireguardApi.update(record.id, { ...record, enable: checked })
    checked ? await wireguardApi.start(record.id) : await wireguardApi.stop(record.id)
    fetchData()
  }

  const handleGenerateKey = async () => {
    try {
      const res: any = await wireguardApi.generateKeyPair()
      form.setFieldsValue({
        private_key: res.data?.private_key || '',
        public_key: res.data?.public_key || '',
      })
      message.success(t('wireguard.keyGenerated'))
    } catch {
      message.error(t('wireguard.keyGenerateFailed'))
    }
  }

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text)
    message.success(t('common.copied'))
  }

  // ===== 对等节点管理 =====
  const fetchPeers = async (wgId: number) => {
    setPeersLoading(true)
    try {
      const res: any = await wireguardApi.listPeers(wgId)
      setPeers(res.data || [])
    } finally { setPeersLoading(false) }
  }

  const openPeerList = (record: any) => {
    setCurrentWgId(record.id)
    setCurrentWgName(record.name)
    fetchPeers(record.id)
    setPeerListModalOpen(true)
  }

  const handleCreatePeer = () => {
    setPeerEditRecord(null)
    peerForm.resetFields()
    peerForm.setFieldsValue({
      enable: true,
      persistent_keepalive: 25,
      allowed_ips: '0.0.0.0/0',
    })
    setPeerModalOpen(true)
  }

  const handleEditPeer = (peer: any) => {
    setPeerEditRecord(peer)
    peerForm.setFieldsValue(peer)
    setPeerModalOpen(true)
  }

  // 下载 Peer 配置文件
  const handleDownloadPeerConfig = (peer: any) => {
    if (!currentWgId) return
    const url = wireguardApi.getPeerConfigUrl(currentWgId, peer.id)
    const a = document.createElement('a')
    a.href = url
    a.download = `${peer.name || 'peer-' + peer.id}.conf`
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
  }

  // 查看 Peer 二维码
  const handleShowQRCode = (peer: any) => {
    if (!currentWgId) return
    const url = wireguardApi.getPeerQRCodeUrl(currentWgId, peer.id)
    setQrImageUrl(url)
    setQrPeerName(peer.name || `Peer ${peer.id}`)
    setQrModalOpen(true)
  }

  const handlePeerSubmit = async () => {
    if (!currentWgId) return
    const values = await peerForm.validateFields()
    if (peerEditRecord) {
      await wireguardApi.updatePeer(currentWgId, peerEditRecord.id, values)
    } else {
      await wireguardApi.createPeer(currentWgId, values)
    }
    message.success(t('common.success'))
    setPeerModalOpen(false)
    fetchPeers(currentWgId)
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
      title: t('wireguard.address'), dataIndex: 'address',
      render: (v: string) => v ? <Text code style={{ fontSize: 12 }}>{v}</Text> : '-',
    },
    {
      title: t('wireguard.listenPort'), dataIndex: 'listen_port', width: 100,
      render: (v: number) => <Text code style={{ fontSize: 12 }}>{v || 51820}</Text>,
    },
    {
      title: t('wireguard.publicKey'), dataIndex: 'public_key',
      ellipsis: true,
      render: (v: string) => v ? (
        <Space size={4}>
          <Text code style={{ fontSize: 11 }}>{v.substring(0, 20)}...</Text>
          <Tooltip title={t('common.copy')}>
            <Button size="small" type="text" icon={<CopyOutlined />} onClick={() => copyToClipboard(v)} />
          </Tooltip>
        </Space>
      ) : '-',
    },
    {
      title: t('wireguard.peers'), width: 100,
      render: (_: any, r: any) => (
        <Button size="small" type="link" onClick={() => openPeerList(r)}>
          {t('wireguard.managePeers')}
        </Button>
      ),
    },
    {
      title: t('common.action'), width: 140,
      render: (_: any, r: any) => (
        <Space size={4}>
          {r.status === 'running'
            ? <Tooltip title={t('common.stop')}><Button size="small" icon={<StopOutlined />} onClick={async () => { await wireguardApi.stop(r.id); fetchData() }} /></Tooltip>
            : <Tooltip title={t('common.start')}><Button size="small" type="primary" icon={<PlayCircleOutlined />} onClick={async () => { await wireguardApi.start(r.id); fetchData() }} /></Tooltip>
          }
          <Tooltip title={t('common.edit')}>
            <Button size="small" icon={<EditOutlined />} onClick={() => handleEdit(r)} />
          </Tooltip>
          <Popconfirm title={t('common.deleteConfirm')} onConfirm={async () => { await wireguardApi.delete(r.id); fetchData() }}>
            <Tooltip title={t('common.delete')}>
              <Button size="small" danger icon={<DeleteOutlined />} />
            </Tooltip>
          </Popconfirm>
        </Space>
      ),
    },
  ]

  // 对等节点表格列
  const peerColumns = [
    {
      title: t('common.enable'), dataIndex: 'enable', width: 70,
      render: (v: boolean) => v ? <Tag color="success">启用</Tag> : <Tag>禁用</Tag>,
    },
    {
      title: t('common.name'), dataIndex: 'name',
      render: (name: string, r: any) => (
        <div>
          <Text strong>{name || '-'}</Text>
          {r.remark && <div><Text type="secondary" style={{ fontSize: 12 }}>{r.remark}</Text></div>}
        </div>
      ),
    },
    {
      title: t('wireguard.endpoint'), dataIndex: 'endpoint',
      render: (v: string) => v ? <Text code style={{ fontSize: 12 }}>{v}</Text> : <Text type="secondary">-</Text>,
    },
    {
      title: t('wireguard.allowedIPs'), dataIndex: 'allowed_ips',
      render: (v: string) => v ? <Text code style={{ fontSize: 12 }}>{v}</Text> : '-',
    },
    {
      title: t('wireguard.publicKey'), dataIndex: 'public_key',
      ellipsis: true,
      render: (v: string) => v ? <Text code style={{ fontSize: 11 }}>{v.substring(0, 16)}...</Text> : '-',
    },
    {
      title: t('wireguard.keepalive'), dataIndex: 'persistent_keepalive', width: 80,
      render: (v: number) => v > 0 ? `${v}s` : '-',
    },
    {
      title: t('common.action'), width: 160,
      render: (_: any, r: any) => (
        <Space size={4}>
          <Tooltip title={t('wireguard.downloadConfig')}>
            <Button size="small" icon={<DownloadOutlined />} onClick={() => handleDownloadPeerConfig(r)} />
          </Tooltip>
          <Tooltip title={t('wireguard.viewQRCode')}>
            <Button size="small" icon={<QrcodeOutlined />} onClick={() => handleShowQRCode(r)} />
          </Tooltip>
          <Tooltip title={t('common.edit')}>
            <Button size="small" icon={<EditOutlined />} onClick={() => handleEditPeer(r)} />
          </Tooltip>
          <Popconfirm title={t('common.deleteConfirm')} onConfirm={async () => {
            if (currentWgId) {
              await wireguardApi.deletePeer(currentWgId, r.id)
              fetchPeers(currentWgId)
            }
          }}>
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
          <Form.Item name="name" label={t('common.name')} rules={[{ required: true, message: t('wireguard.nameRequired') }]}>
            <Input placeholder={t('wireguard.namePlaceholder')} />
          </Form.Item>
        </Col>
        <Col span={6}>
          <Form.Item name="enable" label={t('common.enable')} valuePropName="checked">
            <Switch />
          </Form.Item>
        </Col>
      </Row>

      <SectionTitle>{t('wireguard.networkConfig')}</SectionTitle>
      <Row gutter={16}>
        <Col span={12}>
          <Form.Item name="address" label={t('wireguard.address')}
            rules={[{ required: true, message: t('wireguard.addressRequired') }]}
            extra={<span style={{ fontSize: 11 }}>{t('wireguard.addressTip')}</span>}
          >
            <Input placeholder="10.0.0.1/24" />
          </Form.Item>
        </Col>
        <Col span={6}>
          <Form.Item name="listen_port" label={t('wireguard.listenPort')}
            extra={<span style={{ fontSize: 11 }}>{t('wireguard.listenPortTip')}</span>}
          >
            <InputNumber min={1} max={65535} style={{ width: '100%' }} placeholder="51820" />
          </Form.Item>
        </Col>
        <Col span={6}>
          <Form.Item name="mtu" label="MTU"
            extra={<span style={{ fontSize: 11 }}>{t('wireguard.mtuTip')}</span>}
          >
            <InputNumber min={1280} max={9000} style={{ width: '100%' }} placeholder="1420" />
          </Form.Item>
        </Col>
      </Row>
      <Row gutter={16}>
        <Col span={12}>
          <Form.Item name="dns" label="DNS"
            extra={<span style={{ fontSize: 11 }}>{t('wireguard.dnsTip')}</span>}
          >
            <Input placeholder="8.8.8.8, 1.1.1.1" />
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item name="table" label={t('wireguard.routeTable')}
            extra={<span style={{ fontSize: 11 }}>{t('wireguard.routeTableTip')}</span>}
          >
            <Input placeholder="auto" />
          </Form.Item>
        </Col>
      </Row>
      <Form.Item name="remark" label={t('common.remark')}>
        <Input.TextArea rows={2} placeholder={t('common.remarkPlaceholder')} />
      </Form.Item>
    </>
  )

  // ===== Tab 2: 密钥配置 =====
  const tabKeys = (
    <>
      <SectionTitle>{t('wireguard.keyPair')}</SectionTitle>
      <div style={{ marginBottom: 12 }}>
        <Button icon={<ReloadOutlined />} onClick={handleGenerateKey} size="small">
          {t('wireguard.regenerateKey')}
        </Button>
        <Text type="secondary" style={{ fontSize: 11, marginLeft: 8 }}>{t('wireguard.regenerateKeyTip')}</Text>
      </div>
      <Form.Item name="private_key" label={t('wireguard.privateKey')}
        rules={[{ required: true, message: t('wireguard.privateKeyRequired') }]}
        extra={<span style={{ fontSize: 11 }}>{t('wireguard.privateKeyTip')}</span>}
      >
        <Input.Password placeholder="Base64 编码的私钥" style={{ width: '100%' }} />
      </Form.Item>
      <Form.Item name="public_key" label={t('wireguard.publicKey')}
        extra={<span style={{ fontSize: 11 }}>{t('wireguard.publicKeyTip')}</span>}
      >
        <Input disabled placeholder="自动生成" style={{ width: '100%' }}
          addonAfter={
            <Tooltip title={t('common.copy')}>
              <CopyOutlined onClick={() => copyToClipboard(form.getFieldValue('public_key') || '')} style={{ cursor: 'pointer' }} />
            </Tooltip>
          }
        />
      </Form.Item>
    </>
  )

  // ===== Tab 4: 对等节点（编辑模式下可用） =====
  const tabPeers = (
    <>
      {!editRecord ? (
        <div style={{ textAlign: 'center', padding: '40px 0', color: '#999' }}>
          <TeamOutlined style={{ fontSize: 32, marginBottom: 12, display: 'block' }} />
          <div>{t('wireguard.peersAfterCreate')}</div>
        </div>
      ) : (
        <>
          <div style={{ display: 'flex', justifyContent: 'flex-end', marginBottom: 12 }}>
            <Button type="primary" icon={<PlusOutlined />} onClick={handleCreatePeer} size="small">
              {t('wireguard.addPeer')}
            </Button>
          </div>
          <Table
            dataSource={peers} columns={peerColumns} rowKey="id" loading={peersLoading}
            size="small" pagination={false}
            locale={{ emptyText: t('wireguard.noPeers') }}
          />
        </>
      )}
    </>
  )

  // ===== Tab 3: 高级设置 =====
  const tabAdvanced = (
    <>
      <SectionTitle>{t('wireguard.hookScripts')}</SectionTitle>
      <Row gutter={16}>
        <Col span={12}>
          <Form.Item name="pre_up" label="PreUp"
            extra={<span style={{ fontSize: 11 }}>{t('wireguard.preUpTip')}</span>}
          >
            <Input.TextArea rows={2} placeholder="iptables -A FORWARD ..." />
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item name="post_up" label="PostUp"
            extra={<span style={{ fontSize: 11 }}>{t('wireguard.postUpTip')}</span>}
          >
            <Input.TextArea rows={2} placeholder="iptables -A FORWARD ..." />
          </Form.Item>
        </Col>
      </Row>
      <Row gutter={16}>
        <Col span={12}>
          <Form.Item name="pre_down" label="PreDown"
            extra={<span style={{ fontSize: 11 }}>{t('wireguard.preDownTip')}</span>}
          >
            <Input.TextArea rows={2} placeholder="iptables -D FORWARD ..." />
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item name="post_down" label="PostDown"
            extra={<span style={{ fontSize: 11 }}>{t('wireguard.postDownTip')}</span>}
          >
            <Input.TextArea rows={2} placeholder="iptables -D FORWARD ..." />
          </Form.Item>
        </Col>
      </Row>
    </>
  )

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Typography.Title level={4} style={{ margin: 0 }}>{t('wireguard.title')}</Typography.Title>
        <Space>
          <Button
            icon={<DownloadOutlined />}
            href="https://www.wireguard.com/install/"
            target="_blank"
            rel="noopener noreferrer"
          >
            WireGuard {t('common.officialSite')}
          </Button>
          <Button type="primary" icon={<PlusOutlined />} onClick={handleCreate}>{t('common.create')}</Button>
        </Space>
      </div>

      <Table
        dataSource={data} columns={columns} rowKey="id" loading={loading}
        size="middle" style={tableStyle}
        pagination={{ pageSize: 20, showSizeChanger: true }}
      />

      {/* 接口编辑弹窗 */}
      <Modal
        title={editRecord ? t('common.edit') : t('common.create')}
        open={modalOpen} onOk={handleSubmit} onCancel={() => setModalOpen(false)}
        width={700} destroyOnHidden
        styles={{ body: { padding: '4px 24px 0' } }}
      >
        <Form form={form} layout="vertical" style={{ paddingTop: 4 }}>
          <Tabs
            size="small"
            items={[
              { key: 'basic',    label: <span><SettingOutlined /> {t('wireguard.tabBasic')}</span>,    children: tabBasic },
              { key: 'keys',     label: <span><KeyOutlined />     {t('wireguard.tabKeys')}</span>,     children: tabKeys },
              { key: 'peers',    label: <span><TeamOutlined />    {t('wireguard.tabPeers')}</span>,    children: tabPeers },
              { key: 'advanced', label: <span><SafetyOutlined />  {t('wireguard.tabAdvanced')}</span>, children: tabAdvanced },
            ]}
          />
        </Form>
      </Modal>

      {/* 对等节点列表弹窗 */}
      <Modal
        title={`${t('wireguard.peers')} - ${currentWgName}`}
        open={peerListModalOpen}
        onCancel={() => setPeerListModalOpen(false)}
        footer={null}
        width={900}
        destroyOnHidden
      >
        <div style={{ display: 'flex', justifyContent: 'flex-end', marginBottom: 12 }}>
          <Button type="primary" icon={<PlusOutlined />} onClick={handleCreatePeer} size="small">
            {t('wireguard.addPeer')}
          </Button>
        </div>
        <Table
          dataSource={peers} columns={peerColumns} rowKey="id" loading={peersLoading}
          size="small" pagination={false}
        />
      </Modal>

      {/* 二维码弹窗 */}
      <Modal
        title={`${t('wireguard.viewQRCode')} - ${qrPeerName}`}
        open={qrModalOpen}
        onCancel={() => setQrModalOpen(false)}
        footer={null}
        width={320}
        destroyOnHidden
        styles={{ body: { textAlign: 'center', padding: '16px 24px 24px' } }}
      >
        <div style={{ marginBottom: 12 }}>
          <Tag color="blue" style={{ fontSize: 12 }}>{t('wireguard.qrCodeTip')}</Tag>
        </div>
        {qrImageUrl && (
          <img
            src={qrImageUrl}
            alt="WireGuard QR Code"
            style={{ width: 256, height: 256, borderRadius: 8, border: '1px solid #f0f0f0' }}
          />
        )}
      </Modal>

      {/* 对等节点编辑弹窗 */}
      <Modal
        title={peerEditRecord ? t('common.edit') : t('wireguard.addPeer')}
        open={peerModalOpen} onOk={handlePeerSubmit} onCancel={() => setPeerModalOpen(false)}
        width={600} destroyOnHidden
        styles={{ body: { padding: '4px 24px 0' } }}
      >
        <Form form={peerForm} layout="vertical" style={{ paddingTop: 4 }}>
          <Row gutter={16}>
            <Col span={18}>
              <Form.Item name="name" label={t('common.name')}>
                <Input placeholder={t('wireguard.peerNamePlaceholder')} />
              </Form.Item>
            </Col>
            <Col span={6}>
              <Form.Item name="enable" label={t('common.enable')} valuePropName="checked">
                <Switch />
              </Form.Item>
            </Col>
          </Row>

          <SectionTitle>{t('wireguard.peerConfig')}</SectionTitle>
          <Form.Item name="public_key" label={t('wireguard.peerPublicKey')}
            rules={[{ required: true, message: t('wireguard.peerPublicKeyRequired') }]}
            extra={<span style={{ fontSize: 11 }}>{t('wireguard.peerPublicKeyTip')}</span>}
          >
            <Input placeholder="Base64 编码的对端公钥" />
          </Form.Item>
          <Form.Item name="preshared_key" label={t('wireguard.presharedKey')}
            extra={<span style={{ fontSize: 11 }}>{t('wireguard.presharedKeyTip')}</span>}
          >
            <Input.Password placeholder={t('wireguard.presharedKeyPlaceholder')} />
          </Form.Item>

          <SectionTitle>{t('wireguard.connectionConfig')}</SectionTitle>
          <Row gutter={16}>
            <Col span={16}>
              <Form.Item name="endpoint" label={t('wireguard.endpoint')}
                extra={<span style={{ fontSize: 11 }}>{t('wireguard.endpointTip')}</span>}
              >
                <Input placeholder="1.2.3.4:51820" />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="persistent_keepalive" label={t('wireguard.keepalive')}
                extra={<span style={{ fontSize: 11 }}>{t('wireguard.keepaliveTip')}</span>}
              >
                <InputNumber min={0} max={65535} style={{ width: '100%' }} placeholder="25" />
              </Form.Item>
            </Col>
          </Row>
          <Form.Item name="allowed_ips" label={t('wireguard.allowedIPs')}
            rules={[{ required: true, message: t('wireguard.allowedIPsRequired') }]}
            extra={<span style={{ fontSize: 11 }}>{t('wireguard.allowedIPsTip')}</span>}
          >
            <Input placeholder="0.0.0.0/0, ::/0" />
          </Form.Item>
          <Form.Item name="remark" label={t('common.remark')}>
            <Input.TextArea rows={2} placeholder={t('common.remarkPlaceholder')} />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

export default Wireguard
