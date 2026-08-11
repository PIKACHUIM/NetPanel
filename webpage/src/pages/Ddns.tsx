import React, { useEffect, useState } from 'react'
import {
  Table, Button, Space, Switch, Modal, Form, Input, InputNumber,
  Select, Popconfirm, message, Typography, Tag, Tooltip, Divider, Row, Col,
  Tabs, Badge, Descriptions,
} from 'antd'
import {
  PlusOutlined, EditOutlined, DeleteOutlined,
  PlayCircleOutlined, StopOutlined, SyncOutlined, GlobalOutlined,
  HistoryOutlined, FileTextOutlined,
} from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { ddnsApi, domainAccountApi, adminApi } from '../api'
import StatusTag from '../components/StatusTag'
import dayjs from 'dayjs'
import { useTableStyle } from '../hooks/useTableStyle'

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
  const tableStyle = useTableStyle()
  const [data, setData] = useState<any[]>([])
  const [accounts, setAccounts] = useState<any[]>([])
  const [loading, setLoading] = useState(false)
  const [modalOpen, setModalOpen] = useState(false)
  const [editRecord, setEditRecord] = useState<any>(null)
  const [ipGetType, setIpGetType] = useState('url')
  const [form] = Form.useForm()

  // 历史记录
  const [historyOpen, setHistoryOpen] = useState(false)
  const [historyLoading, setHistoryLoading] = useState(false)
  const [historyData, setHistoryData] = useState<any[]>([])
  const [historyRecord, setHistoryRecord] = useState<any>(null)

  // 执行日志
  const [logsOpen, setLogsOpen] = useState(false)
  const [logsLoading, setLogsLoading] = useState(false)
  const [logsData, setLogsData] = useState<any[]>([])
  const [logsTotal, setLogsTotal] = useState(0)
  const [logsRecord, setLogsRecord] = useState<any>(null)
  const [logsPage, setLogsPage] = useState(1)

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
        webhook_method: 'POST', http_timeout: 10,
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

  const handleViewHistory = async (record: any) => {
    setHistoryRecord(record)
    setHistoryOpen(true)
    setHistoryLoading(true)
    try {
      const res: any = await ddnsApi.getHistory(record.id)
      setHistoryData(res.data?.list || res.data || [])
    } finally {
      setHistoryLoading(false)
    }
  }

  const handleViewLogs = async (record: any, page = 1) => {
    setLogsRecord(record)
    setLogsOpen(true)
    setLogsLoading(true)
    setLogsPage(page)
    try {
      const res: any = await adminApi.queryLogs({
        service: 'ddns',
        keyword: `[${record.id}]`,
        page,
        page_size: 20,
        order: 'desc',
      })
      setLogsData(res.data?.items || [])
      setLogsTotal(res.data?.total || 0)
    } finally {
      setLogsLoading(false)
    }
  }

  const LEVEL_COLORS: Record<string, string> = {
    info: 'blue', warning: 'orange', warn: 'orange', error: 'red', debug: 'default',
  }

  const logsColumns = [
    {
      title: t('ddns.logTime'), dataIndex: 'log_time', width: 160,
      render: (v: string) => dayjs(v).format('YYYY-MM-DD HH:mm:ss'),
    },
    {
      title: t('ddns.logLevel'), dataIndex: 'level', width: 80,
      render: (v: string) => <Tag color={LEVEL_COLORS[v] || 'default'}>{v?.toUpperCase()}</Tag>,
    },
    {
      title: t('ddns.logMessage'), dataIndex: 'message',
      render: (v: string) => <Text style={{ fontSize: 12, wordBreak: 'break-all' }}>{v}</Text>,
    },
  ]

  const historyColumns = [
    {
      title: '时间', dataIndex: 'created_at', width: 150,
      render: (v: string) => dayjs(v).format('MM-DD HH:mm:ss'),
    },
    {
      title: '域名', dataIndex: 'domain',
      render: (v: string) => <Tag icon={<GlobalOutlined />}>{v}</Tag>,
    },
    {
      title: t('ddns.oldIP'), dataIndex: 'old_ip',
      render: (v: string) => v ? <Text code style={{ fontSize: 12 }}>{v}</Text> : <Text type="secondary">-</Text>,
    },
    {
      title: t('ddns.newIP'), dataIndex: 'new_ip',
      render: (v: string) => <Text code style={{ fontSize: 12, color: '#52c41a' }}>{v}</Text>,
    },
    {
      title: '结果', dataIndex: 'success', width: 80,
      render: (v: boolean) => v
        ? <Badge status="success" text={<Text style={{ color: '#52c41a', fontSize: 12 }}>成功</Text>} />
        : <Badge status="error" text={<Text type="danger" style={{ fontSize: 12 }}>失败</Text>} />,
    },
    {
      title: '消息', dataIndex: 'message',
      render: (v: string) => v ? <Text type="secondary" style={{ fontSize: 12 }}>{v}</Text> : '-',
    },
  ]

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
      title: t('common.action'), width: 200,
      render: (_: any, r: any) => (
        <Space size={4}>
          {r.status === 'running'
            ? <Tooltip title={t('common.stop')}><Button size="small" icon={<StopOutlined />} onClick={async () => { await ddnsApi.stop(r.id); fetchData() }} /></Tooltip>
            : <Tooltip title={t('common.start')}><Button size="small" type="primary" icon={<PlayCircleOutlined />} onClick={async () => { await ddnsApi.start(r.id); fetchData() }} /></Tooltip>
          }
          <Tooltip title={t('ddns.runNow')}>
            <Button size="small" icon={<SyncOutlined />} onClick={async () => { await ddnsApi.runNow(r.id); message.success('已触发更新') }} />
          </Tooltip>
          <Tooltip title={t('ddns.viewLogs')}>
            <Button size="small" icon={<FileTextOutlined />} onClick={() => handleViewLogs(r)} />
          </Tooltip>
          <Tooltip title={t('ddns.viewHistory')}>
            <Button size="small" icon={<HistoryOutlined />} onClick={() => handleViewHistory(r)} />
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
        size="middle" style={tableStyle}
        pagination={{ pageSize: 20, showSizeChanger: true }}
      />

      <Modal
        title={editRecord ? t('common.edit') : t('common.create')}
        open={modalOpen} onOk={handleSubmit} onCancel={() => setModalOpen(false)}
        width={640} destroyOnHidden
        styles={{ body: { padding: '4px 24px 0' } }}
      >
        <Form form={form} layout="vertical" style={{ paddingTop: 4 }}>
          <Tabs size="small" items={[
            {
              key: 'basic', label: '基本配置',
              children: (
                <>
                  <Row gutter={16}>
                    <Col span={16}>
                      <Form.Item name="name" label={t('common.name')} rules={[{ required: true }]}>
                        <Input placeholder="DDNS任务名称" style={{ width: '100%' }} />
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
                  <Form.Item name="domain_account_id" label="域名账号（可选）"
                    extra="选择已有域名账号可自动填充凭证，也可以直接在下方手动填写">
                    <Select placeholder="不使用域名账号，直接填写凭证" allowClear
                      onChange={(val) => {
                        if (val) {
                          const account = accounts.find(a => a.id === val)
                          if (account) {
                            form.setFieldsValue({
                              provider: account.provider,
                              access_id: account.access_id,
                              access_secret: account.access_secret,
                            })
                          }
                        }
                      }}>
                      {accounts.map(a => (
                        <Option key={a.id} value={a.id}>
                          {a.name} ({PROVIDERS.find(p => p.value === a.provider)?.label || a.provider})
                        </Option>
                      ))}
                    </Select>
                  </Form.Item>
                  <Form.Item name="provider" label={t('ddns.provider')} rules={[{ required: true }]}>
                    <Select placeholder="选择DNS服务商">
                      {PROVIDERS.map(p => <Option key={p.value} value={p.value}>{p.label}</Option>)}
                    </Select>
                  </Form.Item>
                  <Form.Item noStyle shouldUpdate={(prev, cur) => prev.provider !== cur.provider || prev.domain_account_id !== cur.domain_account_id}>
                    {({ getFieldValue }) => {
                      const provider = getFieldValue('provider')
                      const useDomainAccount = getFieldValue('domain_account_id')
                      if (provider === 'callback') return null
                      
                      // 根据不同服务商显示不同的字段标签
                      let idLabel = 'Access Key ID'
                      let idPlaceholder = 'Access Key ID'
                      let secretLabel = 'Access Key Secret'
                      let secretPlaceholder = 'Access Key Secret'
                      let idExtra = ''
                      
                      if (provider === 'cloudflare') {
                        idLabel = 'Zone ID（可选）'
                        idPlaceholder = 'Cloudflare Zone ID（留空自动查询）'
                        secretLabel = 'API Token'
                        secretPlaceholder = '输入 Cloudflare API Token'
                        idExtra = '留空时将自动根据域名查询 Zone ID'
                      } else if (provider === 'alidns') {
                        idLabel = 'AccessKey ID'
                        secretLabel = 'AccessKey Secret'
                      } else if (provider === 'dnspod') {
                        idLabel = 'SecretId'
                        secretLabel = 'SecretKey'
                      }
                      
                      return (
                        <Row gutter={16}>
                          <Col span={12}>
                            <Form.Item name="access_id" label={idLabel} extra={idExtra}>
                              <Input placeholder={idPlaceholder} style={{ width: '100%' }} 
                                disabled={!!useDomainAccount} />
                            </Form.Item>
                          </Col>
                          <Col span={12}>
                            <Form.Item name="access_secret" label={secretLabel} 
                              rules={provider !== 'callback' ? [{ required: !useDomainAccount, message: '请填写凭证或选择域名账号' }] : []}>
                              <Input.Password placeholder={secretPlaceholder} style={{ width: '100%' }} 
                                disabled={!!useDomainAccount} />
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
                    <Input placeholder="auto（留空自动）" style={{ width: '100%' }} />
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
                      <Input placeholder="如：eth0、ens33（留空自动检测）" style={{ width: '100%' }} />
                    </Form.Item>
                  )}
                  {ipGetType === 'custom' && (
                    <Form.Item name="ip_regex" label="IP 正则过滤">
                      <Input placeholder="可选，用于从命令输出中提取IP" style={{ width: '100%' }} />
                    </Form.Item>
                  )}
                  <Form.Item name="remark" label={t('common.remark')}>
                    <Input.TextArea rows={2} style={{ width: '100%' }} />
                  </Form.Item>
                </>
              ),
            },
            {
              key: 'webhook', label: 'Webhook 通知',
              children: (
                <>
                  <Form.Item name="webhook_enable" label={t('ddns.webhookEnable')} valuePropName="checked">
                    <Switch checkedChildren="已启用" unCheckedChildren="已禁用" />
                  </Form.Item>
                  <Form.Item noStyle shouldUpdate={(prev, cur) => prev.webhook_enable !== cur.webhook_enable}>
                    {({ getFieldValue }) => getFieldValue('webhook_enable') ? (
                      <>
                        <Row gutter={16}>
                          <Col span={16}>
                            <Form.Item name="webhook_url" label={t('ddns.webhookURL')} rules={[{ required: true }]}>
                              <Input placeholder="https://example.com/webhook" style={{ width: '100%' }} />
                            </Form.Item>
                          </Col>
                          <Col span={8}>
                            <Form.Item name="webhook_method" label={t('ddns.webhookMethod')}>
                              <Select>
                                <Option value="GET">GET</Option>
                                <Option value="POST">POST</Option>
                                <Option value="PUT">PUT</Option>
                              </Select>
                            </Form.Item>
                          </Col>
                        </Row>
                        <Form.Item name="webhook_headers" label={t('ddns.webhookHeaders')}
                          extra='JSON数组格式，如：[{"key":"X-Token","value":"abc"}]'>
                          <Input.TextArea rows={3} placeholder='[{"key":"Authorization","value":"Bearer xxx"}]' />
                        </Form.Item>
                        <Form.Item name="webhook_body" label={t('ddns.webhookBody')}
                          extra="支持变量：{ip} {domain} {type}">
                          <Input.TextArea rows={4} placeholder='{"ip":"{ip}","domain":"{domain}","type":"{type}"}' />
                        </Form.Item>
                        <Divider orientation="left" plain style={{ fontSize: 13 }}>高级选项</Divider>
                        <Row gutter={16}>
                          <Col span={12}>
                            <Form.Item name="force_interval" label={t('ddns.forceInterval')}
                              extra="0=禁用，强制定期更新（即使IP未变化）">
                              <InputNumber min={0} style={{ width: '100%' }} addonAfter="秒" />
                            </Form.Item>
                          </Col>
                          <Col span={12}>
                            <Form.Item name="http_timeout" label={t('ddns.httpTimeout')}>
                              <InputNumber min={1} max={60} style={{ width: '100%' }} addonAfter="秒" />
                            </Form.Item>
                          </Col>
                        </Row>
                      </>
                    ) : (
                      <div style={{ color: '#999', fontSize: 13, padding: '8px 0' }}>
                        启用 Webhook 后，每次 IP 更新成功将向指定地址发送通知请求。
                      </div>
                    )}
                  </Form.Item>
                </>
              ),
            },
          ]} />
        </Form>
      </Modal>

      {/* 执行日志弹窗 */}
      <Modal
        title={
          <Space>
            <FileTextOutlined />
            <span>{t('ddns.viewLogs')}</span>
            {logsRecord && <Tag color="blue">{logsRecord.name}</Tag>}
          </Space>
        }
        open={logsOpen}
        onCancel={() => setLogsOpen(false)}
        footer={null}
        width={900}
        destroyOnHidden
      >
        <Table
          dataSource={logsData}
          columns={logsColumns}
          rowKey="id"
          loading={logsLoading}
          size="small"
          pagination={{
            current: logsPage,
            total: logsTotal,
            pageSize: 20,
            showSizeChanger: false,
            showTotal: (total) => `共 ${total} 条`,
            onChange: (p) => logsRecord && handleViewLogs(logsRecord, p),
          }}
          locale={{ emptyText: t('ddns.noLogs') }}
        />
      </Modal>

      {/* 历史记录弹窗 */}
      <Modal
        title={
          <Space>
            <HistoryOutlined />
            <span>IP 变化历史</span>
            {historyRecord && <Tag color="blue">{historyRecord.name}</Tag>}
          </Space>
        }
        open={historyOpen}
        onCancel={() => setHistoryOpen(false)}
        footer={null}
        width={800}
        destroyOnHidden
      >
        {historyRecord && (
          <Descriptions size="small" style={{ marginBottom: 12 }} column={3}>
            <Descriptions.Item label="当前IP">
              <Text code>{historyRecord.current_ip || '-'}</Text>
            </Descriptions.Item>
            <Descriptions.Item label="服务商">
              {PROVIDERS.find(p => p.value === historyRecord.provider)?.label || historyRecord.provider}
            </Descriptions.Item>
            <Descriptions.Item label="最后更新">
              {historyRecord.last_update_time ? dayjs(historyRecord.last_update_time).format('YYYY-MM-DD HH:mm:ss') : '-'}
            </Descriptions.Item>
          </Descriptions>
        )}
        <Table
          dataSource={historyData}
          columns={historyColumns}
          rowKey="id"
          loading={historyLoading}
          size="small"
          pagination={{ pageSize: 10, showSizeChanger: false }}
          locale={{ emptyText: '暂无历史记录' }}
        />
      </Modal>
    </div>
  )
}

export default Ddns
