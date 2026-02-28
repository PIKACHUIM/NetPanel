import React, { useEffect, useState, useCallback } from 'react'
import {
  Table, Button, Space, Modal, Form, Input, Popconfirm, message,
  Typography, Tag, Tabs, Switch, Tooltip, Alert, Row, Col, Badge, InputNumber
} from 'antd'
import {
  PlusOutlined, EditOutlined, DeleteOutlined, SearchOutlined,
  DownloadOutlined, ImportOutlined, CloudDownloadOutlined,
  DatabaseOutlined, ReloadOutlined, SyncOutlined, LinkOutlined,
  CheckCircleOutlined, CloseCircleOutlined
} from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { ipdbApi } from '../api'
import dayjs from 'dayjs'

const { TextArea } = Input
const { Text } = Typography

// ===== IP 条目 Tab =====
const EntryTab: React.FC = () => {
  const { t } = useTranslation()
  const [data, setData] = useState<any[]>([])
  const [loading, setLoading] = useState(false)
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [keyword, setKeyword] = useState('')

  // 单条编辑
  const [editModalOpen, setEditModalOpen] = useState(false)
  const [editRecord, setEditRecord] = useState<any>(null)
  const [editForm] = Form.useForm()

  // 文本批量导入
  const [textImportOpen, setTextImportOpen] = useState(false)
  const [textImportForm] = Form.useForm()
  const [textImporting, setTextImporting] = useState(false)

  // URL 导入
  const [urlImportOpen, setUrlImportOpen] = useState(false)
  const [urlImportForm] = Form.useForm()
  const [urlImporting, setUrlImporting] = useState(false)

  // IP 查询
  const [queryIP, setQueryIP] = useState('')
  const [queryResult, setQueryResult] = useState<any>(null)
  const [querying, setQuerying] = useState(false)

  const fetchData = useCallback(async (p = page, ps = pageSize, kw = keyword) => {
    setLoading(true)
    try {
      const res: any = await ipdbApi.list({ page: p, page_size: ps, keyword: kw })
      setData(res.data?.list || [])
      setTotal(res.data?.total || 0)
    } finally {
      setLoading(false)
    }
  }, [page, pageSize, keyword])

  useEffect(() => { fetchData() }, [])

  const handleSearch = () => { setPage(1); fetchData(1, pageSize, keyword) }

  const handleTableChange = (pagination: any) => {
    setPage(pagination.current)
    setPageSize(pagination.pageSize)
    fetchData(pagination.current, pagination.pageSize, keyword)
  }

  const handleEditSubmit = async () => {
    const values = await editForm.validateFields()
    try {
      if (editRecord) {
        await ipdbApi.update(editRecord.id, values)
      } else {
        await ipdbApi.create(values)
      }
      message.success(t('common.success'))
      setEditModalOpen(false)
      fetchData(page, pageSize, keyword)
    } catch (e: any) {
      message.error(e?.response?.data?.message || t('common.error'))
    }
  }

  const handleTextImport = async () => {
    const values = await textImportForm.validateFields()
    setTextImporting(true)
    try {
      const res: any = await ipdbApi.batchImport({
        text: values.text,
        location: values.location,
        tags: values.tags,
      })
      message.success(t('ipdb.importSuccess', { count: res.data?.data?.count || 0 }))
      setTextImportOpen(false)
      textImportForm.resetFields()
      fetchData(1, pageSize, keyword)
      setPage(1)
    } catch (e: any) {
      message.error(e?.response?.data?.message || t('common.error'))
    } finally {
      setTextImporting(false)
    }
  }

  const handleUrlImport = async () => {
    const values = await urlImportForm.validateFields()
    setUrlImporting(true)
    try {
      const res: any = await ipdbApi.importFromUrl({
        url: values.url,
        location: values.location,
        tags: values.tags,
        clear_first: values.clear_first || false,
      })
      message.success(t('ipdb.importSuccess', { count: res.data?.data?.count || 0 }))
      setUrlImportOpen(false)
      urlImportForm.resetFields()
      fetchData(1, pageSize, keyword)
      setPage(1)
    } catch (e: any) {
      message.error(e?.response?.data?.message || t('common.error'))
    } finally {
      setUrlImporting(false)
    }
  }

  const handleQuery = async () => {
    if (!queryIP.trim()) return
    setQuerying(true)
    setQueryResult(null)
    try {
      const res: any = await ipdbApi.query(queryIP.trim())
      setQueryResult(res.data?.data)
    } catch (e: any) {
      message.error(e?.response?.data?.message || t('common.error'))
    } finally {
      setQuerying(false)
    }
  }

  const columns = [
    {
      title: t('ipdb.cidr'),
      dataIndex: 'cidr',
      width: 200,
      render: (v: string) => <Text code copyable>{v}</Text>,
    },
    {
      title: t('ipdb.location'),
      dataIndex: 'location',
      width: 160,
      render: (v: string) => v || <Text type="secondary">-</Text>,
    },
    {
      title: t('ipdb.tags'),
      dataIndex: 'tags',
      render: (v: string) =>
        v ? v.split(',').map((tag: string) => (
          <Tag key={tag} color="blue" style={{ marginBottom: 2 }}>{tag.trim()}</Tag>
        )) : <Text type="secondary">-</Text>,
    },
    {
      title: t('common.remark'),
      dataIndex: 'remark',
      ellipsis: true,
      render: (v: string) => v || <Text type="secondary">-</Text>,
    },
    {
      title: t('common.action'),
      width: 100,
      render: (_: any, r: any) => (
        <Space size={4}>
          <Tooltip title={t('common.edit')}>
            <Button size="small" icon={<EditOutlined />} onClick={() => {
              setEditRecord(r); editForm.setFieldsValue(r); setEditModalOpen(true)
            }} />
          </Tooltip>
          <Popconfirm title={t('common.deleteConfirm')} onConfirm={async () => {
            await ipdbApi.delete(r.id)
            message.success(t('common.success'))
            fetchData(page, pageSize, keyword)
          }}>
            <Tooltip title={t('common.delete')}>
              <Button size="small" danger icon={<DeleteOutlined />} />
            </Tooltip>
          </Popconfirm>
        </Space>
      ),
    },
  ]

  return (
    <>
      {/* 工具栏 */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 12, flexWrap: 'wrap', gap: 8 }}>
        <Space>
          <Tag color="blue">{t('ipdb.total', { total })}</Tag>
        </Space>
        <Space wrap>
          <Input.Search
            placeholder={t('ipdb.queryPlaceholder')}
            value={queryIP}
            onChange={e => setQueryIP(e.target.value)}
            onSearch={handleQuery}
            loading={querying}
            style={{ width: 220 }}
            enterButton={<SearchOutlined />}
            allowClear
          />
          <Button icon={<ImportOutlined />} onClick={() => { textImportForm.resetFields(); setTextImportOpen(true) }}>
            {t('ipdb.importText')}
          </Button>
          <Button icon={<CloudDownloadOutlined />} onClick={() => { urlImportForm.resetFields(); setUrlImportOpen(true) }}>
            {t('ipdb.importUrl')}
          </Button>
          <Button type="primary" icon={<PlusOutlined />} onClick={() => { setEditRecord(null); editForm.resetFields(); setEditModalOpen(true) }}>
            {t('ipdb.addEntry')}
          </Button>
          <Tooltip title={t('common.refresh')}>
            <Button icon={<ReloadOutlined />} onClick={() => fetchData(page, pageSize, keyword)} />
          </Tooltip>
        </Space>
      </div>

      {/* IP 查询结果 */}
      {queryResult && (
        <Alert
          style={{ marginBottom: 12 }}
          type={queryResult.found === false ? 'warning' : 'success'}
          showIcon closable onClose={() => setQueryResult(null)}
          message={
            queryResult.found === false
              ? `${queryIP} - ${t('ipdb.notFound')}`
              : (
                <Space>
                  <Text code>{queryResult.cidr || queryIP}</Text>
                  {queryResult.location && <Text strong>{queryResult.location}</Text>}
                  {queryResult.tags && queryResult.tags.split(',').map((tag: string) => (
                    <Tag key={tag} color="blue">{tag.trim()}</Tag>
                  ))}
                  {queryResult.remark && <Text type="secondary">{queryResult.remark}</Text>}
                </Space>
              )
          }
        />
      )}

      {/* 搜索栏 */}
      <div style={{ marginBottom: 12, display: 'flex', gap: 8 }}>
        <Input
          placeholder={`${t('common.search')} IP/CIDR、${t('ipdb.location')}、${t('ipdb.tags')}...`}
          value={keyword}
          onChange={e => setKeyword(e.target.value)}
          onPressEnter={handleSearch}
          style={{ maxWidth: 360 }}
          allowClear
          prefix={<SearchOutlined style={{ color: '#bbb' }} />}
        />
        <Button onClick={handleSearch}>{t('common.search')}</Button>
      </div>

      {/* 数据表格 */}
      <Table
        dataSource={data} columns={columns} rowKey="id" loading={loading} size="middle"
        style={{ background: '#fff', borderRadius: 8 }}
        pagination={{
          current: page, pageSize, total,
          showSizeChanger: true, showQuickJumper: true,
          showTotal: (n) => `共 ${n} 条`,
          pageSizeOptions: ['20', '50', '100', '200'],
        }}
        onChange={handleTableChange}
      />

      {/* 单条编辑 Modal */}
      <Modal
        title={editRecord ? t('common.edit') : t('ipdb.addEntry')}
        open={editModalOpen} onOk={handleEditSubmit} onCancel={() => setEditModalOpen(false)}
        width={480} destroyOnHidden
      >
        <Form form={editForm} layout="vertical" style={{ marginTop: 16 }}>
          <Form.Item name="cidr" label={t('ipdb.cidr')} rules={[{ required: true, message: '请输入 IP 或 CIDR' }]}>
            <Input placeholder="192.168.1.0/24 或 1.2.3.4" />
          </Form.Item>
          <Form.Item name="location" label={t('ipdb.location')}>
            <Input placeholder="中国-北京" />
          </Form.Item>
          <Form.Item name="tags" label={t('ipdb.tags')} extra="多个标签用英文逗号分隔">
            <Input placeholder="内网,私有" />
          </Form.Item>
          <Form.Item name="remark" label={t('common.remark')}>
            <TextArea rows={2} />
          </Form.Item>
        </Form>
      </Modal>

      {/* 文本批量导入 Modal */}
      <Modal
        title={<Space><ImportOutlined />{t('ipdb.importText')}</Space>}
        open={textImportOpen} onOk={handleTextImport} onCancel={() => setTextImportOpen(false)}
        confirmLoading={textImporting} width={640} destroyOnHidden okText="开始导入"
      >
        <Alert
          type="info" showIcon style={{ marginBottom: 12 }}
          message="格式说明"
          description={
            <div style={{ lineHeight: '1.8' }}>
              <div>每行可填写<strong>一个或多个</strong> IP/CIDR，用<strong>空格、逗号或分号</strong>分隔，行尾可附加归属地和标签：</div>
              <Text code style={{ display: 'block', margin: '4px 0' }}>192.168.1.0/24 10.0.0.0/8 172.16.0.0/12</Text>
              <Text code style={{ display: 'block', margin: '4px 0' }}>1.2.3.4, 5.6.7.8; 9.10.11.0/24 美国-纽约</Text>
              <Text code style={{ display: 'block', margin: '4px 0' }}>203.0.113.0/24 中国-北京 内网,私有</Text>
              <div style={{ marginTop: 4, color: '#888' }}>以 # 开头的行为注释，将被忽略</div>
            </div>
          }
        />
        <Form form={textImportForm} layout="vertical">
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item name="location" label={t('ipdb.defaultLocation')} extra="可选，未在行内指定时使用">
                <Input placeholder="中国" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="tags" label={t('ipdb.defaultTags')} extra="可选，多个标签用英文逗号分隔">
                <Input placeholder="黑名单,恶意IP" />
              </Form.Item>
            </Col>
          </Row>
          <Form.Item name="text" label="IP 列表" rules={[{ required: true, message: '请输入 IP 列表' }]}>
            <TextArea
              rows={12}
              placeholder={t('ipdb.importTextPlaceholder')}
              style={{ fontFamily: 'monospace', fontSize: 13 }}
            />
          </Form.Item>
        </Form>
      </Modal>

      {/* URL 导入 Modal */}
      <Modal
        title={<Space><CloudDownloadOutlined />{t('ipdb.importUrl')}</Space>}
        open={urlImportOpen} onOk={handleUrlImport} onCancel={() => setUrlImportOpen(false)}
        confirmLoading={urlImporting} width={560} destroyOnHidden
        okText={urlImporting ? t('ipdb.downloading') : '开始下载导入'}
      >
        <Alert
          type="info" showIcon style={{ marginBottom: 16 }}
          message="从网络下载 IP 列表文件，每行支持多个 IP/CIDR（空格/逗号/分号分隔），最大 50MB"
        />
        <Form form={urlImportForm} layout="vertical">
          <Form.Item
            name="url" label={t('ipdb.importUrlLabel')}
            rules={[{ required: true, message: '请输入下载地址' }, { type: 'url', message: '请输入有效的 URL' }]}
          >
            <Input placeholder="https://example.com/ip-list.txt" prefix={<DownloadOutlined />} />
          </Form.Item>
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item name="location" label={t('ipdb.defaultLocation')} extra="可选，为导入条目设置默认归属地">
                <Input placeholder="中国" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="tags" label={t('ipdb.defaultTags')} extra="可选，多个标签用英文逗号分隔">
                <Input placeholder="黑名单,恶意IP" />
              </Form.Item>
            </Col>
          </Row>
          <Form.Item name="clear_first" valuePropName="checked">
            <Space>
              <Switch />
              <span>{t('ipdb.clearFirst')}</span>
              <Text type="secondary" style={{ fontSize: 12 }}>（{t('ipdb.clearFirstTip')}）</Text>
            </Space>
          </Form.Item>
        </Form>
      </Modal>
    </>
  )
}

