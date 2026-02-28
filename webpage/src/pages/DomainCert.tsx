import React, { useEffect, useState } from 'react'
import {
  Table, Button, Space, Modal, Form, Input, Select, Switch,
  Popconfirm, message, Typography, Tag, Tooltip, Progress, Row, Col, InputNumber, Radio, Checkbox,
} from 'antd'
import {
  PlusOutlined, EditOutlined, DeleteOutlined, SyncOutlined,
  SafetyCertificateOutlined, DownloadOutlined, MinusCircleOutlined,
} from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { domainCertApi, domainAccountApi, certAccountApi } from '../api'
import dayjs from 'dayjs'

const { Option } = Select
const { Text } = Typography

// 域名条目结构
interface DomainEntry {
  base: string       // 基础域名，如 example.com
  wildcard: boolean  // 是否包含通配符 *.example.com
  includeRoot: boolean // 是否包含根域名 example.com（仅通配符时可修改）
}

// 将 DomainEntry[] 序列化为域名字符串数组
const entriesToDomains = (entries: DomainEntry[]): string[] => {
  const result: string[] = []
  for (const e of entries) {
    if (!e.base.trim()) continue
    if (e.wildcard) {
      if (e.includeRoot) result.push(e.base.trim())
      result.push(`*.${e.base.trim()}`)
    } else {
      result.push(e.base.trim())
    }
  }
  return result
}

// 将域名字符串数组反序列化为 DomainEntry[]
const domainsToEntries = (domains: string[]): DomainEntry[] => {
  const map = new Map<string, DomainEntry>()
  for (const d of domains) {
    if (d.startsWith('*.')) {
      const base = d.slice(2)
      const existing = map.get(base)
      if (existing) {
        existing.wildcard = true
      } else {
        map.set(base, { base, wildcard: true, includeRoot: false })
      }
    } else {
      const existing = map.get(d)
      if (existing && existing.wildcard) {
        existing.includeRoot = true
      } else if (!existing) {
        map.set(d, { base: d, wildcard: false, includeRoot: true })
      }
    }
  }
  return map.size > 0 ? Array.from(map.values()) : [{ base: '', wildcard: false, includeRoot: true }]
}

// 域名列表编辑器组件
const DomainListEditor: React.FC<{
  value?: DomainEntry[]
  onChange?: (v: DomainEntry[]) => void
}> = ({ value, onChange }) => {
  const { t } = useTranslation()
  const entries: DomainEntry[] = value && value.length > 0 ? value : [{ base: '', wildcard: false, includeRoot: true }]

  const update = (idx: number, patch: Partial<DomainEntry>) => {
    const next = entries.map((e, i) => i === idx ? { ...e, ...patch } : e)
    // 关闭通配符时，includeRoot 强制为 true 且不可改
    if (patch.wildcard === false) next[idx].includeRoot = true
    onChange?.(next)
  }

  const add = () => onChange?.([...entries, { base: '', wildcard: false, includeRoot: true }])

  const remove = (idx: number) => {
    const next = entries.filter((_, i) => i !== idx)
    onChange?.(next.length > 0 ? next : [{ base: '', wildcard: false, includeRoot: true }])
  }

  return (
    <div>
      {entries.map((entry, idx) => (
        <div key={idx} style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 8 }}>
          <Input
            value={entry.base}
            onChange={e => update(idx, { base: e.target.value })}
            placeholder="example.com"
            style={{ flex: 1 }}
          />
          <Checkbox
            checked={entry.wildcard}
            onChange={e => update(idx, { wildcard: e.target.checked })}
          >
            {t('domainCert.wildcard')}
          </Checkbox>
          <Checkbox
            checked={entry.includeRoot}
            disabled={!entry.wildcard}
            onChange={e => update(idx, { includeRoot: e.target.checked })}
          >
            {t('domainCert.includeRoot')}
          </Checkbox>
          <Tooltip title={t('common.delete')}>
            <MinusCircleOutlined
              style={{ color: entries.length === 1 ? '#d9d9d9' : '#ff4d4f', fontSize: 16, cursor: entries.length === 1 ? 'not-allowed' : 'pointer' }}
              onClick={() => entries.length > 1 && remove(idx)}
            />
          </Tooltip>
        </div>
      ))}
      <div style={{ marginTop: 4 }}>
        <Text type="secondary" style={{ fontSize: 11 }}>
          {t('domainCert.domainHint')}
        </Text>
      </div>
      <Button type="dashed" size="small" icon={<PlusOutlined />} onClick={add} style={{ marginTop: 8 }}>
        {t('domainCert.addDomain')}
      </Button>
    </div>
  )
}

