import React, { useEffect, useState } from 'react'
import {
  Table, Button, Space, Switch, Modal, Form, Input, InputNumber,
  Select, Popconfirm, message, Typography, Tag, Tooltip, Divider, Row, Col,
} from 'antd'
import {
  PlusOutlined, EditOutlined, DeleteOutlined,
  PlayCircleOutlined, StopOutlined, LinkOutlined, SafetyCertificateOutlined,
} from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { caddyApi } from '../api'
import StatusTag from '../components/StatusTag'

const { Option } = Select
const { Text } = Typography

const SITE_TYPES = [
  { value: 'reverse_proxy', label: '反向代理', color: 'blue' },
  { value: 'static_file', label: '静态文件', color: 'green' },
  { value: 'redirect', label: '重定向', color: 'orange' },
  { value: 'rewrite', label: 'URL重写', color: 'purple' },
]

const Caddy: React.FC = () => {
  const { t } = useTranslation()
  const [data, setData] = useState<any[]>([])
  const [loading, setLoading] = useState(false)
  const [modalOpen, setModalOpen] = useState(false)
  const [editRecord, setEditRecord] = useState<any>(null)
  const [form] = Form.useForm()
  const [siteType, setSiteType] = useState('reverse_proxy')
  const [tlsEnable, setTlsEnable] = useState(false)

  const fetchData = async () => {
    setLoading(true)
    try { const res: any = await caddyApi.list(); setData(res.data || []) }
    finally { setLoading(false) }
  }
  useEffect(() => { fetchData() }, [])

  const handleOpen = (record?: any) => {
    if (record) {
      setEditRecord(record)
      setSiteType(record.site_type || 'reverse_proxy')
      setTlsEnable(record.tls_enable || false)
      form.setFieldsValue(record)
    } else {
      setEditRecord(null)
      setSiteType('reverse_proxy')
      setTlsEnable(false)
      form.resetFields()
      form.setFieldsValue({
        enable: true, site_type: 'reverse_proxy',
        port: 80, redirect_code: 301, tls_enable: false,
      })
    }
    setModalOpen(true)
  }

  const handleSubmit = async () => {
    const values = await form.validateFields()
    if (editRecord) {
      await caddyApi.update(editRecord.id, values)
    } else {
      await caddyApi.create(values)
    }
    message.success(t('common.success'))
    setModalOpen(false)
    fetchData()
  }

  const handleToggle = async (record: any, checked: boolean) => {
    await caddyApi.update(record.id, { ...record, enable: checked })
    if (checked) await caddyApi.start(record.id)
    else await caddyApi.stop(record.id)
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
      title: '监听地址',
      render: (_: any, r: any) => (
        <Space>
          {r.tls_enable
            ? <Tag icon={<SafetyCertificateOutlined />} color="green">https</Tag>
            : <Tag icon={<LinkOutlined />}>http</Tag>
          }
          <Text code>{r.domain || '*'}:{r.port || 80}</Text>
        </Space>
      ),
    },
    {
      title: t('caddy.siteType'), dataIndex: 'site_type', width: 100,
      render: (v: string) => {
        const st = SITE_TYPES.find(s => s.value === v)
        return <Tag color={st?.color}>{st?.label || v}</Tag>
      },
    },
    {
      title: '目标/路径',
      render: (_: any, r: any) => {
        if (r.site_type === 'reverse_proxy') return <Text code>{r.upstream_addr || '-'}</Text>
        if (r.site_type === 'static_file') return <Text code>{r.root_path || '-'}</Text>
        if (r.site_type === 'redirect') return <Text type="secondary">{r.redirect_to || '-'}</Text>
        return '-'
      },
    },
    {
      title: t('common.action'), width: 140,
      render: (_: any, r: any) => (
        <Space size={4}>
          {r.status === 'running'
            ? <Tooltip title={t('common.stop')}><Button size="small" icon={<StopOutlined />} onClick={async () => { await caddyApi.stop(r.id); fetchData() }} /></Tooltip>
            : <Tooltip title={t('common.start')}><Button size="small" type="primary" icon={<PlayCircleOutlined />} onClick={async () => { await caddyApi.start(r.id); fetchData() }} /></Tooltip>
          }
          <Tooltip title={t('common.edit')}>
            <Button size="small" icon={<EditOutlined />} onClick={() => handleOpen(r)} />
          </Tooltip>
          <Popconfirm title={t('common.deleteConfirm')} onConfirm={async () => { await caddyApi.delete(r.id); fetchData() }}>
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
        <Typography.Title level={4} style={{ margin: 0 }}>{t('caddy.title')}</Typography.Title>
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
        width={600} destroyOnHidden
      >
        <Form form={form} layout="vertical" style={{ marginTop: 16 }}>
          <Row gutter={16}>
            <Col span={16}>
              <Form.Item name="name" label={t('common.name')} rules={[{ required: true }]}>
                <Input placeholder="站点名称" style={{ width: '100%' }} />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="enable" label={t('common.enable')} valuePropName="checked">
                <Switch />
              </Form.Item>
            </Col>
          </Row>

          <Divider orientation="left" plain style={{ fontSize: 13 }}>监听设置</Divider>

          <Row gutter={16}>
            <Col span={16}>
              <Form.Item name="domain" label="域名 / 主机名"
                extra="留空监听所有域名，可填 :8080 仅监听端口">
                <Input placeholder="example.com 或留空" style={{ width: '100%' }} />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="port" label="端口" rules={[{ required: true }]}>
                <InputNumber min={1} max={65535} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
          </Row>

          <Divider orientation="left" plain style={{ fontSize: 13 }}>站点类型</Divider>

          <Form.Item name="site_type" label={t('caddy.siteType')} rules={[{ required: true }]}>
            <Select onChange={(v) => setSiteType(v)}>
              {SITE_TYPES.map(s => <Option key={s.value} value={s.value}>{s.label}</Option>)}
            </Select>
          </Form.Item>

          {siteType === 'reverse_proxy' && (
            <Form.Item name="upstream_addr" label={t('caddy.upstreamAddr')} rules={[{ required: true }]}
              extra="支持多个上游，用空格分隔，如：http://127.0.0.1:3000 http://127.0.0.1:3001">
              <Input placeholder="http://127.0.0.1:3000" style={{ width: '100%' }} />
            </Form.Item>
          )}

          {siteType === 'static_file' && (
            <>
              <Form.Item name="root_path" label={t('caddy.rootPath')} rules={[{ required: true }]}>
                <Input placeholder="/var/www/html" style={{ width: '100%' }} />
              </Form.Item>
              <Form.Item name="file_list" label={t('caddy.fileList')} valuePropName="checked">
                <Switch checkedChildren="开启" unCheckedChildren="关闭" />
              </Form.Item>
            </>
          )}

          {siteType === 'redirect' && (
            <Row gutter={16}>
              <Col span={16}>
                <Form.Item name="redirect_to" label={t('caddy.redirectTo')} rules={[{ required: true }]}>
                  <Input placeholder="https://new.example.com{uri}" style={{ width: '100%' }} />
                </Form.Item>
              </Col>
              <Col span={8}>
                <Form.Item name="redirect_code" label={t('caddy.redirectCode')}>
                  <Select>
                    <Option value={301}>301 永久</Option>
                    <Option value={302}>302 临时</Option>
                    <Option value={307}>307 临时</Option>
                    <Option value={308}>308 永久</Option>
                  </Select>
                </Form.Item>
              </Col>
            </Row>
          )}

          {siteType === 'rewrite' && (
            <Form.Item name="rewrite_to" label="重写目标" rules={[{ required: true }]}
              extra="支持占位符，如：/new{path}">
              <Input placeholder="/new{path}" style={{ width: '100%' }} />
            </Form.Item>
          )}

          <Divider orientation="left" plain style={{ fontSize: 13 }}>SSL / TLS</Divider>

          <Form.Item name="tls_enable" label={t('caddy.tlsEnable')} valuePropName="checked">
            <Switch onChange={setTlsEnable} checkedChildren="启用" unCheckedChildren="关闭" />
          </Form.Item>

          {tlsEnable && (
            <>
              <Form.Item name="tls_mode" label={t('caddy.tlsMode')}>
                <Select>
                  <Option value="auto">自动 (ACME)</Option>
                  <Option value="manual">手动指定证书</Option>
                </Select>
              </Form.Item>
              <Form.Item
                noStyle
                shouldUpdate={(prev, cur) => prev.tls_mode !== cur.tls_mode}
              >
                {({ getFieldValue }) => getFieldValue('tls_mode') === 'manual' && (
                  <Row gutter={16}>
                    <Col span={12}>
                      <Form.Item name="tls_cert_file" label={t('caddy.certFile')}>
                        <Input placeholder="/path/to/cert.pem" style={{ width: '100%' }} />
                      </Form.Item>
                    </Col>
                    <Col span={12}>
                      <Form.Item name="tls_key_file" label={t('caddy.keyFile')}>
                        <Input placeholder="/path/to/key.pem" style={{ width: '100%' }} />
                      </Form.Item>
                    </Col>
                  </Row>
                )}
              </Form.Item>
            </>
          )}

          <Divider orientation="left" plain style={{ fontSize: 13 }}>高级</Divider>

          <Form.Item name="extra_config" label="额外 Caddyfile 配置"
            extra="将直接追加到站点块内，请确保语法正确">
            <Input.TextArea rows={3} placeholder="header / * {\n  X-Frame-Options DENY\n}" style={{ fontFamily: 'monospace', fontSize: 12 }} />
          </Form.Item>

          <Form.Item name="remark" label={t('common.remark')}>
            <Input.TextArea rows={2} style={{ width: '100%' }} />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

export default Caddy
