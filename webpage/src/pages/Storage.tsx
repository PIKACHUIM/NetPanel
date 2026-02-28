import React, { useEffect, useState } from 'react'
import { Table, Button, Space, Switch, Modal, Form, Input, InputNumber, Select, Popconfirm, message, Typography, Tag, Row, Col } from 'antd'
import { PlusOutlined, EditOutlined, DeleteOutlined, PlayCircleOutlined, StopOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { storageApi } from '../api'
import StatusTag from '../components/StatusTag'

const { Option } = Select

const Storage: React.FC = () => {
  const { t } = useTranslation()
  const [data, setData] = useState<any[]>([])
  const [loading, setLoading] = useState(false)
  const [modalOpen, setModalOpen] = useState(false)
  const [editRecord, setEditRecord] = useState<any>(null)
  const [form] = Form.useForm()

  const fetchData = async () => {
    setLoading(true)
    try { const res: any = await storageApi.list(); setData(res.data || []) }
    finally { setLoading(false) }
  }
  useEffect(() => { fetchData() }, [])

  const handleSubmit = async () => {
    const values = await form.validateFields()
    editRecord ? await storageApi.update(editRecord.id, values) : await storageApi.create(values)
    message.success(t('common.success'))
    setModalOpen(false)
    fetchData()
  }

  const columns = [
    { title: t('common.status'), dataIndex: 'status', width: 100, render: (s: string) => <StatusTag status={s} /> },
    { title: t('common.enable'), dataIndex: 'enable', width: 80, render: (v: boolean, r: any) => <Switch size="small" checked={v} onChange={async (c) => { await storageApi.update(r.id, { ...r, enable: c }); c ? await storageApi.start(r.id) : await storageApi.stop(r.id); fetchData() }} /> },
    { title: t('common.name'), dataIndex: 'name' },
    { title: t('storage.protocol'), dataIndex: 'protocol', render: (v: string) => <Tag color="blue">{v?.toUpperCase()}</Tag> },
    { title: '监听', render: (_: any, r: any) => `${r.listen_addr || '0.0.0.0'}:${r.listen_port}` },
    { title: t('storage.rootPath'), dataIndex: 'root_path' },
    { title: t('storage.readOnly'), dataIndex: 'read_only', render: (v: boolean) => v ? <Tag color="orange">只读</Tag> : <Tag color="green">读写</Tag> },
    { title: t('common.action'), width: 140, render: (_: any, r: any) => (
      <Space size={4}>
        {r.status === 'running' ? <Button size="small" icon={<StopOutlined />} onClick={async () => { await storageApi.stop(r.id); fetchData() }} /> : <Button size="small" type="primary" icon={<PlayCircleOutlined />} onClick={async () => { await storageApi.start(r.id); fetchData() }} />}
        <Button size="small" icon={<EditOutlined />} onClick={() => { setEditRecord(r); form.setFieldsValue(r); setModalOpen(true) }} />
        <Popconfirm title={t('common.deleteConfirm')} onConfirm={async () => { await storageApi.delete(r.id); fetchData() }}><Button size="small" danger icon={<DeleteOutlined />} /></Popconfirm>
      </Space>
    )},
  ]

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Typography.Title level={4} style={{ margin: 0 }}>{t('storage.title')}</Typography.Title>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => { setEditRecord(null); form.resetFields(); form.setFieldsValue({ enable: true, protocol: 'webdav', listen_addr: '0.0.0.0', listen_port: 8888 }); setModalOpen(true) }}>{t('common.create')}</Button>
      </div>
      <Table dataSource={data} columns={columns} rowKey="id" loading={loading} size="middle" style={{ background: '#fff', borderRadius: 8 }} pagination={{ pageSize: 20 }} />
      <Modal title={editRecord ? t('common.edit') : t('common.create')} open={modalOpen} onOk={handleSubmit} onCancel={() => setModalOpen(false)} width={520} destroyOnHidden>
        <Form form={form} layout="vertical" style={{ marginTop: 16 }}>
          <Form.Item name="name" label={t('common.name')} rules={[{ required: true }]}><Input style={{ width: '100%' }} /></Form.Item>
          <Form.Item name="enable" label={t('common.enable')} valuePropName="checked"><Switch /></Form.Item>
          <Form.Item name="protocol" label={t('storage.protocol')} rules={[{ required: true }]}>
            <Select style={{ width: '100%' }}><Option value="webdav">WebDAV</Option><Option value="sftp">SFTP</Option><Option value="smb">SMB</Option></Select>
          </Form.Item>
          <Row gutter={16}>
            <Col span={16}>
              <Form.Item name="listen_addr" label={t('storage.listenAddr')}><Input placeholder="0.0.0.0" style={{ width: '100%' }} /></Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="listen_port" label={t('storage.listenPort')} rules={[{ required: true }]}><InputNumber min={1} max={65535} style={{ width: '100%' }} /></Form.Item>
            </Col>
          </Row>
          <Form.Item name="root_path" label={t('storage.rootPath')} rules={[{ required: true }]}><Input placeholder="/data/share" style={{ width: '100%' }} /></Form.Item>
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item name="username" label={t('common.username')}><Input style={{ width: '100%' }} /></Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="password" label={t('common.password')}><Input.Password style={{ width: '100%' }} /></Form.Item>
            </Col>
          </Row>
          <Form.Item name="read_only" label={t('storage.readOnly')} valuePropName="checked"><Switch /></Form.Item>
          <Form.Item name="remark" label={t('common.remark')}><Input.TextArea rows={2} style={{ width: '100%' }} /></Form.Item>
        </Form>
      </Modal>
    </div>
  )
}
export default Storage
