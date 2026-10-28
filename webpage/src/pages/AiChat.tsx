import React, { useEffect, useMemo, useRef, useState } from 'react'
import {
  Button,
  Empty,
  Input,
  Modal,
  Popconfirm,
  Select,
  Spin,
  Tag,
  Tooltip,
  Upload,
  message,
  theme as antTheme,
} from 'antd'
import {
  DeleteOutlined,
  EditOutlined,
  ExportOutlined,
  ImportOutlined,
  MenuOutlined,
  MenuUnfoldOutlined,
  MessageOutlined,
  PlusOutlined,
  RobotOutlined,
  SearchOutlined,
  SendOutlined,
  StopOutlined,
} from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { aiChatApi, aiProviderApi, aiAssistantApi } from '../api'
import { useAppStore } from '../store/appStore'
import dayjs from 'dayjs'

const { TextArea } = Input

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
  const { token: themeToken } = antTheme.useToken()
  const authToken = useAppStore((s) => s.token)

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
  const [sidebarVisible, setSidebarVisible] = useState(() => window.innerWidth > 900)
  const [search, setSearch] = useState('')
  const [hoverId, setHoverId] = useState<number | null>(null)
  const [composerFocused, setComposerFocused] = useState(false)

  const scrollRef = useRef<HTMLDivElement>(null)
  const textareaRef = useRef<any>(null)
  const nearBottomRef = useRef(true)
  const abortRef = useRef<AbortController | null>(null)
  const skipFetchRef = useRef(false)
  const isDraft = !activeConv

  const fetchConversations = async () => {
    try {
      const res: any = await aiChatApi.listConversations()
      setConversations(res.data || [])
    } catch {}
  }

  const fetchMessages = async (convId: number) => {
    try {
      const res: any = await aiChatApi.listMessages(convId)
      setMessages(res.data || [])
    } catch {}
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

  // 仅在用户停留在底部时自动滚动，避免流式输出把正在翻历史的用户拽回去
  useEffect(() => {
    const el = scrollRef.current
    if (el && nearBottomRef.current) {
      el.scrollTop = el.scrollHeight
    }
  }, [messages, streamContent])

  const selectConversation = (conv: Conversation) => {
    setActiveConv(conv)
    setStreamContent('')
    setDraftMessages([])
    setInputValue('')
    nearBottomRef.current = true
  }

  const newChat = () => {
    setActiveConv(null)
    setMessages([])
    setDraftMessages([])
    setStreamContent('')
    setInputValue('')
    nearBottomRef.current = true
  }

  const deleteConversation = async (id: number) => {
    await aiChatApi.deleteConversation(id)
    if (activeConv?.id === id) {
      setActiveConv(null)
      setMessages([])
      setDraftMessages([])
    }
    fetchConversations()
  }

  const handleRename = async () => {
    if (!renaming || !renaming.title.trim()) return
    await aiChatApi.updateConversation(renaming.id, { title: renaming.title.trim() })
    setRenaming(null)
    fetchConversations()
  }

  const startNewConversation = async (firstMsg: string): Promise<Conversation | null> => {
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
      skipFetchRef.current = true // 跳过 useEffect 中的 fetchMessages，保留刚发送的消息
      // 先清草稿再写入正式消息，保证同一帧内只渲染一份
      setDraftMessages([])
      setActiveConv(newConv)
      setMessages([{ role: 'user', content: firstMsg }])
      fetchConversations()
      return newConv
    } catch {
      return null
    }
  }

  const doStream = async (convId: number, content: string) => {
    setStreaming(true)
    setStreamContent('')
    let fullContent = ''

    try {
      abortRef.current = new AbortController()
      const response = await fetch(`/api/v1/ai/conversations/${convId}/stream`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${authToken}`,
        },
        body: JSON.stringify({ content }),
        signal: abortRef.current.signal,
      })

      if (!response.ok) throw new Error(`HTTP ${response.status}`)

      const reader = response.body?.getReader()
      if (!reader) throw new Error('No reader')

      const decoder = new TextDecoder()
      let buffer = ''
      let streamDone = false

      const consumeFrame = (frame: string) => {
        const payload = frame.split(/\r?\n/)
          .filter(line => line.startsWith('data:'))
          .map(line => line.slice(5).trimStart())
          .join('\n')
        if (!payload) return
        if (payload === '[DONE]') {
          streamDone = true
          return
        }
        const chunk = JSON.parse(payload)
        if (chunk.error) throw new Error(chunk.error)
        const delta = chunk.choices?.[0]?.delta?.content
        if (delta) {
          fullContent += delta
          setStreamContent(fullContent)
        }
      }

      while (true) {
        const { done, value } = await reader.read()
        buffer += value ? decoder.decode(value, { stream: !done }) : decoder.decode()
        const frames = buffer.split(/\r?\n\r?\n/)
        buffer = frames.pop() || ''
        for (const frame of frames) consumeFrame(frame)
        if (streamDone || done) break
      }
      if (!streamDone && buffer.trim()) consumeFrame(buffer)
      if (!streamDone) throw new Error('响应流意外结束')

      if (fullContent) {
        setMessages((prev) => [...prev, { role: 'assistant', content: fullContent }])
      }
    } catch (e: any) {
      if (e.name === 'AbortError') {
        // 手动停止：保留已经生成的内容
        if (fullContent) {
          setMessages((prev) => [...prev, { role: 'assistant', content: fullContent }])
        }
      } else {
        message.error('发送失败: ' + (e.message || '未知错误'))
      }
    } finally {
      setStreaming(false)
      setStreamContent('')
      abortRef.current = null
      fetchConversations()
    }
  }

  const sendMessage = async () => {
    if (!inputValue.trim() || streaming) return
    const content = inputValue.trim()

    // 草稿模式：先创建会话，再发消息
    if (isDraft) {
      setDraftMessages([{ role: 'user', content }])
      setInputValue('')
      const conv = await startNewConversation(content)
      if (!conv) {
        setDraftMessages([])
        setInputValue(content)
        return
      }
      await doStream(conv.id, content)
      return
    }

    if (!activeConv) return
    setMessages((prev) => [...prev, { role: 'user', content }])
    setInputValue('')
    nearBottomRef.current = true
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
    const p = providers.find((item: any) => item.id === providerId)
    if (!p) return []
    try {
      return JSON.parse(p.models || '[]')
    } catch {
      return []
    }
  }

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      sendMessage()
    }
  }

  const providerName = (providerId?: number) =>
    providers.find((p: any) => p.id === providerId)?.name || ''

  const formatTime = (value?: string) => {
    if (!value) return ''
    const d = dayjs(value)
    if (!d.isValid()) return ''
    return d.isSame(dayjs(), 'day') ? d.format('HH:mm') : d.format('MM-DD HH:mm')
  }

  const filteredConversations = useMemo(() => {
    const keyword = search.trim().toLowerCase()
    return [...conversations]
      .filter((item) => !keyword
        || (item.title || '').toLowerCase().includes(keyword)
        || (item.model_name || '').toLowerCase().includes(keyword))
      .sort((a, b) => dayjs(b.updated_at).valueOf() - dayjs(a.updated_at).valueOf())
  }, [conversations, search])

  // 面板统一外观：简约模式实色、壁纸模式毛玻璃由 .dashboard-card 全局规则接管
  const panelStyle: React.CSSProperties = {
    borderRadius: 16,
    border: `1px solid ${themeToken.colorBorderSecondary}`,
    background: themeToken.colorBgContainer,
    boxShadow: themeToken.boxShadowTertiary,
    overflow: 'hidden',
  }

  const renderMessage = (msg: ChatMsg, key: string) => {
    if (msg.role === 'user') {
      return (
        <div key={key} style={{ display: 'flex', justifyContent: 'flex-end', marginBottom: 20 }}>
          <div style={{ maxWidth: '78%', minWidth: 0 }}>
            <div style={{
              padding: '10px 14px',
              borderRadius: '14px 14px 4px 14px',
              background: themeToken.colorPrimaryBg,
              border: `1px solid ${themeToken.colorPrimaryBorder}`,
              color: themeToken.colorPrimaryText,
              fontSize: 14,
              lineHeight: 1.7,
              whiteSpace: 'pre-wrap',
              wordBreak: 'break-word',
            }}>
              {msg.content}
            </div>
            {msg.created_at && (
              <div style={{ marginTop: 5, textAlign: 'right', fontSize: 11, color: themeToken.colorTextQuaternary }}>
                {formatTime(msg.created_at)}
              </div>
            )}
          </div>
        </div>
      )
    }

    return (
      <div key={key} style={{ display: 'flex', gap: 12, marginBottom: 22 }}>
        <div style={{
          width: 30,
          height: 30,
          borderRadius: 10,
          flexShrink: 0,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          background: 'linear-gradient(135deg, #0071e3, #0958d9)',
          boxShadow: '0 4px 12px rgba(0, 113, 227, 0.3)',
        }}>
          <RobotOutlined style={{ color: '#fff', fontSize: 15 }}/>
        </div>
        <div style={{ flex: 1, minWidth: 0 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 6 }}>
            <span style={{ fontSize: 13, fontWeight: 600, color: themeToken.colorText }}>NetPanel AI</span>
            {msg.created_at && (
              <span style={{ fontSize: 11, color: themeToken.colorTextQuaternary }}>{formatTime(msg.created_at)}</span>
            )}
          </div>
          <div style={{
            fontSize: 14,
            lineHeight: 1.75,
            color: themeToken.colorText,
            whiteSpace: 'pre-wrap',
            wordBreak: 'break-word',
          }}>
            {msg.content}
          </div>
          {(msg.tokens_prompt || msg.tokens_completion) ? (
            <div style={{ marginTop: 6, fontSize: 11, color: themeToken.colorTextQuaternary }}>
              {t('ai.tokens')} {msg.tokens_prompt || 0} / {msg.tokens_completion || 0}
            </div>
          ) : null}
        </div>
      </div>
    )
  }

  return (
    <div style={{ display: 'flex', gap: 16, height: 'calc(100vh - 96px)' }}>
      {/* 左侧：会话列表 */}
      <aside style={{
        width: sidebarVisible ? 280 : 0,
        flexShrink: 0,
        overflow: 'hidden',
        transition: 'width 0.25s ease',
      }}>
        <div className="dashboard-card ai-chat-panel" style={{
          ...panelStyle,
          width: 280,
          height: '100%',
          display: 'flex',
          flexDirection: 'column',
        }}>
          <div style={{ padding: '16px 14px 12px', display: 'flex', flexDirection: 'column', gap: 10 }}>
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 8 }}>
              <span style={{ fontSize: 14, fontWeight: 600, color: themeToken.colorText }}>{t('ai.chat')}</span>
              <Tag style={{ marginInlineEnd: 0, borderRadius: 999, paddingInline: 8 }}>
                {conversations.length}
              </Tag>
            </div>
            <Button
              type="primary"
              block
              icon={<PlusOutlined/>}
              onClick={newChat}
              style={{ borderRadius: 10, height: 36, fontWeight: 500 }}
            >
              {t('ai.newChat')}
            </Button>
            <Input
              allowClear
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              prefix={<SearchOutlined style={{ color: themeToken.colorTextTertiary }}/>}
              placeholder={t('ai.searchChat')}
              style={{ borderRadius: 10 }}
            />
          </div>

          <div style={{ flex: 1, overflow: 'auto', padding: '2px 10px 10px' }}>
            {filteredConversations.length === 0 ? (
              <div style={{ padding: '32px 8px' }}>
                <Empty
                  image={Empty.PRESENTED_IMAGE_SIMPLE}
                  description={t('ai.noConversation')}
                  styles={{ description: { fontSize: 12, color: themeToken.colorTextTertiary } }}
                />
              </div>
            ) : filteredConversations.map((item) => {
              const active = activeConv?.id === item.id
              const hovered = hoverId === item.id
              return (
                <div
                  key={item.id}
                  onClick={() => selectConversation(item)}
                  onMouseEnter={() => setHoverId(item.id)}
                  onMouseLeave={() => setHoverId(null)}
                  style={{
                    padding: '9px 10px 8px',
                    marginBottom: 6,
                    borderRadius: 10,
                    cursor: 'pointer',
                    background: active
                      ? themeToken.colorPrimaryBg
                      : hovered
                        ? themeToken.colorFillTertiary
                        : 'transparent',
                    border: `1px solid ${active ? themeToken.colorPrimaryBorder : 'transparent'}`,
                    transition: 'background 0.15s ease, border-color 0.15s ease',
                  }}
                >
                  <div style={{ display: 'flex', alignItems: 'center', gap: 7, minWidth: 0 }}>
                    <MessageOutlined style={{
                      fontSize: 13,
                      flexShrink: 0,
                      color: active ? themeToken.colorPrimary : themeToken.colorTextTertiary,
                    }}/>
                    <span style={{
                      flex: 1,
                      minWidth: 0,
                      overflow: 'hidden',
                      textOverflow: 'ellipsis',
                      whiteSpace: 'nowrap',
                      fontSize: 13,
                      fontWeight: active ? 600 : 500,
                      color: active ? themeToken.colorPrimary : themeToken.colorText,
                    }}>
                      {item.title || t('ai.newChat')}
                    </span>
                  </div>
                  <div style={{
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'space-between',
                    gap: 8,
                    marginTop: 4,
                    paddingLeft: 20,
                  }}>
                    <span style={{
                      fontSize: 11,
                      color: themeToken.colorTextTertiary,
                      overflow: 'hidden',
                      textOverflow: 'ellipsis',
                      whiteSpace: 'nowrap',
                    }}>
                      {item.model_name || '-'}
                    </span>
                    {(hovered || active) ? (
                      <span style={{ display: 'flex', alignItems: 'center', gap: 6, flexShrink: 0 }}>
                        <Tooltip title={t('ai.renameChat')}>
                          <EditOutlined
                            style={{ fontSize: 12, color: themeToken.colorTextTertiary }}
                            onClick={(e) => {
                              e.stopPropagation()
                              setRenaming({ id: item.id, title: item.title })
                            }}
                          />
                        </Tooltip>
                        <Popconfirm
                          title={t('ai.deleteChat')}
                          description={t('common.deleteConfirm')}
                          onConfirm={(e) => {
                            e?.stopPropagation()
                            deleteConversation(item.id)
                          }}
                        >
                          <DeleteOutlined
                            style={{ fontSize: 12, color: themeToken.colorTextTertiary }}
                            onClick={(e) => e.stopPropagation()}
                          />
                        </Popconfirm>
                      </span>
                    ) : (
                      <span style={{ fontSize: 11, color: themeToken.colorTextQuaternary, flexShrink: 0 }}>
                        {formatTime(item.updated_at)}
                      </span>
                    )}
                  </div>
                </div>
              )
            })}
          </div>

          <div style={{ padding: '10px 14px 14px', borderTop: `1px solid ${themeToken.colorBorderSecondary}` }}>
            <Upload accept=".json" showUploadList={false} beforeUpload={handleImport as any}>
              <Button block icon={<ImportOutlined/>} style={{ borderRadius: 10 }}>
                {t('ai.importChat')}
              </Button>
            </Upload>
          </div>
        </div>
      </aside>

      {/* 右侧：聊天区 */}
      <section className="dashboard-card ai-chat-panel" style={{
        ...panelStyle,
        flex: 1,
        minWidth: 0,
        display: 'flex',
        flexDirection: 'column',
      }}>
        {/* 顶部工具栏 */}
        <div style={{
          padding: '10px 14px',
          borderBottom: `1px solid ${themeToken.colorBorderSecondary}`,
          display: 'flex',
          alignItems: 'center',
          gap: 10,
          flexWrap: 'wrap',
        }}>
          <Tooltip title={sidebarVisible ? t('common.collapseMenu') : t('common.expandMenu')}>
            <Button
              type="text"
              icon={sidebarVisible ? <MenuOutlined/> : <MenuUnfoldOutlined/>}
              onClick={() => setSidebarVisible(!sidebarVisible)}
              style={{ color: themeToken.colorTextSecondary }}
            />
          </Tooltip>

          <div style={{ flex: 1, minWidth: 140, display: 'flex', flexDirection: 'column', gap: 2 }}>
            <span style={{
              fontSize: 14,
              fontWeight: 600,
              color: themeToken.colorText,
              overflow: 'hidden',
              textOverflow: 'ellipsis',
              whiteSpace: 'nowrap',
            }}>
              {activeConv?.title || t('ai.newChat')}
            </span>
            <span style={{
              fontSize: 11,
              color: themeToken.colorTextTertiary,
              overflow: 'hidden',
              textOverflow: 'ellipsis',
              whiteSpace: 'nowrap',
            }}>
              {activeConv
                ? [providerName(activeConv.provider_id), activeConv.model_name].filter(Boolean).join(' · ')
                : t('ai.selectConversation')}
            </span>
          </div>

          {isDraft && (
            <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
              <Select
                value={draftProviderId}
                placeholder={t('ai.selectProvider')}
                onChange={(v) => {
                  setDraftProviderId(v)
                  setDraftModelName('')
                }}
                style={{ width: 132 }}
                options={providers.filter((p: any) => p.is_active).map((p: any) => ({ value: p.id, label: p.name }))}
              />
              <Select
                value={draftModelName || undefined}
                placeholder={t('ai.selectModel')}
                onChange={(v) => setDraftModelName(v)}
                showSearch
                optionFilterProp="label"
                style={{ width: 156 }}
                popupMatchSelectWidth={false}
                options={getModels(draftProviderId).map((m: string) => ({ value: m, label: m }))}
              />
              <Select
                value={draftAssistantId || undefined}
                placeholder={t('ai.defaultAssistant')}
                allowClear
                onChange={(v) => setDraftAssistantId(v || 0)}
                style={{ width: 140 }}
                options={assistants.map((a: any) => ({ value: a.id, label: a.name }))}
              />
            </div>
          )}

          <Tooltip title={t('ai.exportChat')}>
            <Button icon={<ExportOutlined/>} disabled={!activeConv} onClick={handleExport}/>
          </Tooltip>
        </div>

        {/* 消息区 */}
        <div
          ref={scrollRef}
          onScroll={() => {
            const el = scrollRef.current
            if (el) nearBottomRef.current = el.scrollHeight - el.scrollTop - el.clientHeight < 80
          }}
          style={{ flex: 1, overflow: 'auto' }}
        >
          {!activeConv && draftMessages.length === 0 ? (
            <div style={{
              height: '100%',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              padding: 32,
            }}>
              <div style={{ textAlign: 'center', maxWidth: 460 }}>
                <div style={{
                  width: 64,
                  height: 64,
                  margin: '0 auto 20px',
                  borderRadius: 20,
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  background: 'linear-gradient(135deg, #0071e3, #0958d9)',
                  boxShadow: '0 12px 32px rgba(0, 113, 227, 0.35)',
                }}>
                  <RobotOutlined style={{ fontSize: 30, color: '#fff' }}/>
                </div>
                <div style={{
                  fontSize: 22,
                  fontWeight: 650,
                  letterSpacing: '-0.02em',
                  color: themeToken.colorText,
                }}>
                  NetPanel AI
                </div>
                <div style={{
                  marginTop: 8,
                  fontSize: 13,
                  lineHeight: 1.7,
                  color: themeToken.colorTextSecondary,
                }}>
                  {t('ai.selectConversation')}
                </div>
                <div style={{
                  marginTop: 22,
                  display: 'flex',
                  gap: 8,
                  flexWrap: 'wrap',
                  justifyContent: 'center',
                }}>
                  {[t('ai.suggestion1'), t('ai.suggestion2'), t('ai.suggestion3')].map((text) => (
                    <div
                      key={text}
                      onClick={() => {
                        setInputValue(text)
                        textareaRef.current?.focus()
                      }}
                      style={{
                        padding: '6px 12px',
                        borderRadius: 999,
                        border: `1px solid ${themeToken.colorBorderSecondary}`,
                        background: themeToken.colorFillQuaternary,
                        fontSize: 12,
                        color: themeToken.colorTextSecondary,
                        cursor: 'pointer',
                        transition: 'all 0.15s ease',
                      }}
                    >
                      {text}
                    </div>
                  ))}
                </div>
              </div>
            </div>
          ) : (
            <div style={{ maxWidth: 860, margin: '0 auto', padding: '24px 24px 8px' }}>
              {draftMessages.map((msg, idx) => renderMessage(msg, `draft-${idx}`))}
              {messages.map((msg, idx) => renderMessage(msg, `msg-${idx}`))}

              {streaming && streamContent && (
                <div style={{ display: 'flex', gap: 12, marginBottom: 22 }}>
                  <div style={{
                    width: 30,
                    height: 30,
                    borderRadius: 10,
                    flexShrink: 0,
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    background: 'linear-gradient(135deg, #0071e3, #0958d9)',
                    boxShadow: '0 4px 12px rgba(0, 113, 227, 0.3)',
                  }}>
                    <RobotOutlined style={{ color: '#fff', fontSize: 15 }}/>
                  </div>
                  <div style={{ flex: 1, minWidth: 0 }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 6 }}>
                      <span style={{ fontSize: 13, fontWeight: 600, color: themeToken.colorText }}>NetPanel AI</span>
                      <span style={{ fontSize: 11, color: themeToken.colorPrimary }}>{t('ai.streaming')}</span>
                    </div>
                    <div style={{
                      fontSize: 14,
                      lineHeight: 1.75,
                      color: themeToken.colorText,
                      whiteSpace: 'pre-wrap',
                      wordBreak: 'break-word',
                    }}>
                      {streamContent}
                      <span className="caret-blink" style={{ marginLeft: 2, color: themeToken.colorPrimary }}>▊</span>
                    </div>
                  </div>
                </div>
              )}

              {streaming && !streamContent && (
                <div style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '4px 0 12px' }}>
                  <Spin size="small"/>
                  <span style={{ fontSize: 13, color: themeToken.colorTextTertiary }}>{t('ai.streaming')}</span>
                </div>
              )}
            </div>
          )}
        </div>

        {/* 输入区 */}
        <div style={{ padding: '12px 16px 14px', borderTop: `1px solid ${themeToken.colorBorderSecondary}` }}>
          <div style={{ maxWidth: 860, margin: '0 auto' }}>
            <div className="ai-composer" style={{
              border: `1px solid ${composerFocused ? themeToken.colorPrimary : themeToken.colorBorder}`,
              borderRadius: 14,
              padding: '6px 8px 6px 10px',
              background: themeToken.colorBgContainer,
              boxShadow: composerFocused
                ? `0 0 0 3px ${themeToken.colorPrimaryBg}`
                : '0 6px 20px rgba(0, 0, 0, 0.06)',
              transition: 'border-color 0.2s ease, box-shadow 0.2s ease',
            }}>
              <TextArea
                ref={textareaRef}
                className="ai-chat-textarea"
                value={inputValue}
                onChange={(e) => setInputValue(e.target.value)}
                onKeyDown={handleKeyDown}
                onFocus={() => setComposerFocused(true)}
                onBlur={() => setComposerFocused(false)}
                placeholder={t('ai.typeMessage')}
                autoSize={{ minRows: 1, maxRows: 6 }}
                variant="borderless"
                disabled={streaming}
              />
              <div style={{
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'space-between',
                gap: 8,
                marginTop: 2,
              }}>
                <span style={{ fontSize: 11, color: themeToken.colorTextQuaternary }}>
                  {t('ai.inputHint')}
                </span>
                <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                  {streaming && (
                    <Button
                      size="small"
                      icon={<StopOutlined/>}
                      onClick={() => abortRef.current?.abort()}
                      style={{ borderRadius: 8 }}
                    >
                      {t('ai.stopGenerating')}
                    </Button>
                  )}
                  <Tooltip title={t('ai.sendMessage')}>
                    <Button
                      type="primary"
                      icon={<SendOutlined/>}
                      onClick={sendMessage}
                      disabled={streaming || !inputValue.trim()}
                      style={{ borderRadius: 8, height: 32 }}
                    />
                  </Tooltip>
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* 重命名弹窗 */}
      <Modal
        title={t('ai.renameChat')}
        open={!!renaming}
        onOk={handleRename}
        onCancel={() => setRenaming(null)}
        destroyOnHidden
      >
        <Input
          value={renaming?.title || ''}
          onChange={(e) => setRenaming(renaming ? { ...renaming, title: e.target.value } : null)}
          placeholder={t('ai.chatTitle')}
          onPressEnter={handleRename}
        />
      </Modal>

      <style>{`
        .ai-composer .ai-chat-textarea.ant-input,
        .ai-composer .ai-chat-textarea.ant-input:hover,
        .ai-composer .ai-chat-textarea.ant-input:focus,
        .ai-composer .ai-chat-textarea.ant-input-focused {
          background: transparent !important;
          border: none !important;
          box-shadow: none !important;
          backdrop-filter: none !important;
          -webkit-backdrop-filter: none !important;
          padding: 2px 4px !important;
          color: inherit !important;
          resize: none !important;
        }
      `}</style>
    </div>
  )
}

export default AiChat
