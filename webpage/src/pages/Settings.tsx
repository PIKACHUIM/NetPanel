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
      message.success('密码修改成功，请重新登录')
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
                <LockOutlined style={{ color: '#1677ff' }} />
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
                rules={[{ required: true, message: `请输入${t('settings.oldPassword')}` }]}
              >
                <Input.Password placeholder={t('settings.oldPassword')} />
              </Form.Item>
              <Form.Item
                name="new_password"
                label={t('settings.newPassword')}
                rules={[
                  { required: true, message: `请输入${t('settings.newPassword')}` },
                  { min: 6, message: '密码至少6位' },
                ]}
              >
                <Input.Password placeholder={t('settings.newPassword')} />
              </Form.Item>
              <Form.Item
                name="confirm_password"
                label={t('settings.confirmPassword')}
                rules={[{ required: true, message: `请确认${t('settings.newPassword')}` }]}
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
                <GlobalOutlined style={{ color: '#1677ff' }} />
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
                <InfoCircleOutlined style={{ marginRight: 6, color: '#1677ff' }} />
                {t('settings.about')}
              </Text>
              <div style={{ lineHeight: 2 }}>
                <div>
                  <Text type="secondary">版本：</Text>
                  <Tag color="blue">NetPanel v1.0.0</Tag>
                </div>
                <div>
                  <Text type="secondary">技术栈：</Text>
                  <Text>Go + React + Ant Design</Text>
                </div>
                <div>
                  <Text type="secondary">数据库：</Text>
                  <Text>SQLite</Text>
                </div>
              </div>
            </div>
          </Card>
        </Col>
      </Row>
    </div>
  )
}

export default Settings
