import React from 'react'
import { Card, Typography } from 'antd'

const { Title } = Typography

interface PageContainerProps {
  title: string
  extra?: React.ReactNode
  children: React.ReactNode
}

const PageContainer: React.FC<PageContainerProps> = ({ title, extra, children }) => {
  return (
    <div>
      <div className="page-header" style={{ marginBottom: 16 }}>
        <Title level={4} style={{ margin: 0 }}>{title}</Title>
        {extra && <div>{extra}</div>}
      </div>
      <Card
        style={{
          borderRadius: 8,
          boxShadow: '0 1px 4px rgba(0,0,0,0.06)',
        }}
        styles={{ body: { padding: 0 } }}
      >
        {children}
      </Card>
    </div>
  )
}

export default PageContainer
