import React, { useEffect, useState, useRef } from 'react'
import {
  Alert,
  Badge,
  Button,
  Form,
  Input,
  InputNumber,
  Modal,
  Popconfirm,
  Select,
  Space,
  Switch,
  Table,
  Tag,
  Tooltip,
  Typography,
  message,
} from 'antd'
import {
  CheckCircleOutlined,
  CloseCircleOutlined,
  DeleteOutlined,
  EditOutlined,
  ExclamationCircleOutlined,
  FireOutlined,
  MinusCircleOutlined,
  PlusOutlined,
  ReloadOutlined,
  SyncOutlined,
  CloudDownloadOutlined,
  ClockCircleOutlined,
  DatabaseOutlined,
  DesktopOutlined,
} from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { firewallApi } from '../api'

const { Option } = Select
const { Text } = Typography

// 防火墙后端标签颜色映射
const backendColorMap: Record<string, string> = {
  iptables: 'blue',
  nftables: 'cyan',
  ufw: 'geekblue',
  firewalld: 'purple',
  openwrt: 'orange',
  windows: 'green',
  unknown: 'default',
}

// 应用状态渲染
const ApplyStatusTag: React.FC<{ status: string; error?: string }> = ({ status, error }) => {
  if (status === 'applied') return <Tag icon={<CheckCircleOutlined />} color="success">已应用</Tag>
  if (status === 'error') return (
    <Tooltip title={error || '未知错误'}>
      <Tag icon={<CloseCircleOutlined />} color="error">应用失败</Tag>
    </Tooltip>
  )
  return <Tag icon={<MinusCircleOutlined />} color="default">待应用</Tag>
}

interface SyncStatus {
  syncing: boolean
  last_sync_at: string
  last_sync_err: string
  total: number
}

