import React, { useEffect, useState } from 'react'
import {
  Table, Button, Space, Modal, Form, Input, Select, DatePicker, Switch,
  Popconfirm, message, Typography, Tag, Tooltip, Badge, Checkbox, Spin,
  Alert, Divider, Empty, InputNumber,
} from 'antd'
import {
  PlusOutlined, EditOutlined, DeleteOutlined, ReloadOutlined,
  SearchOutlined, UnorderedListOutlined, SettingOutlined, CloudDownloadOutlined,
} from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router-dom'
import dayjs from 'dayjs'
import relativeTime from 'dayjs/plugin/relativeTime'
import 'dayjs/locale/zh-cn'
dayjs.extend(relativeTime)
dayjs.locale('zh-cn')
import { domainInfoApi, domainAccountApi } from '../api'

// 自动同步间隔预设选项（分钟）
const SYNC_INTERVAL_OPTIONS = [
  { label: '5 分钟', value: 5 },
  { label: '15 分钟', value: 15 },
  { label: '30 分钟', value: 30 },
  { label: '1 小时', value: 60 },
  { label: '2 小时', value: 120 },
  { label: '6 小时', value: 360 },
  { label: '12 小时', value: 720 },
  { label: '24 小时', value: 1440 },
]

const { Option } = Select
const { Text } = Typography

// 服务商标签颜色（与 DomainAccount.tsx 保持一致）
const PROVIDER_COLORS: Record<string, string> = {
  alidns: 'orange', cloudflare: 'blue', dnspod: 'cyan',
  huaweidns: 'red', godaddy: 'green', namesilo: 'purple',
  tencenteo: 'geekblue', aliesa: 'volcano',
}
const PROVIDER_LABELS: Record<string, string> = {
  alidns: '阿里云 DNS', cloudflare: 'Cloudflare', dnspod: 'DNSPod',
  huaweidns: '华为云 DNS', godaddy: 'GoDaddy', namesilo: 'NameSilo',
  tencenteo: '腾讯云 EdgeOne', aliesa: '阿里云 ESA',
}

