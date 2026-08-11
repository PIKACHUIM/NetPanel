import { theme } from 'antd'
import { useAppStore } from '../store/appStore'

/**
 * 表格样式Hook - 自动适配主题
 * 返回适配暗黑/明亮模式的表格样式对象
 */
export const useTableStyle = () => {
  const { token } = theme.useToken()
  const { theme: appTheme } = useAppStore()
  const isDark = appTheme === 'dark'

  return {
    background: isDark ? 'rgba(10,13,18,0.4)' : token.colorBgContainer,
    borderRadius: 8,
  }
}
