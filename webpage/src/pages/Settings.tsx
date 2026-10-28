import React, { useState } from 'react'
import {
  Card, Form, Input, Button, Select, Divider, message,
  Typography, Row, Col, Space, Tag, Alert,
} from 'antd'
import {
  LockOutlined, GlobalOutlined, InfoCircleOutlined,
  CheckCircleOutlined,
} from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router-dom'
import { useAppStore } from '../store/appStore'
import { systemApi } from '../api'
import i18n from '../i18n'
import { useTableStyle } from '../hooks/useTableStyle'

const { Option } = Select
const { Title, Text } = Typography

const Settings: React.FC = () => {
  const { t } = useTranslation()
  const tableStyle = useTableStyle()
  const { language, setLanguage, logout } = useAppStore()
  const navigate = useNavigate()
  const [pwdForm] = Form.useForm()
  const [loading, setLoading] = useState(false)
  const [pwdSuccess, setPwdSuccess] = useState(false)
  // IPv6 状态卡片
  const [ipv6, setIpv6] = useState<{ has_global: boolean; public_reachable: boolean; global_addresses: string[] } | null>(null)
  const [ipv6Loaded, setIpv6Loaded] = useState(false)

  // 读取 IPv6 状态
  const loadIPv6 = async () => {
    try {
      const res = await systemApi.getIPv6Info()
      setIpv6(res?.data ?? null)
    } catch {
      // IPv6 检测失败不阻塞页面
    }
  }
  if (!ipv6Loaded) {
    setIpv6Loaded(true)
    loadIPv6()
  }

  const handleChangePassword = async () => {
    const values = await pwdForm.validateFields()
    if (values.new_password !== values.confirm_password) {
      message.error(t('settings.passwordMismatch'))
      return
    }
    setLoading(true)
    try {
      await systemApi.changePassword({
        old_password: values.old_password,
        new_password: values.new_password,
      })
      message.success(t('settings.passwordChangedToast'))
      pwdForm.resetFields()
      setPwdSuccess(true)
      // 延迟 1.5 秒后退出登录并跳转到登录页
      setTimeout(() => {
        logout()
        navigate('/login')
      }, 1500)
    } finally {
      setLoading(false)
    }
  }

  const handleLanguageChange = (lang: 'zh' | 'en') => {
    setLanguage(lang)
    i18n.changeLanguage(lang)
    message.success(t('settings.languageChanged'))
  }

  return (
    <div>
      <Title level={4} style={{ marginBottom: 20 }}>{t('settings.title')}</Title>

      <Row gutter={[16, 16]}>
        {/* 修改密码 */}
        <Col xs={24} lg={12}>
          <Card
            title={
              <Space>
                <LockOutlined style={{ color: '#0071e3' }} />
                {t('settings.changePassword')}
              </Space>
            }
            style={{ borderRadius: 8 }}
          >
            {pwdSuccess && (
              <Alert
                message={t('settings.passwordChanged')}
                type="success"
                showIcon
                icon={<CheckCircleOutlined />}
                style={{ marginBottom: 16 }}
              />
            )}
            <Form form={pwdForm} layout="vertical">
              <Form.Item
                name="old_password"
                label={t('settings.oldPassword')}
                rules={[{ required: true, message: t('settings.enterField', { field: t('settings.oldPassword') }) }] }
              >
                <Input.Password placeholder={t('settings.oldPassword')} />
              </Form.Item>
              <Form.Item
                name="new_password"
                label={t('settings.newPassword')}
                rules={[
                  { required: true, message: t('settings.enterField', { field: t('settings.newPassword') }) },
                  { min: 6, message: t('settings.passwordMinLength') },
                ]}
              >
                <Input.Password placeholder={t('settings.newPassword')} />
              </Form.Item>
              <Form.Item
                name="confirm_password"
                label={t('settings.confirmPassword')}
                rules={[{ required: true, message: t('settings.confirmField', { field: t('settings.newPassword') }) }] }
              >
                <Input.Password placeholder={t('settings.confirmPassword')} />
              </Form.Item>
              <Button type="primary" loading={loading} onClick={handleChangePassword} icon={<LockOutlined />}>
                {t('settings.changePassword')}
              </Button>
            </Form>
          </Card>
        </Col>

        {/* 界面设置 */}
        <Col xs={24} lg={12}>
          <Card
            title={
              <Space>
                <GlobalOutlined style={{ color: '#0071e3' }} />
                {t('settings.interfaceSettings')}
              </Space>
            }
            style={{ borderRadius: 8 }}
          >
            <div style={{ marginBottom: 24 }}>
              <Text strong style={{ display: 'block', marginBottom: 8 }}>
                {t('settings.language')}
              </Text>
              <Select
                value={language}
                onChange={handleLanguageChange}
                style={{ width: 200 }}
              >
                <Option value="zh">🇨🇳 中文</Option>
                <Option value="en">🇺🇸 English</Option>
              </Select>
            </div>

            <Divider />

              <div>
                <Text strong style={{ display: 'block', marginBottom: 8 }}>
                  <InfoCircleOutlined style={{ marginRight: 6, color: '#0071e3' }} />
                  {t('settings.about')}
                </Text>
              <div style={{ lineHeight: 2 }}>
                <div>
                  <Text type="secondary">{t('settings.version')}: </Text>
                  <Tag color="blue">NetPanel v1.0.0</Tag>
                </div>
                <div>
                  <Text type="secondary">{t('settings.techStack')}: </Text>
                  <Text>Go + React + Ant Design</Text>
                </div>
                <div>
                  <Text type="secondary">{t('settings.database')}: </Text>
                  <Text>SQLite</Text>
                </div>
              </div>
            </div>
          </Card>
        </Col>
        {/* IPv6 状态 */}
        <Col xs={24} lg={12}>
          <Card
            title={
              <Space>
                <GlobalOutlined style={{ color: '#0071e3' }} />
                {t('settings.ipv6.title')}
              </Space>
            }
            style={{ borderRadius: 8 }}
          >
            {!ipv6 ? (
              <Text type="secondary">{t('settings.ipv6.checking')}</Text>
            ) : (
              <div style={{ lineHeight: 2 }}>
                <div>
                  <Text type="secondary" style={{ marginRight: 8 }}>{t('settings.ipv6.globalAddr')}</Text>
                  {ipv6.has_global ? (
                    <Tag color="green">{t('settings.ipv6.yes')}</Tag>
                  ) : (
                    <Tag color="default">{t('settings.ipv6.none')}</Tag>
                  )}
                </div>
                {ipv6.has_global && (
                  <div>
                    <Text type="secondary">{t('settings.ipv6.reachable')}</Text>
                    {ipv6.public_reachable ? (
                      <Tag color="green">{t('settings.ipv6.reachableOk')}</Tag>
                    ) : (
                      <Tag color="orange">{t('settings.ipv6.reachableFail')}</Tag>
                    )}
                  </div>
                )}
                {ipv6.has_global && (
                  <div>
                    <Text type="secondary">{t('settings.ipv6.addresses')}</Text>
                    {ipv6.global_addresses.map((a) => (
                      <Tag key={a} color="blue" style={{ fontFamily: 'monospace' }}>{a}</Tag>
                    ))}
                  </div>
                )}
                {!ipv6.has_global && (
                  <Text type="secondary">{t('settings.ipv6.noGlobalTip')}</Text>
                )}
              </div>
            )}
          </Card>
        </Col>
      </Row>
    </div>
  )
}

export default Settings
