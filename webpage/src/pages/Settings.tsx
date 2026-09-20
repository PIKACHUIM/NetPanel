import React, { useState } from 'react'
import {
  Card, Form, Input, Button, Select, Divider, message, Switch,
  Typography, Row, Col, Space, Tag, Alert,
} from 'antd'
import {
  LockOutlined, GlobalOutlined, InfoCircleOutlined,
  CheckCircleOutlined, ThunderboltOutlined, DeleteOutlined,
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
  // 测速弹窗开关（SystemConfig: speedtest_popup_enabled，默认开启）
  const [speedtestEnabled, setSpeedtestEnabled] = useState(true)
  const [speedtestLoaded, setSpeedtestLoaded] = useState(false)
  // 数据保留天数（SystemConfig: retention_days，默认 30）
  const [retentionDays, setRetentionDays] = useState<number>(30)
  const [retentionLoaded, setRetentionLoaded] = useState(false)
  const [cleanupLoading, setCleanupLoading] = useState(false)
  // 立即清理前的预估删除条数（不实际删除，用于二次确认）
  const [estimated, setEstimated] = useState<number | null>(null)
  const [estimateLoading, setEstimateLoading] = useState(false)

  // 读取系统配置中的测速弹窗开关
  const loadSpeedtestSwitch = async () => {
    try {
      const res = await systemApi.getConfig()
      const val = res?.data?.speedtest_popup_enabled
      if (val !== undefined) {
        setSpeedtestEnabled(val === 'true' || val === '1')
      }
      // 数据保留天数（未配置时后端默认 30）
      const days = parseInt(res?.data?.retention_days, 10)
      if (!isNaN(days) && days > 0) setRetentionDays(days)
      // 预估当前会被清理的条数
      loadEstimate()
    } catch {
      // 读取失败保持默认开启
    }
  }
  if (!speedtestLoaded) {
    setSpeedtestLoaded(true)
    loadSpeedtestSwitch()
  }

  // 预估当前会被清理的过期数据总行数（不实际删除）
  const loadEstimate = async () => {
    try {
      const res: any = await systemApi.estimateRetention()
      const n = res?.data?.estimated
      setEstimated(typeof n === 'number' ? n : null)
    } catch {
      setEstimated(null)
    }
  }

  const handleSpeedtestSwitch = async (checked: boolean) => {
    setSpeedtestEnabled(checked)
    try {
      await systemApi.updateConfig({ speedtest_popup_enabled: checked ? 'true' : 'false' })
      message.success(t('settings.saved'))
    } catch {
      setSpeedtestEnabled(!checked)
      message.error(t('common.failed'))
    }
  }

  const handleRetentionDaysChange = async (days: number) => {
    setRetentionDays(days)
    try {
      await systemApi.updateConfig({ retention_days: String(days) })
      message.success(t('settings.saved'))
    } catch {
      message.error(t('common.failed'))
    }
  }

  const handleCleanup = async () => {
    // 二次确认：破坏性操作，先预估会删多少
    setEstimateLoading(true)
    try {
      const res: any = await systemApi.estimateRetention()
      const n = res?.data?.estimated
      setEstimated(typeof n === 'number' ? n : null)
    } catch {
      setEstimated(null)
    } finally {
      setEstimateLoading(false)
    }
    const countText = (estimated != null && estimated > 0)
      ? `预计删除 ${estimated} 条过期数据，确认执行吗？`
      : '确认执行数据清理吗？（此操作不可撤销）'
    if (!window.confirm(countText)) {
      return
    }
    setCleanupLoading(true)
    try {
      const res: any = await systemApi.cleanupRetention()
      message.success(`清理完成，共删除 ${res?.data?.deleted ?? 0} 条过期数据`)
      loadEstimate()
    } catch {
      message.error(t('common.failed'))
    } finally {
      setCleanupLoading(false)
    }
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
                <ThunderboltOutlined style={{ marginRight: 6, color: '#0071e3' }} />
                {t('settings.speedtestPopup')}
              </Text>
              <Space style={{ marginBottom: 4 }}>
                <Switch checked={speedtestEnabled} onChange={handleSpeedtestSwitch} />
                <Text type="secondary">{t('settings.speedtestPopupTip')}</Text>
              </Space>
              </div>

            <Divider />

            <div>
              <Text strong style={{ display: 'block', marginBottom: 8 }}>
                <DeleteOutlined style={{ marginRight: 6, color: '#0071e3' }} />
                数据保留
              </Text>
              <Space direction="vertical" size={8} style={{ width: '100%' }}>
                <Space>
                  <Text type="secondary">时序数据保留：</Text>
                  <Select
                    value={retentionDays}
                    onChange={handleRetentionDaysChange}
                    style={{ width: 130 }}
                  >
                    {[7, 14, 30, 60, 90, 180, 365].map(d => (
                      <Option key={d} value={d}>{d} 天</Option>
                    ))}
                  </Select>
                </Space>
                <Space style={{ marginBottom: 4 }}>
                  <Button size="small" loading={cleanupLoading} onClick={handleCleanup} icon={<DeleteOutlined />}>
                    立即清理
                  </Button>
                  <Text type="secondary" style={{ fontSize: 12 }}>
                    清理监控指标、探测结果、WAF/系统日志等过期数据
                  </Text>
                </Space>
              </Space>
            </div>

            <Divider />

            <div>
              <Text strong style={{ display: 'block', marginBottom: 8 }}>
                <InfoCircleOutlined style={{ marginRight: 6, color: '#0071e3' }} />
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
