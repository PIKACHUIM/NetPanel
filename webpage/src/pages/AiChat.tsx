import React, { useEffect, useState, useRef } from 'react'
import { Button, Input, List, Modal, Select, Popconfirm, message, Empty, Spin, Upload, Typography, Tooltip } from 'antd'
import { PlusOutlined, DeleteOutlined, EditOutlined, ExportOutlined, ImportOutlined, SendOutlined, MenuOutlined, RobotOutlined, UserOutlined, DownOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { aiChatApi, aiProviderApi, aiAssistantApi } from '../api'
import { useAppStore } from '../store/appStore'
import dayjs from 'dayjs'

const { TextArea } = Input
const { Text } = Typography

interface Conversation {
  id: number
  title: string
  model_name: string
  provider_id: number
  assistant_id: number
  message_count: number
  updated_at: string
}

interface ChatMsg {
  id?: number
  role: string
  content: string
  tokens_prompt?: number
  tokens_completion?: number
  created_at?: string
}

const AiChat: React.FC = () => {
  const { t } = useTranslation()
  const token = useAppStore((s) => s.token)
  const [conversations, setConversations] = useState<Conversation[]>([])
  const [activeConv, setActiveConv] = useState<Conversation | null>(null)
  const [messages, setMessages] = useState<ChatMsg[]>([])
  const [inputValue, setInputValue] = useState('')
  const [streaming, setStreaming] = useState(false)
  const [streamContent, setStreamContent] = useState('')
  const [providers, setProviders] = useState<any[]>([])
  const [assistants, setAssistants] = useState<any[]>([])
  // 草稿模式：还未创建会话，但已显示输入框和模型选择器
  const [draftProviderId, setDraftProviderId] = useState<number | undefined>()
  const [draftModelName, setDraftModelName] = useState('')
  const [draftAssistantId, setDraftAssistantId] = useState<number>(0)
  const [draftMessages, setDraftMessages] = useState<ChatMsg[]>([])
  const [renaming, setRenaming] = useState<{ id: number; title: string } | null>(null)
  const [sidebarVisible, setSidebarVisible] = useState(true)
  const chatEndRef = useRef<HTMLDivElement>(null)
  const abortRef = useRef<AbortController | null>(null)
  const skipFetchRef = useRef(false)
  const isDraft = !activeConv

  const fetchConversations = async () => {
    try { const res: any = await aiChatApi.listConversations(); setConversations(res.data || []) } catch {}
  }

  const fetchMessages = async (convId: number) => {
    try { const res: any = await aiChatApi.listMessages(convId); setMessages(res.data || []) } catch {}
  }

  const loadOptions = async () => {
    try {
      const [pRes, aRes]: any[] = await Promise.all([aiProviderApi.list(), aiAssistantApi.list()])
      const pList = pRes.data || []
      setProviders(pList)
      setAssistants(aRes.data || [])
      // 自动选择第一个可用的 provider 和 model
      if (pList.length > 0 && !draftProviderId) {
        const first = pList[0]
        setDraftProviderId(first.id)
        try {
          const models = JSON.parse(first.models || '[]')
          if (models.length > 0) setDraftModelName(models[0])
        } catch {}
      }
    } catch {}
  }

  useEffect(() => { fetchConversations(); loadOptions() }, [])
  useEffect(() => {
    if (!activeConv) return
    if (skipFetchRef.current) {
      skipFetchRef.current = false
      return
    }
    fetchMessages(activeConv.id)
  }, [activeConv?.id])
  useEffect(() => { chatEndRef.current?.scrollIntoView({ behavior: 'smooth' }) }, [messages, streamContent])

  const selectConversation = (conv: Conversation) => {
    setActiveConv(conv)
    setStreamContent('')
    // 清空草稿状态
    setDraftMessages([])
    setInputValue('')
  }

  const newChat = () => {
    // 进入草稿模式：清空当前会话，显示干净的输入区
    setActiveConv(null)
    setMessages([])
    setDraftMessages([])
    setStreamContent('')
    setInputValue('')
    // 保留上次的模型选择
  }

  const deleteConversation = async (id: number) => {
    await aiChatApi.deleteConversation(id)
    if (activeConv?.id === id) { setActiveConv(null); setMessages([]) }
    fetchConversations()
  }

  const handleRename = async () => {
    if (!renaming) return
    await aiChatApi.updateConversation(renaming.id, { title: renaming.title })
    setRenaming(null)
    fetchConversations()
  }

  const startNewConversation = async (firstMsg: string): Promise<Conversation | null> => {
    // 创建新会话并返回
    if (!draftProviderId || !draftModelName) {
      message.error(t('ai.selectModel'))
      return null
    }
    try {
      const res: any = await aiChatApi.createConversation({
        title: firstMsg.slice(0, 50) || t('ai.newChat'),
        provider_id: draftProviderId,
        model_name: draftModelName,
        assistant_id: draftAssistantId,
      })
      const newConv: Conversation = res.data
      skipFetchRef.current = true // 跳过 useEffect 中的 fetchMessages，保留草稿消息
      setActiveConv(newConv)
      // 将草稿消息追加到正式消息中
      setMessages((prev) => [...prev, ...draftMessages])
      setDraftMessages([])
      fetchConversations()
      return newConv
    } catch {
      return null
    }
  }

  const doStream = async (convId: number, content: string) => {
    setStreaming(true)
    setStreamContent('')

    try {
      abortRef.current = new AbortController()
      const response = await fetch(`/api/v1/ai/conversations/${convId}/stream`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}`,
        },
        body: JSON.stringify({ content }),
        signal: abortRef.current.signal,
      })

      if (!response.ok) throw new Error(`HTTP ${response.status}`)

      const reader = response.body?.getReader()
      if (!reader) throw new Error('No reader')

      const decoder = new TextDecoder()
      let fullContent = ''

      while (true) {
        const { done, value } = await reader.read()
        if (done) break

        const text = decoder.decode(value, { stream: true })
        const lines = text.split('\n')

        for (const line of lines) {
          if (!line.startsWith('data: ')) continue
          const data = line.slice(6)
          if (data === '[DONE]') break

          try {
            const chunk = JSON.parse(data)
            if (chunk.error) {
              message.error(chunk.error)
              break
            }
            if (chunk.choices?.[0]?.delta?.content) {
              fullContent += chunk.choices[0].delta.content
              setStreamContent(fullContent)
            }
          } catch {}
        }
      }

      if (fullContent) {
        setMessages((prev) => [...prev, { role: 'assistant', content: fullContent }])
      }
      setStreamContent('')
    } catch (e: any) {
      if (e.name !== 'AbortError') {
        message.error('发送失败: ' + (e.message || '未知错误'))
      }
    } finally {
      setStreaming(false)
      abortRef.current = null
      fetchConversations()
    }
  }

  const sendMessage = async () => {
    if (!inputValue.trim() || streaming) return
    const content = inputValue.trim()
    const userMsg: ChatMsg = { role: 'user', content }

    // 草稿模式：先创建会话，再发消息
    if (isDraft) {
      setDraftMessages((prev) => [...prev, userMsg])
      setInputValue('')
      const conv = await startNewConversation(content)
      if (!conv) return
      await doStream(conv.id, content)
      return
    }

    // 已有会话：直接发送
    if (!activeConv) return
    setMessages((prev) => [...prev, userMsg])
    setInputValue('')
    await doStream(activeConv.id, content)
  }

  const handleExport = async () => {
    if (!activeConv) return
    try {
      const res: any = await aiChatApi.exportConversation(activeConv.id)
      const blob = new Blob([JSON.stringify(res, null, 2)], { type: 'application/json' })
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = `conversation_${activeConv.id}.json`
      a.click()
      URL.revokeObjectURL(url)
    } catch {}
  }

  const handleImport = async (file: File) => {
    try {
      const text = await file.text()
      const data = JSON.parse(text)
      await aiChatApi.importConversation(data)
      message.success(t('common.success'))
      fetchConversations()
    } catch (e: any) {
      message.error('导入失败: ' + (e.message || ''))
    }
    return false
  }

  const getModels = (providerId?: number): string[] => {
    if (!providerId) return []
    const p = providers.find((p: any) => p.id === providerId)
    if (!p) return []
    try { return JSON.parse(p.models || '[]') } catch { return [] }
  }

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      sendMessage()
    }
  }

  return (
    <div style={{ display: 'flex', height: 'calc(100vh - 120px)', background: '#0F131C' }}>
      {/* 左侧对话列表（可折叠） */}
      <div
        style={{
          width: sidebarVisible ? 260 : 0,
          flexShrink: 0,
          display: 'flex',
          flexDirection: 'column',
          background: '#0A0D12',
          borderRight: sidebarVisible ? '1px solid #1E2636' : 'none',
          overflow: 'hidden',
          transition: 'width 0.2s ease',
        }}
      >
        <div style={{ padding: '12px 8px', display: 'flex', flexDirection: 'column', gap: 8 }}>
          <Button
            type="primary"
            icon={<PlusOutlined />}
            block
            onClick={newChat}
            style={{ borderRadius: 6 }}
          >
            {t('ai.newChat')}
          </Button>
          <div style={{ display: 'flex', gap: 4 }}>
            <Tooltip title={t('ai.exportChat')}>
              <Button size="small" icon={<ExportOutlined />} onClick={handleExport} disabled={!activeConv} style={{ flex: 1 }} />
            </Tooltip>
            <Upload accept=".json" showUploadList={false} beforeUpload={handleImport as any}>
              <Tooltip title={t('ai.importChat')}>
                <Button size="small" icon={<ImportOutlined />} style={{ width: '100%' }} />
              </Tooltip>
            </Upload>
          </div>
        </div>
        <div style={{ flex: 1, overflow: 'auto', padding: '0 8px' }}>
          {conversations.length === 0 ? (
            <Empty description={t('ai.noConversation')} image={Empty.PRESENTED_IMAGE_SIMPLE} />
          ) : (
            <List
              size="small"
              dataSource={conversations}
              renderItem={(item) => (
                <List.Item
                  key={item.id}
                  onClick={() => selectConversation(item)}
                  style={{
                    cursor: 'pointer',
                    padding: '10px 12px',
                    borderRadius: 8,
                    background: activeConv?.id === item.id ? '#1E2636' : 'transparent',
                    marginBottom: 4,
                    border: 'none',
                    transition: 'background 0.2s',
                  }}
                  onMouseEnter={(e) => {
                    if (activeConv?.id !== item.id) {
                      e.currentTarget.style.background = '#161D2B'
                    }
                  }}
                  onMouseLeave={(e) => {
                    if (activeConv?.id !== item.id) {
                      e.currentTarget.style.background = 'transparent'
                    }
                  }}
                >
                  <div style={{ width: '100%', display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
                    <div style={{ flex: 1, overflow: 'hidden' }}>
                      <div style={{ fontSize: 13, color: '#E5E7EB', marginBottom: 2, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                        {item.title}
                      </div>
                      <div style={{ fontSize: 11, color: '#6B7280' }}>
                        {item.model_name} · {dayjs(item.updated_at).format('MM-DD HH:mm')}
                      </div>
                    </div>
                    <div style={{ display: 'flex', gap: 4, marginLeft: 8 }}>
                      <EditOutlined
                        style={{ fontSize: 12, color: '#6B7280', cursor: 'pointer' }}
                        onClick={(e) => {
                          e.stopPropagation()
                          setRenaming({ id: item.id, title: item.title })
                        }}
                      />
                      <Popconfirm
                        title={t('common.deleteConfirm')}
                        onConfirm={(e) => {
                          e?.stopPropagation()
                          deleteConversation(item.id)
                        }}
                      >
                        <DeleteOutlined
                          style={{ fontSize: 12, color: '#6B7280', cursor: 'pointer' }}
                          onClick={(e) => e.stopPropagation()}
                        />
                      </Popconfirm>
                    </div>
                  </div>
                </List.Item>
              )}
            />
          )}
        </div>
      </div>

      {/* 主聊天区域 */}
      <div style={{ flex: 1, display: 'flex', flexDirection: 'column', position: 'relative' }}>
        {/* 顶部工具栏 - 模型选择器 */}
        <div
          style={{
            padding: '8px 16px',
            borderBottom: '1px solid #1E2636',
            display: 'flex',
            alignItems: 'center',
            gap: 10,
            background: '#0A0D12',
          }}
        >
          <Button
            type="text"
            icon={<MenuOutlined />}
            onClick={() => setSidebarVisible(!sidebarVisible)}
            style={{ color: '#9CA3AF' }}
          />
          {activeConv && (
            <div style={{ flex: 1, display: 'flex', alignItems: 'center', gap: 8 }}>
              <Text style={{ color: '#E5E7EB', fontSize: 14, fontWeight: 500 }}>{activeConv.title}</Text>
            </div>
          )}

          {/* 模型选择器 */}
          <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
            <Select
              size="small"
              value={isDraft ? draftProviderId : activeConv?.provider_id}
              placeholder={t('ai.selectProvider')}
              onChange={(v) => {
                if (isDraft) {
                  setDraftProviderId(v)
                  setDraftModelName('')
                }
              }}
              disabled={!isDraft}
              style={{ minWidth: 120 }}
              popupMatchSelectWidth={false}
            >
              {providers.filter((p: any) => p.is_active).map((p: any) => (
                <Select.Option key={p.id} value={p.id}>{p.name}</Select.Option>
              ))}
            </Select>
            <Select
              size="small"
              value={isDraft ? (draftModelName || undefined) : (activeConv?.model_name || undefined)}
              placeholder={t('ai.selectModel')}
              onChange={(v) => {
                if (isDraft) setDraftModelName(v)
              }}
              disabled={!isDraft}
              showSearch
              style={{ minWidth: 140 }}
              popupMatchSelectWidth={false}
            >
              {getModels(isDraft ? draftProviderId : activeConv?.provider_id).map((m: string) => (
                <Select.Option key={m} value={m}>{m}</Select.Option>
              ))}
            </Select>
            <DownOutlined style={{ color: '#6B7280', fontSize: 10 }} />
          </div>
        </div>

        {/* 消息区域 */}
        <div style={{ flex: 1, overflow: 'auto', display: 'flex', flexDirection: 'column', alignItems: 'center' }}>
          {!activeConv && draftMessages.length === 0 ? (
            /* 空状态：欢迎界面 */
            <div style={{ flex: 1, display: 'flex', alignItems: 'center', justifyContent: 'center', width: '100%' }}>
              <div style={{ textAlign: 'center', maxWidth: 640, padding: '0 32px' }}>
                <RobotOutlined style={{ fontSize: 48, color: '#6EE7B7', marginBottom: 24 }} />
                <div style={{ fontSize: 22, fontWeight: 600, color: '#E5E7EB', marginBottom: 8 }}>
                  NetPanel AI
                </div>
                <div style={{ fontSize: 14, color: '#6B7280', lineHeight: 1.6 }}>
                  {t('ai.typeMessage')}
                </div>
              </div>
            </div>
          ) : (
            /* 消息列表 */
            <div style={{ width: '100%', maxWidth: 800, padding: '24px 16px' }}>
              {/* 草稿消息（未保存到后端的临时消息） */}
              {draftMessages.map((msg, idx) => (
                <div key={`draft-${idx}`} style={{ display: 'flex', gap: 16, marginBottom: 24, alignItems: 'flex-start' }}>
                  <div style={{ width: 32, height: 32, borderRadius: '50%', display: 'flex', alignItems: 'center', justifyContent: 'center', background: msg.role === 'user' ? '#38BDF8' : '#6EE7B7', flexShrink: 0 }}>
                    {msg.role === 'user' ? <UserOutlined style={{ color: '#fff', fontSize: 16 }} /> : <RobotOutlined style={{ color: '#0A0D12', fontSize: 16 }} />}
                  </div>
                  <div style={{ flex: 1, minWidth: 0 }}>
                    <div style={{ fontSize: 13, fontWeight: 600, color: '#E5E7EB', marginBottom: 8 }}>
                      {msg.role === 'user' ? 'You' : 'AI Assistant'}
                    </div>
                    <div style={{ fontSize: 14, color: '#D1D5DB', lineHeight: 1.7, whiteSpace: 'pre-wrap', wordBreak: 'break-word' }}>
                      {msg.content}
                    </div>
                  </div>
                </div>
              ))}
              {/* 正式消息 */}
              {messages.map((msg, idx) => (
                <div key={`msg-${idx}`} style={{ display: 'flex', gap: 16, marginBottom: 24, alignItems: 'flex-start' }}>
                  <div style={{ width: 32, height: 32, borderRadius: '50%', display: 'flex', alignItems: 'center', justifyContent: 'center', background: msg.role === 'user' ? '#38BDF8' : '#6EE7B7', flexShrink: 0 }}>
                    {msg.role === 'user' ? <UserOutlined style={{ color: '#fff', fontSize: 16 }} /> : <RobotOutlined style={{ color: '#0A0D12', fontSize: 16 }} />}
                  </div>
                  <div style={{ flex: 1, minWidth: 0 }}>
                    <div style={{ fontSize: 13, fontWeight: 600, color: '#E5E7EB', marginBottom: 8 }}>
                      {msg.role === 'user' ? 'You' : 'AI Assistant'}
                    </div>
                    <div style={{ fontSize: 14, color: '#D1D5DB', lineHeight: 1.7, whiteSpace: 'pre-wrap', wordBreak: 'break-word' }}>
                      {msg.content}
                    </div>
                  </div>
                </div>
              ))}
              {/* 流式输出 */}
              {streaming && streamContent && (
                <div style={{ display: 'flex', gap: 16, marginBottom: 24, alignItems: 'flex-start' }}>
                  <div style={{ width: 32, height: 32, borderRadius: '50%', display: 'flex', alignItems: 'center', justifyContent: 'center', background: '#6EE7B7', flexShrink: 0 }}>
                    <RobotOutlined style={{ color: '#0A0D12', fontSize: 16 }} />
                  </div>
                  <div style={{ flex: 1, minWidth: 0 }}>
                    <div style={{ fontSize: 13, fontWeight: 600, color: '#E5E7EB', marginBottom: 8 }}>AI Assistant</div>
                    <div style={{ fontSize: 14, color: '#D1D5DB', lineHeight: 1.7, whiteSpace: 'pre-wrap', wordBreak: 'break-word' }}>
                      {streamContent}
                      <span className="typing-cursor" style={{ animation: 'blink 1s infinite', marginLeft: 2 }}>▊</span>
                    </div>
                  </div>
                </div>
              )}
              {streaming && !streamContent && (
                <div style={{ display: 'flex', gap: 16, alignItems: 'center' }}>
                  <Spin size="small" />
                  <Text style={{ color: '#6B7280' }}>{t('ai.streaming')}</Text>
                </div>
              )}
              <div ref={chatEndRef} />
            </div>
          )}
        </div>

        {/* 输入区域 - 始终显示 */}
        <div
          style={{
            padding: '16px',
            borderTop: '1px solid #1E2636',
            display: 'flex',
            justifyContent: 'center',
            background: '#0A0D12',
          }}
        >
          <div style={{ width: '100%', maxWidth: 800, display: 'flex', gap: 8, alignItems: 'flex-end' }}>
            <TextArea
              value={inputValue}
              onChange={(e) => setInputValue(e.target.value)}
              onKeyDown={handleKeyDown}
              placeholder={t('ai.typeMessage')}
              autoSize={{ minRows: 1, maxRows: 6 }}
              disabled={streaming}
              style={{
                flex: 1,
                borderRadius: 12,
                background: '#161D2B',
                border: '1px solid #1E2636',
                color: '#E5E7EB',
                resize: 'none',
              }}
            />
            <Button
              type="primary"
              icon={<SendOutlined />}
              onClick={sendMessage}
              loading={streaming}
              disabled={!inputValue.trim()}
              style={{ borderRadius: 8, height: 40 }}
            />
          </div>
        </div>
      </div>

      {/* 重命名弹窗 */}
      <Modal
        title={t('ai.renameChat')}
        open={!!renaming}
        onOk={handleRename}
        onCancel={() => setRenaming(null)}
        destroyOnClose
      >
        <Input
          value={renaming?.title || ''}
          onChange={(e) => setRenaming(renaming ? { ...renaming, title: e.target.value } : null)}
        />
      </Modal>

      <style>{`
        @keyframes blink {
          0%, 50% { opacity: 1; }
          51%, 100% { opacity: 0; }
        }
      `}</style>
    </div>
  )
}

export default AiChat
