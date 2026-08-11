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
  Tabs,
  InputNumber,
  List,
  Typography,
} from 'antd'
import {
  PlusOutlined,
  EditOutlined,
  DeleteOutlined,
  BellOutlined,
  CheckCircleOutlined,
  WarningOutlined,
} from '@ant-design/icons'
import { monitorApi } from '../api'
import { useTranslation } from 'react-i18next'

const { Option } = Select
const { TextArea } = Input
const { Text } = Typography

interface Alert {
  id: number
  name: string
  enable: boolean
  alert_type: string
  target_servers: string
  threshold_config: string
  notify_channels: string
  severity: string
  silence_duration: number
  rate_limit: number
  remark?: string
}

interface AlertRecord {
  id: number
  alert_id: number
  server_id: number
  trigger_time: string
  recover_time?: string
  severity: string
  alert_content: string
  notify_sent: boolean
}

const MonitorAlerts: React.FC = () => {
  const { t } = useTranslation()
  const { message, modal } = App.useApp()
  const [loading, setLoading] = useState(false)
  const [alerts, setAlerts] = useState<Alert[]>([])
  const [alertRecords, setAlertRecords] = useState<AlertRecord[]>([])
  const [modalVisible, setModalVisible] = useState(false)
  const [editingAlert, setEditingAlert] = useState<Alert | null>(null)
  const [activeTab, setActiveTab] = useState('rules')
  const [form] = Form.useForm()

  useEffect(() => {
    loadAlerts()
    loadAlertRecords()
  }, [])

  const loadAlerts = async () => {
    setLoading(true)
    try {
      const response = await monitorApi.listAlerts()
      setAlerts(response.data || [])
    } catch (error: any) {
      message.error(error.message || '加载告警规则失败')
    } finally {
      setLoading(false)
    }
  }

  const loadAlertRecords = async () => {
    try {
      const response = await monitorApi.getAlertRecords({ limit: 100 })
      setAlertRecords(response.data || [])
    } catch (error: any) {
      message.error(error.message || '加载告警记录失败')
    }
  }

  const showModal = (alert?: Alert) => {
    if (alert) {
      setEditingAlert(alert)
      const thresholdConfig = JSON.parse(alert.threshold_config || '{}')
      form.setFieldsValue({
        ...alert,
        operator: thresholdConfig.operator || 'gt',
        value: thresholdConfig.value || 80,
        duration: thresholdConfig.duration || 300,
      })
    } else {
      setEditingAlert(null)
      form.resetFields()
      form.setFieldsValue({
        enable: true,
        alert_type: 'cpu',
        severity: 'warning',
        silence_duration: 3600,
        rate_limit: 300,
        operator: 'gt',
        value: 80,
        duration: 300,
      })
    }
    setModalVisible(true)
  }

  const handleSubmit = async () => {
    try {
      const values = await form.validateFields()
      
      // 构建阈值配置
      const thresholdConfig = {
        operator: values.operator,
        value: values.value,
        duration: values.duration,
      }
      
      const data = {
        name: values.name,
        enable: values.enable,
        alert_type: values.alert_type,
        target_servers: values.target_servers,
        threshold_config: JSON.stringify(thresholdConfig),
        notify_channels: values.notify_channels,
        severity: values.severity,
        silence_duration: values.silence_duration,
        rate_limit: values.rate_limit,
        remark: values.remark,
      }

      if (editingAlert) {
        await monitorApi.updateAlert(editingAlert.id, data)
        message.success('更新成功')
      } else {
        await monitorApi.createAlert(data)
        message.success('创建成功')
      }
      setModalVisible(false)
      loadAlerts()
    } catch (error: any) {
      message.error(error.message || '操作失败')
    }
  }

  const handleDelete = async (id: number) => {
    try {
      await monitorApi.deleteAlert(id)
      message.success('删除成功')
      loadAlerts()
    } catch (error: any) {
      message.error(error.message || '删除失败')
    }
  }

  const getSeverityTag = (severity: string) => {
    const severityMap: Record<string, { color: string; text: string }> = {
      info: { color: 'default', text: '信息' },
      warning: { color: 'warning', text: '警告' },
      error: { color: 'error', text: '错误' },
      critical: { color: 'red', text: '严重' },
    }
    const config = severityMap[severity] || { color: 'default', text: severity }
    return <Tag color={config.color}>{config.text}</Tag>
  }

  const getAlertTypeText = (type: string) => {
    const typeMap: Record<string, string> = {
      cpu: 'CPU 使用率',
      memory: '内存使用率',
      disk: '磁盘使用率',
      network: '网络流量',
      process: '进程数',
      offline: '服务器离线',
      probe: '探测失败',
    }
    return typeMap[type] || type
  }

  const alertColumns = [
    {
      title: t('monitor.alert_name'),
      dataIndex: 'name',
      key: 'name',
    },
    {
      title: t('monitor.status'),
      dataIndex: 'enable',
      key: 'enable',
      render: (enable: boolean) => (
        <Tag icon={enable ? <CheckCircleOutlined /> : <BellOutlined />} color={enable ? 'success' : 'default'}>
          {enable ? t('monitor.enabled') : t('monitor.disabled')}
        </Tag>
      ),
    },
    {
      title: t('monitor.alert_type'),
      dataIndex: 'alert_type',
      key: 'alert_type',
      render: (type: string) => <Tag color="blue">{getAlertTypeText(type)}</Tag>,
    },
    {
      title: t('monitor.threshold'),
      dataIndex: 'threshold_config',
      key: 'threshold_config',
      render: (config: string) => {
        try {
          const { operator, value, duration } = JSON.parse(config || '{}')
          const opMap: Record<string, string> = { gt: '>', lt: '<', eq: '=', gte: '>=', lte: '<=' }
          return `${opMap[operator] || operator} ${value} (持续 ${duration}s)`
        } catch {
          return '-'
        }
      },
    },
    {
      title: t('monitor.severity'),
      dataIndex: 'severity',
      key: 'severity',
      render: (severity: string) => getSeverityTag(severity),
    },
    {
      title: t('monitor.silence'),
      dataIndex: 'silence_duration',
      key: 'silence_duration',
      render: (duration: number) => `${duration / 60}分钟`,
    },
    {
      title: t('common.action'),
      key: 'action',
      render: (_: any, record: Alert) => (
        <Space>
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

  const recordColumns = [
    {
      title: t('monitor.trigger_time'),
      dataIndex: 'trigger_time',
      key: 'trigger_time',
      render: (time: string) => new Date(time).toLocaleString(),
    },
    {
      title: t('monitor.server_id'),
      dataIndex: 'server_id',
      key: 'server_id',
    },
    {
      title: t('monitor.severity'),
      dataIndex: 'severity',
      key: 'severity',
      render: (severity: string) => getSeverityTag(severity),
    },
    {
      title: t('monitor.alert_content'),
      dataIndex: 'alert_content',
      key: 'alert_content',
      ellipsis: true,
    },
    {
      title: t('monitor.recover_time'),
      dataIndex: 'recover_time',
      key: 'recover_time',
      render: (time: string) => {
        if (!time) return <Tag color="error">未恢复</Tag>
        return new Date(time).toLocaleString()
      },
    },
    {
      title: t('monitor.notify_sent'),
      dataIndex: 'notify_sent',
      key: 'notify_sent',
      render: (sent: boolean) => (
        <Tag color={sent ? 'success' : 'default'}>{sent ? '已发送' : '未发送'}</Tag>
      ),
    },
  ]

  return (
    <div style={{ padding: '24px' }}>
      <Card
        title={t('monitor.alert_management')}
        extra={
          <Button type="primary" icon={<PlusOutlined />} onClick={() => showModal()}>
            {t('monitor.add_alert')}
          </Button>
        }
      >
        <Tabs activeKey={activeTab} onChange={setActiveTab}>
          <Tabs.TabPane tab="告警规则" key="rules">
            <Table columns={alertColumns} dataSource={alerts} rowKey="id" loading={loading} />
          </Tabs.TabPane>
          <Tabs.TabPane tab="告警记录" key="records">
            <Table columns={recordColumns} dataSource={alertRecords} rowKey="id" pagination={{ pageSize: 20 }} />
          </Tabs.TabPane>
        </Tabs>
      </Card>

      <Modal
        title={editingAlert ? t('monitor.edit_alert') : t('monitor.add_alert')}
        open={modalVisible}
        onOk={handleSubmit}
        onCancel={() => setModalVisible(false)}
        width={700}
      >
        <Form form={form} layout="vertical">
          <Form.Item name="name" label={t('monitor.alert_name')} rules={[{ required: true }]}>
            <Input placeholder="输入告警规则名称" />
          </Form.Item>

          <Form.Item name="enable" label={t('monitor.enable')} valuePropName="checked">
            <Switch />
          </Form.Item>

          <Form.Item name="alert_type" label={t('monitor.alert_type')} rules={[{ required: true }]}>
            <Select>
              <Option value="cpu">CPU 使用率</Option>
              <Option value="memory">内存使用率</Option>
              <Option value="disk">磁盘使用率</Option>
              <Option value="network">网络流量</Option>
              <Option value="process">进程数</Option>
              <Option value="offline">服务器离线</Option>
              <Option value="probe">探测失败</Option>
            </Select>
          </Form.Item>

          <Form.Item label="阈值配置">
            <Space.Compact style={{ width: '100%' }}>
              <Form.Item name="operator" noStyle rules={[{ required: true }]}>
                <Select style={{ width: '25%' }}>
                  <Option value="gt">大于 (&gt;)</Option>
                  <Option value="gte">大于等于 (&gt;=)</Option>
                  <Option value="lt">小于 (&lt;)</Option>
                  <Option value="lte">小于等于 (&lt;=)</Option>
                  <Option value="eq">等于 (=)</Option>
                </Select>
              </Form.Item>
              <Form.Item name="value" noStyle rules={[{ required: true }]}>
                <InputNumber style={{ width: '35%' }} placeholder="阈值" />
              </Form.Item>
              <Form.Item name="duration" noStyle rules={[{ required: true }]}>
                <InputNumber style={{ width: '40%' }} addonAfter="秒" placeholder="持续时间" />
              </Form.Item>
            </Space.Compact>
          </Form.Item>

          <Form.Item name="target_servers" label={t('monitor.target_servers')} rules={[{ required: true }]}>
            <TextArea
              rows={2}
              placeholder='目标服务器，JSON 格式，例如: ["server:1","server:2"] 或 ["group:default"]'
            />
          </Form.Item>

          <Form.Item name="notify_channels" label={t('monitor.notify_channels')}>
            <Input placeholder='通知渠道 ID 列表，JSON 格式，例如: ["1","2","3"]' />
          </Form.Item>

          <Form.Item name="severity" label={t('monitor.severity')} rules={[{ required: true }]}>
            <Select>
              <Option value="info">信息</Option>
              <Option value="warning">警告</Option>
              <Option value="error">错误</Option>
              <Option value="critical">严重</Option>
            </Select>
          </Form.Item>

          <Form.Item name="silence_duration" label="静默时间（秒）" rules={[{ required: true }]}>
            <InputNumber style={{ width: '100%' }} placeholder="告警触发后静默时间" />
          </Form.Item>

          <Form.Item name="rate_limit" label="通知限流（秒）" rules={[{ required: true }]}>
            <InputNumber style={{ width: '100%' }} placeholder="最小通知间隔" />
          </Form.Item>

          <Form.Item name="remark" label={t('common.remark')}>
            <TextArea rows={2} placeholder="备注信息" />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

export default MonitorAlerts
