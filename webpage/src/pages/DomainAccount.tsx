import React, { useEffect, useState } from 'react'
import {
  Table, Button, Space, Modal, Form, Input, Select,
  Popconfirm, message, Typography, Tag, Tooltip, Radio, Alert,
} from 'antd'
import {
  PlusOutlined, EditOutlined, DeleteOutlined, CheckCircleOutlined,
} from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { domainAccountApi } from '../api'

const { Option } = Select
const { Text } = Typography

// ===== 服务商配置 =====
// auth_fields 说明每个服务商支持的认证方式及字段
// mode: 'key_secret'  => AccessKey ID + Secret（阿里云、腾讯云等）
// mode: 'email_key'   => 邮箱 + API Key（Cloudflare Global API Key）
// mode: 'email_token' => 邮箱 + API Token（Cloudflare API Token，邮箱可选）
// mode: 'key_only'    => 只需一个 API Key/Token（NameSilo 等）
// mode: 'key_secret'  => Key + Secret（GoDaddy）

interface ProviderAuthMode {
  value: string
  label: string
  idLabel?: string       // access_id 字段的标签
  idPlaceholder?: string
  secretLabel: string    // access_secret 字段的标签
  secretPlaceholder: string
  needEmail?: boolean    // 是否需要邮箱
  emailRequired?: boolean
  emailLabel?: string
  emailPlaceholder?: string
  noId?: boolean         // 是否不需要 access_id（只有 secret）
}

interface ProviderConfig {
  value: string
  label: string
  color: string
  desc: string
  // 支持的认证模式列表（第一个为默认）
  authModes: ProviderAuthMode[]
}

const PROVIDERS: ProviderConfig[] = [
  {
    value: 'alidns',
    label: '阿里云 DNS',
    color: 'orange',
    desc: '阿里云域名解析服务',
    authModes: [
      {
        value: 'api_key',
        label: 'API 密钥',
        idLabel: 'AccessKey ID',
        idPlaceholder: 'AccessKey ID',
        secretLabel: 'AccessKey Secret',
        secretPlaceholder: 'AccessKey Secret',
      },
    ],
  },
  {
    value: 'cloudflare',
    label: 'Cloudflare',
    color: 'blue',
    desc: 'Cloudflare DNS 服务',
    authModes: [
      {
        value: 'api_token',
        label: 'API 令牌（推荐）',
        noId: true,
        secretLabel: 'API Token',
        secretPlaceholder: '输入 Cloudflare API Token',
        needEmail: false,
      },
      {
        value: 'api_key',
        label: 'Global API Key',
        noId: true,
        secretLabel: 'Global API Key',
        secretPlaceholder: '输入 Cloudflare Global API Key',
        needEmail: true,
        emailRequired: true,
        emailLabel: '邮箱地址',
        emailPlaceholder: 'Cloudflare 账号邮箱',
      },
    ],
  },
  {
    value: 'dnspod',
    label: 'DNSPod（腾讯云）',
    color: 'cyan',
    desc: '腾讯云 DNSPod 解析服务',
    authModes: [
      {
        value: 'api_key',
        label: 'SecretId + SecretKey',
        idLabel: 'SecretId',
        idPlaceholder: 'SecretId',
        secretLabel: 'SecretKey',
        secretPlaceholder: 'SecretKey',
      },
    ],
  },
  {
    value: 'huaweidns',
    label: '华为云 DNS',
    color: 'red',
    desc: '华为云域名解析服务',
    authModes: [
      {
        value: 'api_key',
        label: 'AccessKey ID + SecretKey',
        idLabel: 'AccessKey ID',
        idPlaceholder: 'AccessKey ID',
        secretLabel: 'SecretKey',
        secretPlaceholder: 'SecretKey',
      },
    ],
  },
  {
    value: 'godaddy',
    label: 'GoDaddy',
    color: 'green',
    desc: 'GoDaddy 域名解析服务',
    authModes: [
      {
        value: 'api_key',
        label: 'API Key + Secret',
        idLabel: 'API Key',
        idPlaceholder: 'GoDaddy API Key',
        secretLabel: 'API Secret',
        secretPlaceholder: 'GoDaddy API Secret',
      },
    ],
  },
  {
    value: 'namesilo',
    label: 'NameSilo',
    color: 'purple',
    desc: 'NameSilo 域名解析服务',
    authModes: [
      {
        value: 'api_key',
        label: 'API Key',
        noId: true,
        secretLabel: 'API Key',
        secretPlaceholder: 'NameSilo API Key',
      },
    ],
  },
  {
    value: 'tencenteo',
    label: '腾讯云 EdgeOne',
    color: 'geekblue',
    desc: '腾讯云 EdgeOne 解析服务',
    authModes: [
      {
        value: 'api_key',
        label: 'SecretId + SecretKey',
        idLabel: 'SecretId',
        idPlaceholder: 'SecretId',
        secretLabel: 'SecretKey',
        secretPlaceholder: 'SecretKey',
      },
    ],
  },
  {
    value: 'aliesa',
    label: '阿里云 ESA',
    color: 'volcano',
    desc: '阿里云 Edge Security Acceleration',
    authModes: [
      {
        value: 'api_key',
        label: 'AccessKey ID + Secret',
        idLabel: 'AccessKey ID',
        idPlaceholder: 'AccessKey ID',
        secretLabel: 'AccessKey Secret',
        secretPlaceholder: 'AccessKey Secret',
      },
    ],
  },
]

