import { useEffect, useRef, useCallback } from 'react'

interface WSMessage {
  type: 'output' | 'status' | 'error'
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

    function connect() {
      const ws = new WebSocket(url)
      wsRef.current = ws

      ws.onmessage = (event) => {
        try {
          const msg = JSON.parse(event.data) as WSMessage
          handlerRef.current(msg)
        } catch {
          // ignore parse errors
        }
      }

      ws.onclose = () => {
        wsRef.current = null
      }

      ws.onerror = () => {
        ws.close()
      }
    }

    connect()

    return () => {
      wsRef.current?.close()
      wsRef.current = null
    }
  }, [sessionId])

  const send = useCallback((message: string) => {
    wsRef.current?.send(JSON.stringify({ type: 'message', content: message }))
  }, [])

  return { send }
}
