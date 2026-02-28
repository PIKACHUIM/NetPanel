import React, { useEffect, useState } from 'react'
import {
  Table, Button, Space, Modal, Form, Input, Select,
  Popconfirm, message, Typography, Tag, Tooltip, Alert,
} from 'antd'
import {
  PlusOutlined, EditOutlined, DeleteOutlined, SafetyCertificateOutlined,
} from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { certAccountApi } from '../api'

const { Option } = Select
const { Text } = Typography

const CA_TYPES = [
  { value: 'letsencrypt', label: "Let's Encrypt", color: 'green', desc: '免费、自动化、开放的证书颁发机构，最广泛使用' },
  { value: 'zerossl', label: 'ZeroSSL', color: 'blue', desc: '免费SSL证书，需要EAB凭据，支持更多功能' },
  { value: 'buypass', label: 'Buypass', color: 'purple', desc: '挪威CA机构，提供免费90天证书' },
  { value: 'google', label: 'Google Trust Services', color: 'red', desc: 'Google公共CA服务，需要EAB凭据绑定账户' },
]

// 需要 EAB 的 CA 类型
const EAB_REQUIRED_CAS = ['zerossl', 'google']

// EAB 提示文案
const EAB_TIPS: Record<string, string> = {
  zerossl: 'ZeroSSL 需要 EAB（External Account Binding）凭据。请登录 ZeroSSL 控制台 → Developer → EAB Credentials 获取。',
  google: 'Google Trust Services 需要 EAB 凭据绑定账户。请在 Google Cloud Console 中申请 Public CA API 并获取 EAB Key。',
}

const CertAccount: React.FC = () => {
  const { t } = useTranslation()
  const [data, setData] = useState<any[]>([])
  const [loading, setLoading] = useState(false)
  const [modalOpen, setModalOpen] = useState(false)
  const [editRecord, setEditRecord] = useState<any>(null)
  const [verifyingId, setVerifyingId] = useState<number | null>(null)
  const [form] = Form.useForm()

  const fetchData = async () => {
    setLoading(true)
    try {
      const res: any = await certAccountApi.list()
      setData(res.data || [])
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { fetchData() }, [])

  const handleOpen = (record?: any) => {
    if (record) {
      setEditRecord(record)
      // eab_kid / eab_hmac_key 已是独立字段，直接回填
      form.setFieldsValue({ ...record })
    } else {
      setEditRecord(null)
      form.resetFields()
      form.setFieldsValue({ type: 'letsencrypt' })
    }
    setModalOpen(true)
  }

  const handleSubmit = async () => {
    const values = await form.validateFields()
    if (editRecord) {
      await certAccountApi.update(editRecord.id, values)
    } else {
      await certAccountApi.create(values)
    }
    message.success(t('common.success'))
    setModalOpen(false)
    fetchData()
  }

  const handleVerify = async (id: number) => {
    setVerifyingId(id)
    try {
      await certAccountApi.verify(id)
      message.success(t('certAccount.verifySuccess'))
    } finally {
      setVerifyingId(null)
    }
  }

  const columns = [
    {
      title: t('common.name'),
      dataIndex: 'name',
      render: (name: string, r: any) => (
        <div>
          <Text strong>{name}</Text>
          {r.remark && (
            <div><Text type="secondary" style={{ fontSize: 12 }}>{r.remark}</Text></div>
          )}
        </div>
      ),
    },
    {
      title: t('certAccount.type'),
      dataIndex: 'type',
      render: (v: string) => {
        const ca = CA_TYPES.find(c => c.value === v)
        return <Tag color={ca?.color}>{ca?.label || v}</Tag>
      },
    },
    {
      title: t('certAccount.email'),
      dataIndex: 'email',
      render: (v: string) => v ? <Text code style={{ fontSize: 12 }}>{v}</Text> : <Text type="secondary">-</Text>,
    },
    {
      title: t('common.action'),
      width: 160,
      render: (_: any, r: any) => (
        <Space size={4}>
          <Tooltip title={t('certAccount.verify')}>
            <Button
              size="small"
              icon={<SafetyCertificateOutlined />}
              loading={verifyingId === r.id}
              onClick={() => handleVerify(r.id)}
            />
          </Tooltip>
          <Tooltip title={t('common.edit')}>
            <Button size="small" icon={<EditOutlined />} onClick={() => handleOpen(r)} />
          </Tooltip>
          <Popconfirm
            title={t('common.deleteConfirm')}
            onConfirm={async () => { await certAccountApi.delete(r.id); fetchData() }}
          >
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
        <Typography.Title level={4} style={{ margin: 0 }}>{t('certAccount.title')}</Typography.Title>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => handleOpen()}>
          {t('common.create')}
        </Button>
      </div>

      <Table
        dataSource={data}
        columns={columns}
        rowKey="id"
        loading={loading}
        size="middle"
        style={{ background: '#fff', borderRadius: 8 }}
        pagination={{ pageSize: 20, showSizeChanger: true }}
      />

      <Modal
        title={editRecord ? t('common.edit') : t('common.create')}
        open={modalOpen}
        onOk={handleSubmit}
        onCancel={() => setModalOpen(false)}
        width={520}
        destroyOnHidden
      >
        <Form form={form} layout="vertical" style={{ marginTop: 16 }}>
          <Form.Item name="name" label={t('common.name')} rules={[{ required: true }]}>
            <Input placeholder="账号名称，如：我的 Let's Encrypt 账号" />
          </Form.Item>

          <Form.Item name="type" label={t('certAccount.type')} rules={[{ required: true }]}>
            <Select placeholder="选择 CA 类型">
              {CA_TYPES.map(ca => (
                <Option key={ca.value} value={ca.value}>
                  <Space>
                    <Tag color={ca.color} style={{ margin: 0 }}>{ca.label}</Tag>
                    <Text type="secondary" style={{ fontSize: 12 }}>{ca.desc}</Text>
                  </Space>
                </Option>
              ))}
            </Select>
          </Form.Item>

          <Form.Item name="email" label={t('certAccount.email')} rules={[{ required: true, type: 'email' }]}>
            <Input placeholder="用于接收证书到期提醒邮件" />
          </Form.Item>

          {/* ZeroSSL / Google Trust Services 需要 EAB 凭据 */}
          <Form.Item noStyle shouldUpdate={(prev, cur) => prev.type !== cur.type}>
            {({ getFieldValue }) => {
              const caType = getFieldValue('type')
              if (!EAB_REQUIRED_CAS.includes(caType)) return null
              return (
                <>
                  <Alert
                    type="info"
                    showIcon
                    message={EAB_TIPS[caType] || t('certAccount.eabTip')}
                    style={{ marginBottom: 16 }}
                  />
                  <Form.Item
                    name="eab_kid"
                    label={t('certAccount.eabKid')}
                    rules={[{ required: true, message: '请输入 EAB Key ID' }]}
                    extra="EAB Key ID（外部账户绑定标识符）"
                  >
                    <Input placeholder="EAB Key ID" />
                  </Form.Item>
                  <Form.Item
                    name="eab_hmac_key"
                    label={t('certAccount.eabHmacKey')}
                    rules={[{ required: true, message: '请输入 EAB HMAC Key' }]}
                    extra="EAB HMAC Key（Base64 编码的密钥）"
                  >
                    <Input.Password placeholder="EAB HMAC Key（Base64）" />
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

export default CertAccount
