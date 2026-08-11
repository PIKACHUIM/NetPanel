import React, { useEffect, useState } from 'react'
import { Card, Table, Button, Modal, Form, Input, Select, Switch, Space, App, Tag, Popconfirm } from 'antd'
import { PlusOutlined, EditOutlined, DeleteOutlined, CheckCircleOutlined, CloseCircleOutlined } from '@ant-design/icons'
import { monitorApi } from '../api'
import { useTranslation } from 'react-i18next'
import ReactECharts from 'echarts-for-react'

const { Option } = Select

interface Probe {
  id: number
  name: string
  enable: boolean
  probe_type: string
  target_addr: string
  target_port?: number
  interval: number
  timeout: number
  fail_threshold: number
  remark?: string
}

interface ProbeResult {
  success: boolean
  response_time: number
  timestamp: string
}

const MonitorProbes: React.FC = () => {
  const { t } = useTranslation()
  const { message, modal } = App.useApp()
  const [loading, setLoading] = useState(false)
  const [probes, setProbes] = useState<Probe[]>([])
  const [modalVisible, setModalVisible] = useState(false)
  const [editingProbe, setEditingProbe] = useState<Probe | null>(null)
  const [resultsModalVisible, setResultsModalVisible] = useState(false)
  const [selectedProbeId, setSelectedProbeId] = useState<number | null>(null)
  const [probeResults, setProbeResults] = useState<ProbeResult[]>([])
  const [form] = Form.useForm()

  useEffect(() => {
    loadProbes()
  }, [])

  const loadProbes = async () => {
    setLoading(true)
    try {
      const response = await monitorApi.listProbes()
      setProbes(response.data || [])
    } catch (error: any) {
      message.error(error.message || '加载探测配置失败')
    } finally {
      setLoading(false)
    }
  }

  const showModal = (probe?: Probe) => {
    if (probe) {
      setEditingProbe(probe)
      form.setFieldsValue(probe)
    } else {
      setEditingProbe(null)
      form.resetFields()
      form.setFieldsValue({ enable: true, probe_type: 'tcp', interval: 60, timeout: 10, fail_threshold: 3 })
    }
    setModalVisible(true)
  }

  const handleSubmit = async () => {
    try {
      const values = await form.validateFields()
      if (editingProbe) {
        await monitorApi.updateProbe(editingProbe.id, values)
        message.success('更新成功')
      } else {
        await monitorApi.createProbe(values)
        message.success('创建成功')
      }
      setModalVisible(false)
      loadProbes()
    } catch (error: any) {
      message.error(error.message || '操作失败')
    }
  }

  const handleDelete = async (id: number) => {
    try {
      await monitorApi.deleteProbe(id)
      message.success('删除成功')
      loadProbes()
    } catch (error: any) {
      message.error(error.message || '删除失败')
    }
  }

  const showResults = async (probeId: number) => {
    setSelectedProbeId(probeId)
    setResultsModalVisible(true)
    try {
      const now = new Date()
      const start = new Date(now.getTime() - 24 * 60 * 60 * 1000) // 最近24小时
      const response = await monitorApi.getProbeResults(probeId, {
        start: start.toISOString(),
        end: now.toISOString(),
      })
      setProbeResults(response.data || [])
    } catch (error: any) {
      message.error(error.message || '加载探测结果失败')
    }
  }

  const getResultsChartOption = () => {
    const successData: any[] = []
    const failData: any[] = []

    probeResults.forEach((result) => {
      const time = new Date(result.timestamp).toLocaleTimeString()
      if (result.success) {
        successData.push([time, result.response_time])
      } else {
        failData.push([time, 0])
      }
    })

    return {
      backgroundColor: 'transparent',
      tooltip: {
        trigger: 'axis',
        axisPointer: { type: 'cross' },
      },
      legend: {
        data: ['响应时间', '失败'],
        textStyle: { color: '#fff' },
      },
      grid: {
        left: '3%',
        right: '4%',
        bottom: '3%',
        containLabel: true,
      },
      xAxis: {
        type: 'category',
        boundaryGap: false,
        data: probeResults.map((r) => new Date(r.timestamp).toLocaleTimeString()),
        axisLine: { lineStyle: { color: '#666' } },
        axisLabel: { color: '#999' },
      },
      yAxis: {
        type: 'value',
        name: '响应时间 (ms)',
        axisLine: { lineStyle: { color: '#666' } },
        axisLabel: { color: '#999' },
        splitLine: { lineStyle: { color: '#333' } },
      },
      series: [
        {
          name: '响应时间',
          type: 'line',
          smooth: true,
          data: probeResults.map((r) => (r.success ? r.response_time : null)),
          itemStyle: { color: '#52c41a' },
          areaStyle: {
            color: {
              type: 'linear',
              x: 0,
              y: 0,
              x2: 0,
              y2: 1,
              colorStops: [
                { offset: 0, color: 'rgba(82, 196, 26, 0.3)' },
                { offset: 1, color: 'rgba(82, 196, 26, 0.05)' },
              ],
            },
          },
        },
        {
          name: '失败',
          type: 'scatter',
          data: failData,
          itemStyle: { color: '#ff4d4f' },
          symbolSize: 8,
        },
      ],
    }
  }

  const columns = [
    {
      title: t('monitor.probe_name'),
      dataIndex: 'name',
      key: 'name',
    },
    {
      title: t('monitor.status'),
      dataIndex: 'enable',
      key: 'enable',
      render: (enable: boolean) => (
        <Tag icon={enable ? <CheckCircleOutlined /> : <CloseCircleOutlined />} color={enable ? 'success' : 'default'}>
          {enable ? t('monitor.enabled') : t('monitor.disabled')}
        </Tag>
      ),
    },
    {
      title: t('monitor.probe_type'),
      dataIndex: 'probe_type',
      key: 'probe_type',
      render: (type: string) => <Tag color="blue">{type.toUpperCase()}</Tag>,
    },
    {
      title: t('monitor.target'),
      key: 'target',
      render: (_: any, record: Probe) => {
        if (record.target_port) {
          return `${record.target_addr}:${record.target_port}`
        }
        return record.target_addr
      },
    },
    {
      title: t('monitor.interval'),
      dataIndex: 'interval',
      key: 'interval',
      render: (interval: number) => `${interval}s`,
    },
    {
      title: t('monitor.timeout'),
      dataIndex: 'timeout',
      key: 'timeout',
      render: (timeout: number) => `${timeout}s`,
    },
    {
      title: t('common.remark'),
      dataIndex: 'remark',
      key: 'remark',
      ellipsis: true,
    },
    {
      title: t('common.action'),
      key: 'action',
      render: (_: any, record: Probe) => (
        <Space>
          <Button type="link" onClick={() => showResults(record.id)}>
            {t('monitor.view_results')}
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
        title={t('monitor.probe_config')}
        extra={
          <Button type="primary" icon={<PlusOutlined />} onClick={() => showModal()}>
            {t('monitor.add_probe')}
          </Button>
        }
      >
        <Table columns={columns} dataSource={probes} rowKey="id" loading={loading} />
      </Card>

      <Modal
        title={editingProbe ? t('monitor.edit_probe') : t('monitor.add_probe')}
        open={modalVisible}
        onOk={handleSubmit}
        onCancel={() => setModalVisible(false)}
        width={600}
      >
        <Form form={form} layout="vertical">
          <Form.Item name="name" label={t('monitor.probe_name')} rules={[{ required: true }]}>
            <Input placeholder="输入探测名称" />
          </Form.Item>

          <Form.Item name="enable" label={t('monitor.enable')} valuePropName="checked">
            <Switch />
          </Form.Item>

          <Form.Item name="probe_type" label={t('monitor.probe_type')} rules={[{ required: true }]}>
            <Select>
              <Option value="tcp">TCP</Option>
              <Option value="udp">UDP</Option>
              <Option value="http">HTTP</Option>
              <Option value="https">HTTPS</Option>
              <Option value="icmp">ICMP (Ping)</Option>
            </Select>
          </Form.Item>

          <Form.Item name="target_addr" label={t('monitor.target_addr')} rules={[{ required: true }]}>
            <Input placeholder="IP 地址或域名" />
          </Form.Item>

          <Form.Item
            noStyle
            shouldUpdate={(prevValues, currentValues) => prevValues.probe_type !== currentValues.probe_type}
          >
            {({ getFieldValue }) =>
              ['tcp', 'udp'].includes(getFieldValue('probe_type')) ? (
                <Form.Item name="target_port" label={t('monitor.target_port')} rules={[{ required: true }]}>
                  <Input type="number" placeholder="端口号" />
                </Form.Item>
              ) : null
            }
          </Form.Item>

          <Form.Item name="interval" label={t('monitor.interval')} rules={[{ required: true }]}>
            <Input type="number" addonAfter="秒" />
          </Form.Item>

          <Form.Item name="timeout" label={t('monitor.timeout')} rules={[{ required: true }]}>
            <Input type="number" addonAfter="秒" />
          </Form.Item>

          <Form.Item name="fail_threshold" label={t('monitor.fail_threshold')} rules={[{ required: true }]}>
            <Input type="number" placeholder="失败次数阈值" />
          </Form.Item>

          <Form.Item name="remark" label={t('common.remark')}>
            <Input.TextArea rows={3} placeholder="备注信息" />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title={t('monitor.probe_results')}
        open={resultsModalVisible}
        onCancel={() => setResultsModalVisible(false)}
        footer={null}
        width={900}
      >
        <ReactECharts option={getResultsChartOption()} style={{ height: '400px' }} />
      </Modal>
    </div>
  )
}

export default MonitorProbes
