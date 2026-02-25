import React, { useEffect, useState } from 'react'
import { Table, Button, Space, Switch, Modal, Form, Input, Select, Popconfirm, message, Typography, Tag } from 'antd'
import { PlusOutlined, EditOutlined, DeleteOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { accessApi } from '../api'

const { Option } = Select

const Access: React.FC = () => {
  const { t } = useTranslation()
  const [data, setData] = useState<any[]>([])
  const [loading, setLoading] = useState(false)
  const [modalOpen, setModalOpen] = useState(false)
  const [editRecord, setEditRecord] = useState<any>(null)
  const [form] = Form.useForm()

  const fetchData = async () => {
    setLoading(true)
    try { const res: any = await accessApi.list(); setData(res.data || []) }
    finally { setLoading(false) }
  }
  useEffect(() => { fetchData() }, [])

  const handleSubmit = async () => {
    const values = await form.validateFields()
    if (typeof values.ip_list === 'string') values.ip_list = JSON.stringify(values.ip_list.split('\n').filter(Boolean))
    editRecord ? await accessApi.update(editRecord.id, values) : await accessApi.create(values)
    message.success(t('common.success'))
    setModalOpen(false)
    fetchData()
  }

  const columns = [
    { title: t('common.enable'), dataIndex: 'enable', width: 80, render: (v: boolean, r: any) => <Switch size="small" checked={v} onChange={async (c) => { await accessApi.update(r.id, { ...r, enable: c }); fetchData() }} /> },
    { title: t('common.name'), dataIndex: 'name' },
    { title: t('access.mode'), dataIndex: 'mode', render: (v: string) => <Tag color={v === 'blacklist' ? 'red' : 'green'}>{v === 'blacklist' ? t('access.blacklist') : t('access.whitelist')}</Tag> },
    { title: t('access.ipList'), dataIndex: 'ip_list', render: (v: string) => { try { const arr = JSON.parse(v); return `${arr.length} 条规则` } catch { return v } } },
    { title: t('common.remark'), dataIndex: 'remark', render: (v: string) => v || '-' },
    { title: t('common.action'), width: 120, render: (_: any, r: any) => (
      <Space size={4}>
        <Button size="small" icon={<EditOutlined />} onClick={() => { setEditRecord(r); form.setFieldsValue({ ...r, ip_list: JSON.parse(r.ip_list || '[]').join('\n') }); setModalOpen(true) }} />
        <Popconfirm title={t('common.deleteConfirm')} onConfirm={async () => { await accessApi.delete(r.id); fetchData() }}><Button size="small" danger icon={<DeleteOutlined />} /></Popconfirm>
      </Space>
    )},
  ]

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Typography.Title level={4} style={{ margin: 0 }}>{t('access.title')}</Typography.Title>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => { setEditRecord(null); form.resetFields(); form.setFieldsValue({ enable: true, mode: 'blacklist' }); setModalOpen(true) }}>{t('common.create')}</Button>
      </div>
      <Table dataSource={data} columns={columns} rowKey="id" loading={loading} size="middle" style={{ background: '#fff', borderRadius: 8 }} pagination={{ pageSize: 20 }} />
      <Modal title={editRecord ? t('common.edit') : t('common.create')} open={modalOpen} onOk={handleSubmit} onCancel={() => setModalOpen(false)} width={480} destroyOnClose>
        <Form form={form} layout="vertical" style={{ marginTop: 16 }}>
          <Form.Item name="name" label={t('common.name')} rules={[{ required: true }]}><Input /></Form.Item>
          <Form.Item name="enable" label={t('common.enable')} valuePropName="checked"><Switch /></Form.Item>
          <Form.Item name="mode" label={t('access.mode')} rules={[{ required: true }]}>
            <Select><Option value="blacklist">{t('access.blacklist')}</Option><Option value="whitelist">{t('access.whitelist')}</Option></Select>
          </Form.Item>
          <Form.Item name="ip_list" label={t('access.ipList')} rules={[{ required: true }]}>
            <Input.TextArea rows={6} placeholder="每行一个IP或CIDR，如：&#10;192.168.1.0/24&#10;10.0.0.1" />
          </Form.Item>
          <Form.Item name="remark" label={t('common.remark')}><Input.TextArea rows={2} /></Form.Item>
        </Form>
      </Modal>
    </div>
  )
}
export default Access
