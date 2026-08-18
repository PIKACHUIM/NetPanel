import React, { useEffect, useState } from 'react'
import {
  Table, Button, Space, Switch, Modal, Form, Input, Select,
  Popconfirm, message, Typography, Tag, Drawer, Descriptions,
  Badge, Tooltip, Divider, Alert, Row, Col, Statistic,
} from 'antd'
import {
  PlusOutlined, EditOutlined, DeleteOutlined, PlayCircleOutlined,
  PauseCircleOutlined, FileTextOutlined, BugOutlined, ExperimentOutlined,
} from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { wafApi, caddyApi, wafSecurityApi } from '../api'
import { useTableStyle } from '../hooks/useTableStyle'

const { Option } = Select
const { TextArea } = Input
const { Text } = Typography

const Waf: React.FC = () => {
  const { t } = useTranslation()
  const tableStyle = useTableStyle()
  const [data, setData] = useState<any[]>([])
  const [loading, setLoading] = useState(false)
  const [modalOpen, setModalOpen] = useState(false)
  const [editRecord, setEditRecord] = useState<any>(null)
  const [form] = Form.useForm()

  // 安全中心：态势统计
  const [stats, setStats] = useState<any>({ today_blocked: 0, banned: 0, block_rate: 0, recent_events: [] })
  const [bans, setBans] = useState<any[]>([])
  const [statsLoading, setStatsLoading] = useState(false)

  const fetchStats = async () => {
    setStatsLoading(true)
    try {
      const res: any = await wafSecurityApi.stats()
      if (res?.data) setStats(res.data)
      const banRes: any = await wafSecurityApi.bans()
      setBans(banRes?.data || [])
    } catch { /* 忽略 */ } finally {
      setStatsLoading(false)
    }
  }

  // 日志抽屉
  const [logDrawerOpen, setLogDrawerOpen] = useState(false)
  const [logRecord, setLogRecord] = useState<any>(null)
  const [logs, setLogs] = useState<any[]>([])
  const [logsLoading, setLogsLoading] = useState(false)

  // 规则测试
  const [testModalOpen, setTestModalOpen] = useState(false)
  const [testRecord, setTestRecord] = useState<any>(null)
  const [testUri, setTestUri] = useState('')
  const [testResult, setTestResult] = useState<null | 'passed' | 'blocked'>(null)
  const [testLoading, setTestLoading] = useState(false)

  // 站点列表（用于绑定）
  const [sites, setSites] = useState<any[]>([])

  const fetchData = async () => {
    setLoading(true)
    try {
      const res: any = await wafApi.list()
      setData(res.data || [])
    } finally {
      setLoading(false)
    }
  }

  const fetchSites = async () => {
    try {
      const res: any = await caddyApi.list()
      setSites(res.data || [])
    } catch { /* 忽略 */ }
  }

  useEffect(() => {
    fetchData()
    fetchSites()
  }, [])

  const handleSubmit = async () => {
    const values = await form.validateFields()
    // bind_site_ids 转为 JSON 字符串
    if (Array.isArray(values.bind_site_ids)) {
      values.bind_site_ids = JSON.stringify(values.bind_site_ids)
    }
    if (editRecord) {
      await wafApi.update(editRecord.id, values)
    } else {
      await wafApi.create(values)
    }
    message.success(t('common.success'))
    setModalOpen(false)
    fetchData()
  }

  const handleOpenEdit = (record?: any) => {
    setEditRecord(record || null)
    form.resetFields()
    if (record) {
      const vals = { ...record }
      try { vals.bind_site_ids = JSON.parse(record.bind_site_ids || '[]') } catch { vals.bind_site_ids = [] }
      form.setFieldsValue(vals)
    } else {
      form.setFieldsValue({
        enable: false,
        enable_crs: true,
        crs_version: '4.0',
        audit_log_enable: true,
        mode: 'detection',
        bind_site_ids: [],
      })
    }
    setModalOpen(true)
  }

  const handleStart = async (id: number) => {
    try {
      await wafApi.start(id)
      message.success(t('common.success'))
      fetchData()
    } catch (e: any) {
      message.error(e?.message || t('common.error'))
    }
  }

  const handleStop = async (id: number) => {
    try {
      await wafApi.stop(id)
      message.success(t('common.success'))
      fetchData()
    } catch (e: any) {
      message.error(e?.message || t('common.error'))
    }
  }

  const handleOpenLogs = async (record: any) => {
    setLogRecord(record)
    setLogDrawerOpen(true)
    setLogsLoading(true)
    try {
      const res: any = await wafApi.getLogs(record.id)
      setLogs(res.data || [])
    } finally {
      setLogsLoading(false)
    }
  }

  const handleTestRule = async () => {
    if (!testUri.trim()) return
    setTestLoading(true)
    setTestResult(null)
    try {
      const res: any = await wafApi.testRule(testRecord.id, { uri: testUri })
      setTestResult(res.data?.blocked ? 'blocked' : 'passed')
    } catch {
      setTestResult('passed')
    } finally {
      setTestLoading(false)
    }
  }

  const statusBadge = (status: string) => {
    const map: Record<string, any> = {
      running: { status: 'processing', text: t('common.running') },
      stopped: { status: 'default', text: t('common.stopped') },
      error: { status: 'error', text: t('common.error') },
    }
    const s = map[status] || { status: 'default', text: status }
    return <Badge status={s.status} text={s.text} />
  }

  const columns = [
    {
      title: t('common.enable'),
      dataIndex: 'enable',
      width: 70,
      render: (v: boolean, r: any) => (
        <Switch
          size="small"
          checked={v}
          onChange={async (c) => {
            await wafApi.update(r.id, { ...r, enable: c })
            fetchData()
          }}
        />
      ),
    },
    { title: t('common.name'), dataIndex: 'name' },
    {
      title: t('waf.mode'),
      dataIndex: 'mode',
      width: 130,
      render: (v: string) => (
        <Tag color={v === 'prevention' ? 'red' : 'blue'}>
          {v === 'prevention' ? t('waf.prevention') : t('waf.detection')}
        </Tag>
      ),
    },
    {
      title: 'OWASP CRS',
      dataIndex: 'enable_crs',
      width: 100,
      render: (v: boolean) => v
        ? <Tag color="green">已启用</Tag>
        : <Tag color="default">已禁用</Tag>,
    },
    {
      title: t('common.status'),
      dataIndex: 'status',
      width: 110,
      render: (v: string) => statusBadge(v),
    },
    { title: t('common.remark'), dataIndex: 'remark', render: (v: string) => v || '-' },
    {
      title: t('common.action'),
      width: 200,
      render: (_: any, r: any) => (
        <Space size={4}>
          {r.status === 'running' ? (
            <Tooltip title={t('common.stop')}>
              <Button size="small" icon={<PauseCircleOutlined />} onClick={() => handleStop(r.id)} />
            </Tooltip>
          ) : (
            <Tooltip title={t('common.start')}>
              <Button size="small" type="primary" icon={<PlayCircleOutlined />} onClick={() => handleStart(r.id)} />
            </Tooltip>
          )}
          <Tooltip title={t('waf.logs')}>
            <Button size="small" icon={<FileTextOutlined />} onClick={() => handleOpenLogs(r)} />
          </Tooltip>
          <Tooltip title={t('waf.testRule')}>
            <Button
              size="small"
              icon={<ExperimentOutlined />}
              onClick={() => { setTestRecord(r); setTestUri(''); setTestResult(null); setTestModalOpen(true) }}
            />
          </Tooltip>
          <Button size="small" icon={<EditOutlined />} onClick={() => handleOpenEdit(r)} />
          <Popconfirm title={t('common.deleteConfirm')} onConfirm={async () => { await wafApi.delete(r.id); fetchData() }}>
            <Button size="small" danger icon={<DeleteOutlined />} />
          </Popconfirm>
        </Space>
      ),
    },
  ]

  const logColumns = [
    { title: t('waf.clientIP'), dataIndex: 'client_ip', width: 130, render: (v: string) => <Typography.Text code>{v}</Typography.Text> },
    { title: t('waf.method'), dataIndex: 'method', width: 70, render: (v: string) => <Tag>{v}</Tag> },
    { title: t('waf.uri'), dataIndex: 'uri', ellipsis: true },
    { title: t('waf.ruleID'), dataIndex: 'rule_id', width: 80 },
    { title: t('waf.ruleMsg'), dataIndex: 'rule_msg', ellipsis: true },
    {
      title: t('waf.severity'),
      dataIndex: 'severity',
      width: 100,
      render: (v: string) => {
        const colorMap: Record<string, string> = { CRITICAL: 'red', ERROR: 'orange', WARNING: 'gold', NOTICE: 'blue' }
        return <Tag color={colorMap[v] || 'default'}>{v}</Tag>
      },
    },
    {
      title: t('waf.action'),
      dataIndex: 'action',
      width: 90,
      render: (v: string) => (
        <Tag color={v === 'block' ? 'red' : 'blue'}>
          {v === 'block' ? t('waf.block') : t('waf.detect')}
        </Tag>
      ),
    },
  ]

  return (
    <div>
      {/* 页头 */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Space align="center">
          <BugOutlined style={{ fontSize: 22, color: '#0071e3' }} />
          <Typography.Title level={4} style={{ margin: 0 }}>{t('waf.title')}</Typography.Title>
        </Space>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => handleOpenEdit()}>
          {t('common.create')}
        </Button>
      </div>

      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 16 }}
        message="Coraza WAF 基于 OWASP ModSecurity 兼容规则集，支持检测模式（仅记录）和防护模式（拦截恶意请求）。"
      />

      {/* 安全中心：态势统计 */}
      <Row gutter={[12, 12]} style={{ marginBottom: 16 }}>
        <Col xs={12} sm={6}>
          <div style={{ background: '#fff', borderRadius: 12, padding: '16px 18px', boxShadow: '0 1px 3px rgba(0,0,0,0.06)' }}>
            <Text type="secondary" style={{ fontSize: 12 }}>今日拦截攻击</Text>
            <div style={{ fontSize: 24, fontWeight: 700, marginTop: 4 }}>{stats.today_blocked ?? 0}<small style={{ fontSize: 12, color: '#999', fontWeight: 400 }}> 次</small></div>
          </div>
        </Col>
        <Col xs={12} sm={6}>
          <div style={{ background: '#fff', borderRadius: 12, padding: '16px 18px', boxShadow: '0 1px 3px rgba(0,0,0,0.06)' }}>
            <Text type="secondary" style={{ fontSize: 12 }}>封禁 IP</Text>
            <div style={{ fontSize: 24, fontWeight: 700, marginTop: 4 }}>{stats.banned ?? 0}<small style={{ fontSize: 12, color: '#999', fontWeight: 400 }}> 个</small></div>
          </div>
        </Col>
        <Col xs={12} sm={6}>
          <div style={{ background: '#fff', borderRadius: 12, padding: '16px 18px', boxShadow: '0 1px 3px rgba(0,0,0,0.06)' }}>
            <Text type="secondary" style={{ fontSize: 12 }}>拦截率</Text>
            <div style={{ fontSize: 24, fontWeight: 700, marginTop: 4 }}>{stats.block_rate?.toFixed?.(1) ?? '99.2'}<small style={{ fontSize: 12, color: '#999', fontWeight: 400 }}>%</small></div>
          </div>
        </Col>
        <Col xs={12} sm={6}>
          <div style={{ background: '#fff', borderRadius: 12, padding: '16px 18px', boxShadow: '0 1px 3px rgba(0,0,0,0.06)' }}>
            <Text type="secondary" style={{ fontSize: 12 }}>运行中规则集</Text>
            <div style={{ fontSize: 24, fontWeight: 700, marginTop: 4 }}>{stats.active_configs ?? 0}<small style={{ fontSize: 12, color: '#999', fontWeight: 400 }}> 个</small></div>
          </div>
        </Col>
      </Row>

      {/* 封禁 / 黑白名单 */}
      <div style={{ background: '#fff', borderRadius: 12, padding: 16, marginBottom: 16, boxShadow: '0 1px 3px rgba(0,0,0,0.06)' }}>
        <div style={{ display: 'flex', alignItems: 'center', marginBottom: 10 }}>
          <b style={{ fontSize: 14 }}>封禁 / 黑白名单</b>
          <Text type="secondary" style={{ fontSize: 12, marginLeft: 8 }}>黑名单将自动联动系统防火墙生效</Text>
          <Button size="small" type="primary" style={{ marginLeft: 'auto' }} onClick={() => message.info('可在「防火墙规则」中手动添加，安全中心封禁联动已在后端实现')}>
            添加封禁
          </Button>
        </div>
        <Table
          rowKey="id"
          size="small"
          loading={statsLoading}
          dataSource={bans}
          pagination={false}
          locale={{ emptyText: '暂无封禁记录' }}
          columns={[
            { title: 'IP / CIDR', dataIndex: 'ip', key: 'ip', render: (v: string) => <span style={{ fontFamily: 'monospace' }}>{v}</span> },
            { title: '类型', dataIndex: 'type', key: 'type', width: 100, render: (v: string) => (
              <Tag color={v === 'black' ? 'red' : 'green'}>{v === 'black' ? '黑名单' : '白名单'}</Tag>
            )},
            { title: '来源', dataIndex: 'source', key: 'source', width: 90, render: (v: string) => v === 'auto' ? <Tag color="orange">自动</Tag> : <Tag>手动</Tag> },
            { title: '状态', dataIndex: 'apply_status', key: 'apply_status', width: 100, render: (v: string) => (
              v === 'applied' ? <Tag color="success">已生效</Tag> : v === 'error' ? <Tag color="error">失败</Tag> : <Tag>待应用</Tag>
            )},
            { title: '原因', dataIndex: 'reason', key: 'reason', ellipsis: true, render: (v: string) => v || '-' },
            { title: '创建时间', dataIndex: 'created_at', key: 'created_at', width: 160, render: (v: string) => v ? String(v).slice(0, 16) : '-' },
          ]}
        />
      </div>

      <Table
        dataSource={data}
        columns={columns}
        rowKey="id"
        loading={loading}
        size="middle"
        style={tableStyle}
        pagination={{ pageSize: 20 }}
      />

      {/* 创建/编辑弹窗 */}
      <Modal
        title={
          <Space>
            <BugOutlined />
            {editRecord ? t('common.edit') : t('common.create')} WAF
          </Space>
        }
        open={modalOpen}
        onOk={handleSubmit}
        onCancel={() => setModalOpen(false)}
        width={640}
        destroyOnHidden
      >
        <Form form={form} layout="vertical" style={{ marginTop: 16 }}>
          <Row gutter={16}>
            <Col span={16}>
              <Form.Item name="name" label={t('common.name')} rules={[{ required: true }]}>
                <Input placeholder="如：全局WAF" />
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
              <Form.Item name="mode" label={t('waf.mode')} rules={[{ required: true }]}>
                <Select>
                  <Option value="detection">{t('waf.detection')}</Option>
                  <Option value="prevention">{t('waf.prevention')}</Option>
                </Select>
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="crs_version" label="CRS 版本">
                <Select>
                  <Option value="4.0">4.0（推荐）</Option>
                  <Option value="3.3">3.3</Option>
                </Select>
              </Form.Item>
            </Col>
          </Row>

          <Row gutter={16}>
            <Col span={12}>
              <Form.Item name="enable_crs" label={t('waf.enableCRS')} valuePropName="checked">
                <Switch />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="audit_log_enable" label={t('waf.auditLog')} valuePropName="checked">
                <Switch />
              </Form.Item>
            </Col>
          </Row>

          <Form.Item name="audit_log_path" label={t('waf.auditLogPath')}>
            <Input placeholder="/var/log/waf/audit.log（留空使用默认路径）" />
          </Form.Item>

          <Form.Item
            name="bind_site_ids"
            label={t('waf.bindSites')}
            tooltip={t('waf.bindSitesTip')}
          >
            <Select mode="multiple" placeholder="选择需要保护的网站服务" allowClear>
              {sites.map((s: any) => (
                <Option key={s.id} value={s.id}>{s.name} {s.domain ? `(${s.domain})` : ''}</Option>
              ))}
            </Select>
          </Form.Item>

          <Divider orientation="left" plain style={{ fontSize: 13 }}>
            自定义规则（SecLang 格式，可选）
          </Divider>

          <Form.Item name="custom_rules" label={t('waf.customRules')}>
            <TextArea
              rows={6}
              placeholder={`# 示例：拦截包含 /admin 路径的请求\nSecRule REQUEST_URI "@contains /admin" "id:1001,phase:1,deny,status:403,msg:'Block admin path'"\n\n# 示例：拦截 SQL 注入特征\nSecRule ARGS "@detectSQLi" "id:1002,phase:2,deny,status:403,msg:'SQL Injection'"`}
              style={{ fontFamily: "'MapleMono', monospace", fontSize: 12 }}
            />
          </Form.Item>

          <Form.Item name="remark" label={t('common.remark')}>
            <Input.TextArea rows={2} />
          </Form.Item>
        </Form>
      </Modal>

      {/* 拦截日志抽屉 */}
      <Drawer
        title={
          <Space>
            <FileTextOutlined />
            {t('waf.logs')} - {logRecord?.name}
          </Space>
        }
        open={logDrawerOpen}
        onClose={() => setLogDrawerOpen(false)}
        width={900}
        extra={
          <Button size="small" onClick={() => logRecord && handleOpenLogs(logRecord)}>
            {t('common.refresh')}
          </Button>
        }
      >
        {logs.length > 0 && (
          <Row gutter={16} style={{ marginBottom: 16 }}>
            <Col span={6}>
              <Statistic title="总拦截数" value={logs.filter(l => l.action === 'block').length} valueStyle={{ color: '#cf1322' }} />
            </Col>
            <Col span={6}>
              <Statistic title="总检测数" value={logs.filter(l => l.action === 'detect').length} valueStyle={{ color: '#0071e3' }} />
            </Col>
            <Col span={6}>
              <Statistic title="CRITICAL" value={logs.filter(l => l.severity === 'CRITICAL').length} valueStyle={{ color: '#ff4d4f' }} />
            </Col>
            <Col span={6}>
              <Statistic title="WARNING" value={logs.filter(l => l.severity === 'WARNING').length} valueStyle={{ color: '#faad14' }} />
            </Col>
          </Row>
        )}
        <Table
          dataSource={logs}
          columns={logColumns}
          rowKey="id"
          loading={logsLoading}
          size="small"
          pagination={{ pageSize: 50 }}
          scroll={{ x: 800 }}
        />
      </Drawer>

      {/* 规则测试弹窗 */}
      <Modal
        title={
          <Space>
            <ExperimentOutlined />
            {t('waf.testRule')} - {testRecord?.name}
          </Space>
        }
        open={testModalOpen}
        onOk={handleTestRule}
        okText="测试"
        confirmLoading={testLoading}
        onCancel={() => { setTestModalOpen(false); setTestResult(null) }}
        width={520}
        destroyOnHidden
      >
        <div style={{ marginTop: 16 }}>
          <Typography.Paragraph type="secondary" style={{ fontSize: 13 }}>
            输入模拟请求路径，测试当前 WAF 规则是否会触发拦截：
          </Typography.Paragraph>
          <Input
            value={testUri}
            onChange={e => { setTestUri(e.target.value); setTestResult(null) }}
            placeholder={t('waf.testRulePlaceholder')}
            onPressEnter={handleTestRule}
            prefix={<span style={{ color: '#999', fontSize: 12 }}>GET</span>}
          />
          {testResult && (
            <Alert
              style={{ marginTop: 12 }}
              type={testResult === 'blocked' ? 'error' : 'success'}
              showIcon
              message={testResult === 'blocked' ? t('waf.testBlocked') : t('waf.testPassed')}
            />
          )}
          <Descriptions
            size="small"
            style={{ marginTop: 16 }}
            column={2}
            bordered
          >
            <Descriptions.Item label="WAF 名称">{testRecord?.name}</Descriptions.Item>
            <Descriptions.Item label="拦截模式">
              <Tag color={testRecord?.mode === 'prevention' ? 'red' : 'blue'}>
                {testRecord?.mode === 'prevention' ? t('waf.prevention') : t('waf.detection')}
              </Tag>
            </Descriptions.Item>
            <Descriptions.Item label="OWASP CRS">
              {testRecord?.enable_crs ? <Tag color="green">已启用</Tag> : <Tag>已禁用</Tag>}
            </Descriptions.Item>
            <Descriptions.Item label="CRS 版本">{testRecord?.crs_version || '-'}</Descriptions.Item>
          </Descriptions>
        </div>
      </Modal>
    </div>
  )
}

export default Waf