const DomainRecord: React.FC = () => {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [data, setData] = useState<any[]>([])
  const [accounts, setAccounts] = useState<any[]>([])
  const [loading, setLoading] = useState(false)
  const [refreshingId, setRefreshingId] = useState<number | null>(null)
  const [modalOpen, setModalOpen] = useState(false)
  const [configModalOpen, setConfigModalOpen] = useState(false)
  const [editRecord, setEditRecord] = useState<any>(null)
  const [searchKeyword, setSearchKeyword] = useState('')
  const [filterAccountId, setFilterAccountId] = useState<number | undefined>()
  const [configForm] = Form.useForm()

  // 添加弹窗状态
  const [addAccountId, setAddAccountId] = useState<number | undefined>()
  const [fetchLoading, setFetchLoading] = useState(false)
  const [providerDomains, setProviderDomains] = useState<any[]>([])
  const [selectedDomains, setSelectedDomains] = useState<string[]>([])
  const [manualMode, setManualMode] = useState(false)
  const [manualForm] = Form.useForm()
  const [addLoading, setAddLoading] = useState(false)

  const fetchData = async () => {
    setLoading(true)
    try {
      const [domRes, accRes]: any[] = await Promise.all([
        domainInfoApi.list({ account_id: filterAccountId, keyword: searchKeyword }),
        domainAccountApi.list(),
      ])
      setData(domRes.data || [])
      setAccounts(accRes.data || [])
    } finally {
      setLoading(false) }
  }

  useEffect(() => { fetchData() }, [filterAccountId])

  // 打开添加弹窗
  const handleAdd = () => {
    setAddAccountId(undefined)
    setProviderDomains([])
    setSelectedDomains([])
    setManualMode(false)
    manualForm.resetFields()
    setModalOpen(true)
  }

  // 选择账号后自动拉取域名列表
  const handleAccountSelect = async (accountId: number) => {
    setAddAccountId(accountId)
    setProviderDomains([])
    setSelectedDomains([])
    setFetchLoading(true)
    try {
      const res: any = await domainInfoApi.fetchFromProvider(accountId)
      const domains = res.data?.domains || []
      setProviderDomains(domains)
      // 默认勾选未添加的域名
      setSelectedDomains(domains.filter((d: any) => !d.added).map((d: any) => d.name))
    } catch {
      // 拉取失败时切换到手动模式
      setManualMode(true)
    } finally {
      setFetchLoading(false)
    }
  }

  // 批量添加选中的域名
  const handleBatchAdd = async () => {
    if (!addAccountId) { message.warning('请先选择账号'); return }
    if (selectedDomains.length === 0) { message.warning('请至少选择一个域名'); return }
    setAddLoading(true)
    try {
      const domainMap = Object.fromEntries(providerDomains.map((d: any) => [d.name, d]))
      await Promise.all(
        selectedDomains.map(name =>
          domainInfoApi.create({
            account_id: addAccountId,
            name,
            third_id: domainMap[name]?.third_id || '',
          })
        )
      )
      message.success(`成功添加 ${selectedDomains.length} 个域名`)
      setModalOpen(false)
      fetchData()
    } finally {
      setAddLoading(false)
    }
  }

  // 手动添加单个域名
  const handleManualAdd = async () => {
    const values = await manualForm.validateFields()
    setAddLoading(true)
    try {
      await domainInfoApi.create({ account_id: addAccountId, ...values })
      message.success('添加域名成功')
      setModalOpen(false)
      fetchData()
    } finally {
      setAddLoading(false)
    }
  }

  // 打开配置弹窗（编辑到期时间、到期提醒、自动同步、备注）
  const handleConfig = (record: any) => {
    setEditRecord(record)
    configForm.setFieldsValue({
      expire_time: record.expire_time ? dayjs(record.expire_time) : null,
      is_notice: record.is_notice,
      auto_sync: record.auto_sync || false,
      sync_interval: record.sync_interval || 60,
      remark: record.remark,
    })
    setConfigModalOpen(true)
  }

  // 提交配置
  const handleConfigSubmit = async () => {
    const values = await configForm.validateFields()
    const payload = {
      ...editRecord,
      expire_time: values.expire_time ? values.expire_time.toISOString() : null,
      is_notice: values.is_notice,
      auto_sync: values.auto_sync,
      sync_interval: values.sync_interval,
      remark: values.remark,
    }
    await domainInfoApi.update(editRecord.id, payload)
    // 同步更新自动同步配置（触发后端定时器注册/取消）
    await domainInfoApi.updateAutoSync(editRecord.id, {
      auto_sync: values.auto_sync,
      sync_interval: values.sync_interval,
    })
    message.success('配置已保存')
    setConfigModalOpen(false)
    fetchData()
  }

  // 刷新单个域名
  const handleRefresh = async (id: number) => {
    setRefreshingId(id)
    try {
      await domainInfoApi.refresh(id)
      message.success('刷新成功')
      fetchData()
    } finally {
      setRefreshingId(null)
    }
  }

  // 搜索
  const handleSearch = () => { fetchData() }

  // 到期状态
  const getExpireStatus = (expireTime: string | null) => {
    if (!expireTime) return null
    const days = dayjs(expireTime).diff(dayjs(), 'day')
    if (days < 0) return <Tag color="red">已过期</Tag>
    if (days <= 30) return <Tag color="orange">剩余 {days} 天</Tag>
    return <Tag color="green">剩余 {days} 天</Tag>
  }

  const columns = [
    {
      title: 'ID', dataIndex: 'id', width: 60,
      render: (v: number) => <Text type="secondary" style={{ fontSize: 12 }}>#{v}</Text>,
    },
    {
      title: '平台账户', dataIndex: 'account_name', width: 140,
      render: (name: string, r: any) => (
        <Space size={4}>
          <Tag color={PROVIDER_COLORS[r.account_provider] || 'default'} style={{ margin: 0 }}>
            {PROVIDER_LABELS[r.account_provider] || r.account_provider || ''}
          </Tag>
          <Text style={{ fontSize: 12 }}>{name}</Text>
        </Space>
      ),
    },
    {
      title: '域名', dataIndex: 'name',
      render: (name: string) => <Text strong>{name}</Text>,
    },
    {
      title: '记录数', dataIndex: 'record_count', width: 80,
      render: (v: number) => <Badge count={v} showZero color="#1677ff" />,
    },
    {
      title: '添加时间', dataIndex: 'created_at', width: 160,
      render: (v: string) => v ? dayjs(v).format('YYYY-MM-DD HH:mm') : '-',
    },
    {
      title: '到期时间', dataIndex: 'expire_time', width: 200,
      render: (v: string) => (
        <Space size={4}>
          <Text style={{ fontSize: 12 }}>{v ? dayjs(v).format('YYYY-MM-DD') : '-'}</Text>
          {getExpireStatus(v)}
        </Space>
      ),
    },
    {
      title: '到期提醒', dataIndex: 'is_notice', width: 90,
      render: (v: boolean) => v ? <Tag color="green">已开启</Tag> : <Tag>未开启</Tag>,
    },
    {
      title: '自动同步', dataIndex: 'auto_sync', width: 100,
      render: (v: boolean, r: any) => (
        <Tooltip title={v ? `每 ${r.sync_interval} 分钟同步一次` : '未开启自动同步'}>
          {v
            ? <Tag color="processing">每{r.sync_interval >= 60 ? `${r.sync_interval / 60}h` : `${r.sync_interval}m`}同步</Tag>
            : <Tag>手动</Tag>}
        </Tooltip>
      ),
    },
    {
      title: '上次同步', dataIndex: 'last_sync_time', width: 150,
      render: (v: string) => v ? (
        <Tooltip title={dayjs(v).format('YYYY-MM-DD HH:mm:ss')}>
          <Text type="secondary" style={{ fontSize: 12 }}>{dayjs(v).fromNow()}</Text>
        </Tooltip>
      ) : <Text type="secondary" style={{ fontSize: 12 }}>从未同步</Text>,
    },
    {
      title: '备注', dataIndex: 'remark', width: 120,
      render: (v: string) => v ? <Text type="secondary" style={{ fontSize: 12 }}>{v}</Text> : '-',
    },
    {
      title: '操作', width: 160, fixed: 'right' as const,
      render: (_: any, r: any) => (
        <Space size={4}>
          <Tooltip title="解析记录">
            <Button
              size="small" type="primary" ghost
              icon={<UnorderedListOutlined />}
              onClick={() => navigate(`/domain/info/${r.id}/records`, { state: { domainName: r.name, accountProvider: r.account_provider } })}
            >
              解析
            </Button>
          </Tooltip>
          <Tooltip title="配置">
            <Button size="small" icon={<SettingOutlined />} onClick={() => handleConfig(r)} />
          </Tooltip>
          <Popconfirm
            title="确定要删除该域名吗？删除后解析记录也将一并删除。"
            onConfirm={async () => { await domainInfoApi.delete(r.id); fetchData() }}
          >
            <Tooltip title={t('common.delete')}>
              <Button size="small" danger icon={<DeleteOutlined />} />
            </Tooltip>
          </Popconfirm>
        </Space>
      ),
    },
  ]

  // 已添加的域名 Set（用于在拉取列表中标记）
  const addedDomainNames = new Set(
    addAccountId ? data.filter(d => d.account_id === addAccountId).map(d => d.name) : []
  )

  return (
    <div>
      {/* 标题栏 */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Typography.Title level={4} style={{ margin: 0 }}>DNS 域名解析</Typography.Title>
        <Space>
          <Input
            placeholder="搜索域名/备注"
            prefix={<SearchOutlined />}
            value={searchKeyword}
            onChange={e => setSearchKeyword(e.target.value)}
            onPressEnter={handleSearch}
            style={{ width: 180 }}
            allowClear
          />
          <Select
            placeholder="筛选账号"
            allowClear
            style={{ width: 150 }}
            onChange={v => setFilterAccountId(v)}
          >
            {accounts.map((a: any) => <Option key={a.id} value={a.id}>{a.name}</Option>)}
          </Select>
          <Tooltip title="刷新列表">
            <Button icon={<ReloadOutlined />} onClick={fetchData} loading={loading} />
          </Tooltip>
          <Button type="primary" icon={<PlusOutlined />} onClick={handleAdd}>
            添加域名
          </Button>
        </Space>
      </div>

      <Table
        dataSource={data}
        columns={columns}
        rowKey="id"
        loading={loading}
        size="middle"
        scroll={{ x: 1100 }}
        style={{ background: '#fff', borderRadius: 8 }}
        pagination={{ pageSize: 20, showSizeChanger: true, showTotal: total => `共 ${total} 条` }}
      />

      {/* 添加域名弹窗 */}
      <Modal
        title="添加域名"
        open={modalOpen}
        onCancel={() => setModalOpen(false)}
        width={520}
        destroyOnHidden
        footer={null}
      >
        <div style={{ marginTop: 16 }}>
          {/* 第一步：选择账号 */}
          <div style={{ marginBottom: 16 }}>
            <div style={{ marginBottom: 8, fontWeight: 500 }}>
              选择 DNS 账号
              <Text type="secondary" style={{ fontSize: 12, fontWeight: 400, marginLeft: 8 }}>
                选择后将自动拉取该账号下的域名列表
              </Text>
            </div>
            <Select
              placeholder="选择域名账号"
              style={{ width: '100%' }}
              value={addAccountId}
              onChange={handleAccountSelect}
              loading={fetchLoading}
            >
              {accounts.map((a: any) => (
                <Option key={a.id} value={a.id}>
                  <Space>
                    <Tag color={PROVIDER_COLORS[a.provider] || 'default'} style={{ margin: 0 }}>
                      {PROVIDER_LABELS[a.provider] || a.provider}
                    </Tag>
                    {a.name}
                  </Space>
                </Option>
              ))}
            </Select>
          </div>

          {/* 拉取中 */}
          {fetchLoading && (
            <div style={{ textAlign: 'center', padding: '24px 0' }}>
              <Spin tip="正在从服务商拉取域名列表..." />
            </div>
          )}

          {/* 域名列表（从服务商拉取） */}
          {!fetchLoading && addAccountId && !manualMode && (
            <>
              {providerDomains.length > 0 ? (
                <>
                  <Divider style={{ margin: '12px 0' }} />
                  <div style={{ marginBottom: 8, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                    <Text style={{ fontWeight: 500 }}>
                      账号下的域名
                      <Text type="secondary" style={{ fontSize: 12, fontWeight: 400, marginLeft: 8 }}>
                        共 {providerDomains.length} 个，已添加 {providerDomains.filter(d => d.added || addedDomainNames.has(d.name)).length} 个
                      </Text>
                    </Text>
                    <Space size={4}>
                      <Button
                        size="small"
                        onClick={() => setSelectedDomains(
                          providerDomains.filter(d => !d.added && !addedDomainNames.has(d.name)).map((d: any) => d.name)
                        )}
                      >
                        全选未添加
                      </Button>
                      <Button size="small" onClick={() => setSelectedDomains([])}>取消全选</Button>
                    </Space>
                  </div>
                  <div style={{ maxHeight: 280, overflowY: 'auto', border: '1px solid #f0f0f0', borderRadius: 6, padding: '4px 0' }}>
                    {providerDomains.map((d: any) => {
                      const isAdded = d.added || addedDomainNames.has(d.name)
                      return (
                        <div
                          key={d.name}
                          style={{
                            display: 'flex', alignItems: 'center', padding: '6px 12px',
                            cursor: isAdded ? 'not-allowed' : 'pointer',
                            background: selectedDomains.includes(d.name) ? '#e6f4ff' : 'transparent',
                          }}
                          onClick={() => {
                            if (isAdded) return
                            setSelectedDomains(prev =>
                              prev.includes(d.name) ? prev.filter(n => n !== d.name) : [...prev, d.name]
                            )
                          }}
                        >
                          <Checkbox
                            checked={selectedDomains.includes(d.name)}
                            disabled={isAdded}
                            style={{ marginRight: 10 }}
                          />
                          <Text style={{ flex: 1, color: isAdded ? '#bbb' : undefined }}>{d.name}</Text>
                          {isAdded && <Tag color="green" style={{ margin: 0 }}>已添加</Tag>}
                          {d.third_id && !isAdded && (
                            <Text type="secondary" style={{ fontSize: 11 }}>ID: {d.third_id}</Text>
                          )}
                        </div>
                      )
                    })}
                  </div>
                  <div style={{ marginTop: 12, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                    <Button
                      size="small"
                      type="link"
                      icon={<EditOutlined />}
                      onClick={() => setManualMode(true)}
                    >
                      手动输入域名
                    </Button>
                    <Space>
                      <Button onClick={() => setModalOpen(false)}>取消</Button>
                      <Button
                        type="primary"
                        loading={addLoading}
                        disabled={selectedDomains.length === 0}
                        onClick={handleBatchAdd}
                      >
                        添加选中的 {selectedDomains.length > 0 ? `${selectedDomains.length} 个` : ''}域名
                      </Button>
                    </Space>
                  </div>
                </>
              ) : (
                <>
                  <Alert
                    type="info"
                    showIcon
                    message="未能从服务商获取域名列表"
                    description="可能是该服务商暂不支持自动拉取，请手动输入域名。"
                    style={{ marginBottom: 12 }}
                  />
                  <ManualAddForm
                    form={manualForm}
                    loading={addLoading}
                    onCancel={() => setModalOpen(false)}
                    onSubmit={handleManualAdd}
                  />
                </>
              )}
            </>
          )}

          {/* 手动输入模式 */}
          {!fetchLoading && addAccountId && manualMode && (
            <>
              <Divider style={{ margin: '12px 0' }} />
              <Button
                size="small" type="link" style={{ padding: 0, marginBottom: 8 }}
                icon={<CloudDownloadOutlined />}
                onClick={() => { setManualMode(false); handleAccountSelect(addAccountId) }}
              >
                重新从服务商拉取
              </Button>
              <ManualAddForm
                form={manualForm}
                loading={addLoading}
                onCancel={() => setModalOpen(false)}
                onSubmit={handleManualAdd}
              />
            </>
          )}

          {/* 未选择账号时的提示 */}
          {!addAccountId && !fetchLoading && (
            <Empty description="请先选择 DNS 账号" image={Empty.PRESENTED_IMAGE_SIMPLE} />
          )}
        </div>
      </Modal>

      <Modal
        title={`配置域名：${editRecord?.name || ''}`}
        open={configModalOpen}
        onOk={handleConfigSubmit}
        onCancel={() => setConfigModalOpen(false)}
        width={460}
        destroyOnHidden
      >
        <Form form={configForm} layout="vertical" style={{ marginTop: 16 }}>
          <Form.Item name="expire_time" label="到期时间">
            <DatePicker style={{ width: '100%' }} placeholder="选择到期时间（可选）" />
          </Form.Item>
          <Form.Item name="is_notice" label="到期提醒" valuePropName="checked">
            <Switch checkedChildren="开启" unCheckedChildren="关闭" />
          </Form.Item>
          <Form.Item
            name="auto_sync"
            label="自动同步解析记录"
            valuePropName="checked"
            extra="开启后将定时从 DNS 服务商同步解析记录到本地"
          >
            <Switch checkedChildren="开启" unCheckedChildren="关闭" />
          </Form.Item>
          <Form.Item
            noStyle
            shouldUpdate={(prev, cur) => prev.auto_sync !== cur.auto_sync}
          >
            {({ getFieldValue }) =>
              getFieldValue('auto_sync') ? (
                <Form.Item
                  name="sync_interval"
                  label="同步间隔"
                  rules={[{ required: true, message: '请选择同步间隔' }]}
                >
                  <Select style={{ width: '100%' }} placeholder="选择同步间隔">
                    {SYNC_INTERVAL_OPTIONS.map(o => (
                      <Option key={o.value} value={o.value}>{o.label}</Option>
                    ))}
                  </Select>
                </Form.Item>
              ) : null
            }
          </Form.Item>
          <Form.Item name="remark" label="备注">
            <Input placeholder="可选" />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

// 手动输入域名表单（抽离为子组件）
const ManualAddForm: React.FC<{
  form: any
  loading: boolean
  onCancel: () => void
  onSubmit: () => void
}> = ({ form, loading, onCancel, onSubmit }) => (
  <>
    <Form form={form} layout="vertical">
      <Form.Item name="name" label="域名" rules={[{ required: true, message: '请输入域名' }]}
        extra="填写根域名，如：example.com">
        <Input placeholder="example.com" />
      </Form.Item>
      <Form.Item name="third_id" label="服务商域名ID"
        extra="可选，填写服务商侧的域名ID，用于精确匹配">
        <Input placeholder="可选" />
      </Form.Item>
      <Form.Item name="remark" label="备注">
        <Input placeholder="可选" />
      </Form.Item>
    </Form>
    <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 8 }}>
      <Button onClick={onCancel}>取消</Button>
      <Button type="primary" loading={loading} onClick={onSubmit}>添加</Button>
    </div>
  </>
)

export default DomainRecord
