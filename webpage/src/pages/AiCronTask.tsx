import React, { useEffect, useState } from 'react'
import { Table, Button, Space, Modal, Form, Input, Select, Popconfirm, message, Tag, Switch } from 'antd'
import { PlusOutlined, EditOutlined, DeleteOutlined, ClockCircleOutlined, CaretRightOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { aiCronTaskApi, aiProviderApi } from '../api'
import { useTableStyle } from '../hooks/useTableStyle'
import CronExprInput from '../components/CronExprInput'
import dayjs from 'dayjs'

const { TextArea } = Input

const AiCronTask: React.FC = () => {
  const { t } = useTranslation()
  const tableStyle = useTableStyle()
  const [data, setData] = useState<any[]>([])
  const [loading, setLoading] = useState(false)
  const [modalOpen, setModalOpen] = useState(false)
  const [editRecord, setEditRecord] = useState<any>(null)
  const [form] = Form.useForm()
  const [providers, setProviders] = useState<any[]>([])
  const [logModal, setLogModal] = useState<{ open: boolean; taskId: number; logs: any[] }>({ open: false, taskId: 0, logs: [] })

  const fetchData = async () => {
    setLoading(true)
    try { const res: any = await aiCronTaskApi.list(); setData(res.data || []) }
    finally { setLoading(false) }
  }

  const loadProviders = async () => {
    try { const res: any = await aiProviderApi.list(); setProviders(res.data || []) } catch {}
  }

  useEffect(() => { fetchData() }, [])

  const handleSubmit = async () => {
    const values = await form.validateFields()
    if (editRecord) {
      await aiCronTaskApi.update(editRecord.id, values)
    } else {
      await aiCronTaskApi.create(values)
    }
    message.success(t('common.success'))
    setModalOpen(false)
    fetchData()
  }

  const openModal = (record?: any) => {
    setEditRecord(record || null)
    loadProviders()
    if (record) {
      form.setFieldsValue(record)
    } else {
      form.resetFields()
    }
    setModalOpen(true)
  }

  const handleToggle = async (id: number, enable: boolean) => {
    if (enable) {
      await aiCronTaskApi.enable(id)
    } else {
      await aiCronTaskApi.disable(id)
    }
    fetchData()
  }

  const handleRun = async (id: number) => {
    await aiCronTaskApi.run(id)
    message.success(t('ai.runNow') + ' - 已触发')
  }

  const openLogs = async (taskId: number) => {
    try {
      const res: any = await aiCronTaskApi.getLogs(taskId, 20)
      setLogModal({ open: true, taskId, logs: res.data || [] })
    } catch {}
  }

  const getModels = (providerId: number): string[] => {
    const p = providers.find((p: any) => p.id === providerId)
    if (!p) return []
    try { return JSON.parse(p.models || '[]') } catch { return [] }
  }

  const columns = [
    {
      title: t('common.status'), key: 'enable', width: 80,
      render: (_: any, r: any) => (
        <Switch size="small" checked={r.enable} onChange={(v) => handleToggle(r.id, v)} />
      ),
    },
    { title: t('common.name'), dataIndex: 'name', key: 'name', width: 150 },
    { title: t('ai.cronExpr'), dataIndex: 'cron_expr', key: 'cron_expr', width: 130 },
    { title: t('ai.model'), dataIndex: 'model_name', key: 'model_name', width: 150 },
    {
      title: t('ai.lastRun'), dataIndex: 'last_run_time', key: 'last_run_time', width: 170,
      render: (v: string) => v ? dayjs(v).format('YYYY-MM-DD HH:mm:ss') : '-',
    },
    {
      title: t('ai.lastResult'), dataIndex: 'last_run_result', key: 'last_run_result', ellipsis: true,
      render: (v: string) => v ? <span title={v}>{v.substring(0, 80)}</span> : '-',
    },
    {
      title: t('common.action'), key: 'action', width: 240,
      render: (_: any, record: any) => (
        <Space size="small">
          <Button size="small" icon={<CaretRightOutlined />} onClick={() => handleRun(record.id)}>
            {t('ai.runNow')}
          </Button>
          <Button size="small" onClick={() => openLogs(record.id)}>
            {t('ai.executionLogs')}
          </Button>
          <Button size="small" icon={<EditOutlined />} onClick={() => openModal(record)}>
            {t('common.edit')}
          </Button>
          <Popconfirm title={t('common.deleteConfirm')} onConfirm={async () => { await aiCronTaskApi.delete(record.id); message.success(t('common.success')); fetchData() }}>
            <Button size="small" danger icon={<DeleteOutlined />}>{t('common.delete')}</Button>
          </Popconfirm>
        </Space>
      ),
    },
  ]

  return (
    <div>
      <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <h2 style={{ margin: 0 }}><ClockCircleOutlined /> {t('ai.cronTask')}</h2>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => openModal()}>
          {t('ai.addCronTask')}
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
        title={editRecord ? t('ai.editCronTask') : t('ai.addCronTask')}
        open={modalOpen}
        onOk={handleSubmit}
        onCancel={() => setModalOpen(false)}
        width={700}
        destroyOnClose
      >
        <Form form={form} layout="vertical" preserve={false}>
          <Form.Item name="name" label={t('common.name')} rules={[{ required: true }]}>
            <Input placeholder="每日报告生成 / 数据汇总..." />
          </Form.Item>
          <Form.Item name="cron_expr" label={t('ai.cronExpr')} rules={[{ required: true }]}>
            <CronExprInput />
          </Form.Item>
          <Form.Item name="prompt" label={t('ai.prompt')} rules={[{ required: true }]} extra={t('ai.promptTip')}>
            <TextArea rows={4} placeholder="请帮我生成今日系统运行报告..." />
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
          <Form.Item name="max_tokens" label={t('ai.maxTokens')} initialValue={2000}>
            <Input type="number" />
          </Form.Item>
          <Form.Item name="enable" label={t('common.enable')} valuePropName="checked" initialValue={false}>
            <Switch />
          </Form.Item>
          <Form.Item name="remark" label={t('common.remark')}>
            <Input />
          </Form.Item>
        </Form>
      </Modal>

      {/* 执行日志弹窗 */}
      <Modal
        title={t('ai.executionLogs')}
        open={logModal.open}
        onCancel={() => setLogModal({ open: false, taskId: 0, logs: [] })}
        footer={null}
        width={800}
      >
        <Table
          size="small"
          dataSource={logModal.logs}
          rowKey="id"
          pagination={{ pageSize: 10 }}
          columns={[
            {
              title: '时间', dataIndex: 'created_at', width: 170,
              render: (v: string) => dayjs(v).format('YYYY-MM-DD HH:mm:ss'),
            },
            {
              title: '状态', dataIndex: 'success', width: 80,
              render: (v: boolean) => <Tag color={v ? 'green' : 'red'}>{v ? '成功' : '失败'}</Tag>,
            },
            {
              title: t('ai.duration'), dataIndex: 'duration_ms', width: 100,
              render: (v: number) => `${(v / 1000).toFixed(1)}s`,
            },
            {
              title: t('ai.tokens'), key: 'tokens', width: 120,
              render: (_: any, r: any) => `${r.tokens_prompt || 0}+${r.tokens_completion || 0}`,
            },
            { title: '结果', dataIndex: 'result', ellipsis: true },
            { title: '错误', dataIndex: 'error_msg', ellipsis: true, width: 150 },
          ]}
        />
      </Modal>
    </div>
  )
}

export default AiCronTask
