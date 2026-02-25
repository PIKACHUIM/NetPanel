import React, { useEffect, useState } from 'react'
import {
  Table, Button, Space, Modal, Form, Input, Select, Switch,
  Popconfirm, message, Typography, Tag, Tooltip, Progress, Row, Col,
} from 'antd'
import {
  PlusOutlined, EditOutlined, DeleteOutlined, SyncOutlined,
  SafetyCertificateOutlined, DownloadOutlined,
} from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { domainCertApi, domainAccountApi } from '../api'
import dayjs from 'dayjs'

const { Option } = Select
const { Text } = Typography

const DomainCert: React.FC = () => {
  const { t } = useTranslation()
  const [data, setData] = useState<any[]>([])
  const [accounts, setAccounts] = useState<any[]>([])
  const [loading, setLoading] = useState(false)
  const [modalOpen, setModalOpen] = useState(false)
  const [editRecord, setEditRecord] = useState<any>(null)
  const [applyingIds, setApplyingIds] = useState<Set<number>>(new Set())
  const [form] = Form.useForm()

  const fetchData = async () => {
    setLoading(true)
    try {
      const [certRes, accRes]: any[] = await Promise.all([domainCertApi.list(), domainAccountApi.list()])
      setData(certRes.data || [])
      setAccounts(accRes.data || [])
    } finally { setLoading(false) }
  }
  useEffect(() => { fetchData() }, [])

  const handleOpen = (record?: any) => {
    if (record) {
      setEditRecord(record)
      form.setFieldsValue({
        ...record,
        domains: (() => { try { return JSON.parse(record.domains || '[]').join('\n') } catch { return record.domains } })(),
      })
    } else {
      setEditRecord(null)
      form.resetFields()
      form.setFieldsValue({ ca: 'letsencrypt', challenge_type: 'dns', auto_renew: true })
    }
    setModalOpen(true)
  }

  const handleSubmit = async () => {
    const values = await form.validateFields()
    if (typeof values.domains === 'string') {
      values.domains = JSON.stringify(values.domains.split('\n').filter(Boolean))
    }
    if (editRecord) {
      await domainCertApi.update(editRecord.id, values)
    } else {
      await domainCertApi.create(values)
    }
    message.success(t('common.success'))
    setModalOpen(false)
    fetchData()
  }

  const handleApply = async (id: number) => {
    setApplyingIds(prev => new Set(prev).add(id))
    try {
      await domainCertApi.apply(id)
      message.success('已触发证书申请，请稍后刷新查看结果')
      setTimeout(fetchData, 3000)
    } finally {
      setApplyingIds(prev => { const s = new Set(prev); s.delete(id); return s })
    }
  }

  const getExpireInfo = (expireAt: string) => {
    if (!expireAt) return { tag: <Tag>未申请</Tag>, percent: 0 }
    const days = dayjs(expireAt).diff(dayjs(), 'day')
    if (days < 0) return { tag: <Tag color="error">已过期</Tag>, percent: 0 }
    if (days < 7) return { tag: <Tag color="error">{days}天后到期</Tag>, percent: Math.min(days / 90 * 100, 100) }
    if (days < 30) return { tag: <Tag color="warning">{days}天后到期</Tag>, percent: Math.min(days / 90 * 100, 100) }
    return { tag: <Tag color="success">{days}天后到期</Tag>, percent: Math.min(days / 90 * 100, 100) }
  }

  const columns = [
    {
      title: t('common.name'), dataIndex: 'name',
      render: (name: string, r: any) => (
        <div>
          <Space>
            <SafetyCertificateOutlined style={{ color: '#1677ff' }} />
            <Text strong>{name}</Text>
          </Space>
          {r.remark && <div><Text type="secondary" style={{ fontSize: 12 }}>{r.remark}</Text></div>}
        </div>
      ),
    },
    {
      title: t('domainCert.domains'), dataIndex: 'domains',
      render: (v: string) => {
        try {
          const arr = JSON.parse(v || '[]')
          return arr.map((d: string) => <Tag key={d}>{d}</Tag>)
        } catch { return v }
      },
    },
    {
      title: t('domainCert.ca'), dataIndex: 'ca',
      render: (v: string) => {
        const labels: Record<string, string> = { letsencrypt: "Let's Encrypt", zerossl: 'ZeroSSL', buypass: 'Buypass' }
        return <Tag color="blue">{labels[v] || v || "Let's Encrypt"}</Tag>
      },
    },
    {
      title: t('domainCert.expireAt'), dataIndex: 'expire_at', width: 200,
      render: (v: string) => {
        const { tag, percent } = getExpireInfo(v)
        return (
          <div>
            {tag}
            {v && <Progress percent={percent} size="small" showInfo={false} style={{ marginTop: 4, width: 100 }} />}
          </div>
        )
      },
    },
    {
      title: t('domainCert.autoRenew'), dataIndex: 'auto_renew', width: 80,
      render: (v: boolean) => v ? <Tag color="blue">自动</Tag> : <Tag>手动</Tag>,
    },
    {
      title: t('common.action'), width: 180,
      render: (_: any, r: any) => (
        <Space size={4}>
          <Tooltip title={t('domainCert.renew')}>
            <Button
              size="small" icon={<SyncOutlined />}
              loading={applyingIds.has(r.id)}
              onClick={() => handleApply(r.id)}
            />
          </Tooltip>
          {r.cert_path && (
            <Tooltip title="下载证书">
              <Button size="small" icon={<DownloadOutlined />}
                onClick={() => window.open(`/api/v1/domain-certs/${r.id}/download`, '_blank')} />
            </Tooltip>
          )}
          <Tooltip title={t('common.edit')}>
            <Button size="small" icon={<EditOutlined />} onClick={() => handleOpen(r)} />
          </Tooltip>
          <Popconfirm title={t('common.deleteConfirm')} onConfirm={async () => { await domainCertApi.delete(r.id); fetchData() }}>
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
        <Typography.Title level={4} style={{ margin: 0 }}>{t('domainCert.title')}</Typography.Title>
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
        width={540} destroyOnClose
      >
        <Form form={form} layout="vertical" style={{ marginTop: 16 }}>
          <Form.Item name="name" label={t('common.name')} rules={[{ required: true }]}>
            <Input placeholder="证书名称" />
          </Form.Item>

          <Form.Item name="domains" label={t('domainCert.domains')} rules={[{ required: true }]}
            extra="每行一个域名，支持通配符如：*.example.com">
            <Input.TextArea rows={3} placeholder={'example.com\n*.example.com'} />
          </Form.Item>

          <Form.Item name="email" label={t('domainCert.email')}
            extra="用于 ACME 账号注册，接收证书到期提醒">
            <Input placeholder="your@email.com" />
          </Form.Item>

          <Row gutter={16}>
            <Col span={12}>
              <Form.Item name="ca" label={t('domainCert.ca')}>
                <Select>
                  <Option value="letsencrypt">Let's Encrypt</Option>
                  <Option value="zerossl">ZeroSSL</Option>
                  <Option value="buypass">Buypass</Option>
                </Select>
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="challenge_type" label={t('domainCert.challengeType')}>
                <Select>
                  <Option value="dns">DNS-01（推荐，支持通配符）</Option>
                  <Option value="http">HTTP-01</Option>
                </Select>
              </Form.Item>
            </Col>
          </Row>

          <Form.Item
            noStyle
            shouldUpdate={(prev, cur) => prev.challenge_type !== cur.challenge_type}
          >
            {({ getFieldValue }) => getFieldValue('challenge_type') === 'dns' && (
              <Form.Item name="account_id" label="DNS 账号"
                extra="选择用于 DNS-01 验证的域名账号">
                <Select placeholder="选择域名账号" allowClear>
                  {accounts.map(a => <Option key={a.id} value={a.id}>{a.name}</Option>)}
                </Select>
              </Form.Item>
            )}
          </Form.Item>

          <Form.Item name="auto_renew" label={t('domainCert.autoRenew')} valuePropName="checked">
            <Switch checkedChildren="自动续期" unCheckedChildren="手动" />
          </Form.Item>

          <Form.Item name="remark" label={t('common.remark')}>
            <Input.TextArea rows={2} />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

export default DomainCert
