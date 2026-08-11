import React, { useEffect, useState } from 'react'
import { Table, Button, Space, Modal, Form, Input, Popconfirm, message, Tag, Switch, Select } from 'antd'
import { PlusOutlined, EditOutlined, DeleteOutlined, ApiOutlined, CheckCircleOutlined, SyncOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { aiProviderApi } from '../api'
import { useTableStyle } from '../hooks/useTableStyle'

const AiProvider: React.FC = () => {
  const { t } = useTranslation()
  const tableStyle = useTableStyle()
  const [data, setData] = useState<any[]>([])
  const [loading, setLoading] = useState(false)
  const [modalOpen, setModalOpen] = useState(false)
  const [editRecord, setEditRecord] = useState<any>(null)
  const [form] = Form.useForm()
  const [fetchingModels, setFetchingModels] = useState<number | null>(null)
  const [testing, setTesting] = useState<number | null>(null)
  const [testingInModal, setTestingInModal] = useState(false)
  const [fetchingInModal, setFetchingInModal] = useState(false)

  const fetchData = async () => {
    setLoading(true)
    try { const res: any = await aiProviderApi.list(); setData(res.data || []) }
    finally { setLoading(false) }
  }
  useEffect(() => { fetchData() }, [])

  const handleSubmit = async () => {
    const values = await form.validateFields()
    // 将 models 数组转为 JSON 字符串
    const submitData = {
      ...values,
      models: JSON.stringify(values.models || [])
    }
    if (editRecord) {
      await aiProviderApi.update(editRecord.id, submitData)
    } else {
      await aiProviderApi.create(submitData)
    }
    message.success(t('common.success'))
    setModalOpen(false)
    fetchData()
  }

  const handleFetchModels = async (id: number) => {
    setFetchingModels(id)
    try {
      const res: any = await aiProviderApi.fetchModels(id)
      message.success(`${t('ai.fetchModels')}: ${(res.data || []).length} ${t('ai.models')}`)
      fetchData()
    } catch (e: any) {
      message.error(e?.message || t('ai.testFailed'))
    } finally {
      setFetchingModels(null)
    }
  }

  const handleTest = async (id: number) => {
    setTesting(id)
    try {
      const res: any = await aiProviderApi.test(id)
      if (res.code === 0) {
        message.success(t('ai.testSuccess'))
      } else {
        message.error(res.message || t('ai.testFailed'))
      }
    } catch (e: any) {
      message.error(e?.message || t('ai.testFailed'))
    } finally {
      setTesting(null)
    }
  }

  // Modal 内测试连接
  const handleTestInModal = async () => {
    try {
      await form.validateFields(['name', 'base_url', 'api_key'])
      const values = form.getFieldsValue()
      setTestingInModal(true)
      
      // 如果是编辑模式，直接测试；如果是新建，需要先临时保存
      let testId = editRecord?.id
      if (!testId) {
        const submitData = {
          ...values,
          models: JSON.stringify(values.models || [])
        }
        const res: any = await aiProviderApi.create(submitData)
        testId = res.data.id
        setEditRecord(res.data) // 转为编辑模式
        message.info(t('ai.savedForTest'))
      }
      
      const res: any = await aiProviderApi.test(testId)
      if (res.code === 0) {
        message.success(t('ai.testSuccess'))
      } else {
        message.error(res.message || t('ai.testFailed'))
      }
    } catch (e: any) {
      message.error(e?.message || t('ai.testFailed'))
    } finally {
      setTestingInModal(false)
    }
  }

  // Modal 内获取模型列表
  const handleFetchModelsInModal = async () => {
    try {
      await form.validateFields(['name', 'base_url', 'api_key'])
      const values = form.getFieldsValue()
      setFetchingInModal(true)
      
      // 如果是编辑模式，直接获取；如果是新建，需要先临时保存
      let fetchId = editRecord?.id
      if (!fetchId) {
        const submitData = {
          ...values,
          models: JSON.stringify(values.models || [])
        }
        const res: any = await aiProviderApi.create(submitData)
        fetchId = res.data.id
        setEditRecord(res.data) // 转为编辑模式
        message.info(t('ai.savedForFetch'))
      }
      
      const res: any = await aiProviderApi.fetchModels(fetchId)
      const models = res.data || []
      form.setFieldsValue({ models })
      message.success(`${t('ai.fetchModelsSuccess')}: ${models.length} ${t('ai.models')}`)
      fetchData() // 刷新列表
    } catch (e: any) {
      message.error(e?.message || t('ai.fetchModelsFailed'))
    } finally {
      setFetchingInModal(false)
    }
  }

  const getModels = (modelsStr: string): string[] => {
    try { return JSON.parse(modelsStr || '[]') }
    catch { return [] }
  }

  const columns = [
    { title: t('common.name'), dataIndex: 'name', key: 'name', width: 150 },
    { title: t('ai.baseUrl'), dataIndex: 'base_url', key: 'base_url', ellipsis: true },
    {
      title: t('ai.apiKey'), dataIndex: 'api_key', key: 'api_key', width: 200,
      render: (v: string) => v ? '••••••••' + v.slice(-4) : '-',
    },
    {
      title: t('ai.modelCount'), key: 'model_count', width: 100,
      render: (_: any, r: any) => {
        const models = getModels(r.models)
        return <Tag color="blue">{models.length}</Tag>
      },
    },
    {
      title: t('common.status'), dataIndex: 'is_active', key: 'is_active', width: 80,
      render: (v: boolean) => <Tag color={v ? 'green' : 'default'}>{v ? t('common.enable') : t('common.disable')}</Tag>,
    },
    {
      title: t('common.action'), key: 'action', width: 280,
      render: (_: any, record: any) => (
        <Space size="small">
          <Button size="small" icon={<EditOutlined />} onClick={() => { 
            setEditRecord(record); 
            form.setFieldsValue({
              ...record,
              models: getModels(record.models)
            }); 
            setModalOpen(true) 
          }}>
            {t('common.edit')}
          </Button>
          <Button size="small" icon={<SyncOutlined spin={fetchingModels === record.id} />} loading={fetchingModels === record.id} onClick={() => handleFetchModels(record.id)}>
            {t('ai.fetchModels')}
          </Button>
          <Button size="small" icon={<CheckCircleOutlined />} loading={testing === record.id} onClick={() => handleTest(record.id)}>
            {t('ai.testConnection')}
          </Button>
          <Popconfirm title={t('common.deleteConfirm')} onConfirm={async () => { await aiProviderApi.delete(record.id); message.success(t('common.success')); fetchData() }}>
            <Button size="small" danger icon={<DeleteOutlined />}>{t('common.delete')}</Button>
          </Popconfirm>
        </Space>
      ),
    },
  ]

  return (
    <div>
      <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <h2 style={{ margin: 0 }}><ApiOutlined /> {t('ai.providerManage')}</h2>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => { setEditRecord(null); form.resetFields(); setModalOpen(true) }}>
          {t('ai.addProvider')}
        </Button>
      </div>

      <Table
        {...tableStyle}
        columns={columns}
        dataSource={data}
        loading={loading}
        rowKey="id"
        expandable={{
          expandedRowRender: (record: any) => {
            const models = getModels(record.models)
            return (
              <div style={{ padding: '8px 0' }}>
                <strong>{t('ai.models')}：</strong>
                <div style={{ marginTop: 8 }}>
                  {models.length > 0 ? models.map((m: string) => <Tag key={m} style={{ marginBottom: 4 }}>{m}</Tag>) : '-'}
                </div>
                {record.remark && <div style={{ marginTop: 8 }}><strong>{t('common.remark')}：</strong>{record.remark}</div>}
              </div>
            )
          },
        }}
      />

      <Modal
        title={editRecord ? t('ai.editProvider') : t('ai.addProvider')}
        open={modalOpen}
        onOk={handleSubmit}
        onCancel={() => setModalOpen(false)}
        width={700}
        destroyOnClose
      >
        <Form form={form} layout="vertical" preserve={false}>
          <Form.Item name="name" label={t('common.name')} rules={[{ required: true }]}>
            <Input placeholder="OpenAI / DeepSeek / Claude..." />
          </Form.Item>
          <Form.Item name="base_url" label={t('ai.baseUrl')} rules={[{ required: true }]}>
            <Input placeholder="https://api.openai.com/v1" />
          </Form.Item>
          <Form.Item name="api_key" label={t('ai.apiKey')} rules={[{ required: true }]}>
            <Input.Password placeholder="sk-..." />
          </Form.Item>
          
          {/* 测试连接和获取模型按钮 */}
          <Form.Item label={t('ai.operations')}>
            <Space>
              <Button 
                icon={<CheckCircleOutlined />} 
                loading={testingInModal} 
                onClick={handleTestInModal}
              >
                {t('ai.testConnection')}
              </Button>
              <Button 
                icon={<SyncOutlined />} 
                loading={fetchingInModal} 
                onClick={handleFetchModelsInModal}
              >
                {t('ai.fetchModels')}
              </Button>
            </Space>
          </Form.Item>

          {/* 模型列表 */}
          <Form.Item 
            name="models" 
            label={t('ai.models')}
            tooltip={t('ai.modelsTooltip')}
            initialValue={[]}
          >
            <Select
              mode="tags"
              placeholder={t('ai.modelsPlaceholder')}
              style={{ width: '100%' }}
              tokenSeparators={[',']}
            />
          </Form.Item>

          <Form.Item name="is_active" label={t('common.status')} valuePropName="checked" initialValue={true}>
            <Switch checkedChildren={t('common.enable')} unCheckedChildren={t('common.disable')} />
          </Form.Item>
          <Form.Item name="remark" label={t('common.remark')}>
            <Input.TextArea rows={2} />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

export default AiProvider
