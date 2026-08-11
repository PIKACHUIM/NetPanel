import React, { useEffect, useState } from 'react'
import { Table, Button, Space, Switch, Modal, Form, Input, Select, Popconfirm, message, Typography, Tag } from 'antd'
import { PlusOutlined, EditOutlined, DeleteOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { cronApi, domainCertApi, ddnsApi, wolApi, domainInfoApi } from '../api'
import StatusTag from '../components/StatusTag'
import CronExprInput from '../components/CronExprInput'
import dayjs from 'dayjs'
import { useTableStyle } from '../hooks/useTableStyle'

const { Option } = Select

const Cron: React.FC = () => {
  const { t } = useTranslation()
  const tableStyle = useTableStyle()
  const [data, setData] = useState<any[]>([])
  const [loading, setLoading] = useState(false)
  const [modalOpen, setModalOpen] = useState(false)
  const [editRecord, setEditRecord] = useState<any>(null)
  const [form] = Form.useForm()
  const [taskType, setTaskType] = useState('shell')
  const [certList, setCertList] = useState<any[]>([])
  const [ddnsList, setDdnsList] = useState<any[]>([])
  const [wolList, setWolList] = useState<any[]>([])
  const [domainList, setDomainList] = useState<any[]>([])

  const fetchData = async () => {
    setLoading(true)
    try { const res: any = await cronApi.list(); setData(res.data || []) }
    finally { setLoading(false) }
  }
  useEffect(() => { fetchData() }, [])

  // 加载关联数据列表（打开弹窗时）
  const loadTargetLists = async () => {
    try {
      const [certRes, ddnsRes, wolRes, domainRes]: any[] = await Promise.all([
        domainCertApi.list(),
        ddnsApi.list(),
        wolApi.list(),
        domainInfoApi.list(),
      ])
      setCertList(certRes.data || [])
      setDdnsList(ddnsRes.data || [])
      setWolList(wolRes.data || [])
      setDomainList(domainRes.data || [])
    } catch { /* ignore */ }
  }

  const handleSubmit = async () => {
    const values = await form.validateFields()
    editRecord ? await cronApi.update(editRecord.id, values) : await cronApi.create(values)
    message.success(t('common.success'))
    setModalOpen(false)
    fetchData()
  }

  const getTaskTypeLabel = (v: string) => {
    const map: Record<string, string> = {
      shell: t('cron.typeShell'),
      http: t('cron.typeHttp'),
      renew_cert: t('cron.typeRenewCert'),
      update_ddns: t('cron.typeUpdateDdns'),
      wol: t('cron.typeWol'),
      sync_dns_record: t('cron.typeSyncDnsRecord'),
    }
    return map[v] || v
  }

  const columns = [
    { title: t('common.status'), dataIndex: 'status', width: 100, render: (s: string) => <StatusTag status={s} /> },
    { title: t('common.enable'), dataIndex: 'enable', width: 80, render: (v: boolean, r: any) => <Switch size="small" checked={v} onChange={async (c) => { c ? await cronApi.enable(r.id) : await cronApi.disable(r.id); fetchData() }} /> },
    { title: t('common.name'), dataIndex: 'name' },
    { title: t('cron.cronExpr'), dataIndex: 'cron_expr', render: (v: string) => <Typography.Text code>{v}</Typography.Text> },
    { title: t('cron.taskType'), dataIndex: 'task_type', render: (v: string) => <Tag>{getTaskTypeLabel(v)}</Tag> },
    { title: t('cron.lastRunTime'), dataIndex: 'last_run_time', render: (v: string) => v ? dayjs(v).format('MM-DD HH:mm:ss') : '-' },
    { title: t('common.action'), width: 180, render: (_: any, r: any) => (
      <Space size={4}>
        <Button size="small" type="primary" onClick={async () => { await cronApi.runNow(r.id); message.success('已触发执行') }}>{t('cron.runNow')}</Button>
        <Button size="small" icon={<EditOutlined />} onClick={() => { setEditRecord(r); setTaskType(r.task_type || 'shell'); form.setFieldsValue(r); loadTargetLists(); setModalOpen(true) }} />
        <Popconfirm title={t('common.deleteConfirm')} onConfirm={async () => { await cronApi.delete(r.id); fetchData() }}><Button size="small" danger icon={<DeleteOutlined />} /></Popconfirm>
      </Space>
    )},
  ]

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Typography.Title level={4} style={{ margin: 0 }}>{t('cron.title')}</Typography.Title>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => { setEditRecord(null); form.resetFields(); form.setFieldsValue({ enable: true, task_type: 'shell' }); setTaskType('shell'); loadTargetLists(); setModalOpen(true) }}>{t('common.create')}</Button>
      </div>
      <Table dataSource={data} columns={columns} rowKey="id" loading={loading} size="middle" style={tableStyle} pagination={{ pageSize: 20 }} />
      <Modal title={editRecord ? t('common.edit') : t('common.create')} open={modalOpen} onOk={handleSubmit} onCancel={() => setModalOpen(false)} width={520} destroyOnHidden>
        <Form form={form} layout="vertical" style={{ marginTop: 16 }}>
          <Form.Item name="name" label={t('common.name')} rules={[{ required: true }]}><Input style={{ width: '100%' }} /></Form.Item>
          <Form.Item name="enable" label={t('common.enable')} valuePropName="checked"><Switch /></Form.Item>
          <Form.Item name="cron_expr" label={t('cron.cronExpr')} rules={[{ required: true }]}>
            <CronExprInput />
          </Form.Item>
          <Form.Item name="task_type" label={t('cron.taskType')} rules={[{ required: true }]}>
            <Select onChange={(v) => { setTaskType(v); form.setFieldValue('target_id', undefined) }} style={{ width: '100%' }}>
              <Option value="shell">{t('cron.typeShell')}</Option>
              <Option value="http">{t('cron.typeHttp')}</Option>
              <Option value="renew_cert">{t('cron.typeRenewCert')}</Option>
              <Option value="update_ddns">{t('cron.typeUpdateDdns')}</Option>
              <Option value="wol">{t('cron.typeWol')}</Option>
              <Option value="sync_dns_record">{t('cron.typeSyncDnsRecord')}</Option>
            </Select>
          </Form.Item>
          {taskType === 'shell' && <Form.Item name="command" label={t('cron.command')} rules={[{ required: true }]}><Input.TextArea rows={3} placeholder="shell命令" /></Form.Item>}
          {taskType === 'http' && <>
            <Form.Item name="http_url" label={t('cron.httpUrl')} rules={[{ required: true }]}><Input style={{ width: '100%' }} /></Form.Item>
            <Form.Item name="http_method" label={t('cron.httpMethod')}><Select style={{ width: '100%' }}><Option value="GET">GET</Option><Option value="POST">POST</Option></Select></Form.Item>
            <Form.Item name="http_body" label={t('cron.httpBody')}><Input.TextArea rows={2} style={{ width: '100%' }} /></Form.Item>
          </>}
          {taskType === 'renew_cert' && (
            <Form.Item name="target_id" label={t('cron.targetCert')} rules={[{ required: true, message: t('cron.targetCertRequired') }]}>
              <Select placeholder={t('cron.targetCertPlaceholder')} style={{ width: '100%' }}>
                {certList.map(c => <Option key={c.id} value={c.id}>{c.name} ({c.domains ? JSON.parse(c.domains).join(', ') : '-'})</Option>)}
              </Select>
            </Form.Item>
          )}
          {taskType === 'update_ddns' && (
            <Form.Item name="target_id" label={t('cron.targetDdns')} rules={[{ required: true, message: t('cron.targetDdnsRequired') }]}>
              <Select placeholder={t('cron.targetDdnsPlaceholder')} style={{ width: '100%' }}>
                {ddnsList.map(d => <Option key={d.id} value={d.id}>{d.name}</Option>)}
              </Select>
            </Form.Item>
          )}
          {taskType === 'wol' && (
            <Form.Item name="target_id" label={t('cron.targetWol')} rules={[{ required: true, message: t('cron.targetWolRequired') }]}>
              <Select placeholder={t('cron.targetWolPlaceholder')} style={{ width: '100%' }}>
                {wolList.map(w => <Option key={w.id} value={w.id}>{w.name} ({w.mac_address})</Option>)}
              </Select>
            </Form.Item>
          )}
          {taskType === 'sync_dns_record' && (
            <Form.Item name="target_id" label={t('cron.targetDomain')} rules={[{ required: true, message: t('cron.targetDomainRequired') }]}>
              <Select placeholder={t('cron.targetDomainPlaceholder')} style={{ width: '100%' }}>
                {domainList.map(d => <Option key={d.id} value={d.id}>{d.name}</Option>)}
              </Select>
            </Form.Item>
          )}
          <Form.Item name="remark" label={t('common.remark')}><Input.TextArea rows={2} style={{ width: '100%' }} /></Form.Item>
        </Form>
      </Modal>
    </div>
  )
}
export default Cron