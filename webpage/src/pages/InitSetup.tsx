import React, { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Form, Input, Button, Typography, Steps, message, Spin } from 'antd'
import { LockOutlined, UserOutlined, SafetyOutlined, RocketOutlined } from '@ant-design/icons'
import { initApi } from '../api'
import { useAppStore } from '../store/appStore'

const { Title, Text } = Typography

// 首次初始化向导：无管理员时强制设置首个管理员密码
const InitSetup: React.FC = () => {
  const navigate = useNavigate()
  const setToken = useAppStore(s => s.setToken)
  const setUsername = useAppStore(s => s.setUsername)
  const [checking, setChecking] = useState(true)
  const [initialized, setInitialized] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [step, setStep] = useState(0)
  const [form] = Form.useForm()

  useEffect(() => {
    initApi.status()
      .then((res: any) => {
        const init = res?.data?.initialized !== false
        setInitialized(init)
        if (init) navigate('/login', { replace: true })
      })
      .catch(() => setInitialized(true))
      .finally(() => setChecking(false))
  }, [navigate])

  const handleFinish = async (values: { username: string; password: string }) => {
    setSubmitting(true)
    try {
      const res: any = await initApi.setup({
        username: values.username.trim(),
        password: values.password,
      })
      message.success(res?.message || '初始化完成')
      // 初始化成功后自动登录
      const loginRes: any = await fetch('/api/v1/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username: values.username.trim(), password: values.password }),
      }).then(r => r.json())
      if (loginRes?.data?.token) {
        setToken(loginRes.data.token)
        setUsername(loginRes.data.username)
      }
      navigate('/dashboard', { replace: true })
    } catch (e: any) {
      message.error(e?.message || '初始化失败')
    } finally {
      setSubmitting(false)
    }
  }

  if (checking) {
    return (
      <div style={{ minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
        <Spin size="large" />
      </div>
    )
  }
  if (initialized) return null

  return (
    <div style={{
      minHeight: '100vh',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      padding: 24,
      background: 'var(--bg-body)',
    }}>
      <div style={{
        width: 460,
        maxWidth: '100%',
        padding: 36,
        borderRadius: 'var(--radius-lg)',
        background: 'var(--bg-card)',
        boxShadow: 'var(--shadow-card)',
        border: '1px solid var(--border-color)',
      }}>
        <div style={{ textAlign: 'center', marginBottom: 24 }}>
          <div style={{
            width: 56, height: 56, borderRadius: 16, margin: '0 auto 16px',
            background: 'linear-gradient(135deg, #0071e3, #5e5ce6)',
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            fontSize: 26, color: '#fff', fontWeight: 700,
          }}>N</div>
          <Title level={3} style={{ marginBottom: 4 }}>欢迎使用 NetPanel</Title>
          <Text type="secondary">首次启动，请创建管理员账号并设置密码</Text>
        </div>

        <Steps
          size="small"
          current={step}
          items={[
            { title: '创建管理员' },
            { title: '安全加固' },
            { title: '完成' },
          ]}
          style={{ marginBottom: 28 }}
        />

        {step === 0 && (
          <Form form={form} layout="vertical" onFinish={handleFinish} requiredMark={false}>
            <Form.Item
              name="username"
              label="管理员用户名"
              rules={[
                { required: true, message: '请输入用户名' },
                { min: 2, max: 50, message: '长度 2-50 位' },
              ]}
            >
              <Input prefix={<UserOutlined />} placeholder="默认 admin" size="large" />
            </Form.Item>
            <Form.Item
              name="password"
              label="登录密码"
              rules={[
                { required: true, message: '请输入密码' },
                { min: 8, message: '密码至少 8 位' },
              ]}
              extra="建议使用字母、数字与符号组合"
            >
              <Input.Password prefix={<LockOutlined />} placeholder="至少 8 位" size="large" />
            </Form.Item>
            <Form.Item
              name="confirm"
              label="确认密码"
              dependencies={['password']}
              rules={[
                { required: true, message: '请再次输入密码' },
                ({ getFieldValue }) => ({
                  validator(_, value) {
                    if (!value || getFieldValue('password') === value) return Promise.resolve()
                    return Promise.reject(new Error('两次输入的密码不一致'))
                  },
                }),
              ]}
            >
              <Input.Password prefix={<LockOutlined />} placeholder="再次输入密码" size="large" />
            </Form.Item>
            <Button type="primary" htmlType="submit" size="large" block loading={submitting}
              onClick={() => setStep(1)}>
              下一步：安全加固 →
            </Button>
          </Form>
        )}

        {step === 1 && (
          <div>
            <div style={{ textAlign: 'center', padding: '12px 0 20px' }}>
              <SafetyOutlined style={{ fontSize: 40, color: '#0071e3', marginBottom: 12 }} />
              <Title level={5}>安全建议</Title>
              <Text type="secondary" style={{ display: 'block', lineHeight: 1.8 }}>
                建议登录后立即：<br />
                ① 在「安全中心」开启 WAF 防护并添加黑白名单<br />
                ② 通过「CF 隧道」安全暴露内网服务<br />
                ③ 定期备份数据目录
              </Text>
            </div>
            <Button type="primary" size="large" block loading={submitting} onClick={() => form.submit()}>
              完成初始化 <RocketOutlined />
            </Button>
          </div>
        )}
      </div>
    </div>
  )
}

export default InitSetup
