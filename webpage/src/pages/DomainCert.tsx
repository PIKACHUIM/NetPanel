import React, { useEffect, useState, useCallback, useRef } from 'react'
import {
  Table, Button, Space, Modal, Form, Input, Select, Switch,
  Popconfirm, message, Typography, Tag, Tooltip, Progress, Row, Col,
  InputNumber, Radio, Checkbox, Alert, Steps, Descriptions, Card,
} from 'antd'
import {
  PlusOutlined, EditOutlined, DeleteOutlined, SyncOutlined,
  SafetyCertificateOutlined, DownloadOutlined, MinusCircleOutlined,
  ExclamationCircleOutlined, CheckCircleOutlined, ClockCircleOutlined,
  LoadingOutlined, CloseCircleOutlined, CloudServerOutlined,
  GlobalOutlined, AuditOutlined, FileProtectOutlined,
} from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { domainCertApi, domainAccountApi, certAccountApi, domainInfoApi } from '../api'
import dayjs from 'dayjs'
import { useTableStyle } from '../hooks/useTableStyle'

const { Option } = Select
const { Text } = Typography

// 域名条目结构
interface DomainEntry {
  base: string
  wildcard: boolean
  includeRoot: boolean
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

// 从 PEM 证书内容中解析 SAN 域名（纯前端正则解析，仅用于辅助填充）
// 注意：浏览器无法直接解析 ASN.1，这里通过后端接口解析
const parseCertDomains = async (certPem: string): Promise<string[]> => {
  try {
    const res: any = await domainCertApi.parseCert({ cert_content: certPem })
    return res?.data?.domains || []
  } catch {
    return []
  }
}

// 域名列表编辑器组件
const DomainListEditor: React.FC<{
  value?: DomainEntry[]
  onChange?: (v: DomainEntry[]) => void
  readonly?: boolean
}> = ({ value, onChange, readonly }) => {
  const { t } = useTranslation()
  const entries: DomainEntry[] = value && value.length > 0 ? value : [{ base: '', wildcard: false, includeRoot: true }]

  const update = (idx: number, patch: Partial<DomainEntry>) => {
    if (readonly) return
    const next = entries.map((e, i) => i === idx ? { ...e, ...patch } : e)
    if (patch.wildcard === false) next[idx].includeRoot = true
    onChange?.(next)
  }

  const add = () => { if (!readonly) onChange?.([...entries, { base: '', wildcard: false, includeRoot: true }]) }

  const remove = (idx: number) => {
    if (readonly) return
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
            readOnly={readonly}
          />
          <Checkbox
            checked={entry.wildcard}
            onChange={e => update(idx, { wildcard: e.target.checked })}
            disabled={readonly}
          >
            {t('domainCert.wildcard')}
          </Checkbox>
          <Checkbox
            checked={entry.includeRoot}
            disabled={readonly || !entry.wildcard}
            onChange={e => update(idx, { includeRoot: e.target.checked })}
          >
            {t('domainCert.includeRoot')}
          </Checkbox>
          {!readonly && (
            <Tooltip title={t('common.delete')}>
              <MinusCircleOutlined
                style={{ color: entries.length === 1 ? '#d9d9d9' : '#ff4d4f', fontSize: 16, cursor: entries.length === 1 ? 'not-allowed' : 'pointer' }}
                onClick={() => entries.length > 1 && remove(idx)}
              />
            </Tooltip>
          )}
        </div>
      ))}
      {!readonly && (
        <>
          <div style={{ marginTop: 4 }}>
            <Text type="secondary" style={{ fontSize: 11 }}>{t('domainCert.domainHint')}</Text>
          </div>
          <Button type="dashed" size="small" icon={<PlusOutlined />} onClick={add} style={{ marginTop: 8 }}>
            {t('domainCert.addDomain')}
          </Button>
        </>
      )}
    </div>
  )
}