const DomainCert: React.FC = () => {
  const { t } = useTranslation()
  const [data, setData] = useState<any[]>([])
  const [accounts, setAccounts] = useState<any[]>([])
  const [certAccounts, setCertAccounts] = useState<any[]>([])
  const [loading, setLoading] = useState(false)
  const [modalOpen, setModalOpen] = useState(false)
  const [editRecord, setEditRecord] = useState<any>(null)
  const [applyingIds, setApplyingIds] = useState<Set<number>>(new Set())
  const [form] = Form.useForm()

  const fetchData = async () => {
    setLoading(true)
    try {
      const [certRes, accRes, caRes]: any[] = await Promise.all([
        domainCertApi.list(),
        domainAccountApi.list(),
        certAccountApi.list(),
      ])
      setData(certRes.data || [])
      setAccounts(accRes.data || [])
      setCertAccounts(caRes.data || [])
    } finally { setLoading(false) }
  }
  useEffect(() => { fetchData() }, [])

  const handleOpen = (record?: any) => {
    if (record) {
      setEditRecord(record)
      let domainEntries: DomainEntry[]
      try {
        const arr: string[] = JSON.parse(record.domains || '[]')
        domainEntries = domainsToEntries(arr)
      } catch {
        domainEntries = [{ base: '', wildcard: false, includeRoot: true }]
      }
      form.setFieldsValue({ ...record, domains: domainEntries })
    } else {
      setEditRecord(null)
      form.resetFields()
      form.setFieldsValue({
        ca: 'letsencrypt',
        challenge_type: 'dns',
        auto_renew: true,
        cert_account_id: undefined,
        cert_type: 'acme',
        renew_before_days: 7,
        domains: [{ base: '', wildcard: false, includeRoot: true }],
      })
    }
    setModalOpen(true)
  }

  const handleSubmit = async () => {
    const values = await form.validateFields()
    // 将 DomainEntry[] 序列化为 JSON 字符串
    if (Array.isArray(values.domains) && values.domains[0] && typeof values.domains[0] === 'object' && 'base' in values.domains[0]) {
      values.domains = JSON.stringify(entriesToDomains(values.domains as DomainEntry[]))
    } else if (typeof values.domains === 'string') {
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
      render: (v: string, r: any) => {
        const labels: Record<string, string> = { letsencrypt: "Let's Encrypt", zerossl: 'ZeroSSL', buypass: 'Buypass', google: 'Google Trust' }
        const caName = labels[v] || v || "Let's Encrypt"
        const certAcc = certAccounts.find(a => a.id === r.cert_account_id)
        return (
          <div>
            <Tag color="blue">{caName}</Tag>
            {certAcc && <div><Text type="secondary" style={{ fontSize: 11 }}>账号: {certAcc.name}</Text></div>}
          </div>
        )
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
        width={580} destroyOnHidden
      >
        <Form form={form} layout="vertical" style={{ marginTop: 16 }}>
          <Form.Item name="name" label={t('common.name')} rules={[{ required: true }]}>
            <Input placeholder="证书名称" />
          </Form.Item>

          {/* 证书类型切换 */}
          <Form.Item name="cert_type" label={t('domainCert.certType')}>
            <Radio.Group>
              <Radio value="acme">{t('domainCert.certTypeAcme')}</Radio>
              <Radio value="manual">{t('domainCert.certTypeManual')}</Radio>
            </Radio.Group>
          </Form.Item>

          <Form.Item
            noStyle
            shouldUpdate={(prev, cur) => prev.cert_type !== cur.cert_type}
          >
            {({ getFieldValue }) => getFieldValue('cert_type') === 'manual' ? (
              <>
                <Form.Item
                  name="domains"
                  label={t('domainCert.domains')}
                  rules={[{
                    validator: (_, val: DomainEntry[]) => {
                      const domains = entriesToDomains(val || [])
                      return domains.length > 0 ? Promise.resolve() : Promise.reject(t('domainCert.domainsRequired'))
                    }
                  }]}
                >
                  <DomainListEditor />
                </Form.Item>
                <Form.Item name="cert_content" label={t('domainCert.certContent')} rules={[{ required: true }]}
                  extra="PEM 格式证书内容（包含完整证书链）">
                  <Input.TextArea rows={5} placeholder="-----BEGIN CERTIFICATE-----&#10;...&#10;-----END CERTIFICATE-----" />
                </Form.Item>
                <Form.Item name="key_content" label={t('domainCert.keyContent')} rules={[{ required: true }]}
                  extra="PEM 格式私钥内容">
                  <Input.TextArea rows={5} placeholder="-----BEGIN PRIVATE KEY-----&#10;...&#10;-----END PRIVATE KEY-----" />
                </Form.Item>
              </>
            ) : (
              <>
                {/* CA机构放最前面 */}
                <Row gutter={16}>
                  <Col span={12}>
                    <Form.Item name="ca" label={t('domainCert.ca')}>
                      <Select>
                        <Option value="letsencrypt">Let's Encrypt</Option>
                        <Option value="zerossl">ZeroSSL</Option>
                        <Option value="buypass">Buypass</Option>
                        <Option value="google">Google Trust Services</Option>
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
                  name="domains"
                  label={t('domainCert.domains')}
                  rules={[{
                    validator: (_, val: DomainEntry[]) => {
                      const domains = entriesToDomains(val || [])
                      return domains.length > 0 ? Promise.resolve() : Promise.reject(t('domainCert.domainsRequired'))
                    }
                  }]}
                >
                  <DomainListEditor />
                </Form.Item>

                {/* 证书账号（ACME CA 账号） */}
                <Form.Item
                  name="cert_account_id"
                  label={t('domainCert.certAccount')}
                  extra="选择预先注册的 ACME CA 账号（可选，不选则使用默认账号）"
                >
                  <Select placeholder="选择证书账号（可选）" allowClear>
                    {certAccounts.map(a => (
                      <Option key={a.id} value={a.id}>
                        <Space size={4}>
                          <Tag color={({ letsencrypt: 'green', zerossl: 'blue', buypass: 'purple', google: 'red' } as Record<string, string>)[a.type] || 'default'}
                            style={{ margin: 0 }}>{a.type}</Tag>
                          {a.name}
                          {a.email && <Text type="secondary" style={{ fontSize: 11 }}>({a.email})</Text>}
                        </Space>
                      </Option>
                    ))}
                  </Select>
                </Form.Item>

                <Form.Item
                  noStyle
                  shouldUpdate={(prev, cur) => prev.challenge_type !== cur.challenge_type}
                >
                  {({ getFieldValue: getInner }) => getInner('challenge_type') === 'dns' && (
                    <Form.Item name="domain_account_id" label="DNS 账号"
                      extra="选择用于 DNS-01 验证的域名账号">
                      <Select placeholder="选择域名账号" allowClear>
                        {accounts.map(a => <Option key={a.id} value={a.id}>{a.name}</Option>)}
                      </Select>
                    </Form.Item>
                  )}
                </Form.Item>

                <Row gutter={16}>
                  <Col span={12}>
                    <Form.Item name="auto_renew" label={t('domainCert.autoRenew')} valuePropName="checked">
                      <Switch checkedChildren="自动续期" unCheckedChildren="手动" />
                    </Form.Item>
                  </Col>
                  <Col span={12}>
                    <Form.Item
                      name="renew_before_days"
                      label={t('domainCert.renewBeforeDays')}
                      extra={<span style={{ fontSize: 11 }}>到期前多少天自动续期</span>}
                    >
                      <InputNumber min={1} max={60} style={{ width: '100%' }} addonAfter="天" />
                    </Form.Item>
                  </Col>
                </Row>
              </>
            )}
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