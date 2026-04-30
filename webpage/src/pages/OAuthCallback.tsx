import React, { useEffect } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { Spin, message } from 'antd'
import { useAppStore } from '../store/appStore'
import { useTranslation } from 'react-i18next'

const OAuthCallback: React.FC = () => {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const { setToken, setUsername } = useAppStore()

  useEffect(() => {
    const token = searchParams.get('token')
    const username = searchParams.get('username')

    if (token && username) {
      setToken(token)
      setUsername(username)
      message.success(t('login.loginSuccess'))
      navigate('/dashboard', { replace: true })
    } else {
      message.error(t('login.loginFailed') || '登录失败')
      navigate('/login', { replace: true })
    }
  }, [searchParams, setToken, setUsername, navigate, t])

  return (
    <div style={{
      minHeight: '100vh',
      background: 'linear-gradient(160deg, #030810 0%, #05101e 25%, #081830 50%, #0a1228 75%, #040a14 100%)',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
    }}>
      <Spin size="large" tip="正在登录..." />
    </div>
  )
}

export default OAuthCallback
