import React, { useEffect, useState } from 'react'
import { Table, Button, Space, Modal, Form, Input, Select, Popconfirm, message, Tag, Switch } from 'antd'
import { PlusOutlined, EditOutlined, DeleteOutlined, RobotOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { aiAssistantApi, aiProviderApi, aiPluginApi } from '../api'
import { useTableStyle } from '../hooks/useTableStyle'

const { TextArea } = Input

const AiAssistant: React.FC = () => {
  const { t } = useTranslation()
  const tableStyle = useTableStyle()
  const [data, setData] = useState<any[]>([])
  const [loading, setLoading] = useState(false)
  const [modalOpen, setModalOpen] = useState(false)
  const [editRecord, setEditRecord] = useState<any>(null)
  const [form] = Form.useForm()
  const [providers, setProviders] = useState<any[]>([])
  const [skills, setSkills] = useState<any[]>([])

  const fetchData = async () => {
    setLoading(true)
    try { const res: any = await aiAssistantApi.list(); setData(res.data || []) }
    finally { setLoading(false) }
  }

  const loadOptions = async () => {
    try {
      const [pRes, sRes]: any[] = await Promise.all([
        aiProviderApi.list(),
        aiPluginApi.list('skill'),
      ])
      setProviders(pRes.data || [])
      setSkills(sRes.data || [])
    } catch {}
  }

  useEffect(() => { fetchData() }, [])

  const handleSubmit = async () => {
    const values = await form.validateFields()
    // skills 字段序列化为 JSON
    if (values.skills && Array.isArray(values.skills)) {
      values.skills = JSON.stringify(values.skills)
    }
    if (editRecord) {
      await aiAssistantApi.update(editRecord.id, values)
    } else {
      await aiAssistantApi.create(values)
    }
    message.success(t('common.success'))
    setModalOpen(false)
    fetchData()
  }

  const openModal = (record?: any) => {
    setEditRecord(record || null)
    loadOptions()
    if (record) {
      const formValues = { ...record }
      // 解析 skills JSON
      try { formValues.skills = JSON.parse(record.skills || '[]') } catch { formValues.skills = [] }
      form.setFieldsValue(formValues)
    } else {
      form.resetFields()
    }
    setModalOpen(true)
  }

  const getModels = (providerId: number): string[] => {
    const p = providers.find((p: any) => p.id === providerId)
    if (!p) return []
    try { return JSON.parse(p.models || '[]') } catch { return [] }
  }

  const columns = [
    { title: t('ai.assistantName'), dataIndex: 'name', key: 'name', width: 150 },
    { title: t('ai.description'), dataIndex: 'description', key: 'description', ellipsis: true },
    { title: t('ai.model'), dataIndex: 'model_name', key: 'model_name', width: 160 },
    {
      title: t('common.status'), dataIndex: 'is_active', key: 'is_active', width: 80,
      render: (v: boolean) => <Tag color={v ? 'green' : 'default'}>{v ? t('common.enable') : t('common.disable')}</Tag>,
    },
    {
      title: t('common.action'), key: 'action', width: 160,
      render: (_: any, record: any) => (
        <Space size="small">
          <Button size="small" icon={<EditOutlined />} onClick={() => openModal(record)}>
            {t('common.edit')}
          </Button>
          <Popconfirm title={t('common.deleteConfirm')} onConfirm={async () => { await aiAssistantApi.delete(record.id); message.success(t('common.success')); fetchData() }}>
            <Button size="small" danger icon={<DeleteOutlined />}>{t('common.delete')}</Button>
          </Popconfirm>
        </Space>
      ),
    },
  ]

  return (
    <div>
      <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <h2 style={{ margin: 0 }}><RobotOutlined /> {t('ai.assistant')}</h2>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => openModal()}>
          {t('ai.addAssistant')}
        </Button>
      </div>

      <Table
        {...tableStyle}
        columns={columns}
        dataSource={data}
        loading={loading}
        rowKey="id"
      />

      <Modal
        title={editRecord ? t('ai.editAssistant') : t('ai.addAssistant')}
        open={modalOpen}
        onOk={handleSubmit}
        onCancel={() => setModalOpen(false)}
        width={700}
        destroyOnClose
      >
        <Form form={form} layout="vertical" preserve={false}>
          <Form.Item name="name" label={t('ai.assistantName')} rules={[{ required: true }]}>
            <Input placeholder="翻译助手 / 代码专家 / 数据分析师..." />
          </Form.Item>
          <Form.Item name="description" label={t('ai.description')}>
            <Input placeholder="简短描述助理的功能" />
          </Form.Item>
          <Form.Item name="system_prompt" label={t('ai.systemPrompt')} extra={t('ai.systemPromptTip')}>
            <TextArea rows={5} placeholder="你是一个专业的..." />
          </Form.Item>
          <Form.Item name="provider_id" label={t('ai.provider')}>
            <Select placeholder={t('ai.selectProvider')} allowClear>
              {providers.filter((p: any) => p.is_active).map((p: any) => (
                <Select.Option key={p.id} value={p.id}>{p.name}</Select.Option>
              ))}
            </Select>
          </Form.Item>
          <Form.Item noStyle shouldUpdate={(prev, cur) => prev.provider_id !== cur.provider_id}>
            {({ getFieldValue }) => {
              const pid = getFieldValue('provider_id')
              const models = getModels(pid)
              return (
                <Form.Item name="model_name" label={t('ai.model')}>
                  <Select placeholder={t('ai.selectModel')} allowClear showSearch>
                    {models.map((m: string) => (
                      <Select.Option key={m} value={m}>{m}</Select.Option>
                    ))}
                  </Select>
                </Form.Item>
              )
            }}
          </Form.Item>
          <Form.Item name="skills" label={t('ai.skills')}>
            <Select mode="multiple" placeholder="选择技能包" allowClear>
              {skills.filter((s: any) => s.is_active).map((s: any) => (
                <Select.Option key={s.id} value={s.id}>{s.name}</Select.Option>
              ))}
            </Select>
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

export default AiAssistant
