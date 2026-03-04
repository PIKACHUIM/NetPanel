import React, { useEffect, useState } from 'react'
import {
  Table, Button, Space, Modal, Form, Input, Select,
  Popconfirm, message, Typography, Tag, Tooltip, Alert, Radio, Row, Col,
} from 'antd'
import {
  PlusOutlined, EditOutlined, DeleteOutlined, SafetyCertificateOutlined,
  GlobalOutlined,
} from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { certAccountApi } from '../api'

const { Option } = Select
const { Text } = Typography

const CA_TYPES = [
  { value: 'letsencrypt', label: "Let's Encrypt", color: 'green', desc: '免费、自动化、开放的证书颁发机构，最广泛使用' },
  { value: 'zerossl', label: 'ZeroSSL', color: 'blue', desc: '免费SSL证书，需要EAB凭据，支持更多功能' },
  { value: 'buypass', label: 'Buypass', color: 'purple', desc: '挪威CA机构，提供免费90天证书' },
  { value: 'google', label: 'Google Trust Services', color: 'red', desc: 'Google公共CA服务，需要EAB凭据绑定账户' },
]

// 代理模式选项
const PROXY_OPTIONS = [
  { value: 'none', label: '否' },
  { value: 'proxy', label: '是' },
  { value: 'reverse', label: '是（反向代理）' },
]

// EAB 获取方式提示
const EAB_AUTO_TIPS: Record<string, string> = {
  zerossl: '系统将自动调用 ZeroSSL API 申请 EAB 凭据，仅需填写注册邮箱即可。',
  google: '系统将自动调用 Google Public CA API 申请 EAB 凭据，需填写 EAB 申请邮箱。',
}
const EAB_MANUAL_TIPS: Record<string, string> = {
  zerossl: '请登录 ZeroSSL 控制台 → Developer → EAB Credentials 手动获取 Key ID 和 HMAC Key。',
  google: '请在 Google Cloud Console → Certificate Manager → External Account Keys 手动获取 Key ID 和 MAC Key。',
}