const DomainCert: React.FC = () => {
  const { t } = useTranslation()
  const tableStyle = useTableStyle()
  const [data, setData] = useState<any[]>([])
  const [accounts, setAccounts] = useState<any[]>([])       // DNS 域名账号
  const [certAccounts, setCertAccounts] = useState<any[]>([]) // ACME 证书账号
  const [domainInfoList, setDomainInfoList] = useState<any[]>([]) // DNS 域名解析列表
  const [loading, setLoading] = useState(false)
  const [modalOpen, setModalOpen] = useState(false)
  const [editRecord, setEditRecord] = useState<any>(null)
  const [applyingIds, setApplyingIds] = useState<Set<number>>(new Set())
  const [parsingCert, setParsigCert] = useState(false)
  // ACME 流程状态弹窗
  const [statusModalOpen, setStatusModalOpen] = useState(false)
  const [statusCert, setStatusCert] = useState<any>(null)
  const [certStatus, setCertStatus] = useState<any>(null)
  const [statusLoading, setStatusLoading] = useState(false)
  const statusTimerRef = useRef<ReturnType<typeof setInterval> | null>(null)
  // DNS 账号校验警告
  const [dnsWarnings, setDnsWarnings] = useState<{ domain: string; missing: boolean }[]>([])
  // DNS 模式（auto/manual）
  const [dnsMode, setDnsMode] = useState<'auto' | 'manual'>('auto')
  const [form] = Form.useForm()

  // 自动刷新定时器
  const autoRefreshRef = useRef<ReturnType<typeof setInterval> | null>(null)

  const fetchData = async () => {
    setLoading(true)
    try {
      const [certRes, accRes, caRes, diRes]: any[] = await Promise.all([
        domainCertApi.list(),
        domainAccountApi.list(),
        certAccountApi.list(),
        domainInfoApi.list(),
      ])
      setData(certRes.data || [])
      setAccounts(accRes.data || [])
      setCertAccounts(caRes.data || [])
      setDomainInfoList(diRes.data || [])
    } finally { setLoading(false) }
  }
  useEffect(() => { fetchData() }, [])

  // 如果有正在进行中的证书，自动刷新列表
  useEffect(() => {
    const inProgress = data.some(d =>
      ['applying', 'order_created', 'dns_set', 'validating'].includes(d.status)
    )
    if (inProgress && !autoRefreshRef.current) {
      autoRefreshRef.current = setInterval(() => {
        domainCertApi.list().then((res: any) => {
          setData(res.data || [])
        }).catch(() => {})
      }, 10000) // 每 10 秒刷新
    } else if (!inProgress && autoRefreshRef.current) {
      clearInterval(autoRefreshRef.current)
      autoRefreshRef.current = null
    }
    return () => {
      if (autoRefreshRef.current) {
        clearInterval(autoRefreshRef.current)
        autoRefreshRef.current = null
      }
    }
  }, [data])

  // 校验当前表单中的域名是否已在 DNS 解析中添加
  const checkDnsWarnings = useCallback((entries: DomainEntry[], dnsAccountId?: number) => {
    if (!dnsAccountId) {
      setDnsWarnings([])
      setDnsMode('manual')
      return
    }
    const accountDomains = domainInfoList
      .filter(d => d.account_id === dnsAccountId)
      .map(d => (d.name as string).toLowerCase())

    const warnings: { domain: string; missing: boolean }[] = []
    for (const e of entries) {
      if (!e.base.trim()) continue
      const base = e.base.trim().toLowerCase()
      const missing = !accountDomains.includes(base)
      warnings.push({ domain: base, missing })
    }
    setDnsWarnings(warnings)

    // 如果有域名不在 DNS 解析中，自动切换为手动模式
    const hasMissing = warnings.some(w => w.missing)
    if (hasMissing) {
      setDnsMode('manual')
      form.setFieldsValue({ dns_mode: 'manual' })
    } else {
      setDnsMode('auto')
      form.setFieldsValue({ dns_mode: 'auto' })
    }
  }, [domainInfoList, form])

  const handleOpen = (record?: any) => {
    setDnsWarnings([])
    setDnsMode('auto')
    if (record) {
      setEditRecord(record)
      let domainEntries: DomainEntry[]
      try {
        const arr: string[] = JSON.parse(record.domains || '[]')
        domainEntries = domainsToEntries(arr)
      } catch {
        domainEntries = [{ base: '', wildcard: false, includeRoot: true }]
      }
      setDnsMode(record.dns_mode || 'auto')
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
        dns_mode: 'auto',
        domains: [{ base: '', wildcard: false, includeRoot: true }],
      })
    }
    setModalOpen(true)
  }

  const handleSubmit = async () => {
    const values = await form.validateFields()
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
      message.success(t('domainCert.applySubmitted'))
      setTimeout(fetchData, 2000)
    } finally {
      setApplyingIds(prev => { const s = new Set(prev); s.delete(id); return s })
    }
  }

  // 手动上传：粘贴证书内容后自动解析域名
  const handleCertContentChange = async (certPem: string) => {
    if (!certPem || certPem.length < 100) return
    setParsigCert(true)
    try {
      const domains = await parseCertDomains(certPem)
      if (domains.length > 0) {
        form.setFieldsValue({ domains: domainsToEntries(domains) })
        message.success(t('domainCert.certContentParsed', { count: domains.length }))
      }
    } finally {
      setParsigCert(false)
    }
  }

  // ===== ACME 流程状态弹窗 =====
  const openStatusModal = async (record: any) => {
    setStatusCert(record)
    setStatusModalOpen(true)
    setCertStatus(null)
    await fetchCertStatus(record.id)
    // 启动定时刷新
    if (statusTimerRef.current) clearInterval(statusTimerRef.current)
    statusTimerRef.current = setInterval(() => {
      fetchCertStatus(record.id)
    }, 5000)
  }

  const closeStatusModal = () => {
    setStatusModalOpen(false)
    setStatusCert(null)
    setCertStatus(null)
    if (statusTimerRef.current) {
      clearInterval(statusTimerRef.current)
      statusTimerRef.current = null
    }
  }

  const fetchCertStatus = async (id: number) => {
    setStatusLoading(true)
    try {
      const res: any = await domainCertApi.getStatus(id)
      setCertStatus(res.data || {})
      // 如果已完成或出错，停止刷新
      if (['valid', 'error', 'pending', 'expired'].includes(res.data?.status)) {
        if (statusTimerRef.current) {
          clearInterval(statusTimerRef.current)
          statusTimerRef.current = null
        }
        // 刷新列表
        fetchData()
      }
    } finally {
      setStatusLoading(false)
    }
  }

  // 手动触发 ACME 步骤
  const handleStep = async (id: number, step: string) => {
    try {
      switch (step) {
        case 'create-order':
          await domainCertApi.stepCreateOrder(id)
          break
        case 'set-dns':
          await domainCertApi.stepSetDNS(id)
          break
        case 'validate':
          await domainCertApi.stepValidate(id)
          break
        case 'obtain':
          await domainCertApi.stepObtain(id)
          break
        case 'confirm-dns':
          await domainCertApi.confirmDNS(id)
          break
      }
      message.success(t('domainCert.stepSubmitted'))
      setTimeout(() => fetchCertStatus(id), 2000)
      // 重新启动定时刷新
      if (statusTimerRef.current) clearInterval(statusTimerRef.current)
      statusTimerRef.current = setInterval(() => {
        fetchCertStatus(id)
      }, 5000)
    } catch (e: any) {
      message.error(e?.message || t('common.error'))
    }
  }

  // 清理定时器
  useEffect(() => {
    return () => {
      if (statusTimerRef.current) clearInterval(statusTimerRef.current)
    }
  }, [])

  const getExpireInfo = (expireAt: string) => {
    if (!expireAt) return { tag: <Tag>{t('domainCert.pending')}</Tag>, percent: 0 }
    const days = dayjs(expireAt).diff(dayjs(), 'day')
    if (days < 0) return { tag: <Tag color="error">{t('domainCert.expired')}</Tag>, percent: 0 }
    if (days < 7) return { tag: <Tag color="error">{days}{t('domainCert.daysLeft')}</Tag>, percent: Math.min(days / 90 * 100, 100) }
    if (days < 30) return { tag: <Tag color="warning">{days}{t('domainCert.daysLeft')}</Tag>, percent: Math.min(days / 90 * 100, 100) }
    return { tag: <Tag color="success">{days}{t('domainCert.daysLeft')}</Tag>, percent: Math.min(days / 90 * 100, 100) }
  }

  // 获取状态标签
  const getStatusTag = (status: string) => {
    switch (status) {
      case 'pending':
        return <Tag icon={<ClockCircleOutlined />} color="default">{t('domainCert.statusPending')}</Tag>
      case 'applying':
      case 'order_created':
        return <Tag icon={<LoadingOutlined />} color="processing">{t('domainCert.statusCreatingOrder')}</Tag>
      case 'dns_set':
        return <Tag icon={<LoadingOutlined />} color="processing">{t('domainCert.statusDnsSet')}</Tag>
      case 'validating':
        return <Tag icon={<LoadingOutlined />} color="processing">{t('domainCert.statusValidating')}</Tag>
      case 'valid':
        return <Tag icon={<CheckCircleOutlined />} color="success">{t('domainCert.statusValid')}</Tag>
      case 'expired':
        return <Tag icon={<CloseCircleOutlined />} color="error">{t('domainCert.statusExpired')}</Tag>
      case 'error':
        return <Tag icon={<CloseCircleOutlined />} color="error">{t('domainCert.statusError')}</Tag>
      default:
        return <Tag>{status || '-'}</Tag>
    }
  }

  // 获取 ACME 步骤序号
  const getAcmeStepIndex = (status: string, step: number): number => {
    switch (status) {
      case 'pending': return -1
      case 'applying':
      case 'order_created': return 0
      case 'dns_set': return 1
      case 'validating': return 2
      case 'valid': return 4
      case 'error': return step > 0 ? step - 1 : 0
      default: return -1
    }
  }

  const CA_COLOR: Record<string, string> = { letsencrypt: 'green', zerossl: 'blue', buypass: 'purple', google: 'red' }
  const CA_LABEL: Record<string, string> = { letsencrypt: "Let's Encrypt", zerossl: 'ZeroSSL', buypass: 'Buypass', google: 'Google Trust' }

  const columns = [
    {
      title: t('common.name'), dataIndex: 'name',
      render: (name: string, r: any) => (
        <div>
          <Space>
            <SafetyCertificateOutlined style={{ color: '#1677ff' }} />
            <Text strong>{name}</Text>
            {r.cert_type === 'manual' && <Tag color="orange" style={{ fontSize: 11 }}>{t('domainCert.certTypeManual')}</Tag>}
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
          return <Space size={4} wrap>{arr.map((d: string) => <Tag key={d}>{d}</Tag>)}</Space>
        } catch { return v }
      },
    },
    {
      title: t('domainCert.ca'), dataIndex: 'ca',
      render: (v: string, r: any) => {
        if (r.cert_type === 'manual') return <Tag color="orange">{t('domainCert.certTypeManual')}</Tag>
        const certAcc = certAccounts.find(a => a.id === r.cert_account_id)
        return (
          <div>
            <Tag color={CA_COLOR[v] || 'blue'}>{CA_LABEL[v] || v || "Let's Encrypt"}</Tag>
            {certAcc && <div><Text type="secondary" style={{ fontSize: 11 }}>{t('domainCert.certAccount')}: {certAcc.name}</Text></div>}
          </div>
        )
      },
    },
    {
      title: t('domainCert.status'), dataIndex: 'status', width: 150,
      render: (status: string, r: any) => (
        <div>
          {getStatusTag(status)}
          {r.cert_type === 'acme' && ['applying', 'order_created', 'dns_set', 'validating', 'error'].includes(status) && (
            <div style={{ marginTop: 4 }}>
              <Button type="link" size="small" style={{ padding: 0, fontSize: 12 }} onClick={() => openStatusModal(r)}>
                {t('domainCert.viewProgress')}
              </Button>
            </div>
          )}
        </div>
      ),
    },
    {
      title: t('domainCert.expireAt'), dataIndex: 'expire_at', width: 180,
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
      render: (v: boolean, r: any) => r.cert_type === 'manual'
        ? <Tag color="default">-</Tag>
        : (v ? <Tag color="blue">{t('domainCert.autoRenewOn')}</Tag> : <Tag>{t('domainCert.autoRenewOff')}</Tag>),
    },
    {
      title: t('common.action'), width: 200,
      render: (_: any, r: any) => (
        <Space size={4}>
          {r.cert_type !== 'manual' && (
            <>
              <Tooltip title={r.status === 'pending' ? t('domainCert.apply') : t('domainCert.renew')}>
                <Button size="small" type={r.status === 'pending' ? 'primary' : 'default'}
                  icon={<SyncOutlined />} loading={applyingIds.has(r.id)}
                  onClick={() => handleApply(r.id)}
                  disabled={['applying', 'order_created', 'dns_set', 'validating'].includes(r.status)}
                />
              </Tooltip>
              {['applying', 'order_created', 'dns_set', 'validating', 'error', 'valid'].includes(r.status) && (
                <Tooltip title={t('domainCert.viewProgress')}>
                  <Button size="small" icon={<AuditOutlined />} onClick={() => openStatusModal(r)} />
                </Tooltip>
              )}
            </>
          )}
          {r.cert_file && (
            <Tooltip title={t('domainCert.downloadCert')}>
              <Button size="small" icon={<DownloadOutlined />}
                onClick={() => window.open(`/api/v1/domain/certs/${r.id}/download`, '_blank')} />
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

  // 渲染 DNS 账号校验警告
  const renderDnsWarnings = () => {
    const missing = dnsWarnings.filter(w => w.missing)
    if (missing.length === 0) return null
    return (
      <>
        <Alert
          type="warning"
          showIcon
          icon={<ExclamationCircleOutlined />}
          style={{ marginBottom: 12 }}
          message={t('domainCert.dnsWarningTitle')}
          description={
            <Space wrap size={4}>
              {missing.map(w => <Tag key={w.domain} color="warning">{w.domain}</Tag>)}
              <Text type="secondary" style={{ fontSize: 11 }}>
                {t('domainCert.dnsWarningHint')}
              </Text>
            </Space>
          }
        />
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 12 }}
          message={t('domainCert.manualDnsModeAutoSet')}
          description={t('domainCert.manualDnsModeAutoSetHint')}
        />
      </>
    )
  }

  // 渲染 ACME 流程状态弹窗内容
  const renderStatusModal = () => {
    if (!statusCert) return null
    const status = certStatus || statusCert
    const currentStep = getAcmeStepIndex(status.status, status.acme_step || 0)
    const isError = status.status === 'error'
    const isValid = status.status === 'valid'

    return (
      <div>
        {/* 步骤进度条 */}
        <Steps
          current={isValid ? 4 : currentStep}
          status={isError ? 'error' : (isValid ? 'finish' : 'process')}
          size="small"
          style={{ marginBottom: 24 }}
          items={[
            {
              title: t('domainCert.step1Title'),
              description: t('domainCert.step1Desc'),
              icon: <CloudServerOutlined />,
            },
            {
              title: t('domainCert.step2Title'),
              description: t('domainCert.step2Desc'),
              icon: <GlobalOutlined />,
            },
            {
              title: t('domainCert.step3Title'),
              description: t('domainCert.step3Desc'),
              icon: <AuditOutlined />,
            },
            {
              title: t('domainCert.step4Title'),
              description: t('domainCert.step4Desc'),
              icon: <FileProtectOutlined />,
            },
          ]}
        />

        {/* 状态详情 */}
        <Card size="small" style={{ marginBottom: 16 }}>
          <Descriptions column={2} size="small">
            <Descriptions.Item label={t('domainCert.currentStatus')}>
              {getStatusTag(status.status)}
            </Descriptions.Item>
            <Descriptions.Item label={t('domainCert.currentStep')}>
              {status.acme_step || 0} / 4
            </Descriptions.Item>
            {status.dns_mode === 'manual' && (
              <Descriptions.Item label={t('domainCert.dnsMode')} span={2}>
                <Tag color="orange">{t('domainCert.dnsModeManual')}</Tag>
              </Descriptions.Item>
            )}
            {status.acme_next_action && (
              <Descriptions.Item label={t('domainCert.nextAction')} span={2}>
                <Space>
                  <ClockCircleOutlined />
                  {dayjs(status.acme_next_action).format('YYYY-MM-DD HH:mm:ss')}
                  <Text type="secondary">
                    ({dayjs(status.acme_next_action).diff(dayjs(), 'second') > 0
                      ? `${dayjs(status.acme_next_action).diff(dayjs(), 'second')} ${t('domainCert.secondsLater')}`
                      : t('domainCert.executing')})
                  </Text>
                </Space>
              </Descriptions.Item>
            )}
            {status.expire_at && (
              <Descriptions.Item label={t('domainCert.expireAt')} span={2}>
                {dayjs(status.expire_at).format('YYYY-MM-DD HH:mm:ss')}
              </Descriptions.Item>
            )}
          </Descriptions>
        </Card>

        {/* DNS 记录信息 */}
        {status.acme_dns_record && (
          <Card size="small" title={t('domainCert.dnsRecordInfo')} style={{ marginBottom: 16 }}>
            {status.dns_mode === 'manual' && ['order_created', 'dns_set'].includes(status.status) && (
              <Alert
                type="info"
                showIcon
                style={{ marginBottom: 12 }}
                message={t('domainCert.manualDnsHint')}
                description={t('domainCert.manualDnsHintDesc')}
              />
            )}
            {status.acme_dns_record.split('\n').map((record: string, idx: number) => {
              const value = status.acme_dns_value?.split('\n')[idx] || ''
              return (
                <div key={idx} style={{ marginBottom: 8 }}>
                  <div>
                    <Text strong>{t('domainCert.dnsRecordName')}:</Text>{' '}
                    <Text code copyable>{record}</Text>
                  </div>
                  <div>
                    <Text strong>{t('domainCert.dnsRecordValue')}:</Text>{' '}
                    <Text code copyable>{value}</Text>
                  </div>
                </div>
              )
            })}
          </Card>
        )}

        {/* 错误信息 */}
        {status.last_error && (
          <Alert
            type="error"
            showIcon
            message={t('domainCert.errorInfo')}
            description={status.last_error}
            style={{ marginBottom: 16 }}
          />
        )}

        {/* 手动操作按钮 */}
        <Card size="small" title={t('domainCert.manualOps')}>
          <Space wrap>
            <Button
              size="small"
              icon={<CloudServerOutlined />}
              onClick={() => handleStep(statusCert.id, 'create-order')}
              disabled={!['pending', 'error', 'valid', 'expired'].includes(status.status)}
            >
              {t('domainCert.step1Title')}
            </Button>
            {status.dns_mode !== 'manual' && (
              <Button
                size="small"
                icon={<GlobalOutlined />}
                onClick={() => handleStep(statusCert.id, 'set-dns')}
                disabled={status.status !== 'order_created'}
              >
                {t('domainCert.step2Title')}
              </Button>
            )}
            {status.dns_mode === 'manual' && (
              <Button
                size="small"
                type="primary"
                icon={<CheckCircleOutlined />}
                onClick={() => handleStep(statusCert.id, 'confirm-dns')}
                disabled={!['order_created', 'dns_set'].includes(status.status)}
              >
                {t('domainCert.confirmDns')}
              </Button>
            )}
            <Button
              size="small"
              icon={<AuditOutlined />}
              onClick={() => handleStep(statusCert.id, 'validate')}
              disabled={status.status !== 'dns_set'}
            >
              {t('domainCert.step3Title')}
            </Button>
            <Button
              size="small"
              icon={<FileProtectOutlined />}
              onClick={() => handleStep(statusCert.id, 'obtain')}
              disabled={status.status !== 'validating'}
            >
              {t('domainCert.step4Title')}
            </Button>
          </Space>
          <div style={{ marginTop: 8 }}>
            <Text type="secondary" style={{ fontSize: 11 }}>
              {status.dns_mode === 'manual'
                ? t('domainCert.manualDnsOpsHint')
                : t('domainCert.manualOpsHint')}
            </Text>
          </div>
        </Card>
      </div>
    )
  }

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
        size="middle" style={tableStyle}
        pagination={{ pageSize: 20, showSizeChanger: true }}
      />

      {/* 创建/编辑弹窗 */}
      <Modal
        title={editRecord ? t('common.edit') : t('common.create')}
        open={modalOpen} onOk={handleSubmit} onCancel={() => { setModalOpen(false); setDnsWarnings([]) }}
        width={600} destroyOnHidden
      >
        <Form form={form} layout="vertical" style={{ marginTop: 16 }}>
          <Form.Item name="name" label={t('common.name')} rules={[{ required: true }]}>
            <Input placeholder={t('domainCert.namePlaceholder')} />
          </Form.Item>

          {/* 证书类型切换 */}
          <Form.Item name="cert_type" label={t('domainCert.certType')}>
            <Radio.Group onChange={() => { setDnsWarnings([]) }}>
              <Radio value="acme">{t('domainCert.certTypeAcme')}</Radio>
              <Radio value="manual">{t('domainCert.certTypeManual')}</Radio>
            </Radio.Group>
          </Form.Item>

          <Form.Item noStyle shouldUpdate={(prev, cur) => prev.cert_type !== cur.cert_type}>
            {({ getFieldValue }) => getFieldValue('cert_type') === 'manual' ? (
              /* ===== 手动上传模式 ===== */
              <>
                <Form.Item
                  name="cert_content"
                  label={t('domainCert.certContent')}
                  rules={[{ required: true, message: t('domainCert.certContentRequired') }]}
                  extra={parsingCert ? t('domainCert.certContentParsing') : t('domainCert.certContentHint')}
                >
                  <Input.TextArea
                    rows={6}
                    placeholder="-----BEGIN CERTIFICATE-----&#10;...&#10;-----END CERTIFICATE-----"
                    onBlur={e => handleCertContentChange(e.target.value)}
                  />
                </Form.Item>

                <Form.Item
                  name="domains"
                  label={t('domainCert.domainsAutoDetect')}
                  rules={[{
                    validator: (_, val: DomainEntry[]) => {
                      const domains = entriesToDomains(val || [])
                      return domains.length > 0 ? Promise.resolve() : Promise.reject(t('domainCert.domainsRequired'))
                    }
                  }]}
                >
                  <DomainListEditor />
                </Form.Item>

                <Form.Item
                  name="key_content"
                  label={t('domainCert.keyContent')}
                  rules={[{ required: true, message: t('domainCert.keyContentRequired') }]}
                  extra={t('domainCert.keyContentHint')}
                >
                  <Input.TextArea rows={5} placeholder="-----BEGIN PRIVATE KEY-----&#10;...&#10;-----END PRIVATE KEY-----" />
                </Form.Item>
              </>
            ) : (
              /* ===== ACME 自动申请模式 ===== */
              <>
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
                      <Select onChange={() => setDnsWarnings([])}>
                        <Option value="dns">DNS-01（{t('domainCert.dnsRecommended')}）</Option>
                        <Option value="http">HTTP-01</Option>
                      </Select>
                    </Form.Item>
                  </Col>
                </Row>

                {/* 证书账号 */}
                <Form.Item
                  name="cert_account_id"
                  label={t('domainCert.certAccount')}
                  rules={[{ required: true, message: t('domainCert.certAccountRequired') }]}
                  extra={t('domainCert.certAccountHint')}
                >
                  <Select placeholder={t('domainCert.certAccountPlaceholder')}>
                    {certAccounts.map(a => (
                      <Option key={a.id} value={a.id}>
                        <Space size={4}>
                          <Tag color={CA_COLOR[a.type] || 'default'} style={{ margin: 0 }}>{a.type}</Tag>
                          {a.name}
                          {a.email && <Text type="secondary" style={{ fontSize: 11 }}>({a.email})</Text>}
                        </Space>
                      </Option>
                    ))}
                  </Select>
                </Form.Item>

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

                {/* DNS 账号 */}
                <Form.Item
                  noStyle
                  shouldUpdate={(prev, cur) =>
                    prev.challenge_type !== cur.challenge_type ||
                    prev.domain_account_id !== cur.domain_account_id ||
                    prev.domains !== cur.domains
                  }
                >
                  {({ getFieldValue: gfv }) => gfv('challenge_type') === 'dns' && (
                    <>
                      <Form.Item
                        name="domain_account_id"
                        label={t('domainCert.dnsAccount')}
                        extra={t('domainCert.dnsAccountHint')}
                      >
                        <Select
                          placeholder={t('domainCert.dnsAccountPlaceholder')}
                          allowClear
                          onChange={(val) => {
                            const entries: DomainEntry[] = gfv('domains') || []
                            checkDnsWarnings(entries, val)
                          }}
                        >
                          {accounts.map(a => (
                            <Option key={a.id} value={a.id}>
                              <Space size={4}>
                                <Tag color="blue" style={{ margin: 0 }}>{a.provider}</Tag>
                                {a.name}
                              </Space>
                            </Option>
                          ))}
                        </Select>
                      </Form.Item>
                      {renderDnsWarnings()}

                      {/* DNS 模式选择 */}
                      <Form.Item
                        name="dns_mode"
                        label={t('domainCert.dnsMode')}
                        extra={t('domainCert.dnsModeHint')}
                      >
                        <Radio.Group
                          value={dnsMode}
                          onChange={(e) => {
                            setDnsMode(e.target.value)
                          }}
                        >
                          <Radio value="auto">{t('domainCert.dnsModeAuto')}</Radio>
                          <Radio value="manual">{t('domainCert.dnsModeManual')}</Radio>
                        </Radio.Group>
                      </Form.Item>
                    </>
                  )}
                </Form.Item>

                <Row gutter={16}>
                  <Col span={12}>
                    <Form.Item name="auto_renew" label={t('domainCert.autoRenew')} valuePropName="checked">
                      <Switch checkedChildren={t('domainCert.autoRenewOn')} unCheckedChildren={t('domainCert.autoRenewOff')} />
                    </Form.Item>
                  </Col>
                  <Col span={12}>
                    <Form.Item
                      name="renew_before_days"
                      label={t('domainCert.renewBeforeDays')}
                      extra={<span style={{ fontSize: 11 }}>{t('domainCert.renewBeforeDaysHint')}</span>}
                    >
                      <InputNumber min={1} max={60} style={{ width: '100%' }} addonAfter={t('domainCert.days')} />
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

      {/* ACME 流程状态弹窗 */}
      <Modal
        title={
          <Space>
            <SafetyCertificateOutlined />
            {t('domainCert.acmeFlowTitle')}
            {statusCert && <Text type="secondary">- {statusCert.name}</Text>}
          </Space>
        }
        open={statusModalOpen}
        onCancel={closeStatusModal}
        footer={[
          <Button key="refresh" icon={<SyncOutlined />} loading={statusLoading}
            onClick={() => statusCert && fetchCertStatus(statusCert.id)}>
            {t('common.refresh')}
          </Button>,
          <Button key="close" onClick={closeStatusModal}>
            {t('common.close')}
          </Button>,
        ]}
        width={700}
        destroyOnHidden
      >
        {renderStatusModal()}
      </Modal>
    </div>
  )
}

export default DomainCert