// ===== 订阅管理 Tab =====
const SubscriptionTab: React.FC = () => {
  const { t } = useTranslation()
  const [data, setData] = useState<any[]>([])
  const [loading, setLoading] = useState(false)
  const [refreshingId, setRefreshingId] = useState<number | null>(null)

  // 编辑
  const [editModalOpen, setEditModalOpen] = useState(false)
  const [editRecord, setEditRecord] = useState<any>(null)
  const [editForm] = Form.useForm()
  const [submitting, setSubmitting] = useState(false)

  const fetchData = useCallback(async () => {
    setLoading(true)
    try {
      const res: any = await ipdbApi.listSubscriptions()
      setData(res.data?.data || [])
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { fetchData() }, [])

  const handleSubmit = async () => {
    const values = await editForm.validateFields()
    setSubmitting(true)
    try {
      if (editRecord) {
        await ipdbApi.updateSubscription(editRecord.id, values)
      } else {
        await ipdbApi.createSubscription(values)
      }
      message.success(t('common.success'))
      setEditModalOpen(false)
      fetchData()
    } catch (e: any) {
      message.error(e?.response?.data?.message || t('common.error'))
    } finally {
      setSubmitting(false)
    }
  }

  const handleRefresh = async (id: number) => {
    setRefreshingId(id)
    try {
      const res: any = await ipdbApi.refreshSubscription(id)
      message.success(t('ipdb.subscriptionRefreshSuccess', { count: res.data?.data?.count || 0 }))
      fetchData()
    } catch (e: any) {
      message.error(e?.response?.data?.message || t('common.error'))
      fetchData() // 刷新以显示错误信息
    } finally {
      setRefreshingId(null)
    }
  }

  const handleToggleEnable = async (record: any, checked: boolean) => {
    try {
      await ipdbApi.updateSubscription(record.id, { ...record, enable: checked })
      message.success(t('common.success'))
      fetchData()
    } catch (e: any) {
      message.error(e?.response?.data?.message || t('common.error'))
    }
  }

  const columns = [
    {
      title: t('common.enable'),
      dataIndex: 'enable',
      width: 70,
      render: (v: boolean, r: any) => (
        <Switch size="small" checked={v} onChange={(checked) => handleToggleEnable(r, checked)} />
      ),
    },
    {
      title: t('ipdb.subscriptionName'),
      dataIndex: 'name',
      width: 140,
      render: (v: string) => <Text strong>{v}</Text>,
    },
    {
      title: t('ipdb.subscriptionUrl'),
      dataIndex: 'url',
      ellipsis: true,
      render: (v: string) => (
        <Tooltip title={v}>
          <a href={v} target="_blank" rel="noopener noreferrer">
            <LinkOutlined style={{ marginRight: 4 }} />{v}
          </a>
        </Tooltip>
      ),
    },
    {
      title: t('ipdb.location'),
      dataIndex: 'location',
      width: 120,
      render: (v: string) => v || <Text type="secondary">-</Text>,
    },
    {
      title: t('ipdb.tags'),
      dataIndex: 'tags',
      width: 120,
      render: (v: string) =>
        v ? v.split(',').map((tag: string) => (
          <Tag key={tag} color="blue" style={{ marginBottom: 2 }}>{tag.trim()}</Tag>
        )) : <Text type="secondary">-</Text>,
    },
    {
      title: t('ipdb.subscriptionInterval'),
      dataIndex: 'interval',
      width: 110,
      render: (v: number) => v > 0
        ? <Tag color="geekblue">{v}h</Tag>
        : <Text type="secondary">{t('ipdb.subscriptionIntervalTip').split('，')[0]}</Text>,
    },
    {
      title: t('ipdb.subscriptionLastSync'),
      dataIndex: 'last_sync_time',
      width: 160,
      render: (v: string, r: any) => {
        if (!v) return <Text type="secondary">{t('ipdb.subscriptionNever')}</Text>
        const hasError = !!r.last_sync_error
        return (
          <Space direction="vertical" size={0}>
            <Space size={4}>
              {hasError
                ? <CloseCircleOutlined style={{ color: '#ff4d4f' }} />
                : <CheckCircleOutlined style={{ color: '#52c41a' }} />}
              <Text style={{ fontSize: 12 }}>{dayjs(v).format('MM-DD HH:mm')}</Text>
            </Space>
            {r.last_sync_count > 0 && (
              <Text type="secondary" style={{ fontSize: 11 }}>{t('ipdb.subscriptionLastCount')}：{r.last_sync_count}</Text>
            )}
            {hasError && (
              <Tooltip title={r.last_sync_error}>
                <Text type="danger" style={{ fontSize: 11 }} ellipsis>{r.last_sync_error}</Text>
              </Tooltip>
            )}
          </Space>
        )
      },
    },
    {
      title: t('common.action'),
      width: 130,
      render: (_: any, r: any) => (
        <Space size={4}>
          <Tooltip title={t('ipdb.subscriptionRefresh')}>
            <Button
              size="small"
              icon={<SyncOutlined spin={refreshingId === r.id} />}
              loading={refreshingId === r.id}
              onClick={() => handleRefresh(r.id)}
            />
          </Tooltip>
          <Tooltip title={t('common.edit')}>
            <Button size="small" icon={<EditOutlined />} onClick={() => {
              setEditRecord(r); editForm.setFieldsValue(r); setEditModalOpen(true)
            }} />
          </Tooltip>
          <Popconfirm title={t('common.deleteConfirm')} onConfirm={async () => {
            await ipdbApi.deleteSubscription(r.id)
            message.success(t('common.success'))
            fetchData()
          }}>
            <Tooltip title={t('common.delete')}>
              <Button size="small" danger icon={<DeleteOutlined />} />
            </Tooltip>
          </Popconfirm>
        </Space>
      ),
    },
  ]

  return (
    <>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 12 }}>
        <Text type="secondary" style={{ fontSize: 13 }}>
          {t('ipdb.subscriptionUrl')} · 支持每行多个 IP/CIDR（空格/逗号/分号分隔）
        </Text>
        <Space>
          <Button type="primary" icon={<PlusOutlined />} onClick={() => {
            setEditRecord(null); editForm.resetFields(); setEditModalOpen(true)
          }}>
            {t('ipdb.subscriptionAdd')}
          </Button>
          <Tooltip title={t('common.refresh')}>
            <Button icon={<ReloadOutlined />} onClick={fetchData} />
          </Tooltip>
        </Space>
      </div>

      <Table
        dataSource={data} columns={columns} rowKey="id" loading={loading} size="middle"
        style={{ background: '#fff', borderRadius: 8 }}
        pagination={false}
      />

      {/* 订阅编辑 Modal */}
      <Modal
        title={editRecord ? t('common.edit') : t('ipdb.subscriptionAdd')}
        open={editModalOpen} onOk={handleSubmit} onCancel={() => setEditModalOpen(false)}
        confirmLoading={submitting} width={520} destroyOnHidden
      >
        <Form form={editForm} layout="vertical" style={{ marginTop: 16 }}>
          <Form.Item name="name" label={t('ipdb.subscriptionName')} rules={[{ required: true, message: '请输入订阅名称' }]}>
            <Input placeholder="我的IP黑名单" />
          </Form.Item>
          <Form.Item
            name="url" label={t('ipdb.subscriptionUrl')}
            rules={[{ required: true, message: '请输入订阅地址' }, { type: 'url', message: '请输入有效的 URL' }]}
          >
            <Input placeholder="https://example.com/ip-list.txt" prefix={<LinkOutlined />} />
          </Form.Item>
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item name="location" label={t('ipdb.defaultLocation')} extra="可选，未在文件中指定时使用">
                <Input placeholder="中国" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="tags" label={t('ipdb.defaultTags')} extra="可选，逗号分隔">
                <Input placeholder="黑名单,恶意IP" />
              </Form.Item>
            </Col>
          </Row>
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item
                name="interval" label={t('ipdb.subscriptionInterval')}
                extra={t('ipdb.subscriptionIntervalTip')}
                initialValue={0}
              >
                <InputNumber min={0} max={8760} style={{ width: '100%' }} addonAfter="小时" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="clear_first" label={t('ipdb.clearFirst')} valuePropName="checked" extra={t('ipdb.clearFirstTip')}>
                <Switch />
              </Form.Item>
            </Col>
          </Row>
          <Form.Item name="enable" label={t('common.enable')} valuePropName="checked" initialValue={true}>
            <Switch />
          </Form.Item>
          <Form.Item name="remark" label={t('common.remark')}>
            <TextArea rows={2} />
          </Form.Item>
        </Form>
      </Modal>
    </>
  )
}

// ===== 主页面 =====
const IpDb: React.FC = () => {
  const { t } = useTranslation()

  const tabItems = [
    {
      key: 'entries',
      label: <Space><DatabaseOutlined />{t('ipdb.title')}</Space>,
      children: <EntryTab />,
    },
    {
      key: 'subscriptions',
      label: <Space><SyncOutlined />{t('ipdb.subscription')}</Space>,
      children: <SubscriptionTab />,
    },
  ]

  return (
    <div>
      <div style={{ marginBottom: 16 }}>
        <Typography.Title level={4} style={{ margin: 0 }}>
          <DatabaseOutlined style={{ marginRight: 8, color: '#1677ff' }} />
          {t('ipdb.title')}
        </Typography.Title>
      </div>
      <Tabs items={tabItems} defaultActiveKey="entries" />
    </div>
  )
}

export default IpDb
