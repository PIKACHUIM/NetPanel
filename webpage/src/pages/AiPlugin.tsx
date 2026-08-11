import React, { useEffect, useState } from 'react'
import { Table, Button, Space, Modal, Form, Input, Tabs, Card, Switch, Popconfirm, message, Tag, Select } from 'antd'
import { PlusOutlined, EditOutlined, DeleteOutlined, AppstoreOutlined, ToolOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { aiPluginApi } from '../api'
import { useTableStyle } from '../hooks/useTableStyle'

const { TextArea } = Input

const AiPlugin: React.FC = () => {
  const { t } = useTranslation()
  const tableStyle = useTableStyle()
  const [data, setData] = useState<any[]>([])
  const [loading, setLoading] = useState(false)
  const [modalOpen, setModalOpen] = useState(false)
  const [editRecord, setEditRecord] = useState<any>(null)
  const [form] = Form.useForm()
  const [activeTab, setActiveTab] = useState('skill')

  const fetchData = async () => {
    setLoading(true)
    try { const res: any = await aiPluginApi.list(activeTab); setData(res.data || []) }
    finally { setLoading(false) }
  }
  useEffect(() => { fetchData() }, [activeTab])

  const handleSubmit = async () => {
    const values = await form.validateFields()
    values.type = activeTab
    if (editRecord) {
      await aiPluginApi.update(editRecord.id, values)
    } else {
      await aiPluginApi.create(values)
    }
    message.success(t('common.success'))
    setModalOpen(false)
    fetchData()
  }

  const handleToggle = async (id: number, active: boolean) => {
    await aiPluginApi.toggle(id, active)
    fetchData()
  }

  const mcpColumns = [
    { title: t('common.name'), dataIndex: 'name', key: 'name', width: 150 },
    { title: t('ai.description'), dataIndex: 'description', key: 'description', ellipsis: true },
    { title: t('ai.endpoint'), dataIndex: 'endpoint', key: 'endpoint', width: 250, ellipsis: true },
    {
      title: t('common.status'), key: 'is_active', width: 100,
      render: (_: any, r: any) => (
        <Switch
          checked={r.is_active}
          onChange={(checked) => handleToggle(r.id, checked)}
          checkedChildren={t('common.enable')}
          unCheckedChildren={t('common.disable')}
        />
      ),
    },
    {
      title: t('ai.pluginType'), key: 'is_system', width: 100,
      render: (_: any, r: any) => r.is_system ? <Tag color="blue">{t('ai.systemPlugin')}</Tag> : <Tag>{t('ai.skillPlugin')}</Tag>,
    },
    {
      title: t('common.action'), key: 'action', width: 160,
      render: (_: any, record: any) => (
        <Space size="small">
          <Button size="small" icon={<EditOutlined />} onClick={() => { setEditRecord(record); form.setFieldsValue(record); setModalOpen(true) }}>
            {t('common.edit')}
          </Button>
          {!record.is_system && (
            <Popconfirm title={t('common.deleteConfirm')} onConfirm={async () => { await aiPluginApi.delete(record.id); message.success(t('common.success')); fetchData() }}>
              <Button size="small" danger icon={<DeleteOutlined />}>{t('common.delete')}</Button>
            </Popconfirm>
          )}
        </Space>
      ),
    },
  ]

  const skillColumns = [
    { title: t('common.name'), dataIndex: 'name', key: 'name', width: 150 },
    { title: t('ai.description'), dataIndex: 'description', key: 'description', ellipsis: true },
    { title: t('ai.triggerKeywords'), dataIndex: 'trigger_keywords', key: 'trigger_keywords', width: 200, ellipsis: true },
    {
      title: t('common.status'), key: 'is_active', width: 100,
      render: (_: any, r: any) => (
        <Switch
          checked={r.is_active}
          onChange={(checked) => handleToggle(r.id, checked)}
          checkedChildren={t('common.enable')}
          unCheckedChildren={t('common.disable')}
        />
      ),
    },
    {
      title: t('common.action'), key: 'action', width: 160,
      render: (_: any, record: any) => (
        <Space size="small">
          <Button size="small" icon={<EditOutlined />} onClick={() => { setEditRecord(record); form.setFieldsValue(record); setModalOpen(true) }}>
            {t('common.edit')}
          </Button>
          {!record.is_system && (
            <Popconfirm title={t('common.deleteConfirm')} onConfirm={async () => { await aiPluginApi.delete(record.id); message.success(t('common.success')); fetchData() }}>
              <Button size="small" danger icon={<DeleteOutlined />}>{t('common.delete')}</Button>
            </Popconfirm>
          )}
        </Space>
      ),
    },
  ]

  return (
    <div>
      <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <h2 style={{ margin: 0 }}><AppstoreOutlined /> {t('ai.plugin')}</h2>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => { setEditRecord(null); form.resetFields(); setModalOpen(true) }}>
          {t('ai.addPlugin')}
        </Button>
      </div>

      <Tabs activeKey={activeTab} onChange={setActiveTab} items={[
        { key: 'mcp', label: <span><ToolOutlined /> {t('ai.mcpPlugin')}</span> },
        { key: 'skill', label: <span><AppstoreOutlined /> {t('ai.skillPlugin')}</span> },
      ]} />

      <Table
        {...tableStyle}
        columns={activeTab === 'mcp' ? mcpColumns : skillColumns}
        dataSource={data}
        loading={loading}
        rowKey="id"
      />

      <Modal
        title={editRecord ? t('ai.editPlugin') : t('ai.addPlugin')}
        open={modalOpen}
        onOk={handleSubmit}
        onCancel={() => setModalOpen(false)}
        width={700}
        destroyOnClose
      >
        <Form form={form} layout="vertical" preserve={false}>
          <Form.Item name="name" label={t('common.name')} rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="description" label={t('ai.description')}>
            <Input />
          </Form.Item>
          {activeTab === 'mcp' && (
            <Form.Item name="endpoint" label={t('ai.endpoint')}>
              <Input placeholder="http://localhost:3000/mcp" />
            </Form.Item>
          )}
          {activeTab === 'skill' && (
            <>
              <Form.Item name="trigger_keywords" label={t('ai.triggerKeywords')}>
                <Input placeholder="关键词1, 关键词2, ..." />
              </Form.Item>
              <Form.Item name="content" label={t('ai.content')}>
                <TextArea rows={6} placeholder="技能内容 / 执行脚本 / System Prompt 片段" />
              </Form.Item>
            </>
          )}
          <Form.Item name="config" label={t('ai.config')}>
            <TextArea rows={3} placeholder='{"key": "value"}' />
          </Form.Item>
          <Form.Item name="is_active" label={t('common.status')} valuePropName="checked" initialValue={true}>
            <Switch checkedChildren={t('common.enable')} unCheckedChildren={t('common.disable')} />
          </Form.Item>
          <Form.Item name="remark" label={t('common.remark')}>
            <Input />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

export default AiPlugin
