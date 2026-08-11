import React, { useEffect, useState } from 'react'
import { Table, Button, Space, Switch, Modal, Form, Input, Select, Popconfirm, message, Typography, Tag, Tooltip, Divider } from 'antd'
import { PlusOutlined, EditOutlined, DeleteOutlined, LockOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { accessApi } from '../api'
import { useTableStyle } from '../hooks/useTableStyle'

const { Option } = Select

interface UserItem {
  id: number
  username: string
  email: string
  enable: boolean
  is_admin: boolean
  remark: string
}

const Access: React.FC = () => {
  const { t } = useTranslation()
  const tableStyle = useTableStyle()
  const [data, setData] = useState<any[]>([])
  const [loading, setLoading] = useState(false)
  const [modalOpen, setModalOpen] = useState(false)
  const [editRecord, setEditRecord] = useState<any>(null)
  const [form] = Form.useForm()
  // IPDB 条目和 Caddy 站点列表（从 List 接口获取）
  const [ipdbEntries, setIpdbEntries] = useState<any[]>([])
  const [caddySites, setCaddySites] = useState<any[]>([])
  const [users, setUsers] = useState<UserItem[]>([])
  // 控制用户认证表单区域的显隐
  const [authMode, setAuthMode] = useState<string>('')

  const fetchData = async () => {
    setLoading(true)
    try {
      const res: any = await accessApi.list()
      setData(res.data || [])
      setIpdbEntries(res.ipdb_entries || [])
      setCaddySites(res.caddy_sites || [])
      setUsers(res.users || [])
    } finally {
      setLoading(false)
    }
  }
  useEffect(() => { fetchData() }, [])

  // 解析 JSON 数组字段为数字数组
  const parseIds = (val: any): number[] => {
    if (!val) return []
    if (Array.isArray(val)) return val
    try { return JSON.parse(val) } catch { return [] }
  }

  const handleSubmit = async () => {
    const values = await form.validateFields()
    // 将手动 IP 列表转为 JSON 数组
    if (typeof values.ip_list === 'string') {
      values.ip_list = JSON.stringify(values.ip_list.split('\n').filter(Boolean))
    } else if (!values.ip_list) {
      values.ip_list = '[]'
    }
    // 将 IPDB IDs 和 Site IDs 转为 JSON 数组
    values.bind_ipdb_ids = JSON.stringify(values.bind_ipdb_ids || [])
    values.bind_site_ids = JSON.stringify(values.bind_site_ids || [])
    // 将允许用户 IDs 转为 JSON 数组
    values.allowed_user_ids = JSON.stringify(values.allowed_user_ids || [])
    // 空 auth_mode 设为空字符串
    if (!values.auth_mode) values.auth_mode = ''

    editRecord ? await accessApi.update(editRecord.id, values) : await accessApi.create(values)
    message.success(t('common.success'))
    setModalOpen(false)
    fetchData()
  }

  // 根据 IPDB ID 列表获取显示文本
  const renderIpdbTags = (idsJson: string) => {
    const ids = parseIds(idsJson)
    if (ids.length === 0) return '-'
    return ids.map(id => {
      const entry = ipdbEntries.find((e: any) => e.id === id)
      return entry ? (
        <Tag key={id} color="blue" style={{ marginBottom: 2 }}>
          <Tooltip title={`${entry.cidr}${entry.location ? ' - ' + entry.location : ''}`}>
            {entry.cidr}
          </Tooltip>
        </Tag>
      ) : <Tag key={id} color="default">ID:{id}</Tag>
    })
  }

  // 根据 Site ID 列表获取显示文本
  const renderSiteTags = (idsJson: string) => {
    const ids = parseIds(idsJson)
    if (ids.length === 0) return <Tag color="purple">{t('access.allSites')}</Tag>
    return ids.map(id => {
      const site = caddySites.find((s: any) => s.id === id)
      return site ? (
        <Tag key={id} color="cyan" style={{ marginBottom: 2 }}>
          {site.name || site.domain || `#${id}`}
        </Tag>
      ) : <Tag key={id} color="default">ID:{id}</Tag>
    })
  }

  // 渲染认证方式标签
  const renderAuthMode = (mode: string, record: any) => {
    if (!mode) return <Tag color="default">无需登录</Tag>
    const allowedIds = parseIds(record.allowed_user_ids)
    const userCount = allowedIds.length > 0 ? `(${allowedIds.length}人)` : '(全部用户)'
    if (mode === 'basic_auth') return <Tag color="orange" icon={<LockOutlined />}>Basic Auth {userCount}</Tag>
    if (mode === 'page_login') return <Tag color="green" icon={<LockOutlined />}>页面登录 {userCount}</Tag>
    return <Tag color="default">{mode}</Tag>
  }

  const columns = [
    { title: t('common.enable'), dataIndex: 'enable', width: 80, render: (v: boolean, r: any) => <Switch size="small" checked={v} onChange={async (c) => { await accessApi.update(r.id, { ...r, enable: c }); fetchData() }} /> },
    { title: t('common.name'), dataIndex: 'name' },
    { title: t('access.mode'), dataIndex: 'mode', width: 100, render: (v: string) => <Tag color={v === 'blacklist' ? 'red' : 'green'}>{v === 'blacklist' ? t('access.blacklist') : t('access.whitelist')}</Tag> },
    { title: t('access.bindIpdb'), dataIndex: 'bind_ipdb_ids', render: (v: string) => renderIpdbTags(v) },
    { title: t('access.bindSites'), dataIndex: 'bind_site_ids', render: (v: string) => renderSiteTags(v) },
    { title: '登录策略', dataIndex: 'auth_mode', width: 160, render: (v: string, r: any) => renderAuthMode(v, r) },
    { title: t('common.remark'), dataIndex: 'remark', render: (v: string) => v || '-' },
    { title: t('common.action'), width: 120, render: (_: any, r: any) => (
      <Space size={4}>
        <Button size="small" icon={<EditOutlined />} onClick={() => {
          setEditRecord(r)
          const formValues = {
            ...r,
            ip_list: (() => { try { return JSON.parse(r.ip_list || '[]').join('\n') } catch { return '' } })(),
            bind_ipdb_ids: parseIds(r.bind_ipdb_ids),
            bind_site_ids: parseIds(r.bind_site_ids),
            auth_mode: r.auth_mode || '',
            allowed_user_ids: parseIds(r.allowed_user_ids),
          }
          form.setFieldsValue(formValues)
          setAuthMode(r.auth_mode || '')
          setModalOpen(true)
        }} />
        <Popconfirm title={t('common.deleteConfirm')} onConfirm={async () => { await accessApi.delete(r.id); fetchData() }}><Button size="small" danger icon={<DeleteOutlined />} /></Popconfirm>
      </Space>
    )},
  ]

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Typography.Title level={4} style={{ margin: 0 }}>{t('access.title')}</Typography.Title>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => { setEditRecord(null); form.resetFields(); form.setFieldsValue({ enable: true, mode: 'blacklist', bind_ipdb_ids: [], bind_site_ids: [], auth_mode: '', allowed_user_ids: [] }); setAuthMode(''); setModalOpen(true) }}>{t('common.create')}</Button>
      </div>
      <Table dataSource={data} columns={columns} rowKey="id" loading={loading} size="middle" style={tableStyle} pagination={{ pageSize: 20 }} />
      <Modal title={editRecord ? t('common.edit') : t('common.create')} open={modalOpen} onOk={handleSubmit} onCancel={() => setModalOpen(false)} width={680} destroyOnHidden>
        <Form form={form} layout="vertical" style={{ marginTop: 16 }}>
          <Form.Item name="name" label={t('common.name')} rules={[{ required: true }]}><Input /></Form.Item>
          <Form.Item name="enable" label={t('common.enable')} valuePropName="checked"><Switch /></Form.Item>
          <Form.Item name="mode" label={t('access.mode')} rules={[{ required: true }]}>
            <Select><Option value="blacklist">{t('access.blacklist')}</Option><Option value="whitelist">{t('access.whitelist')}</Option></Select>
          </Form.Item>
          <Form.Item name="bind_ipdb_ids" label={t('access.bindIpdb')} extra={t('access.bindIpdbTip')}>
            <Select
              mode="multiple"
              placeholder={t('access.selectIpdb')}
              allowClear
              showSearch
              optionFilterProp="label"
              options={ipdbEntries.map((e: any) => ({
                value: e.id,
                label: `${e.cidr}${e.location ? ' (' + e.location + ')' : ''}${e.tags ? ' [' + e.tags + ']' : ''}`,
              }))}
            />
          </Form.Item>
          <Form.Item name="bind_site_ids" label={t('access.bindSites')} extra={t('access.bindSitesTip')}>
            <Select
              mode="multiple"
              placeholder={t('access.selectSites')}
              allowClear
              showSearch
              optionFilterProp="label"
              options={caddySites.map((s: any) => ({
                value: s.id,
                label: `${s.name}${s.domain ? ' (' + s.domain + ':' + s.port + ')' : ''}`,
              }))}
            />
          </Form.Item>
          <Form.Item name="ip_list" label={t('access.ipList')} extra={t('access.ipListTip')}>
            <Input.TextArea rows={4} placeholder={t('access.ipListPlaceholder')} />
          </Form.Item>

          <Divider orientation="left" style={{ fontSize: 13 }}>
            <LockOutlined style={{ marginRight: 6 }} />
            用户登录策略
          </Divider>

          <Form.Item name="auth_mode" label="认证方式" extra="选择对该规则绑定的站点要求的用户登录策略。选择后，匹配的站点请求需要用户认证才能访问。">
            <Select
              placeholder="不要求登录"
              allowClear
              onChange={(v) => setAuthMode(v || '')}
            >
              <Option value="">不要求登录</Option>
              <Option value="basic_auth">Basic Auth（浏览器弹窗认证）</Option>
              <Option value="page_login">页面跳转登录（跳转到登录页面）</Option>
            </Select>
          </Form.Item>

          {authMode && (
            <Form.Item
              name="allowed_user_ids"
              label="允许访问的用户"
              extra="选择哪些用户登录后可以访问绑定的站点。不选择则表示所有已登录用户均可访问。"
            >
              <Select
                mode="multiple"
                placeholder="全部已登录用户可访问"
                allowClear
                showSearch
                optionFilterProp="label"
                options={users.filter(u => u.enable).map((u) => ({
                  value: u.id,
                  label: `${u.username}${u.email ? ' (' + u.email + ')' : ''}${u.is_admin ? ' [管理员]' : ''}`,
                }))}
              />
            </Form.Item>
          )}

          <Form.Item name="remark" label={t('common.remark')}><Input.TextArea rows={2} /></Form.Item>
        </Form>
      </Modal>
    </div>
  )
}
export default Access
