import React, { useEffect, useState } from 'react'
import {
  Card,
  Table,
  Button,
  Modal,
  Form,
  Input,
  Select,
  Switch,
  Space,
  App,
  Tag,
  Popconfirm,
  Drawer,
  List,
  Typography,
} from 'antd'
import {
  PlusOutlined,
  EditOutlined,
  DeleteOutlined,
  PlayCircleOutlined,
  ClockCircleOutlined,
  CheckCircleOutlined,
} from '@ant-design/icons'
import { monitorApi } from '../api'
import { useTranslation } from 'react-i18next'

const { Option } = Select
const { TextArea } = Input
const { Text } = Typography

interface Task {
  id: number
  name: string
  enable: boolean
  task_type: string
  cron_expr?: string
  trigger_event?: string
  command: string
  timeout: number
  concurrent: boolean
  last_run_time?: string
  last_run_result?: string
  remark?: string
}

interface TaskLog {
  id: number
  task_id: number
  server_id: number
  start_time: string
  end_time?: string
  status: string
  exit_code: number
  stdout: string
  stderr: string
}

const MonitorTasks: React.FC = () => {
  const { t } = useTranslation()
  const { message, modal } = App.useApp()
  const [loading, setLoading] = useState(false)
  const [tasks, setTasks] = useState<Task[]>([])
  const [modalVisible, setModalVisible] = useState(false)
  const [editingTask, setEditingTask] = useState<Task | null>(null)
  const [logsDrawerVisible, setLogsDrawerVisible] = useState(false)
  const [selectedTaskId, setSelectedTaskId] = useState<number | null>(null)
  const [taskLogs, setTaskLogs] = useState<TaskLog[]>([])
  const [form] = Form.useForm()

  useEffect(() => {
    loadTasks()
  }, [])

  const loadTasks = async () => {
    setLoading(true)
    try {
      const response = await monitorApi.listTasks()
      setTasks(response.data || [])
    } catch (error: any) {
      message.error(error.message || '加载任务失败')
    } finally {
      setLoading(false)
    }
  }

  const showModal = (task?: Task) => {
    if (task) {
      setEditingTask(task)
      form.setFieldsValue(task)
    } else {
      setEditingTask(null)
      form.resetFields()
      form.setFieldsValue({
        enable: true,
        task_type: 'cron',
        timeout: 300,
        concurrent: false,
      })
    }
    setModalVisible(true)
  }

  const handleSubmit = async () => {
    try {
      const values = await form.validateFields()
      if (editingTask) {
        await monitorApi.updateTask(editingTask.id, values)
        message.success('更新成功')
      } else {
        await monitorApi.createTask(values)
        message.success('创建成功')
      }
      setModalVisible(false)
      loadTasks()
    } catch (error: any) {
      message.error(error.message || '操作失败')
    }
  }

  const handleDelete = async (id: number) => {
    try {
      await monitorApi.deleteTask(id)
      message.success('删除成功')
      loadTasks()
    } catch (error: any) {
      message.error(error.message || '删除失败')
    }
  }

  const handleExecute = async (id: number) => {
    try {
      await monitorApi.executeTask(id)
      message.success('任务已提交执行')
      setTimeout(loadTasks, 2000)
    } catch (error: any) {
      message.error(error.message || '执行失败')
    }
  }

  const showLogs = async (taskId: number) => {
    setSelectedTaskId(taskId)
    setLogsDrawerVisible(true)
    try {
      const response = await monitorApi.getTaskLogs({ task_id: taskId, limit: 50 })
      setTaskLogs(response.data || [])
    } catch (error: any) {
      message.error(error.message || '加载日志失败')
    }
  }

  const getStatusTag = (status: string) => {
    const statusMap: Record<string, { color: string; text: string }> = {
      running: { color: 'processing', text: '运行中' },
      success: { color: 'success', text: '成功' },
      failed: { color: 'error', text: '失败' },
      timeout: { color: 'warning', text: '超时' },
    }
    const config = statusMap[status] || { color: 'default', text: status }
    return <Tag color={config.color}>{config.text}</Tag>
  }

  const columns = [
    {
      title: t('monitor.task_name'),
      dataIndex: 'name',
      key: 'name',
    },
    {
      title: t('monitor.status'),
      dataIndex: 'enable',
      key: 'enable',
      render: (enable: boolean) => (
        <Tag icon={enable ? <CheckCircleOutlined /> : <ClockCircleOutlined />} color={enable ? 'success' : 'default'}>
          {enable ? t('monitor.enabled') : t('monitor.disabled')}
        </Tag>
      ),
    },
    {
      title: t('monitor.task_type'),
      dataIndex: 'task_type',
      key: 'task_type',
      render: (type: string) => {
        const typeMap: Record<string, string> = {
          cron: '定时任务',
          trigger: '触发任务',
          manual: '手动任务',
        }
        return <Tag color="blue">{typeMap[type] || type}</Tag>
      },
    },
    {
      title: t('monitor.schedule'),
      key: 'schedule',
      render: (_: any, record: Task) => {
        if (record.task_type === 'cron') {
          return <Text code>{record.cron_expr}</Text>
        }
        if (record.task_type === 'trigger') {
          return <Tag>{record.trigger_event}</Tag>
        }
        return '-'
      },
    },
    {
      title: t('monitor.last_run'),
      dataIndex: 'last_run_time',
      key: 'last_run_time',
      render: (time: string) => {
        if (!time) return '-'
        return new Date(time).toLocaleString()
      },
    },
    {
      title: t('monitor.timeout'),
      dataIndex: 'timeout',
      key: 'timeout',
      render: (timeout: number) => `${timeout}s`,
    },
    {
      title: t('common.action'),
      key: 'action',
      render: (_: any, record: Task) => (
        <Space>
          <Button type="link" icon={<PlayCircleOutlined />} onClick={() => handleExecute(record.id)}>
            {t('monitor.execute')}
          </Button>
          <Button type="link" onClick={() => showLogs(record.id)}>
            {t('monitor.view_logs')}
          </Button>
          <Button type="link" icon={<EditOutlined />} onClick={() => showModal(record)}>
            {t('common.edit')}
          </Button>
          <Popconfirm title={t('common.confirm_delete')} onConfirm={() => handleDelete(record.id)}>
            <Button type="link" danger icon={<DeleteOutlined />}>
              {t('common.delete')}
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ]

  return (
    <div style={{ padding: '24px' }}>
      <Card
        title={t('monitor.task_management')}
        extra={
          <Button type="primary" icon={<PlusOutlined />} onClick={() => showModal()}>
            {t('monitor.add_task')}
          </Button>
        }
      >
        <Table columns={columns} dataSource={tasks} rowKey="id" loading={loading} />
      </Card>

      <Modal
        title={editingTask ? t('monitor.edit_task') : t('monitor.add_task')}
        open={modalVisible}
        onOk={handleSubmit}
        onCancel={() => setModalVisible(false)}
        width={700}
      >
        <Form form={form} layout="vertical">
          <Form.Item name="name" label={t('monitor.task_name')} rules={[{ required: true }]}>
            <Input placeholder="输入任务名称" />
          </Form.Item>

          <Form.Item name="enable" label={t('monitor.enable')} valuePropName="checked">
            <Switch />
          </Form.Item>

          <Form.Item name="task_type" label={t('monitor.task_type')} rules={[{ required: true }]}>
            <Select>
              <Option value="cron">定时任务</Option>
              <Option value="trigger">触发任务</Option>
              <Option value="manual">手动任务</Option>
            </Select>
          </Form.Item>

          <Form.Item
            noStyle
            shouldUpdate={(prevValues, currentValues) => prevValues.task_type !== currentValues.task_type}
          >
            {({ getFieldValue }) => {
              const taskType = getFieldValue('task_type')
              if (taskType === 'cron') {
                return (
                  <Form.Item name="cron_expr" label="Cron 表达式" rules={[{ required: true }]}>
                    <Input placeholder="例如: 0 0 * * * (每天0点)" />
                  </Form.Item>
                )
              }
              if (taskType === 'trigger') {
                return (
                  <Form.Item name="trigger_event" label="触发事件" rules={[{ required: true }]}>
                    <Select>
                      <Option value="server_offline">服务器离线</Option>
                      <Option value="server_online">服务器上线</Option>
                      <Option value="alert_trigger">告警触发</Option>
                    </Select>
                  </Form.Item>
                )
              }
              return null
            }}
          </Form.Item>

          <Form.Item name="command" label={t('monitor.command')} rules={[{ required: true }]}>
            <TextArea rows={5} placeholder="输入要执行的命令或脚本" />
          </Form.Item>

          <Form.Item name="server_ids" label={t('monitor.target_servers')} rules={[{ required: true }]}>
            <Input placeholder='服务器 ID 列表，JSON 格式，例如: ["1","2","3"]' />
          </Form.Item>

          <Form.Item name="timeout" label={t('monitor.timeout')} rules={[{ required: true }]}>
            <Input type="number" addonAfter="秒" />
          </Form.Item>

          <Form.Item name="concurrent" label={t('monitor.concurrent')} valuePropName="checked">
            <Switch checkedChildren="并发" unCheckedChildren="串行" />
          </Form.Item>

          <Form.Item name="remark" label={t('common.remark')}>
            <TextArea rows={2} placeholder="备注信息" />
          </Form.Item>
        </Form>
      </Modal>

      <Drawer
        title={t('monitor.task_logs')}
        open={logsDrawerVisible}
        onClose={() => setLogsDrawerVisible(false)}
        width={800}
      >
        <List
          dataSource={taskLogs}
          renderItem={(log) => (
            <List.Item key={log.id}>
              <Card size="small" style={{ width: '100%' }}>
                <Space direction="vertical" style={{ width: '100%' }}>
                  <Space>
                    <Text strong>服务器 ID:</Text>
                    <Text>{log.server_id}</Text>
                    {getStatusTag(log.status)}
                    <Text type="secondary">{new Date(log.start_time).toLocaleString()}</Text>
                  </Space>
                  {log.stdout && (
                    <div>
                      <Text strong>标准输出:</Text>
                      <pre style={{ background: '#f5f5f5', padding: '8px', borderRadius: '4px', marginTop: '4px' }}>
                        {log.stdout}
                      </pre>
                    </div>
                  )}
                  {log.stderr && (
                    <div>
                      <Text strong>错误输出:</Text>
                      <pre
                        style={{
                          background: '#fff1f0',
                          padding: '8px',
                          borderRadius: '4px',
                          marginTop: '4px',
                          color: '#cf1322',
                        }}
                      >
                        {log.stderr}
                      </pre>
                    </div>
                  )}
                  <Text type="secondary">
                    退出码: {log.exit_code} | 耗时:{' '}
                    {log.end_time
                      ? `${((new Date(log.end_time).getTime() - new Date(log.start_time).getTime()) / 1000).toFixed(2)}s`
                      : '运行中'}
                  </Text>
                </Space>
              </Card>
            </List.Item>
          )}
        />
      </Drawer>
    </div>
  )
}

export default MonitorTasks
