import React, { useLayoutEffect } from 'react'
import ReactDOM from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import { ConfigProvider, theme as antTheme } from 'antd'
import zhCN from 'antd/locale/zh_CN'
import enUS from 'antd/locale/en_US'
import App from './App'
import './i18n'
import './index.css'
import { useAppStore, hasWallpaper, getWallpaperColor } from './store/appStore'

const Root: React.FC = () => {
  const { language, uiMode, wallpaper } = useAppStore()
  const locale = language === 'zh' ? zhCN : enUS
  const isDark = uiMode === 'dark'
  const hasWp = hasWallpaper(wallpaper)
  const primaryColor = getWallpaperColor(wallpaper)

  // 同步主题属性到 HTML，供 CSS 使用
  // data-theme: 用于匹配暗色/亮色 CSS 选择器
  // data-wallpaper: 用于匹配壁纸毛玻璃 CSS
  useLayoutEffect(() => {
    document.documentElement.setAttribute('data-theme', uiMode)
    document.documentElement.setAttribute('data-wallpaper', wallpaper)
  }, [uiMode, wallpaper, hasWp])

  return (
    <ConfigProvider
      locale={locale}
      theme={{
        algorithm: isDark ? antTheme.darkAlgorithm : antTheme.defaultAlgorithm,
        token: {
          colorPrimary: primaryColor,
          borderRadius: 8,
          borderRadiusLG: 12,
          borderRadiusSM: 6,
          fontFamily: "'MapleMono', monospace",
          // 暗黑模式下的颜色调整
          ...(isDark ? {
            colorBgContainer: '#0f1838',
            colorBgElevated: '#162044',
            colorBgSpotlight: '#1a2550',
            colorBgMask: 'rgba(0,0,0,0.65)',
            colorBorder: 'rgba(255,255,255,0.1)',
            colorBorderSecondary: 'rgba(255,255,255,0.06)',
            colorFill: 'rgba(255,255,255,0.08)',
            colorFillSecondary: 'rgba(255,255,255,0.05)',
            colorFillTertiary: 'rgba(255,255,255,0.03)',
            colorFillQuaternary: 'rgba(255,255,255,0.02)',
            colorText: 'rgba(255,255,255,0.88)',
            colorTextSecondary: 'rgba(255,255,255,0.5)',
            colorTextTertiary: 'rgba(255,255,255,0.3)',
            colorTextQuaternary: 'rgba(255,255,255,0.2)',
          } : {}),
          // colorBgLayout 始终透明，由 CSS 统一控制背景
          colorBgLayout: 'transparent',
          ...(hasWp ? {
            colorBgContainer: isDark ? 'rgba(20,25,40,0.65)' : 'rgba(255,255,255,0.55)',
            colorBgElevated: isDark ? 'rgba(20,25,40,0.8)' : 'rgba(255,255,255,0.75)',
            colorBorder: isDark ? 'rgba(255,255,255,0.12)' : 'rgba(0,0,0,0.1)',
            colorBorderSecondary: isDark ? 'rgba(255,255,255,0.08)' : 'rgba(0,0,0,0.06)',
          } : {}),
        },
        components: {
          Layout: {
            siderBg: hasWp ? (isDark ? 'rgba(10,20,50,0.75)' : 'rgba(255,255,255,0.7)') : (isDark ? '#0a1028' : '#001529'),
            triggerBg: hasWp ? (isDark ? 'rgba(10,20,50,0.85)' : 'rgba(255,255,255,0.8)') : (isDark ? '#0f1838' : '#002140'),
            headerBg: hasWp ? 'rgba(255,255,255,0.06)' : (isDark ? '#0f1838' : '#ffffff'),
          },
          Menu: {
            darkItemBg: hasWp ? 'transparent' : (isDark ? '#0a1028' : '#001529'),
            darkSubMenuItemBg: hasWp ? 'rgba(0,0,0,0.2)' : (isDark ? '#060b1e' : '#000c17'),
            darkItemSelectedBg: hasWp ? 'rgba(22,119,255,0.2)' : (isDark ? '#1677ff30' : '#1677ff'),
            ...(hasWp && !isDark ? {
              itemBg: 'transparent',
              subMenuItemBg: 'rgba(0,0,0,0.03)',
              itemSelectedBg: 'rgba(22,119,255,0.1)',
              itemSelectedColor: '#1677ff',
              itemHoverBg: 'rgba(0,0,0,0.04)',
            } : {}),
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
            headerBg: hasWp ? 'rgba(255,255,255,0.05)' : (isDark ? '#1f1f1f' : '#fafafa'),
            footerBg: hasWp ? 'transparent' : (isDark ? '#1a1a1a' : '#fafafa'),
            rowHoverBg: (isDark || hasWp) ? 'rgba(255,255,255,0.04)' : undefined,
            rowSelectedBg: isDark ? 'rgba(22,119,255,0.12)' : undefined,
            rowSelectedHoverBg: isDark ? 'rgba(22,119,255,0.18)' : undefined,
          },
          Tooltip: {
            colorBgSpotlight: isDark ? '#2a2a2a' : undefined,
            colorTextLightSolid: isDark ? 'rgba(255,255,255,0.85)' : undefined,
          },
          Popover: {
            colorBgElevated: isDark ? '#242424' : undefined,
          },
          Dropdown: {
            colorBgElevated: isDark ? '#242424' : undefined,
          },
          Alert: {
            colorInfoBg: isDark ? 'rgba(22,119,255,0.12)' : undefined,
            colorSuccessBg: isDark ? 'rgba(82,196,26,0.12)' : undefined,
            colorWarningBg: isDark ? 'rgba(250,173,20,0.12)' : undefined,
            colorErrorBg: isDark ? 'rgba(255,77,79,0.12)' : undefined,
            colorInfoBorder: isDark ? 'rgba(22,119,255,0.3)' : undefined,
            colorSuccessBorder: isDark ? 'rgba(82,196,26,0.3)' : undefined,
            colorWarningBorder: isDark ? 'rgba(250,173,20,0.3)' : undefined,
            colorErrorBorder: isDark ? 'rgba(255,77,79,0.3)' : undefined,
          },
          Tabs: {
            cardBg: hasWp ? 'rgba(255,255,255,0.04)' : (isDark ? '#1a1a1a' : undefined),
            itemColor: isDark ? 'rgba(255,255,255,0.5)' : undefined,
            itemActiveColor: isDark ? '#4096ff' : undefined,
            itemHoverColor: isDark ? 'rgba(255,255,255,0.75)' : undefined,
          },
          Drawer: {
            colorBgElevated: hasWp ? 'rgba(15,25,50,0.88)' : (isDark ? '#1a1a1a' : undefined),
          },
          Tag: {
            defaultBg: isDark ? 'rgba(255,255,255,0.06)' : undefined,
            defaultColor: isDark ? 'rgba(255,255,255,0.75)' : undefined,
          },
          Checkbox: {
            colorBgContainer: isDark ? 'rgba(255,255,255,0.05)' : undefined,
          },
          Radio: {
            colorBgContainer: isDark ? 'rgba(255,255,255,0.05)' : undefined,
          },
          Switch: {
            colorTextQuaternary: isDark ? 'rgba(255,255,255,0.2)' : undefined,
          },
          Form: {
            labelColor: isDark ? 'rgba(255,255,255,0.75)' : undefined,
          },
          Pagination: {
            itemBg: 'transparent',
            itemActiveBg: isDark ? 'rgba(22,119,255,0.2)' : undefined,
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
