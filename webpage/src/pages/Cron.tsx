import React, { useEffect, useState } from 'react'
import { Table, Button, Space, Switch, Modal, Form, Input, Select, Popconfirm, message, Typography, Tag } from 'antd'
import { PlusOutlined, EditOutlined, DeleteOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { cronApi } from '../api'
import StatusTag from '../components/StatusTag'
import CronExprInput from '../components/CronExprInput'
import dayjs from 'dayjs'

const { Option } = Select

const Cron: React.FC = () => {
  const { t } = useTranslation()
  const [data, setData] = useState<any[]>([])
  const [loading, setLoading] = useState(false)
  const [modalOpen, setModalOpen] = useState(false)
  const [editRecord, setEditRecord] = useState<any>(null)
  const [form] = Form.useForm()
  const [taskType, setTaskType] = useState('shell')

  const fetchData = async () => {
    setLoading(true)
    try { const res: any = await cronApi.list(); setData(res.data || []) }
    finally { setLoading(false) }
  }
  useEffect(() => { fetchData() }, [])

  const handleSubmit = async () => {
    const values = await form.validateFields()
    editRecord ? await cronApi.update(editRecord.id, values) : await cronApi.create(values)
    message.success(t('common.success'))
    setModalOpen(false)
    fetchData()
  }

  const columns = [
    { title: t('common.status'), dataIndex: 'status', width: 100, render: (s: string) => <StatusTag status={s} /> },
    { title: t('common.enable'), dataIndex: 'enable', width: 80, render: (v: boolean, r: any) => <Switch size="small" checked={v} onChange={async (c) => { c ? await cronApi.enable(r.id) : await cronApi.disable(r.id); fetchData() }} /> },
    { title: t('common.name'), dataIndex: 'name' },
    { title: t('cron.cronExpr'), dataIndex: 'cron_expr', render: (v: string) => <Typography.Text code>{v}</Typography.Text> },
    { title: t('cron.taskType'), dataIndex: 'task_type', render: (v: string) => <Tag>{v}</Tag> },
    { title: t('cron.lastRunTime'), dataIndex: 'last_run_time', render: (v: string) => v ? dayjs(v).format('MM-DD HH:mm:ss') : '-' },
    { title: t('common.action'), width: 180, render: (_: any, r: any) => (
      <Space size={4}>
        <Button size="small" type="primary" onClick={async () => { await cronApi.runNow(r.id); message.success('已触发执行') }}>{t('cron.runNow')}</Button>
        <Button size="small" icon={<EditOutlined />} onClick={() => { setEditRecord(r); setTaskType(r.task_type || 'shell'); form.setFieldsValue(r); setModalOpen(true) }} />
        <Popconfirm title={t('common.deleteConfirm')} onConfirm={async () => { await cronApi.delete(r.id); fetchData() }}><Button size="small" danger icon={<DeleteOutlined />} /></Popconfirm>
      </Space>
    )},
  ]

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Typography.Title level={4} style={{ margin: 0 }}>{t('cron.title')}</Typography.Title>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => { setEditRecord(null); form.resetFields(); form.setFieldsValue({ enable: true, task_type: 'shell' }); setTaskType('shell'); setModalOpen(true) }}>{t('common.create')}</Button>
      </div>
      <Table dataSource={data} columns={columns} rowKey="id" loading={loading} size="middle" style={{ background: '#fff', borderRadius: 8 }} pagination={{ pageSize: 20 }} />
      <Modal title={editRecord ? t('common.edit') : t('common.create')} open={modalOpen} onOk={handleSubmit} onCancel={() => setModalOpen(false)} width={520} destroyOnHidden>
        <Form form={form} layout="vertical" style={{ marginTop: 16 }}>
          <Form.Item name="name" label={t('common.name')} rules={[{ required: true }]}><Input style={{ width: '100%' }} /></Form.Item>
          <Form.Item name="enable" label={t('common.enable')} valuePropName="checked"><Switch /></Form.Item>
          <Form.Item name="cron_expr" label={t('cron.cronExpr')} rules={[{ required: true }]}>
            <CronExprInput />
          </Form.Item>
          <Form.Item name="task_type" label={t('cron.taskType')} rules={[{ required: true }]}>
            <Select onChange={(v) => setTaskType(v)} style={{ width: '100%' }}><Option value="shell">Shell命令</Option><Option value="http">HTTP请求</Option></Select>
          </Form.Item>
          {taskType === 'shell' && <Form.Item name="command" label={t('cron.command')} rules={[{ required: true }]}><Input.TextArea rows={3} placeholder="shell命令" /></Form.Item>}
          {taskType === 'http' && <>
            <Form.Item name="http_url" label={t('cron.httpUrl')} rules={[{ required: true }]}><Input style={{ width: '100%' }} /></Form.Item>
            <Form.Item name="http_method" label={t('cron.httpMethod')}><Select style={{ width: '100%' }}><Option value="GET">GET</Option><Option value="POST">POST</Option></Select></Form.Item>
            <Form.Item name="http_body" label={t('cron.httpBody')}><Input.TextArea rows={2} style={{ width: '100%' }} /></Form.Item>
          </>}
          <Form.Item name="remark" label={t('common.remark')}><Input.TextArea rows={2} style={{ width: '100%' }} /></Form.Item>
        </Form>
      </Modal>
    </div>
  )
}
export default Cron