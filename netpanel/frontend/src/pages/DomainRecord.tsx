import React, { useEffect, useState } from 'react'
import {
  Table, Button, Space, Modal, Form, Input, Select, InputNumber,
  Popconfirm, message, Typography, Tag, Tooltip,
} from 'antd'
import {
  PlusOutlined, EditOutlined, DeleteOutlined, SyncOutlined,
} from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { domainRecordApi, domainAccountApi } from '../api'

const { Option } = Select
const { Text } = Typography

const RECORD_TYPES = ['A', 'AAAA', 'CNAME', 'MX', 'TXT', 'NS', 'SRV', 'CAA']

const RECORD_TYPE_COLORS: Record<string, string> = {
  A: 'blue', AAAA: 'purple', CNAME: 'green', MX: 'orange',
  TXT: 'cyan', NS: 'geekblue', SRV: 'magenta', CAA: 'gold',
}

const DomainRecord: React.FC = () => {
  const { t } = useTranslation()
  const [data, setData] = useState<any[]>([])
  const [accounts, setAccounts] = useState<any[]>([])
  const [selectedAccount, setSelectedAccount] = useState<number | null>(null)
  const [loading, setLoading] = useState(false)
  const [syncingId, setSyncingId] = useState<number | null>(null)
  const [modalOpen, setModalOpen] = useState(false)
  const [editRecord, setEditRecord] = useState<any>(null)
  const [form] = Form.useForm()

  const fetchData = async (accountId?: number) => {
    setLoading(true)
    try {
      const [recRes, accRes]: any[] = await Promise.all([
        domainRecordApi.list(accountId ? { account_id: accountId } : undefined),
        domainAccountApi.list(),
      ])
      setData(recRes.data || [])
      setAccounts(accRes.data || [])
    } finally { setLoading(false) }
  }
  useEffect(() => { fetchData() }, [])

  const handleOpen = (record?: any) => {
    if (record) {
      setEditRecord(record)
      form.setFieldsValue(record)
    } else {
      setEditRecord(null)
      form.resetFields()
      form.setFieldsValue({
        record_type: 'A', ttl: 600,
        account_id: selectedAccount || undefined,
      })
    }
    setModalOpen(true)
  }

  const handleSubmit = async () => {
    const values = await form.validateFields()
    if (editRecord) {
      await domainRecordApi.update(editRecord.id, values)
    } else {
      await domainRecordApi.create(values)
    }
    message.success(t('common.success'))
    setModalOpen(false)
    fetchData(selectedAccount || undefined)
  }

  const handleSync = async (accountId: number) => {
    setSyncingId(accountId)
    try {
      await domainRecordApi.sync(accountId)
      message.success('同步成功')
      fetchData(selectedAccount || undefined)
    } finally {
      setSyncingId(null)
    }
  }

  const columns = [
    {
      title: '账号', dataIndex: 'account_id',
      render: (v: number) => {
        const acc = accounts.find(a => a.id === v)
        return acc ? <Tag color="blue">{acc.name}</Tag> : <Text type="secondary">{v}</Text>
      },
    },
    {
      title: t('domainRecord.host'), dataIndex: 'host',
      render: (v: string) => <Text code>{v}</Text>,
    },
    {
      title: t('domainRecord.recordType'), dataIndex: 'record_type', width: 80,
      render: (v: string) => <Tag color={RECORD_TYPE_COLORS[v] || 'default'}>{v}</Tag>,
    },
    {
      title: t('domainRecord.value'), dataIndex: 'value',
      render: (v: string) => <Text style={{ fontSize: 12 }}>{v}</Text>,
    },
    {
      title: t('domainRecord.ttl'), dataIndex: 'ttl', width: 80,
      render: (v: number) => v ? `${v}s` : 'auto',
    },
    {
      title: t('common.action'), width: 120,
      render: (_: any, r: any) => (
        <Space size={4}>
          <Tooltip title={t('common.edit')}>
            <Button size="small" icon={<EditOutlined />} onClick={() => handleOpen(r)} />
          </Tooltip>
          <Popconfirm title={t('common.deleteConfirm')} onConfirm={async () => { await domainRecordApi.delete(r.id); fetchData(selectedAccount || undefined) }}>
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
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Typography.Title level={4} style={{ margin: 0 }}>{t('domainRecord.title')}</Typography.Title>
        <Space>
          <Select
            placeholder="筛选账号"
            allowClear
            style={{ width: 160 }}
            onChange={(v) => { setSelectedAccount(v || null); fetchData(v || undefined) }}
          >
            {accounts.map(a => <Option key={a.id} value={a.id}>{a.name}</Option>)}
          </Select>
          {selectedAccount && (
            <Tooltip title="从服务商同步解析记录">
              <Button
                icon={<SyncOutlined />}
                loading={syncingId === selectedAccount}
                onClick={() => handleSync(selectedAccount)}
              >
                同步
              </Button>
            </Tooltip>
          )}
          <Button type="primary" icon={<PlusOutlined />} onClick={() => handleOpen()}>
            {t('common.create')}
          </Button>
        </Space>
      </div>

      <Table
        dataSource={data} columns={columns} rowKey="id" loading={loading}
        size="middle" style={{ background: '#fff', borderRadius: 8 }}
        pagination={{ pageSize: 20, showSizeChanger: true }}
      />

      <Modal
        title={editRecord ? t('common.edit') : t('common.create')}
        open={modalOpen} onOk={handleSubmit} onCancel={() => setModalOpen(false)}
        width={480} destroyOnClose
      >
        <Form form={form} layout="vertical" style={{ marginTop: 16 }}>
          <Form.Item name="account_id" label="域名账号" rules={[{ required: true }]}>
            <Select placeholder="选择域名账号">
              {accounts.map(a => <Option key={a.id} value={a.id}>{a.name}</Option>)}
            </Select>
          </Form.Item>

          <Form.Item name="host" label={t('domainRecord.host')} rules={[{ required: true }]}
            extra="@ 表示根域名，* 表示通配符，或填写子域名如：www">
            <Input placeholder="@ 或 www 或 sub" />
          </Form.Item>

          <Form.Item name="record_type" label={t('domainRecord.recordType')} rules={[{ required: true }]}>
            <Select>
              {RECORD_TYPES.map(t => <Option key={t} value={t}>{t}</Option>)}
            </Select>
          </Form.Item>

          <Form.Item name="value" label={t('domainRecord.value')} rules={[{ required: true }]}
            extra="A记录填IP，CNAME填域名，MX填邮件服务器，TXT填文本">
            <Input placeholder="记录值" />
          </Form.Item>

          <Form.Item name="ttl" label={t('domainRecord.ttl')}>
            <InputNumber min={1} style={{ width: '100%' }} placeholder="600（秒，留空自动）" />
          </Form.Item>

          <Form.Item
            noStyle
            shouldUpdate={(prev, cur) => prev.record_type !== cur.record_type}
          >
            {({ getFieldValue }) => getFieldValue('record_type') === 'MX' && (
              <Form.Item name="priority" label="优先级">
                <InputNumber min={0} max={65535} style={{ width: '100%' }} placeholder="10" />
              </Form.Item>
            )}
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

export default DomainRecord
