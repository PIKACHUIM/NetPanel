import React, { useEffect, useState } from 'react'
import { Table, Button, Space, Modal, Form, Input, InputNumber, Switch, Popconfirm, message, Typography, Card, Row, Col, Divider } from 'antd'
import { PlusOutlined, EditOutlined, DeleteOutlined, PlayCircleOutlined, StopOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { dnsmasqApi } from '../api'
import StatusTag from '../components/StatusTag'

const Dnsmasq: React.FC = () => {
  const { t } = useTranslation()
  const [config, setConfig] = useState<any>({})
  const [records, setRecords] = useState<any[]>([])
  const [loading, setLoading] = useState(false)
  const [recordModalOpen, setRecordModalOpen] = useState(false)
  const [editRecord, setEditRecord] = useState<any>(null)
  const [configForm] = Form.useForm()
  const [recordForm] = Form.useForm()

  const fetchData = async () => {
    setLoading(true)
    try {
      const [cfgRes, recRes]: any[] = await Promise.all([dnsmasqApi.getConfig(), dnsmasqApi.listRecords()])
      setConfig(cfgRes.data || {})
      configForm.setFieldsValue(cfgRes.data || {})
      setRecords(recRes.data || [])
    } finally { setLoading(false) }
  }
  useEffect(() => { fetchData() }, [])

  const handleSaveConfig = async () => {
    const values = await configForm.validateFields()
    await dnsmasqApi.updateConfig(values)
    message.success(t('common.success'))
    fetchData()
  }

  const handleRecordSubmit = async () => {
    const values = await recordForm.validateFields()
    editRecord ? await dnsmasqApi.updateRecord(editRecord.id, values) : await dnsmasqApi.createRecord(values)
    message.success(t('common.success'))
    setRecordModalOpen(false)
    fetchData()
  }

  const recordColumns = [
    { title: t('dnsmasq.customRecords'), dataIndex: 'domain' },
    { title: 'IP', dataIndex: 'ip' },
    { title: t('common.enable'), dataIndex: 'enable', render: (v: boolean, r: any) => <Switch size="small" checked={v} onChange={async (c) => { await dnsmasqApi.updateRecord(r.id, { ...r, enable: c }); fetchData() }} /> },
    { title: t('common.action'), width: 120, render: (_: any, r: any) => (
      <Space size={4}>
        <Button size="small" icon={<EditOutlined />} onClick={() => { setEditRecord(r); recordForm.setFieldsValue(r); setRecordModalOpen(true) }} />
        <Popconfirm title={t('common.deleteConfirm')} onConfirm={async () => { await dnsmasqApi.deleteRecord(r.id); fetchData() }}><Button size="small" danger icon={<DeleteOutlined />} /></Popconfirm>
      </Space>
    )},
  ]

  return (
    <div>
      <Typography.Title level={4} style={{ marginBottom: 16 }}>{t('dnsmasq.title')}</Typography.Title>
      <Card title="服务配置" style={{ borderRadius: 8, marginBottom: 16 }}
        extra={
          <Space>
            <StatusTag status={config.status || 'stopped'} />
            {config.status === 'running'
              ? <Button size="small" icon={<StopOutlined />} onClick={async () => { await dnsmasqApi.stop(); fetchData() }}>停止</Button>
              : <Button size="small" type="primary" icon={<PlayCircleOutlined />} onClick={async () => { await dnsmasqApi.start(); fetchData() }}>启动</Button>
            }
          </Space>
        }
      >
        <Form form={configForm} layout="vertical">
          <Row gutter={16}>
            <Col span={8}><Form.Item name="listen_addr" label={t('dnsmasq.listenAddr')}><Input placeholder="0.0.0.0" style={{ width: '100%' }} /></Form.Item></Col>
            <Col span={8}><Form.Item name="listen_port" label={t('dnsmasq.listenPort')}><InputNumber min={1} max={65535} style={{ width: '100%' }} /></Form.Item></Col>
            <Col span={8}><Form.Item name="upstream_dns" label={t('dnsmasq.upstreamDNS')}><Input placeholder="8.8.8.8,114.114.114.114" style={{ width: '100%' }} /></Form.Item></Col>
          </Row>
          <Button type="primary" onClick={handleSaveConfig}>{t('common.save')}</Button>
        </Form>
      </Card>

      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 12 }}>
        <Typography.Text strong>自定义解析记录</Typography.Text>
        <Button size="small" type="primary" icon={<PlusOutlined />} onClick={() => { setEditRecord(null); recordForm.resetFields(); recordForm.setFieldsValue({ enable: true }); setRecordModalOpen(true) }}>添加记录</Button>
      </div>
      <Table dataSource={records} columns={recordColumns} rowKey="id" loading={loading} size="small" style={{ background: '#fff', borderRadius: 8 }} pagination={{ pageSize: 20 }} />

      <Modal title={editRecord ? t('common.edit') : '添加解析记录'} open={recordModalOpen} onOk={handleRecordSubmit} onCancel={() => setRecordModalOpen(false)} width={400} destroyOnHidden>
        <Form form={recordForm} layout="vertical" style={{ marginTop: 16 }}>
          <Form.Item name="domain" label="域名" rules={[{ required: true }]}><Input placeholder="example.local" style={{ width: '100%' }} /></Form.Item>
          <Form.Item name="ip" label="IP地址" rules={[{ required: true }]}><Input placeholder="192.168.1.100" style={{ width: '100%' }} /></Form.Item>
          <Form.Item name="enable" label={t('common.enable')} valuePropName="checked"><Switch /></Form.Item>
        </Form>
      </Modal>
    </div>
  )
}
export default Dnsmasq
