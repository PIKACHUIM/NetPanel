import React, { useState, useEffect } from 'react'
import { Table, Button, Modal, Form, Input, Switch, Select, Space, message, Tag, Popconfirm, InputNumber } from 'antd'
import { PlusOutlined, EditOutlined, DeleteOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import request from '../api/request'
import { useTableStyle } from '../hooks/useTableStyle'

interface OAuthProviderItem {
  id: number
  name: string
  type: string
  client_id: string
  client_secret: string
  auth_url: string
  token_url: string
  userinfo_url: string
  issuer_url: string
  scopes: string
  redirect_uri: string
  icon: string
  display_order: number
  enable: boolean
  remark: string
}

const OAuthProviders: React.FC = () => {
  const { t } = useTranslation()
  const tableStyle = useTableStyle()
  const [data, setData] = useState<OAuthProviderItem[]>([])
  const [loading, setLoading] = useState(false)
  const [modalOpen, setModalOpen] = useState(false)
  const [editing, setEditing] = useState<OAuthProviderItem | null>(null)
  const [form] = Form.useForm()

  const fetchData = async () => {
    setLoading(true)
    try {
      const res: any = await request.get('/v1/admin/oauth-providers')
      setData(res.data || [])
    } catch {
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { fetchData() }, [])

  const handleCreate = () => {
    setEditing(null)
    form.resetFields()
    form.setFieldsValue({ type: 'oidc', scopes: 'openid profile email', enable: true, display_order: 0 })
    setModalOpen(true)
  }

  const handleEdit = (record: OAuthProviderItem) => {
    setEditing(record)
    form.setFieldsValue(record)
    setModalOpen(true)
  }

  const handleDelete = async (id: number) => {
    await request.delete(`/v1/admin/oauth-providers/${id}`)
    message.success(t('common.deleteSuccess') || '删除成功')
    fetchData()
  }

  const handleSubmit = async () => {
    const values = await form.validateFields()
    if (editing) {
      await request.put(`/v1/admin/oauth-providers/${editing.id}`, values)
      message.success(t('common.updateSuccess') || '更新成功')
    } else {
      await request.post('/v1/admin/oauth-providers', values)
      message.success(t('common.createSuccess') || '创建成功')
    }
    setModalOpen(false)
    fetchData()
  }

  const columns = [
    { title: '名称', dataIndex: 'name', key: 'name' },
    {
      title: '类型', dataIndex: 'type', key: 'type',
      render: (v: string) => <Tag color={v === 'oidc' ? 'blue' : 'purple'}>{v.toUpperCase()}</Tag>,
    },
    { title: 'Client ID', dataIndex: 'client_id', key: 'client_id', ellipsis: true },
    {
      title: '状态', dataIndex: 'enable', key: 'enable',
      render: (v: boolean) => <Tag color={v ? 'green' : 'default'}>{v ? '启用' : '禁用'}</Tag>,
    },
    { title: '排序', dataIndex: 'display_order', key: 'display_order', width: 80 },
    {
      title: '操作', key: 'actions', width: 140,
      render: (_: unknown, record: OAuthProviderItem) => (
        <Space>
          <Button type="link" icon={<EditOutlined />} onClick={() => handleEdit(record)} />
          <Popconfirm title="确认删除？" onConfirm={() => handleDelete(record.id)}>
            <Button type="link" danger icon={<DeleteOutlined />} />
          </Popconfirm>
        </Space>
      ),
    },
  ]

  const providerType = Form.useWatch('type', form)

  return (
    <div>
      <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <h2 style={{ margin: 0 }}>OAuth2/OIDC 登录管理</h2>
        <Button type="primary" icon={<PlusOutlined />} onClick={handleCreate}>
          添加 Provider
        </Button>
      </div>

      <Table
        dataSource={data}
        columns={columns}
        loading={loading}
        rowKey="id"
        pagination={false}
      />

      <Modal
        title={editing ? '编辑 OAuth Provider' : '添加 OAuth Provider'}
        open={modalOpen}
        onOk={handleSubmit}
        onCancel={() => setModalOpen(false)}
        width={640}
        destroyOnClose
      >
        <Form form={form} layout="vertical">
          <Form.Item name="name" label="名称" rules={[{ required: true }]}>
            <Input placeholder="如 Hiauth、Keycloak、Google" />
          </Form.Item>
          <Form.Item name="type" label="类型">
            <Select options={[
              { label: 'OpenID Connect (OIDC)', value: 'oidc' },
              { label: 'OAuth2 (自定义)', value: 'oauth2' },
            ]} />
          </Form.Item>
          <Form.Item name="client_id" label="Client ID" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="client_secret" label="Client Secret" rules={[{ required: !editing }]}>
            <Input.Password placeholder={editing ? '不修改请留空' : ''} />
          </Form.Item>

          {providerType === 'oidc' && (
            <Form.Item name="issuer_url" label="Issuer URL (OIDC Discovery)" extra="将自动从 .well-known/openid-configuration 获取 endpoints">
              <Input placeholder="https://auth.example.com/realms/master" />
            </Form.Item>
          )}

          {providerType === 'oauth2' && (
            <>
              <Form.Item name="auth_url" label="Authorization URL">
                <Input placeholder="https://auth.example.com/oauth/authorize" />
              </Form.Item>
              <Form.Item name="token_url" label="Token URL">
                <Input placeholder="https://auth.example.com/oauth/token" />
              </Form.Item>
              <Form.Item name="userinfo_url" label="UserInfo URL">
                <Input placeholder="https://auth.example.com/oauth/userinfo" />
              </Form.Item>
            </>
          )}

          <Form.Item name="scopes" label="Scopes">
            <Input placeholder="openid profile email" />
          </Form.Item>
          <Form.Item name="redirect_uri" label="Redirect URI" extra="回调地址，格式：http(s)://your-domain/api/v1/auth/oauth/{name}/callback">
            <Input placeholder="http://localhost:8080/api/v1/auth/oauth/{name}/callback" />
          </Form.Item>
          <Form.Item name="icon" label="图标">
            <Input placeholder="图标名称（可选）" />
          </Form.Item>
          <Form.Item name="display_order" label="显示顺序">
            <InputNumber min={0} />
          </Form.Item>
          <Form.Item name="enable" label="启用" valuePropName="checked">
            <Switch />
          </Form.Item>
          <Form.Item name="remark" label="备注">
            <Input.TextArea rows={2} />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

export default OAuthProviders
