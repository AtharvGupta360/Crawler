import { useEffect, useRef, useCallback } from 'react';
import { getToken } from '../api/client';

/**
 * WebSocket hook for real-time notifications.
 * @param {Function} onMessage – called with parsed JSON message
 * @param {boolean} enabled – whether to connect
 */
export function useWebSocket(onMessage, enabled = true) {
  const wsRef = useRef(null);
  const reconnectTimer = useRef(null);

  const connect = useCallback(() => {
    const token = getToken();
    if (!token || !enabled) return;

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const host = window.location.host;
    const url = `${protocol}//${host}/api/v1/ws/alerts?token=${token}`;

    try {
      const ws = new WebSocket(url);
      wsRef.current = ws;

      ws.onmessage = (event) => {
        try {
          const data = JSON.parse(event.data);
          onMessage(data);
        } catch {
          // ignore non-JSON messages
        }
      };

      ws.onclose = () => {
        // Reconnect after 5 seconds
        reconnectTimer.current = setTimeout(() => {
          connect();
        }, 5000);
      };

      ws.onerror = () => {
        ws.close();
      };
    } catch {
      // WebSocket not available
    }
  }, [onMessage, enabled]);

  useEffect(() => {
    connect();

    return () => {
      if (reconnectTimer.current) clearTimeout(reconnectTimer.current);
      if (wsRef.current) {
        wsRef.current.onclose = null; // prevent reconnect on intentional close
        wsRef.current.close();
      }
    };
  }, [connect]);

  return wsRef;
}
