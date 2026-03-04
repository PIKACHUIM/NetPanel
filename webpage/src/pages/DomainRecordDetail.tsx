import React, { useEffect, useState } from 'react'
import {
  Table, Button, Space, Modal, Form, Input, Select, InputNumber,
  Popconfirm, message, Typography, Tag, Tooltip, Breadcrumb, Switch,
  Descriptions,
} from 'antd'
import {
  PlusOutlined, EditOutlined, DeleteOutlined, ReloadOutlined,
  SyncOutlined, ArrowLeftOutlined, CloudOutlined, ApiOutlined,
} from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { useNavigate, useParams, useLocation } from 'react-router-dom'
import dayjs from 'dayjs'
import { domainRecordApi } from '../api'

const { Option } = Select
const { Text } = Typography

const RECORD_TYPES = ['A', 'AAAA', 'CNAME', 'MX', 'TXT', 'NS', 'SRV', 'CAA', 'PTR']

const DomainRecordDetail: React.FC = () => {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { domainInfoId } = useParams<{ domainInfoId: string }>()
  const location = useLocation()
  const domainName = (location.state as any)?.domainName || ''
  // 从路由 state 获取账号服务商，用于判断是否显示 Proxied 字段
  const accountProvider: string = (location.state as any)?.accountProvider || ''
  const isCloudflare = accountProvider === 'cloudflare'

  const [data, setData] = useState<any[]>([])
  const [loading, setLoading] = useState(false)
  const [syncing, setSyncing] = useState(false)
  const [lastSyncTime, setLastSyncTime] = useState<string>('')
  const [modalOpen, setModalOpen] = useState(false)
  const [editRecord, setEditRecord] = useState<any>(null)
  const [form] = Form.useForm()
  // 监听 record_type 变化，用于 Proxied 字段的显示逻辑
  const recordType = Form.useWatch('record_type', form)

  const fetchData = async () => {
    if (!domainInfoId) return
    setLoading(true)
    try {
      const res: any = await domainRecordApi.list({ domain_info_id: Number(domainInfoId) })
      setData(res.data || [])
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { fetchData() }, [domainInfoId])

  const handleAdd = () => {
    setEditRecord(null)
    form.resetFields()
    form.setFieldsValue({ record_type: 'A', ttl: 600, proxied: false })
    setModalOpen(true)
  }

  const handleEdit = (record: any) => {
    setEditRecord(record)
    form.setFieldsValue({
      record_type: record.record_type,
      host: record.host,
      value: record.value,
      ttl: record.ttl,
      proxied: record.proxied || false,
      remark: record.remark,
    })
    setModalOpen(true)
  }

  const handleSubmit = async () => {
    const values = await form.validateFields()
    if (editRecord) {
      await domainRecordApi.update(editRecord.id, { ...editRecord, ...values })
      message.success('更新成功')
    } else {
      await domainRecordApi.create({ ...values, domain_info_id: Number(domainInfoId) })
      message.success('添加成功')
    }
    setModalOpen(false)
    fetchData()
  }

  const handleDelete = async (id: number) => {
    await domainRecordApi.delete(id)
    message.success('删除成功')
    fetchData()
  }

  const handleSync = async () => {
    if (!domainInfoId) return
    setSyncing(true)
    try {
      const res: any = await domainRecordApi.sync(Number(domainInfoId))
      const count = res.data?.count ?? 0
      message.success(`同步完成，共 ${count} 条记录`)
      setLastSyncTime(dayjs().format('HH:mm:ss'))
      fetchData()
    } finally {
      setSyncing(false)
    }
  }

  // Proxied 字段仅对 Cloudflare 的 A/AAAA/CNAME 记录显示
  const showProxied = isCloudflare && ['A', 'AAAA', 'CNAME'].includes(recordType)

  const recordTypeColor: Record<string, string> = {
    A: 'blue', AAAA: 'geekblue', CNAME: 'cyan', MX: 'purple',
    TXT: 'orange', NS: 'green', SRV: 'magenta', CAA: 'gold', PTR: 'lime',
  }

  const columns = [
    {
      title: 'ID', dataIndex: 'id', width: 60,
      render: (v: number) => <Text type="secondary" style={{ fontSize: 12 }}>#{v}</Text>,
    },
    {
      title: '记录类型', dataIndex: 'record_type', width: 100,
      render: (v: string) => <Tag color={recordTypeColor[v] || 'default'}>{v}</Tag>,
    },
    {
      title: '主机记录', dataIndex: 'host', width: 160,
      render: (v: string, r: any) => (
        <Text code style={{ fontSize: 12 }}>{v || '@'}.{r.domain || domainName}</Text>
      ),
    },
    {
      title: '记录值', dataIndex: 'value',
      render: (v: string) => <Text style={{ fontSize: 12, wordBreak: 'break-all' }}>{v}</Text>,
    },
    {
      title: 'TTL', dataIndex: 'ttl', width: 80,
      render: (v: number) => <Text type="secondary">{v}s</Text>,
    },
    // 仅 Cloudflare 显示代理状态列
    ...(isCloudflare ? [{
      title: 'CDN代理', dataIndex: 'proxied', width: 90,
      render: (v: boolean, r: any) => {
        // 只有 A/AAAA/CNAME 才支持代理
        if (!['A', 'AAAA', 'CNAME'].includes(r.record_type)) return <Text type="secondary">-</Text>
        return v
          ? <Tag color="orange" icon={<CloudOutlined />}>代理中</Tag>
          : <Tag icon={<ApiOutlined />}>仅DNS</Tag>
      },
    }] : []),
    {
      title: '备注', dataIndex: 'remark', width: 120,
      render: (v: string) => v ? <Text type="secondary" style={{ fontSize: 12 }}>{v}</Text> : '-',
    },
    {
      title: '添加时间', dataIndex: 'created_at', width: 150,
      render: (v: string) => v ? dayjs(v).format('YYYY-MM-DD HH:mm') : '-',
    },
    {
      title: '操作', width: 120, fixed: 'right' as const,
      render: (_: any, r: any) => (
        <Space size={4}>
          <Tooltip title={t('common.edit')}>
            <Button size="small" icon={<EditOutlined />} onClick={() => handleEdit(r)} />
          </Tooltip>
          <Popconfirm title={t('common.deleteConfirm')} onConfirm={() => handleDelete(r.id)}>
            <Tooltip title={t('common.delete')}>
              <Button size="small" danger icon={<DeleteOutlined />} />
            </Tooltip>
          </Popconfirm>
        </Space>
      ),
    },
  ]

  return (
    <div>
      {/* 面包屑 */}
      <Breadcrumb
        style={{ marginBottom: 12 }}
        items={[
          {
            title: (
              <span style={{ cursor: 'pointer' }} onClick={() => navigate('/domain/info')}>
                <ArrowLeftOutlined style={{ marginRight: 4 }} />
                DNS 域名解析
              </span>
            ),
          },
          { title: domainName || `域名 #${domainInfoId}` },
        ]}
      />

      {/* 标题栏 */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Typography.Title level={4} style={{ margin: 0 }}>
          {domainName || `域名 #${domainInfoId}`} — 解析记录
        </Typography.Title>
        <Space>
          <Tooltip title={lastSyncTime ? `上次同步：${lastSyncTime}` : '从服务商同步解析记录'}>
            <Button icon={<SyncOutlined spin={syncing} />} loading={syncing} onClick={handleSync}>
              同步{lastSyncTime ? ` (${lastSyncTime})` : ''}
            </Button>
          </Tooltip>
          <Tooltip title="刷新列表">
            <Button icon={<ReloadOutlined />} onClick={fetchData} loading={loading} />
          </Tooltip>
          <Button type="primary" icon={<PlusOutlined />} onClick={handleAdd}>
            添加记录
          </Button>
        </Space>
      </div>

      <Table
        dataSource={data}
        columns={columns}
        rowKey="id"
        loading={loading}
        size="middle"
        scroll={{ x: isCloudflare ? 1000 : 900 }}
        style={{ background: '#fff', borderRadius: 8 }}
        pagination={{ pageSize: 50, showSizeChanger: true, showTotal: total => `共 ${total} 条` }}
      />

      {/* 添加/编辑弹窗 */}
      <Modal
        title={editRecord ? '编辑解析记录' : '添加解析记录'}
        open={modalOpen}
        onOk={handleSubmit}
        onCancel={() => setModalOpen(false)}
        width={480}
        destroyOnHidden
      >
        <Form form={form} layout="vertical" style={{ marginTop: 16 }}>
          <Form.Item name="record_type" label="记录类型" rules={[{ required: true }]}>
            <Select>
              {RECORD_TYPES.map(t => <Option key={t} value={t}>{t}</Option>)}
            </Select>
          </Form.Item>
          <Form.Item name="host" label="主机记录" rules={[{ required: true, message: '请输入主机记录' }]}
            extra="填写子域名前缀，如 www、@ 表示根域名">
            <Input placeholder="@ 或 www 或 mail" />
          </Form.Item>
          <Form.Item name="value" label="记录值" rules={[{ required: true, message: '请输入记录值' }]}>
            <Input.TextArea rows={2} placeholder="IP地址、域名或文本内容" />
          </Form.Item>
          <Form.Item name="ttl" label="TTL（秒）">
            <InputNumber min={1} max={86400} style={{ width: '100%' }} />
          </Form.Item>
          {/* CDN 代理：仅 Cloudflare 的 A/AAAA/CNAME 记录显示 */}
          {showProxied && (
            <Form.Item
              name="proxied"
              label="CDN 代理"
              valuePropName="checked"
              extra={
                <span style={{ fontSize: 12 }}>
                  开启后流量经过 Cloudflare CDN（橙色云朵），关闭则仅 DNS 解析（灰色云朵）
                </span>
              }
            >
              <Switch
                checkedChildren={<><CloudOutlined /> 代理中</>}
                unCheckedChildren={<><ApiOutlined /> 仅DNS</>}
              />
            </Form.Item>
          )}
          <Form.Item name="remark" label="备注">
            <Input placeholder="可选" />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

export default DomainRecordDetail
