import React, { useEffect, useState } from 'react'
import {
  Table, Button, Space, Switch, Modal, Form, Input, InputNumber,
  Select, Popconfirm, message, Typography, Tag, Tooltip, Divider, Row, Col,
} from 'antd'
import {
  PlusOutlined, EditOutlined, DeleteOutlined,
  PlayCircleOutlined, StopOutlined, SyncOutlined, GlobalOutlined,
} from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { ddnsApi, domainAccountApi } from '../api'
import StatusTag from '../components/StatusTag'
import dayjs from 'dayjs'

const { Option } = Select
const { Text } = Typography

const PROVIDERS = [
  { value: 'alidns', label: '阿里云 DNS' },
  { value: 'cloudflare', label: 'Cloudflare' },
  { value: 'dnspod', label: 'DNSPod (腾讯云)' },
  { value: 'huaweidns', label: '华为云 DNS' },
  { value: 'godaddy', label: 'GoDaddy' },
  { value: 'namesilo', label: 'NameSilo' },
  { value: 'callback', label: 'Webhook 回调' },
]

const Ddns: React.FC = () => {
  const { t } = useTranslation()
  const [data, setData] = useState<any[]>([])
  const [accounts, setAccounts] = useState<any[]>([])
  const [loading, setLoading] = useState(false)
  const [modalOpen, setModalOpen] = useState(false)
  const [editRecord, setEditRecord] = useState<any>(null)
  const [ipGetType, setIpGetType] = useState('url')
  const [form] = Form.useForm()

  const fetchData = async () => {
    setLoading(true)
    try {
      const [ddnsRes, accRes]: any[] = await Promise.all([ddnsApi.list(), domainAccountApi.list()])
      setData(ddnsRes.data || [])
      setAccounts(accRes.data || [])
    } finally { setLoading(false) }
  }
  useEffect(() => { fetchData() }, [])

  const handleOpen = (record?: any) => {
    if (record) {
      setEditRecord(record)
      setIpGetType(record.ip_get_type || 'url')
      form.setFieldsValue({
        ...record,
        domains: (() => { try { return JSON.parse(record.domains || '[]').join('\n') } catch { return record.domains } })(),
        ip_get_urls: (() => { try { return JSON.parse(record.ip_get_urls || '[]').join('\n') } catch { return record.ip_get_urls } })(),
      })
    } else {
      setEditRecord(null)
      setIpGetType('url')
      form.resetFields()
      form.setFieldsValue({
        enable: true, task_type: 'IPv4', ip_get_type: 'url',
        interval: 300, ttl: 'auto',
        ip_get_urls: 'https://api.ipify.org\nhttps://api4.ipify.org',
      })
    }
    setModalOpen(true)
  }

  const handleSubmit = async () => {
    const values = await form.validateFields()
    if (typeof values.domains === 'string') {
      values.domains = JSON.stringify(values.domains.split('\n').filter(Boolean))
    }
    if (typeof values.ip_get_urls === 'string') {
      values.ip_get_urls = JSON.stringify(values.ip_get_urls.split('\n').filter(Boolean))
    }
    if (editRecord) {
      await ddnsApi.update(editRecord.id, values)
    } else {
      await ddnsApi.create(values)
    }
    message.success(t('common.success'))
    setModalOpen(false)
    fetchData()
  }

  const handleToggle = async (record: any, checked: boolean) => {
    await ddnsApi.update(record.id, { ...record, enable: checked })
    if (checked) await ddnsApi.start(record.id)
    else await ddnsApi.stop(record.id)
    fetchData()
  }

  const columns = [
    {
      title: t('common.status'), dataIndex: 'status', width: 100,
      render: (s: string) => <StatusTag status={s} />,
    },
    {
      title: t('common.enable'), dataIndex: 'enable', width: 70,
      render: (v: boolean, r: any) => (
        <Switch size="small" checked={v} onChange={(c) => handleToggle(r, c)} />
      ),
    },
    {
      title: t('common.name'), dataIndex: 'name',
      render: (name: string, r: any) => (
        <div>
          <Text strong>{name}</Text>
          {r.remark && <div><Text type="secondary" style={{ fontSize: 12 }}>{r.remark}</Text></div>}
        </div>
      ),
    },
    {
      title: t('ddns.taskType'), dataIndex: 'task_type', width: 80,
      render: (v: string) => <Tag color={v === 'IPv6' ? 'purple' : 'blue'}>{v || 'IPv4'}</Tag>,
    },
    {
      title: t('ddns.provider'), dataIndex: 'provider',
      render: (v: string) => PROVIDERS.find(p => p.value === v)?.label || v,
    },
    {
      title: t('ddns.domains'), dataIndex: 'domains',
      render: (v: string) => {
        try {
          const arr = JSON.parse(v || '[]')
          return arr.map((d: string) => <Tag key={d} icon={<GlobalOutlined />}>{d}</Tag>)
        } catch { return v }
      },
    },
    {
      title: t('ddns.currentIP'), dataIndex: 'current_ip',
      render: (v: string) => v ? <Text code>{v}</Text> : <Text type="secondary">-</Text>,
    },
    {
      title: t('ddns.lastUpdateTime'), dataIndex: 'last_update_time', width: 120,
      render: (v: string) => v ? dayjs(v).format('MM-DD HH:mm') : '-',
    },
    {
      title: t('common.action'), width: 160,
      render: (_: any, r: any) => (
        <Space size={4}>
          {r.status === 'running'
            ? <Tooltip title={t('common.stop')}><Button size="small" icon={<StopOutlined />} onClick={async () => { await ddnsApi.stop(r.id); fetchData() }} /></Tooltip>
            : <Tooltip title={t('common.start')}><Button size="small" type="primary" icon={<PlayCircleOutlined />} onClick={async () => { await ddnsApi.start(r.id); fetchData() }} /></Tooltip>
          }
          <Tooltip title={t('ddns.runNow')}>
            <Button size="small" icon={<SyncOutlined />} onClick={async () => { await ddnsApi.runNow(r.id); message.success('已触发更新') }} />
          </Tooltip>
          <Tooltip title={t('common.edit')}>
            <Button size="small" icon={<EditOutlined />} onClick={() => handleOpen(r)} />
          </Tooltip>
          <Popconfirm title={t('common.deleteConfirm')} onConfirm={async () => { await ddnsApi.delete(r.id); fetchData() }}>
            <Tooltip title={t('common.delete')}>
              <Button size="small" danger icon={<DeleteOutlined />} />
            </Tooltip>
          </Popconfirm>
        </Space>
      ),
    },
  ]

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Typography.Title level={4} style={{ margin: 0 }}>{t('ddns.title')}</Typography.Title>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => handleOpen()}>
          {t('common.create')}
        </Button>
      </div>

      <Table
        dataSource={data} columns={columns} rowKey="id" loading={loading}
        size="middle" style={{ background: '#fff', borderRadius: 8 }}
        pagination={{ pageSize: 20, showSizeChanger: true }}
      />

      <Modal
        title={editRecord ? t('common.edit') : t('common.create')}
        open={modalOpen} onOk={handleSubmit} onCancel={() => setModalOpen(false)}
        width={600} destroyOnClose
      >
        <Form form={form} layout="vertical" style={{ marginTop: 16 }}>
          <Row gutter={16}>
            <Col span={16}>
              <Form.Item name="name" label={t('common.name')} rules={[{ required: true }]}>
                <Input placeholder="DDNS任务名称" />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="enable" label={t('common.enable')} valuePropName="checked">
                <Switch />
              </Form.Item>
            </Col>
          </Row>

          <Row gutter={16}>
            <Col span={12}>
              <Form.Item name="task_type" label={t('ddns.taskType')} rules={[{ required: true }]}>
                <Select>
                  <Option value="IPv4">IPv4 (A记录)</Option>
                  <Option value="IPv6">IPv6 (AAAA记录)</Option>
                </Select>
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="interval" label={t('ddns.interval')}>
                <InputNumber min={60} max={86400} style={{ width: '100%' }} addonAfter="秒" />
              </Form.Item>
            </Col>
          </Row>

          <Divider orientation="left" plain style={{ fontSize: 13 }}>DNS 服务商</Divider>

          <Form.Item name="provider" label={t('ddns.provider')} rules={[{ required: true }]}>
            <Select placeholder="选择DNS服务商">
              {PROVIDERS.map(p => <Option key={p.value} value={p.value}>{p.label}</Option>)}
            </Select>
          </Form.Item>

          <Form.Item
            noStyle
            shouldUpdate={(prev, cur) => prev.provider !== cur.provider}
          >
            {({ getFieldValue }) => {
              const provider = getFieldValue('provider')
              if (provider === 'callback') return null
              return (
                <Row gutter={16}>
                  <Col span={12}>
                    <Form.Item name="access_id" label={t('ddns.accessID')}>
                      <Input placeholder="Access Key ID" />
                    </Form.Item>
                  </Col>
                  <Col span={12}>
                    <Form.Item name="access_secret" label={t('ddns.accessSecret')}>
                      <Input.Password placeholder="Access Key Secret" />
                    </Form.Item>
                  </Col>
                </Row>
              )
            }}
          </Form.Item>

          <Form.Item name="domains" label={t('ddns.domains')} rules={[{ required: true }]}
            extra="每行一个域名，如：home.example.com 或 *.example.com">
            <Input.TextArea rows={3} placeholder={'home.example.com\nwww.example.com'} />
          </Form.Item>

          <Form.Item name="ttl" label={t('ddns.ttl')}>
            <Input placeholder="auto（留空自动）" />
          </Form.Item>

          <Divider orientation="left" plain style={{ fontSize: 13 }}>IP 获取方式</Divider>

          <Form.Item name="ip_get_type" label={t('ddns.ipGetType')}>
            <Select onChange={setIpGetType}>
              <Option value="url">URL 查询</Option>
              <Option value="interface">网络接口</Option>
              <Option value="custom">自定义命令</Option>
            </Select>
          </Form.Item>

          {ipGetType === 'url' && (
            <Form.Item name="ip_get_urls" label={t('ddns.ipGetURLs')}
              extra="每行一个URL，依次尝试直到成功">
              <Input.TextArea rows={3} placeholder={'https://api.ipify.org\nhttps://api4.ipify.org'} />
            </Form.Item>
          )}
          {ipGetType === 'interface' && (
            <Form.Item name="net_interface" label="网络接口">
              <Input placeholder="如：eth0、ens33（留空自动检测）" />
            </Form.Item>
          )}
          {ipGetType === 'custom' && (
            <Form.Item name="ip_regex" label="IP 正则过滤">
              <Input placeholder="可选，用于从命令输出中提取IP" />
            </Form.Item>
          )}

          <Form.Item name="remark" label={t('common.remark')}>
            <Input.TextArea rows={2} />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

export default Ddns