const CertAccount: React.FC = () => {
  const { t } = useTranslation()
  const [data, setData] = useState<any[]>([])
  const [loading, setLoading] = useState(false)
  const [modalOpen, setModalOpen] = useState(false)
  const [editRecord, setEditRecord] = useState<any>(null)
  const [verifyingId, setVerifyingId] = useState<number | null>(null)
  const [form] = Form.useForm()

  const fetchData = async () => {
    setLoading(true)
    try {
      const res: any = await certAccountApi.list()
      setData(res.data || [])
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { fetchData() }, [])

  const handleOpen = (record?: any) => {
    if (record) {
      setEditRecord(record)
      form.setFieldsValue({
        ...record,
        eab_mode: record.eab_mode || 'auto',
        env: record.env || 'production',
        use_proxy: record.use_proxy || 'none',
      })
    } else {
      setEditRecord(null)
      form.resetFields()
      form.setFieldsValue({
        type: 'letsencrypt',
        eab_mode: 'auto',
        env: 'production',
        use_proxy: 'none',
      })
    }
    setModalOpen(true)
  }

  const handleSubmit = async () => {
    const values = await form.validateFields()
    if (editRecord) {
      await certAccountApi.update(editRecord.id, values)
    } else {
      await certAccountApi.create(values)
    }
    message.success(t('common.success'))
    setModalOpen(false)
    fetchData()
  }

  const handleVerify = async (id: number) => {
    setVerifyingId(id)
    try {
      await certAccountApi.verify(id)
      message.success(t('certAccount.verifySuccess'))
    } finally {
      setVerifyingId(null)
    }
  }

  const columns = [
    {
      title: t('common.name'),
      dataIndex: 'name',
      render: (name: string, r: any) => (
        <div>
          <Text strong>{name}</Text>
          {r.remark && <div><Text type="secondary" style={{ fontSize: 12 }}>{r.remark}</Text></div>}
        </div>
      ),
    },
    {
      title: t('certAccount.type'),
      dataIndex: 'type',
      render: (v: string) => {
        const ca = CA_TYPES.find(c => c.value === v)
        return <Tag color={ca?.color}>{ca?.label || v}</Tag>
      },
    },
    {
      title: t('certAccount.email'),
      dataIndex: 'email',
      render: (v: string) => v ? <Text code style={{ fontSize: 12 }}>{v}</Text> : <Text type="secondary">-</Text>,
    },
    {
      title: t('certAccount.env'),
      dataIndex: 'env',
      render: (v: string) => v === 'staging'
        ? <Tag color="orange">测试环境</Tag>
        : <Tag color="green">正式环境</Tag>,
    },
    {
      title: t('certAccount.useProxy'),
      dataIndex: 'use_proxy',
      render: (v: string) => {
        const opt = PROXY_OPTIONS.find(o => o.value === v)
        return opt?.value === 'none'
          ? <Text type="secondary">否</Text>
          : <Tag color="blue" icon={<GlobalOutlined />}>{opt?.label || v}</Tag>
      },
    },
    {
      title: t('common.action'),
      width: 160,
      render: (_: any, r: any) => (
        <Space size={4}>
          <Tooltip title={t('certAccount.verify')}>
            <Button
              size="small"
              icon={<SafetyCertificateOutlined />}
              loading={verifyingId === r.id}
              onClick={() => handleVerify(r.id)}
            />
          </Tooltip>
          <Tooltip title={t('common.edit')}>
            <Button size="small" icon={<EditOutlined />} onClick={() => handleOpen(r)} />
          </Tooltip>
          <Popconfirm
            title={t('common.deleteConfirm')}
            onConfirm={async () => { await certAccountApi.delete(r.id); fetchData() }}
          >
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
        <Typography.Title level={4} style={{ margin: 0 }}>{t('certAccount.title')}</Typography.Title>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => handleOpen()}>
          {t('common.create')}
        </Button>
      </div>

      <Table
        dataSource={data}
        columns={columns}
        rowKey="id"
        loading={loading}
        size="middle"
        style={{ background: '#fff', borderRadius: 8 }}
        pagination={{ pageSize: 20, showSizeChanger: true }}
      />

      <Modal
        title={editRecord ? t('common.edit') : t('common.create')}
        open={modalOpen}
        onOk={handleSubmit}
        onCancel={() => setModalOpen(false)}
        width={560}
        destroyOnHidden
      >
        <Form form={form} layout="vertical" style={{ marginTop: 16 }}>
          {/* 账号名称 */}
          <Form.Item name="name" label={t('common.name')} rules={[{ required: true }]}>
            <Input placeholder="账号名称，如：我的 Let's Encrypt 账号" />
          </Form.Item>

          {/* CA 类型 */}
          <Form.Item name="type" label={t('certAccount.type')} rules={[{ required: true }]}>
            <Select placeholder="选择 CA 类型">
              {CA_TYPES.map(ca => (
                <Option key={ca.value} value={ca.value}>
                  <Space>
                    <Tag color={ca.color} style={{ margin: 0 }}>{ca.label}</Tag>
                    <Text type="secondary" style={{ fontSize: 12 }}>{ca.desc}</Text>
                  </Space>
                </Option>
              ))}
            </Select>
          </Form.Item>

          {/* 根据 CA 类型动态展示不同字段 */}
          <Form.Item noStyle shouldUpdate={(prev, cur) => prev.type !== cur.type || prev.eab_mode !== cur.eab_mode}>
            {({ getFieldValue }) => {
              const caType: string = getFieldValue('type')
              const eabMode: string = getFieldValue('eab_mode')
              const isGoogle = caType === 'google'
              const isZeroSSL = caType === 'zerossl'
              const needEab = isGoogle || isZeroSSL

              return (
                <>
                  {/* 邮箱地址 */}
                  <Form.Item
                    name="email"
                    label={isGoogle ? t('certAccount.eabEmail') : t('certAccount.email')}
                    rules={[{ required: true, type: 'email' }]}
                    extra={isGoogle ? 'EAB 申请时使用的邮箱地址' : '用于接收证书到期提醒邮件'}
                  >
                    <Input placeholder={isGoogle ? 'EAB 申请邮箱' : 'ACME 注册邮箱'} />
                  </Form.Item>

                  {/* EAB 凭据区域（ZeroSSL / Google） */}
                  {needEab && (
                    <>
                      {/* EAB 获取方式 */}
                      <Form.Item
                        name="eab_mode"
                        label={t('certAccount.eabMode')}
                      >
                        <Radio.Group>
                          <Radio value="auto">自动获取</Radio>
                          <Radio value="manual">手动输入</Radio>
                        </Radio.Group>
                      </Form.Item>

                      {/* 提示信息 */}
                      <Alert
                        type="info"
                        showIcon
                        message={eabMode === 'manual'
                          ? (EAB_MANUAL_TIPS[caType] || '')
                          : (EAB_AUTO_TIPS[caType] || '')}
                        style={{ marginBottom: 16 }}
                      />

                      {/* 手动输入时展示 Key 字段 */}
                      {eabMode === 'manual' && (
                        <>
                          <Form.Item
                            name="eab_kid"
                            label={isGoogle ? 'keyId' : t('certAccount.eabKid')}
                            rules={[{ required: true, message: '请输入 Key ID' }]}
                            extra={isGoogle ? 'Google EAB Key ID' : 'EAB Key ID（外部账户绑定标识符）'}
                          >
                            <Input placeholder="Key ID" />
                          </Form.Item>
                          <Form.Item
                            name="eab_hmac_key"
                            label={isGoogle ? 'b64MacKey' : t('certAccount.eabHmacKey')}
                            rules={[{ required: true, message: '请输入 MAC Key' }]}
                            extra={isGoogle ? 'Base64 编码的 MAC Key' : 'EAB HMAC Key（Base64 编码）'}
                          >
                            <Input.Password placeholder={isGoogle ? 'b64MacKey（Base64）' : 'EAB HMAC Key（Base64）'} />
                          </Form.Item>
                        </>
                      )}
                    </>
                  )}

                  {/* 非 Let's Encrypt / Buypass 时不展示邮箱（已在上方展示） */}
                  {!needEab && null}
                </>
              )
            }}
          </Form.Item>

          {/* 环境选择（正式/测试） */}
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item name="env" label={t('certAccount.env')}>
                <Select>
                  <Option value="production">正式环境</Option>
                  <Option value="staging">测试环境</Option>
                </Select>
              </Form.Item>
            </Col>
            <Col span={12}>
              {/* 使用代理服务器 */}
              <Form.Item name="use_proxy" label={t('certAccount.useProxy')}>
                <Select>
                  {PROXY_OPTIONS.map(o => (
                    <Option key={o.value} value={o.value}>{o.label}</Option>
                  ))}
                </Select>
              </Form.Item>
            </Col>
          </Row>

          {/* 备注 */}
          <Form.Item name="remark" label={t('common.remark')}>
            <Input.TextArea rows={2} placeholder="备注（可选）" />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

export default CertAccount
