import React, { useEffect, useState } from 'react'
import { Table, Button, Space, Modal, Form, Input, Popconfirm, message, Typography, Tag } from 'antd'
import { PlusOutlined, EditOutlined, DeleteOutlined, SearchOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { ipdbApi } from '../api'

const IpDb: React.FC = () => {
  const { t } = useTranslation()
  const [data, setData] = useState<any[]>([])
  const [loading, setLoading] = useState(false)
  const [modalOpen, setModalOpen] = useState(false)
  const [editRecord, setEditRecord] = useState<any>(null)
  const [queryResult, setQueryResult] = useState<any>(null)
  const [queryIP, setQueryIP] = useState('')
  const [form] = Form.useForm()

  const fetchData = async () => {
    setLoading(true)
    try { const res: any = await ipdbApi.list(); setData(res.data?.list || []) }
    finally { setLoading(false) }
  }
  useEffect(() => { fetchData() }, [])

  const handleSubmit = async () => {
    const values = await form.validateFields()
    editRecord ? await ipdbApi.update(editRecord.id, values) : await ipdbApi.create(values)
    message.success(t('common.success'))
    setModalOpen(false)
    fetchData()
  }

  const handleQuery = async () => {
    if (!queryIP) return
    const res: any = await ipdbApi.query(queryIP)
    setQueryResult(res.data)
  }

  const columns = [
    { title: t('ipdb.cidr'), dataIndex: 'cidr', render: (v: string) => <Typography.Text code>{v}</Typography.Text> },
    { title: t('ipdb.location'), dataIndex: 'location', render: (v: string) => v || '-' },
    { title: t('ipdb.tags'), dataIndex: 'tags', render: (v: string) => v ? v.split(',').map((tag: string) => <Tag key={tag}>{tag}</Tag>) : '-' },
    { title: t('common.remark'), dataIndex: 'remark', render: (v: string) => v || '-' },
    { title: t('common.action'), width: 120, render: (_: any, r: any) => (
      <Space size={4}>
        <Button size="small" icon={<EditOutlined />} onClick={() => { setEditRecord(r); form.setFieldsValue(r); setModalOpen(true) }} />
        <Popconfirm title={t('common.deleteConfirm')} onConfirm={async () => { await ipdbApi.delete(r.id); fetchData() }}><Button size="small" danger icon={<DeleteOutlined />} /></Popconfirm>
      </Space>
    )},
  ]

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Typography.Title level={4} style={{ margin: 0 }}>{t('ipdb.title')}</Typography.Title>
        <Space>
          <Input.Search
            placeholder="查询IP归属"
            value={queryIP}
            onChange={e => setQueryIP(e.target.value)}
            onSearch={handleQuery}
            style={{ width: 220 }}
            enterButton={<SearchOutlined />}
          />
          <Button type="primary" icon={<PlusOutlined />} onClick={() => { setEditRecord(null); form.resetFields(); setModalOpen(true) }}>{t('common.create')}</Button>
        </Space>
      </div>
      {queryResult && (
        <div style={{ marginBottom: 12, padding: '8px 16px', background: '#f6ffed', border: '1px solid #b7eb8f', borderRadius: 6 }}>
          查询结果：{queryResult.cidr} - {queryResult.location || '未知'} {queryResult.tags ? `[${queryResult.tags}]` : ''}
        </div>
      )}
      <Table dataSource={data} columns={columns} rowKey="id" loading={loading} size="middle" style={{ background: '#fff', borderRadius: 8 }} pagination={{ pageSize: 20 }} />
      <Modal title={editRecord ? t('common.edit') : t('common.create')} open={modalOpen} onOk={handleSubmit} onCancel={() => setModalOpen(false)} width={440} destroyOnClose>
        <Form form={form} layout="vertical" style={{ marginTop: 16 }}>
          <Form.Item name="cidr" label={t('ipdb.cidr')} rules={[{ required: true }]}><Input placeholder="192.168.1.0/24 或 1.2.3.4" /></Form.Item>
          <Form.Item name="location" label={t('ipdb.location')}><Input placeholder="中国-北京" /></Form.Item>
          <Form.Item name="tags" label={t('ipdb.tags')}><Input placeholder="标签1,标签2" /></Form.Item>
          <Form.Item name="remark" label={t('common.remark')}><Input.TextArea rows={2} /></Form.Item>
        </Form>
      </Modal>
    </div>
  )
}
export default IpDb
