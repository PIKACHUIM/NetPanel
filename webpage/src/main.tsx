import React, { useEffect } from 'react'
import ReactDOM from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import { ConfigProvider, theme as antTheme } from 'antd'
import zhCN from 'antd/locale/zh_CN'
import enUS from 'antd/locale/en_US'
import App from './App'
import './i18n'
import './index.css'
import { useAppStore } from './store/appStore'

const Root: React.FC = () => {
  const { language, theme } = useAppStore()
  const locale = language === 'zh' ? zhCN : enUS
  const isDark = theme === 'dark' || theme === 'glass-dark'
  const isGlass = theme === 'glass-light' || theme === 'glass-dark'

  // 同步主题到 data-theme 属性，供 CSS 变量使用
  useEffect(() => {
    document.documentElement.setAttribute('data-theme', theme)
  }, [theme])

  return (
    <ConfigProvider
      locale={locale}
      theme={{
        algorithm: isDark || isGlass ? antTheme.darkAlgorithm : antTheme.defaultAlgorithm,
        token: {
          colorPrimary: '#1677ff',
          borderRadius: 8,
          borderRadiusLG: 12,
          borderRadiusSM: 6,
          fontFamily: "'DM Sans', -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif",
          // 暗黑/玻璃模式下的颜色调整
          ...(isDark ? {
            colorBgContainer: '#1a1a1a',
            colorBgElevated: '#242424',
            colorBgLayout: '#0d0d0d',
            colorBorder: 'rgba(255,255,255,0.1)',
            colorBorderSecondary: 'rgba(255,255,255,0.06)',
          } : {}),
          ...(isGlass ? {
            colorBgContainer: 'rgba(255,255,255,0.06)',
            colorBgElevated: 'rgba(255,255,255,0.1)',
            colorBgLayout: 'transparent',
            colorBorder: 'rgba(255,255,255,0.12)',
            colorBorderSecondary: 'rgba(255,255,255,0.08)',
            colorText: 'rgba(255,255,255,0.88)',
            colorTextSecondary: 'rgba(255,255,255,0.5)',
            colorTextTertiary: 'rgba(255,255,255,0.3)',
          } : {}),
        },
        components: {
          Layout: {
            siderBg: isDark ? '#141414' : isGlass ? 'rgba(10,20,50,0.75)' : '#001529',
            triggerBg: isDark ? '#1f1f1f' : isGlass ? 'rgba(10,20,50,0.85)' : '#002140',
            headerBg: isDark ? '#1a1a1a' : isGlass ? 'rgba(255,255,255,0.06)' : '#ffffff',
          },
          Menu: {
            darkItemBg: isDark ? '#141414' : isGlass ? 'transparent' : '#001529',
            darkSubMenuItemBg: isDark ? '#1a1a1a' : isGlass ? 'rgba(0,0,0,0.2)' : '#000c17',
            darkItemSelectedBg: isDark ? '#1677ff20' : isGlass ? 'rgba(22,119,255,0.2)' : '#1677ff',
          },
          Card: {
            borderRadiusLG: 12,
            paddingLG: 20,
          },
          Modal: {
            borderRadiusLG: 16,
          },
          Table: {
            borderRadius: 10,
            headerBg: isDark ? '#1f1f1f' : isGlass ? 'rgba(255,255,255,0.05)' : '#fafafa',
            footerBg: isDark ? '#1a1a1a' : isGlass ? 'transparent' : '#fafafa',
          },
          Pagination: {
            itemBg: 'transparent',
            itemActiveBg: isDark || isGlass ? 'rgba(22,119,255,0.2)' : undefined,
          },
          Button: {
            borderRadius: 8,
            controlHeight: 34,
          },
          Input: {
            borderRadius: 8,
            controlHeight: 34,
          },
          Select: {
            borderRadius: 8,
            controlHeight: 34,
          },
          InputNumber: {
            borderRadius: 8,
            controlHeight: 34,
          },
        },
      }}
    >
      <BrowserRouter>
        <App />
      </BrowserRouter>
    </ConfigProvider>
  )
}

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <Root />
  </React.StrictMode>
)
