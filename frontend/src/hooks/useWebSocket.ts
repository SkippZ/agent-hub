import { useEffect, useRef, useCallback } from 'react'

interface WSMessage {
  type: 'output' | 'status' | 'error' | 'reasoning'
  data?: string
  status?: string
  message?: string
}

type MessageHandler = (msg: WSMessage) => void

export function useWebSocket(sessionId: string | undefined, onMessage: MessageHandler) {
  const wsRef = useRef<WebSocket | null>(null)
  const handlerRef = useRef<MessageHandler>(onMessage)
  handlerRef.current = onMessage

  useEffect(() => {
    if (!sessionId) return

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const host = window.location.hostname === 'localhost' ? 'localhost:8080' : window.location.host
    const url = `${protocol}//${host}/ws/sessions/${sessionId}`
    console.log(`[WS] connecting to ${url}`)

    function connect() {
      const ws = new WebSocket(url)
      wsRef.current = ws

      ws.onopen = () => console.log(`[WS] connected (session=${sessionId})`)

      ws.onmessage = (event) => {
        try {
          const msg = JSON.parse(event.data) as WSMessage
          if (msg.type === 'output' || msg.type === 'reasoning') {
            const preview = (msg.data ?? '').slice(0, 100)
            console.log(`[WS] ${msg.type}: ${JSON.stringify(preview)}`)
          } else {
            console.log(`[WS] ${msg.type}:`, msg.status ?? msg.message ?? '')
          }
          handlerRef.current(msg)
        } catch {
          console.warn('[WS] failed to parse message:', event.data)
        }
      }

      ws.onclose = (ev) => {
        console.log(`[WS] disconnected (code=${ev.code}, session=${sessionId})`)
        wsRef.current = null
      }

      ws.onerror = () => {
        console.error(`[WS] connection error (session=${sessionId})`)
        ws.close()
      }
    }

    connect()

    return () => {
      console.log(`[WS] cleanup (session=${sessionId})`)
      wsRef.current?.close()
      wsRef.current = null
    }
  }, [sessionId])

  const send = useCallback((message: string) => {
    const payload = JSON.stringify({ type: 'message', content: message })
    console.log(`[WS] send: ${JSON.stringify({ type: 'message', content: message.slice(0, 80) })}`)
    wsRef.current?.send(payload)
  }, [])

  return { send }
}
