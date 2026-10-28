import React, { useEffect, useRef } from 'react'
import { useNavigate } from 'react-router-dom'
import { Spin, message } from 'antd'
import { useAppStore } from '../store/appStore'
import { authApi } from '../api'
import { useTranslation } from 'react-i18next'

const OAuthCallback: React.FC = () => {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { setToken, setUsername, setRoles } = useAppStore()
  const processed = useRef(false)

  useEffect(() => {
    if (processed.current) return
    processed.current = true
    const params = new URLSearchParams(window.location.hash.slice(1))
    const token = params.get('token')
    const username = params.get('username')
    const roles = params.get('roles') || ''
    const rawRedirect = params.get('redirect') || ''
    const safeRedirect = (() => {
      if (!rawRedirect || /^https?:\/\//i.test(rawRedirect)) return ''
      try {
        const target = new URL(rawRedirect, window.location.origin)
        if (!['http:', 'https:'].includes(target.protocol) || target.origin !== window.location.origin || target.username || target.password) return ''
        return `${target.pathname}${target.search}${target.hash}`
      } catch {
        return ''
      }
    })()
    const externalRedirect = (() => {
      if (!/^https?:\/\//i.test(rawRedirect)) return ''
      try {
        const target = new URL(rawRedirect)
        if (!['http:', 'https:'].includes(target.protocol) || target.origin === window.location.origin || target.username || target.password) return ''
        return target.href
      } catch {
        return ''
      }
    })()
    window.history.replaceState({}, document.title, `${window.location.pathname}${window.location.search}`)

    if (token && username) {
      setToken(token)
      setUsername(username)
      setRoles(roles)
      message.success(t('login.loginSuccess'))
      const finishLogin = (target: string) => {
        if (target) {
          window.location.assign(target)
          return
        }
        let hasSeenOnboarding = false
        try { hasSeenOnboarding = localStorage.getItem('netpanel-onboarded') === '1' } catch { /* ignore */ }
        navigate(hasSeenOnboarding ? '/dashboard' : '/onboarding', { replace: true })
      }
      if (safeRedirect) {
        finishLogin(safeRedirect)
      } else if (externalRedirect) {
        authApi.validateRedirect(externalRedirect)
          .then((res: any) => finishLogin(res?.data?.valid ? externalRedirect : ''))
          .catch(() => finishLogin(''))
      } else {
        finishLogin('')
      }
    } else {
      message.error(t('login.loginFailed') || '登录失败')
      navigate('/login', { replace: true })
    }
  }, [setToken, setUsername, setRoles, navigate, t])

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
