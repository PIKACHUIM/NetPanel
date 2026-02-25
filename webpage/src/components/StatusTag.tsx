import React from 'react'
import { Tag } from 'antd'
import { CheckCircleOutlined, StopOutlined, ExclamationCircleOutlined, LoadingOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'

interface StatusTagProps {
  status: string
}

const StatusTag: React.FC<StatusTagProps> = ({ status }) => {
  const { t } = useTranslation()

  switch (status) {
    case 'running':
      return <Tag icon={<CheckCircleOutlined />} color="success">{t('common.running')}</Tag>
    case 'stopped':
      return <Tag icon={<StopOutlined />} color="default">{t('common.stopped')}</Tag>
    case 'error':
      return <Tag icon={<ExclamationCircleOutlined />} color="error">{t('common.error')}</Tag>
    case 'pending':
      return <Tag icon={<LoadingOutlined />} color="processing">处理中</Tag>
    default:
      return <Tag color="default">{status || t('common.stopped')}</Tag>
  }
}

export default StatusTag
