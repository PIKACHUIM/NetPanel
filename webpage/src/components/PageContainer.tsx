import React from 'react'
import { Card, Typography, Space, theme as antTheme } from 'antd'
import { useAppStore } from '../store/appStore'

const { Title } = Typography

interface PageContainerProps {
  /** 页面标题 */
  title: string
  /** 标题图标 */
  icon?: React.ReactNode
  /** 标题右侧操作区 */
  extra?: React.ReactNode
  /** 子内容 */
  children: React.ReactNode
  /** 是否显示卡片包裹 */
  withCard?: boolean
}

/**
 * 页面容器组件
 * 提供统一的页面标题和内容区域布局
 */
const PageContainer: React.FC<PageContainerProps> = ({
  title,
  icon,
  extra,
  children,
  withCard = true,
}) => {
  const { theme } = useAppStore()
  const { token } = antTheme.useToken()
const isGlass = theme === 'glass-light' || theme === 'glass-dark'

  const cardStyle = isGlass ? {
    background: 'rgba(255,255,255,0.05)',
    backdropFilter: 'blur(20px)',
    WebkitBackdropFilter: 'blur(20px)',
    border: '1px solid rgba(255,255,255,0.08)',
    boxShadow: '0 8px 32px rgba(0,0,0,0.2)',
  } : {
    boxShadow: '0 1px 3px rgba(0,0,0,0.06), 0 4px 12px rgba(0,0,0,0.03)',
  }

  return (
    <div>
      {/* 页面标题栏 */}
      <div style={{
        display: 'flex',
        justifyContent: 'space-between',
        alignItems: 'center',
        marginBottom: 16,
      }}>
        <Space align="center" size={10}>
          {icon && (
            <div style={{
              width: 34, height: 34, borderRadius: 9,
              background: 'linear-gradient(135deg, #1677ff 0%, #0958d9 100%)',
              display: 'flex', alignItems: 'center', justifyContent: 'center',
              boxShadow: '0 4px 12px rgba(22,119,255,0.35)',
              color: '#fff', fontSize: 16,
            }}>
              {icon}
            </div>
          )}
          <Title level={4} style={{ margin: 0, fontSize: 18 }}>{title}</Title>
        </Space>
        {extra && <div>{extra}</div>}
      </div>

      {/* 内容区域 */}
      {withCard ? (
        <Card
          style={{
            borderRadius: 12,
            ...cardStyle,
          }}
          styles={{ body: { padding: 0 } }}
        >
          {children}
        </Card>
      ) : (
        children
      )}
    </div>
  )
}

export default PageContainer
