import React, { useEffect, useState } from 'react'
import { Table, Button, Space, Switch, Modal, Form, Input, Select, Popconfirm, message, Typography } from 'antd'
import { PlusOutlined, EditOutlined, DeleteOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { callbackTaskApi, callbackAccountApi } from '../api'
import dayjs from 'dayjs'

const { Option } = Select

const CallbackTask: React.FC = () => {
  const { t } = useTranslation()
  const [data, setData] = useState<any[]>([])
  const [accounts, setAccounts] = useState<any[]>([])
  const [loading, setLoading] = useState(false)
  const [modalOpen, setModalOpen] = useState(false)
  const [editRecord, setEditRecord] = useState<any>(null)
  const [form] = Form.useForm()

  const fetchData = async () => {
    setLoading(true)
    try {
      const [taskRes, accRes]: any[] = await Promise.all([callbackTaskApi.list(), callbackAccountApi.list()])
      setData(taskRes.data || [])
      setAccounts(accRes.data || [])
    } finally { setLoading(false) }
  }
  useEffect(() => { fetchData() }, [])

  const handleSubmit = async () => {
    const values = await form.validateFields()
    editRecord ? await callbackTaskApi.update(editRecord.id, values) : await callbackTaskApi.create(values)
    message.success(t('common.success'))
    setModalOpen(false)
    fetchData()
  }

  const columns = [
    { title: t('common.enable'), dataIndex: 'enable', width: 80, render: (v: boolean, r: any) => <Switch size="small" checked={v} onChange={async (c) => { await callbackTaskApi.update(r.id, { ...r, enable: c }); fetchData() }} /> },
    { title: t('common.name'), dataIndex: 'name' },
    { title: '使用账号', dataIndex: 'account_id', render: (v: number) => accounts.find(a => a.id === v)?.name || v },
    { title: t('callback.triggerType'), dataIndex: 'trigger_type', render: (v: string) => ({ stun_ip_change: 'STUN IP变化' }[v] || v) },
    { title: '最后执行', dataIndex: 'last_run_time', render: (v: string) => v ? dayjs(v).format('MM-DD HH:mm') : '-' },
    { title: t('common.action'), width: 120, render: (_: any, r: any) => (
      <Space size={4}>
        <Button size="small" icon={<EditOutlined />} onClick={() => { setEditRecord(r); form.setFieldsValue(r); setModalOpen(true) }} />
        <Popconfirm title={t('common.deleteConfirm')} onConfirm={async () => { await callbackTaskApi.delete(r.id); fetchData() }}><Button size="small" danger icon={<DeleteOutlined />} /></Popconfirm>
      </Space>
    )},
  ]

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Typography.Title level={4} style={{ margin: 0 }}>{t('callback.taskTitle')}</Typography.Title>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => { setEditRecord(null); form.resetFields(); form.setFieldsValue({ enable: true, trigger_type: 'stun_ip_change' }); setModalOpen(true) }}>{t('common.create')}</Button>
      </div>
      <Table dataSource={data} columns={columns} rowKey="id" loading={loading} size="middle" style={{ background: '#fff', borderRadius: 8 }} pagination={{ pageSize: 20 }} />
      <Modal title={editRecord ? t('common.edit') : t('common.create')} open={modalOpen} onOk={handleSubmit} onCancel={() => setModalOpen(false)} width={480} destroyOnHidden>
        <Form form={form} layout="vertical" style={{ marginTop: 16 }}>
          <Form.Item name="name" label={t('common.name')} rules={[{ required: true }]}><Input /></Form.Item>
          <Form.Item name="enable" label={t('common.enable')} valuePropName="checked"><Switch /></Form.Item>
          <Form.Item name="account_id" label="回调账号" rules={[{ required: true }]}>
            <Select>{accounts.map(a => <Option key={a.id} value={a.id}>{a.name} ({a.account_type})</Option>)}</Select>
          </Form.Item>
          <Form.Item name="trigger_type" label={t('callback.triggerType')} rules={[{ required: true }]}>
            <Select><Option value="stun_ip_change">STUN IP变化</Option></Select>
          </Form.Item>
          <Form.Item name="trigger_source_id" label="触发来源ID（留空=所有）"><Input placeholder="STUN规则ID，留空匹配所有" /></Form.Item>
          <Form.Item name="remark" label={t('common.remark')}><Input.TextArea rows={2} /></Form.Item>
        </Form>
      </Modal>
    </div>
  )
}
export default CallbackTask
