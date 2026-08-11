import React, { useEffect, useRef, useState } from 'react'
import { Card, Select, Button, Space, App, Typography } from 'antd'
import { useTranslation } from 'react-i18next'
import { Terminal } from 'xterm'
import { FitAddon } from 'xterm-addon-fit'
import { WebLinksAddon } from 'xterm-addon-web-links'
import 'xterm/css/xterm.css'
import { monitorApi } from '../api'

const { Option } = Select
const { Text } = Typography

interface Server {
  id: number
  name: string
  display_name: string
  is_online: boolean
  access_type: string
}

const MonitorTerminal: React.FC = () => {
  const { t } = useTranslation()
  const { message } = App.useApp()
  const [servers, setServers] = useState<Server[]>([])
  const [selectedServerId, setSelectedServerId] = useState<number | null>(null)
  const [connected, setConnected] = useState(false)
  const terminalRef = useRef<HTMLDivElement>(null)
  const terminalInstance = useRef<Terminal | null>(null)
  const fitAddon = useRef<FitAddon | null>(null)
  const wsRef = useRef<WebSocket | null>(null)

  useEffect(() => {
    loadServers()
    return () => {
      disconnect()
    }
  }, [])

  useEffect(() => {
    if (terminalRef.current && !terminalInstance.current) {
      initTerminal()
    }
  }, [terminalRef.current])

  const loadServers = async () => {
    try {
      const response = await monitorApi.listServers()
      const onlineServers = (response.data || []).filter(
        (s: Server) => s.is_online && ['agent', 'ssh'].includes(s.access_type)
      )
      setServers(onlineServers)
    } catch (error: any) {
      message.error(error.message || '加载服务器列表失败')
    }
  }

  const initTerminal = () => {
    const term = new Terminal({
      cursorBlink: true,
      fontSize: 14,
      fontFamily: 'Consolas, "Courier New", monospace',
      theme: {
        background: '#0A0D12',
        foreground: '#F8F8F2',
        cursor: '#F8F8F2',
        black: '#000000',
        red: '#FF5555',
        green: '#50FA7B',
        yellow: '#F1FA8C',
        blue: '#BD93F9',
        magenta: '#FF79C6',
        cyan: '#8BE9FD',
        white: '#BFBFBF',
        brightBlack: '#4D4D4D',
        brightRed: '#FF6E67',
        brightGreen: '#5AF78E',
        brightYellow: '#F4F99D',
        brightBlue: '#CAA9FA',
        brightMagenta: '#FF92D0',
        brightCyan: '#9AEDFE',
        brightWhite: '#E6E6E6',
      },
      rows: 30,
      cols: 120,
    })

    fitAddon.current = new FitAddon()
    term.loadAddon(fitAddon.current)
    term.loadAddon(new WebLinksAddon())

    term.open(terminalRef.current!)
    fitAddon.current.fit()

    term.writeln('╔═══════════════════════════════════════════════════════════════╗')
    term.writeln('║         NetPanel 服务器监控 - 远程终端                        ║')
    term.writeln('╚═══════════════════════════════════════════════════════════════╝')
    term.writeln('')
    term.writeln('请先选择服务器，然后点击"连接"按钮建立会话。')
    term.writeln('')

    terminalInstance.current = term

    // 监听窗口大小变化
    window.addEventListener('resize', handleResize)
  }

  const handleResize = () => {
    if (fitAddon.current) {
      fitAddon.current.fit()
    }
  }

  const connect = async () => {
    if (!selectedServerId) {
      message.warning('请先选择服务器')
      return
    }

    if (!terminalInstance.current) {
      message.error('终端未初始化')
      return
    }

    const term = terminalInstance.current
    term.clear()
    term.writeln('正在连接到服务器...')

    try {
      // 获取认证 token
      const token = localStorage.getItem('token') || ''
      
      // 构建 WebSocket URL
      const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
      const host = window.location.host
      const wsUrl = `${protocol}//${host}/ws/terminal?server_id=${selectedServerId}&token=${token}`

      const ws = new WebSocket(wsUrl)

      ws.onopen = () => {
        setConnected(true)
        term.clear()
        term.writeln('✓ 已连接到服务器')
        term.writeln('')

        // 监听终端输入
        term.onData((data) => {
          if (ws.readyState === WebSocket.OPEN) {
            ws.send(
              JSON.stringify({
                type: 'input',
                data: data,
              })
            )
          }
        })
      }

      ws.onmessage = (event) => {
        try {
          const msg = JSON.parse(event.data)
          if (msg.type === 'output') {
            term.write(msg.data)
          } else if (msg.type === 'error') {
            term.writeln(`\r\n错误: ${msg.message}`)
          }
        } catch {
          // 如果不是 JSON，直接写入
          term.write(event.data)
        }
      }

      ws.onerror = (error) => {
        console.error('WebSocket 错误:', error)
        term.writeln('\r\n✗ 连接错误')
        setConnected(false)
      }

      ws.onclose = () => {
        term.writeln('\r\n✗ 连接已关闭')
        setConnected(false)
        wsRef.current = null
      }

      wsRef.current = ws
    } catch (error: any) {
      message.error(error.message || '连接失败')
      term.writeln(`\r\n✗ 连接失败: ${error.message}`)
    }
  }

  const disconnect = () => {
    if (wsRef.current) {
      wsRef.current.close()
      wsRef.current = null
    }
    setConnected(false)
    if (terminalInstance.current) {
      terminalInstance.current.writeln('\r\n已断开连接')
    }
  }

  const clearTerminal = () => {
    if (terminalInstance.current) {
      terminalInstance.current.clear()
    }
  }

  return (
    <div style={{ padding: '24px' }}>
      <Card
        title={
          <Space>
            <span>{t('monitor.remote_terminal')}</span>
            {connected && <Text type="success">● 已连接</Text>}
          </Space>
        }
        extra={
          <Space>
            <Select
              style={{ width: 250 }}
              placeholder="选择服务器"
              value={selectedServerId}
              onChange={setSelectedServerId}
              disabled={connected}
            >
              {servers.map((server) => (
                <Option key={server.id} value={server.id}>
                  {server.display_name || server.name}
                  {server.access_type === 'agent' && ' (Agent)'}
                  {server.access_type === 'ssh' && ' (SSH)'}
                </Option>
              ))}
            </Select>
            {!connected ? (
              <Button type="primary" onClick={connect} disabled={!selectedServerId}>
                连接
              </Button>
            ) : (
              <Button danger onClick={disconnect}>
                断开连接
              </Button>
            )}
            <Button onClick={clearTerminal}>清屏</Button>
          </Space>
        }
      >
        <div
          ref={terminalRef}
          style={{
            width: '100%',
            height: '600px',
            backgroundColor: '#0A0D12',
            padding: '10px',
            borderRadius: '4px',
          }}
        />
      </Card>
    </div>
  )
}

export default MonitorTerminal
