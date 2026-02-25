import React, { useEffect, useState } from 'react'
import {
  Table, Button, Space, Modal, Form, Input, Select,
  Popconfirm, message, Typography, Tag, Tooltip,
} from 'antd'
import {
  PlusOutlined, EditOutlined, DeleteOutlined, CheckCircleOutlined,
} from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { domainAccountApi } from '../api'

const { Option } = Select
const { Text } = Typography

const PROVIDERS = [
  { value: 'alidns', label: '阿里云 DNS', color: 'orange' },
  { value: 'cloudflare', label: 'Cloudflare', color: 'blue' },
  { value: 'dnspod', label: 'DNSPod (腾讯云)', color: 'cyan' },
  { value: 'huaweidns', label: '华为云 DNS', color: 'red' },
  { value: 'godaddy', label: 'GoDaddy', color: 'green' },
  { value: 'namesilo', label: 'NameSilo', color: 'purple' },
  { value: 'tencenteo', label: '腾讯云 EdgeOne', color: 'geekblue' },
  { value: 'aliesa', label: '阿里云 ESA', color: 'volcano' },
]

const DomainAccount: React.FC = () => {
  const { t } = useTranslation()
  const [data, setData] = useState<any[]>([])
  const [loading, setLoading] = useState(false)
  const [modalOpen, setModalOpen] = useState(false)
  const [editRecord, setEditRecord] = useState<any>(null)
  const [testingId, setTestingId] = useState<number | null>(null)
  const [form] = Form.useForm()

  const fetchData = async () => {
    setLoading(true)
    try { const res: any = await domainAccountApi.list(); setData(res.data || []) }
    finally { setLoading(false) }
  }
  useEffect(() => { fetchData() }, [])

  const handleOpen = (record?: any) => {
    if (record) {
      setEditRecord(record)
      form.setFieldsValue(record)
    } else {
      setEditRecord(null)
      form.resetFields()
    }
    setModalOpen(true)
  }

  const handleSubmit = async () => {
    const values = await form.validateFields()
    if (editRecord) {
      await domainAccountApi.update(editRecord.id, values)
    } else {
      await domainAccountApi.create(values)
    }
    message.success(t('common.success'))
    setModalOpen(false)
    fetchData()
  }

  const handleTest = async (id: number) => {
    setTestingId(id)
    try {
      await domainAccountApi.test(id)
      message.success('连接测试成功！')
    } catch {
      // 错误已在拦截器处理
    } finally {
      setTestingId(null)
    }
  }

  const columns = [
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
      title: t('domainAccount.provider'), dataIndex: 'provider',
      render: (v: string) => {
        const p = PROVIDERS.find(p => p.value === v)
        return <Tag color={p?.color}>{p?.label || v}</Tag>
      },
    },
    {
      title: t('domainAccount.accessID'), dataIndex: 'access_id',
      render: (v: string) => <Text code style={{ fontSize: 12 }}>{v}</Text>,
    },
    {
      title: t('domainAccount.accessSecret'), dataIndex: 'access_secret',
      render: () => <Text type="secondary">••••••••</Text>,
    },
    {
      title: t('common.action'), width: 160,
      render: (_: any, r: any) => (
        <Space size={4}>
          <Tooltip title="测试连接">
            <Button
              size="small" icon={<CheckCircleOutlined />}
              loading={testingId === r.id}
              onClick={() => handleTest(r.id)}
            />
          </Tooltip>
          <Tooltip title={t('common.edit')}>
            <Button size="small" icon={<EditOutlined />} onClick={() => handleOpen(r)} />
          </Tooltip>
          <Popconfirm title={t('common.deleteConfirm')} onConfirm={async () => { await domainAccountApi.delete(r.id); fetchData() }}>
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
        <Typography.Title level={4} style={{ margin: 0 }}>{t('domainAccount.title')}</Typography.Title>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => handleOpen()}>
          {t('common.create')}
        </Button>
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
          <Form.Item name="name" label={t('common.name')} rules={[{ required: true }]}>
            <Input placeholder="账号名称，如：我的阿里云" />
          </Form.Item>

          <Form.Item name="provider" label={t('domainAccount.provider')} rules={[{ required: true }]}>
            <Select placeholder="选择DNS服务商">
              {PROVIDERS.map(p => <Option key={p.value} value={p.value}>{p.label}</Option>)}
            </Select>
          </Form.Item>

          <Form.Item
            noStyle
            shouldUpdate={(prev, cur) => prev.provider !== cur.provider}
          >
            {({ getFieldValue }) => {
              const provider = getFieldValue('provider')
              const isToken = provider === 'cloudflare' || provider === 'namesilo'
              return isToken ? (
                <Form.Item name="access_secret" label="API Token" rules={[{ required: true }]}>
                  <Input.Password placeholder="API Token" />
                </Form.Item>
              ) : (
                <>
                  <Form.Item name="access_id" label={t('domainAccount.accessID')} rules={[{ required: true }]}>
                    <Input placeholder="Access Key ID / App ID" />
                  </Form.Item>
                  <Form.Item name="access_secret" label={t('domainAccount.accessSecret')} rules={[{ required: true }]}>
                    <Input.Password placeholder="Access Key Secret" />
                  </Form.Item>
                </>
              )
            }}
          </Form.Item>

          <Form.Item name="remark" label={t('common.remark')}>
            <Input.TextArea rows={2} placeholder="备注（可选）" />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

export default DomainAccount
