import React, { useEffect, useState } from 'react'
import { Table, Button, Space, Modal, Form, Input, Select, Popconfirm, message, Typography } from 'antd'
import { PlusOutlined, EditOutlined, DeleteOutlined, ApiOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { callbackAccountApi } from '../api'

const { Option } = Select

const CallbackAccount: React.FC = () => {
  const { t } = useTranslation()
  const [data, setData] = useState<any[]>([])
  const [loading, setLoading] = useState(false)
  const [modalOpen, setModalOpen] = useState(false)
  const [editRecord, setEditRecord] = useState<any>(null)
  const [accountType, setAccountType] = useState('webhook')
  const [form] = Form.useForm()

  const fetchData = async () => {
    setLoading(true)
    try { const res: any = await callbackAccountApi.list(); setData(res.data || []) }
    finally { setLoading(false) }
  }
  useEffect(() => { fetchData() }, [])

  const handleSubmit = async () => {
    const values = await form.validateFields()
    // 将配置字段合并为 config JSON
    const { name, account_type, remark, ...configFields } = values
    const config = JSON.stringify(configFields)
    const payload = { name, account_type, config, remark }
    editRecord ? await callbackAccountApi.update(editRecord.id, payload) : await callbackAccountApi.create(payload)
    message.success(t('common.success'))
    setModalOpen(false)
    fetchData()
  }

  const columns = [
    { title: t('common.name'), dataIndex: 'name' },
    { title: t('callback.accountType'), dataIndex: 'account_type', render: (v: string) => ({ webhook: 'WebHook', cf_origin: 'CF回源', ali_esa: '阿里云ESA', tencent_eo: '腾讯云EO' }[v] || v) },
    { title: t('common.remark'), dataIndex: 'remark', render: (v: string) => v || '-' },
    { title: t('common.action'), width: 160, render: (_: any, r: any) => (
      <Space size={4}>
        <Button size="small" icon={<ApiOutlined />} onClick={async () => { await callbackAccountApi.test(r.id); message.success('测试成功') }}>{t('callback.test')}</Button>
        <Button size="small" icon={<EditOutlined />} onClick={() => { setEditRecord(r); setAccountType(r.account_type); const cfg = JSON.parse(r.config || '{}'); form.setFieldsValue({ ...r, ...cfg }); setModalOpen(true) }} />
        <Popconfirm title={t('common.deleteConfirm')} onConfirm={async () => { await callbackAccountApi.delete(r.id); fetchData() }}><Button size="small" danger icon={<DeleteOutlined />} /></Popconfirm>
      </Space>
    )},
  ]

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Typography.Title level={4} style={{ margin: 0 }}>{t('callback.accountTitle')}</Typography.Title>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => { setEditRecord(null); form.resetFields(); form.setFieldsValue({ account_type: 'webhook' }); setAccountType('webhook'); setModalOpen(true) }}>{t('common.create')}</Button>
      </div>
      <Table dataSource={data} columns={columns} rowKey="id" loading={loading} size="middle" style={{ background: '#fff', borderRadius: 8 }} pagination={{ pageSize: 20 }} />
      <Modal title={editRecord ? t('common.edit') : t('common.create')} open={modalOpen} onOk={handleSubmit} onCancel={() => setModalOpen(false)} width={520} destroyOnClose>
        <Form form={form} layout="vertical" style={{ marginTop: 16 }}>
          <Form.Item name="name" label={t('common.name')} rules={[{ required: true }]}><Input /></Form.Item>
          <Form.Item name="account_type" label={t('callback.accountType')} rules={[{ required: true }]}>
            <Select onChange={(v) => setAccountType(v)}>
              <Option value="webhook">WebHook</Option>
              <Option value="cf_origin">Cloudflare 回源端口</Option>
              <Option value="ali_esa">阿里云 ESA</Option>
              <Option value="tencent_eo">腾讯云 EO</Option>
            </Select>
          </Form.Item>
          {accountType === 'webhook' && <>
            <Form.Item name="url" label="Webhook URL" rules={[{ required: true }]}><Input placeholder="https://example.com/webhook" /></Form.Item>
            <Form.Item name="method" label="HTTP方法"><Select><Option value="POST">POST</Option><Option value="GET">GET</Option></Select></Form.Item>
          </>}
          {accountType === 'cf_origin' && <>
            <Form.Item name="api_token" label="API Token" rules={[{ required: true }]}><Input.Password /></Form.Item>
            <Form.Item name="zone_id" label="Zone ID" rules={[{ required: true }]}><Input /></Form.Item>
            <Form.Item name="rule_id" label="规则 ID" rules={[{ required: true }]}><Input /></Form.Item>
          </>}
          {(accountType === 'ali_esa' || accountType === 'tencent_eo') && <>
            <Form.Item name="access_id" label="Access ID" rules={[{ required: true }]}><Input /></Form.Item>
            <Form.Item name="access_secret" label="Access Secret" rules={[{ required: true }]}><Input.Password /></Form.Item>
          </>}
          <Form.Item name="remark" label={t('common.remark')}><Input.TextArea rows={2} /></Form.Item>
        </Form>
      </Modal>
    </div>
  )
}
export default CallbackAccount
