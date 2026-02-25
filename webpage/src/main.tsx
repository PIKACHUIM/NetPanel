import React from 'react'
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
  const isDark = theme === 'dark'

  return (
    <ConfigProvider
      locale={locale}
      theme={{
        algorithm: isDark ? antTheme.darkAlgorithm : antTheme.defaultAlgorithm,
        token: {
          colorPrimary: '#1677ff',
          borderRadius: 6,
          fontFamily: "'DM Sans', -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif",
        },
        components: {
          Layout: {
            siderBg: isDark ? '#141414' : '#001529',
            triggerBg: isDark ? '#1f1f1f' : '#002140',
          },
          Menu: {
            darkItemBg: isDark ? '#141414' : '#001529',
            darkSubMenuItemBg: isDark ? '#1a1a1a' : '#000c17',
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