const getProviderConfig = (provider: string) =>
  PROVIDERS.find(p => p.value === provider)

const getAuthMode = (provider: string, authType: string) => {
  const cfg = getProviderConfig(provider)
  if (!cfg) return null
  return cfg.authModes.find(m => m.value === authType) || cfg.authModes[0]
}

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
      form.setFieldsValue({ ...record })
    } else {
      setEditRecord(null)
      form.resetFields()
      // 默认阿里云，api_key 模式
      form.setFieldsValue({ provider: 'alidns', auth_type: 'api_key', use_proxy: false })
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

  // 切换服务商时，自动设置默认认证方式
  const handleProviderChange = (provider: string) => {
    const cfg = getProviderConfig(provider)
    if (cfg) {
      form.setFieldsValue({
        auth_type: cfg.authModes[0].value,
        access_id: undefined,
        access_secret: undefined,
        email: undefined,
      })
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
        const p = getProviderConfig(v)
        return <Tag color={p?.color}>{p?.label || v}</Tag>
      },
    },
    {
      title: '邮箱', dataIndex: 'email',
      render: (v: string) => v ? <Text style={{ fontSize: 12 }}>{v}</Text> : <Text type="secondary">-</Text>,
    },
    {
      title: '认证方式', dataIndex: 'auth_type',
      render: (v: string, r: any) => {
        const mode = getAuthMode(r.provider, v)
        return <Tag color={v === 'api_token' ? 'green' : 'blue'}>{mode?.label || v}</Tag>
      },
    },
    {
      title: '密钥', dataIndex: 'access_secret',
      render: () => <Text type="secondary">••••••••</Text>,
    },
    {
      title: '代理', dataIndex: 'use_proxy', width: 60,
      render: (v: boolean) => v ? <Tag color="orange">是</Tag> : <Text type="secondary">否</Text>,
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
        width={520} destroyOnHidden
      >
        <Form form={form} layout="vertical" style={{ marginTop: 16 }}>
          {/* 账号名称 */}
          <Form.Item name="name" label={t('common.name')} rules={[{ required: true }]}>
            <Input placeholder="账号名称，如：我的 Cloudflare 账号" />
          </Form.Item>

          {/* DNS 服务商 */}
          <Form.Item name="provider" label={t('domainAccount.provider')} rules={[{ required: true }]}>
            <Select placeholder="选择 DNS 服务商" onChange={handleProviderChange}>
              {PROVIDERS.map(p => (
                <Option key={p.value} value={p.value}>
                  <Space>
                    <Tag color={p.color} style={{ margin: 0 }}>{p.label}</Tag>
                    <Text type="secondary" style={{ fontSize: 12 }}>{p.desc}</Text>
                  </Space>
                </Option>
              ))}
            </Select>
          </Form.Item>

          {/* 根据服务商动态显示认证字段 */}
          <Form.Item
            noStyle
            shouldUpdate={(prev, cur) => prev.provider !== cur.provider || prev.auth_type !== cur.auth_type}
          >
            {({ getFieldValue }) => {
              const provider: string = getFieldValue('provider')
              const authType: string = getFieldValue('auth_type')
              const cfg = getProviderConfig(provider)
              if (!cfg) return null

              const currentMode = getAuthMode(provider, authType) || cfg.authModes[0]
              const hasMultipleModes = cfg.authModes.length > 1

              return (
                <>
                  {/* 多种认证方式时显示切换 */}
                  {hasMultipleModes && (
                    <Form.Item name="auth_type" label="认证方式" rules={[{ required: true }]}>
                      <Radio.Group>
                        {cfg.authModes.map(m => (
                          <Radio key={m.value} value={m.value}>{m.label}</Radio>
                        ))}
                      </Radio.Group>
                    </Form.Item>
                  )}

                  {/* 邮箱字段（部分服务商/模式需要） */}
                  {currentMode.needEmail && (
                    <Form.Item
                      name="email"
                      label={currentMode.emailLabel || '邮箱地址'}
                      rules={[
                        { required: currentMode.emailRequired, message: '请输入邮箱地址' },
                        { type: 'email', message: '请输入有效的邮箱地址' },
                      ]}
                    >
                      <Input placeholder={currentMode.emailPlaceholder || '账号邮箱'} />
                    </Form.Item>
                  )}

                  {/* Access ID 字段（部分服务商不需要） */}
                  {!currentMode.noId && (
                    <Form.Item
                      name="access_id"
                      label={currentMode.idLabel || 'Access Key ID'}
                      rules={[{ required: true, message: `请输入 ${currentMode.idLabel || 'Access Key ID'}` }]}
                    >
                      <Input placeholder={currentMode.idPlaceholder || 'Access Key ID'} />
                    </Form.Item>
                  )}

                  {/* Secret / Token / Key 字段 */}
                  <Form.Item
                    name="access_secret"
                    label={currentMode.secretLabel}
                    rules={[{ required: true, message: `请输入 ${currentMode.secretLabel}` }]}
                    extra={provider === 'cloudflare' && authType === 'api_token'
                      ? '推荐使用 API Token，可精细控制权限范围'
                      : provider === 'cloudflare' && authType === 'api_key'
                        ? '在 Cloudflare 控制台 → My Profile → API Tokens → Global API Key 获取'
                        : undefined}
                  >
                    <Input.Password placeholder={currentMode.secretPlaceholder} />
                  </Form.Item>

                  {/* Cloudflare API Token 提示 */}
                  {provider === 'cloudflare' && authType === 'api_token' && (
                    <Alert
                      type="info"
                      showIcon
                      message="在 Cloudflare 控制台 → My Profile → API Tokens → Create Token 创建，建议授予 Zone:DNS:Edit 权限"
                      style={{ marginBottom: 16 }}
                    />
                  )}
                </>
              )
            }}
          </Form.Item>

          {/* 使用代理服务器 */}
          <Form.Item name="use_proxy" label="使用代理服务器">
            <Radio.Group>
              <Radio value={false}>否</Radio>
              <Radio value={true}>是</Radio>
            </Radio.Group>
          </Form.Item>

          {/* 备注 */}
          <Form.Item name="remark" label={t('common.remark')}>
            <Input.TextArea rows={2} placeholder="备注（可选）" />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

export default DomainAccount