const Firewall: React.FC = () => {
  const { t } = useTranslation()
  const [data, setData] = useState<any[]>([])
  const [loading, setLoading] = useState(false)
  const [modalOpen, setModalOpen] = useState(false)
  const [editRecord, setEditRecord] = useState<any>(null)
  const [backend, setBackend] = useState<string>('')
  const [backendLoading, setBackendLoading] = useState(false)
  const [applyingId, setApplyingId] = useState<number | null>(null)
  const [syncStatus, setSyncStatus] = useState<SyncStatus | null>(null)
  const [syncing, setSyncing] = useState(false)
  const [form] = Form.useForm()
  const pollTimerRef = useRef<ReturnType<typeof setInterval> | null>(null)

  const fetchData = async () => {
    setLoading(true)
    try {
      const res: any = await firewallApi.list()
      setData(res.data || [])
    } finally {
      setLoading(false)
    }
  }

  const fetchBackend = async () => {
    setBackendLoading(true)
    try {
      const res: any = await firewallApi.detectBackend()
      setBackend(res.data?.backend || 'unknown')
    } catch {
      setBackend('unknown')
    } finally {
      setBackendLoading(false)
    }
  }

  const fetchSyncStatus = async () => {
    try {
      const res: any = await firewallApi.getSyncStatus()
      const status: SyncStatus = res.data
      setSyncStatus(status)
      // 同步中时自动刷新规则列表
      if (status.syncing) {
        fetchData()
      }
      return status
    } catch {
      return null
    }
  }

  // 启动/停止轮询
  const startPolling = () => {
    if (pollTimerRef.current) return
    pollTimerRef.current = setInterval(async () => {
      const status = await fetchSyncStatus()
      // 同步完成后停止轮询并刷新一次
      if (status && !status.syncing) {
        stopPolling()
        fetchData()
      }
    }, 2000)
  }

  const stopPolling = () => {
    if (pollTimerRef.current) {
      clearInterval(pollTimerRef.current)
      pollTimerRef.current = null
    }
  }

  useEffect(() => {
    fetchData()
    fetchBackend()
    // 初始获取同步状态，若正在同步则启动轮询
    fetchSyncStatus().then(status => {
      if (status?.syncing) startPolling()
    })
    return () => stopPolling()
  }, [])

  // 监听 syncStatus.syncing 变化，自动管理轮询
  useEffect(() => {
    if (syncStatus?.syncing) {
      startPolling()
    }
  }, [syncStatus?.syncing])

  const handleSyncSystem = async () => {
    setSyncing(true)
    try {
      await firewallApi.syncSystem()
      message.success('同步任务已触发，正在后台读取系统防火墙规则...')
      // 立即查一次状态并启动轮询
      await fetchSyncStatus()
      startPolling()
    } catch (e: any) {
      message.error('触发同步失败: ' + (e?.response?.data?.message || e.message))
    } finally {
      setSyncing(false)
    }
  }

  const handleSubmit = async () => {
    const values = await form.validateFields()
    if (editRecord) {
      await firewallApi.update(editRecord.id, values)
    } else {
      await firewallApi.create(values)
    }
    message.success(t('common.success'))
    setModalOpen(false)
    fetchData()
  }

  const handleApply = async (id: number) => {
    setApplyingId(id)
    try {
      await firewallApi.apply(id)
      message.success('规则已应用到系统防火墙')
      fetchData()
    } catch (e: any) {
      message.error('应用失败: ' + (e?.response?.data?.message || e.message))
    } finally {
      setApplyingId(null)
    }
  }

  const handleRemove = async (id: number) => {
    setApplyingId(id)
    try {
      await firewallApi.remove(id)
      message.success('规则已从系统防火墙移除')
      fetchData()
    } catch (e: any) {
      message.error('移除失败: ' + (e?.response?.data?.message || e.message))
    } finally {
      setApplyingId(null)
    }
  }

  const openCreate = () => {
    setEditRecord(null)
    form.resetFields()
    form.setFieldsValue({
      enable: true,
      direction: 'in',
      action: 'deny',
      protocol: 'tcp',
      priority: 100,
    })
    setModalOpen(true)
  }

  const openEdit = (record: any) => {
    setEditRecord(record)
    form.setFieldsValue(record)
    setModalOpen(true)
  }

  // 格式化同步时间
  const formatSyncTime = (timeStr: string) => {
    if (!timeStr || timeStr.startsWith('0001')) return '从未同步'
    const d = new Date(timeStr)
    return d.toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit' })
  }

  const columns = [
    {
      title: '来源',
      dataIndex: 'is_system',
      width: 75,
      render: (v: boolean) => v
        ? <Tag icon={<DesktopOutlined />} color="blue">系统</Tag>
        : <Tag icon={<DatabaseOutlined />} color="default">手动</Tag>,
    },
    {
      title: t('common.enable'),
      dataIndex: 'enable',
      width: 70,
      render: (v: boolean, r: any) => (
        <Switch
          size="small"
          checked={v}
          onChange={async (checked) => {
            await firewallApi.update(r.id, { ...r, enable: checked })
            fetchData()
          }}
        />
      ),
    },
    {
      title: t('common.name'),
      dataIndex: 'name',
      ellipsis: true,
    },
    {
      title: '方向',
      dataIndex: 'direction',
      width: 80,
      render: (v: string) => (
        <Tag color={v === 'in' ? 'blue' : 'orange'}>
          {v === 'in' ? '入站' : '出站'}
        </Tag>
      ),
    },
    {
      title: '动作',
      dataIndex: 'action',
      width: 80,
      render: (v: string) => (
        <Tag color={v === 'allow' ? 'success' : 'error'}>
          {v === 'allow' ? '允许' : '拒绝'}
        </Tag>
      ),
    },
    {
      title: '协议',
      dataIndex: 'protocol',
      width: 90,
      render: (v: string) => <Tag>{v || 'all'}</Tag>,
    },
    {
      title: '源IP',
      dataIndex: 'src_ip',
      ellipsis: true,
      render: (v: string) => v || <Text type="secondary">任意</Text>,
    },
    {
      title: '端口',
      dataIndex: 'port',
      width: 100,
      render: (v: string) => v || <Text type="secondary">任意</Text>,
    },
    {
      title: '优先级',
      dataIndex: 'priority',
      width: 80,
      render: (v: number) => <Text type="secondary">{v}</Text>,
    },
    {
      title: '应用状态',
      dataIndex: 'apply_status',
      width: 110,
      render: (v: string, r: any) => <ApplyStatusTag status={v} error={r.last_error} />,
    },
    {
      title: t('common.action'),
      width: 200,
      render: (_: any, r: any) => (
        <Space size={4}>
          <Tooltip title="应用到系统防火墙">
            <Button
              size="small"
              type="primary"
              icon={<FireOutlined />}
              loading={applyingId === r.id}
              onClick={() => handleApply(r.id)}
            />
          </Tooltip>
          <Tooltip title="从系统防火墙移除">
            <Button
              size="small"
              icon={<MinusCircleOutlined />}
              loading={applyingId === r.id}
              onClick={() => handleRemove(r.id)}
            />
          </Tooltip>
          <Button size="small" icon={<EditOutlined />} onClick={() => openEdit(r)} />
          <Popconfirm
            title={t('common.deleteConfirm')}
            onConfirm={async () => {
              await firewallApi.delete(r.id)
              message.success(t('common.success'))
              fetchData()
            }}
          >
            <Button size="small" danger icon={<DeleteOutlined />} />
          </Popconfirm>
        </Space>
      ),
    },
  ]

  const isSyncing = syncStatus?.syncing || syncing

  return (
    <div>
      {/* 标题栏 */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 12 }}>
        <Space align="center">
          <Typography.Title level={4} style={{ margin: 0 }}>
            IP 防火墙管理
          </Typography.Title>
          {backend && (
            <Tooltip title="当前系统检测到的防火墙后端">
              <Tag
                color={backendColorMap[backend] || 'default'}
                icon={backendLoading ? <SyncOutlined spin /> : <FireOutlined />}
                style={{ cursor: 'pointer' }}
                onClick={fetchBackend}
              >
                {backend === 'unknown' ? '未检测到防火墙' : backend}
              </Tag>
            </Tooltip>
          )}
        </Space>
        <Space>
          <Button
            icon={<ReloadOutlined />}
            onClick={() => { fetchData(); fetchBackend(); fetchSyncStatus() }}
          >
            {t('common.refresh')}
          </Button>
          <Button
            icon={isSyncing ? <SyncOutlined spin /> : <CloudDownloadOutlined />}
            loading={syncing}
            disabled={isSyncing}
            onClick={handleSyncSystem}
          >
            {isSyncing ? '同步中...' : '立即同步'}
          </Button>
          <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
            {t('common.create')}
          </Button>
        </Space>
      </div>

      {/* 同步状态条 */}
      {syncStatus && (
        <div style={{
          display: 'flex', alignItems: 'center', gap: 16,
          padding: '8px 14px', marginBottom: 12, borderRadius: 8,
          background: isSyncing ? 'rgba(22,119,255,0.06)' : 'rgba(0,0,0,0.02)',
          border: `1px solid ${isSyncing ? 'rgba(22,119,255,0.2)' : 'rgba(0,0,0,0.06)'}`,
          fontSize: 12, color: 'rgba(0,0,0,0.55)',
        }}>
          {isSyncing
            ? <><SyncOutlined spin style={{ color: '#1677ff' }} /><span style={{ color: '#1677ff' }}>正在从系统防火墙同步规则，请稍候...</span></>
            : <><ClockCircleOutlined /><span>上次同步：{formatSyncTime(syncStatus.last_sync_at)}</span></>
          }
          <span>·</span>
          <span>已同步 <strong>{syncStatus.total}</strong> 条系统规则</span>
          {syncStatus.last_sync_err && !isSyncing && (
            <>
              <span>·</span>
              <Tooltip title={syncStatus.last_sync_err}>
                <span style={{ color: '#ff4d4f', cursor: 'help' }}>
                  <ExclamationCircleOutlined style={{ marginRight: 4 }} />上次同步出错
                </span>
              </Tooltip>
            </>
          )}
          <span style={{ marginLeft: 'auto', color: 'rgba(0,0,0,0.35)' }}>每 30 分钟自动同步</span>
        </div>
      )}

      {/* 提示信息 */}
      {backend === 'unknown' && (
        <Alert
          type="warning"
          showIcon
          icon={<ExclamationCircleOutlined />}
          message="未检测到受支持的防火墙后端"
          description="当前系统未检测到 iptables、nftables、ufw、firewalld（Linux）或 Windows 防火墙。规则可以保存，但无法应用到系统。macOS 暂不支持。"
          style={{ marginBottom: 12 }}
        />
      )}
      {backend !== 'unknown' && backend && (
        <Alert
          type="info"
          showIcon
          message={`当前防火墙后端：${backend}`}
          description={
            backend === 'windows'
              ? '使用 Windows 高级防火墙（netsh advfirewall），需要管理员权限。'
              : backend === 'ufw'
              ? '使用 ufw（Uncomplicated Firewall），需要 root 权限。'
              : backend === 'firewalld'
              ? '使用 firewalld，需要 root 权限。规则将永久保存（--permanent）。'
              : backend === 'openwrt'
              ? '使用 OpenWrt iptables，需要 root 权限。'
              : `使用 ${backend}，需要 root 权限。`
          }
          style={{ marginBottom: 12 }}
          closable
        />
      )}

      {/* 规则表格 */}
      <Table
        dataSource={data}
        columns={columns}
        rowKey="id"
        loading={loading || isSyncing}
        size="middle"
        style={{ background: '#fff', borderRadius: 8 }}
        pagination={{ pageSize: 20 }}
        rowClassName={(r) => r.apply_status === 'error' ? 'ant-table-row-error' : ''}
      />

      {/* 新建/编辑弹窗 */}
      <Modal
        title={
          <Space>
            <FireOutlined />
            {editRecord ? '编辑防火墙规则' : '新建防火墙规则'}
          </Space>
        }
        open={modalOpen}
        onOk={handleSubmit}
        onCancel={() => setModalOpen(false)}
        width={560}
        destroyOnHidden
      >
        <Form form={form} layout="vertical" style={{ marginTop: 16 }}>
          <Form.Item name="name" label={t('common.name')} rules={[{ required: true, message: '请输入规则名称' }]}>
            <Input placeholder="如：拒绝外部访问 SSH" />
          </Form.Item>

          <Form.Item name="enable" label={t('common.enable')} valuePropName="checked">
            <Switch />
          </Form.Item>

          <Space style={{ width: '100%' }} size={12}>
            <Form.Item name="direction" label="方向" rules={[{ required: true }]} style={{ flex: 1 }}>
              <Select>
                <Option value="in">入站（Inbound）</Option>
                <Option value="out">出站（Outbound）</Option>
              </Select>
            </Form.Item>
            <Form.Item name="action" label="动作" rules={[{ required: true }]} style={{ flex: 1 }}>
              <Select>
                <Option value="allow">
                  <Badge status="success" text="允许（Allow）" />
                </Option>
                <Option value="deny">
                  <Badge status="error" text="拒绝（Deny/Drop）" />
                </Option>
              </Select>
            </Form.Item>
          </Space>

          <Space style={{ width: '100%' }} size={12}>
            <Form.Item name="protocol" label="协议" style={{ flex: 1 }}>
              <Select>
                <Option value="tcp">TCP</Option>
                <Option value="udp">UDP</Option>
                <Option value="tcp+udp">TCP + UDP</Option>
                <Option value="icmp">ICMP</Option>
                <Option value="all">全部</Option>
              </Select>
            </Form.Item>
            <Form.Item name="port" label="端口/范围" style={{ flex: 1 }}>
              <Input placeholder="如：80 或 8080-8090，留空=任意" />
            </Form.Item>
          </Space>

          <Space style={{ width: '100%' }} size={12}>
            <Form.Item name="src_ip" label="源IP/CIDR" style={{ flex: 1 }}>
              <Input placeholder="如：192.168.1.0/24，留空=任意" />
            </Form.Item>
            <Form.Item name="dst_ip" label="目标IP/CIDR" style={{ flex: 1 }}>
              <Input placeholder="如：10.0.0.1，留空=任意" />
            </Form.Item>
          </Space>

          <Space style={{ width: '100%' }} size={12}>
            <Form.Item name="interface" label="网络接口" style={{ flex: 1 }}>
              <Input placeholder="如：eth0，留空=所有接口" />
            </Form.Item>
            <Form.Item name="priority" label="优先级（越小越优先）" style={{ flex: 1 }}>
              <InputNumber min={1} max={9999} style={{ width: '100%' }} />
            </Form.Item>
          </Space>

          <Form.Item name="remark" label={t('common.remark')}>
            <Input.TextArea rows={2} placeholder="可选备注" />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

export default Firewall
