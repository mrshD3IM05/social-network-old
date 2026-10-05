'use client'

import { socketUrl } from './api'
import { forgetMe } from './userStore'

// One WebSocket for the whole app. Reconnects with backoff (1s → 30s) and gives
// up after MAX_ATTEMPTS; a 4401 close means the session is gone, so it stops.
const MAX_ATTEMPTS = 10
const MAX_DELAY = 30000
const MAX_QUEUE = 100

let socket = null
let stopped = false // closed on purpose (logout)
let revoked = false // the server ended the session (4401)
let everOpened = false
let attempts = 0
let onlineBound = false
let reconnectTimer = null
let status = 'closed' // closed | connecting | online | offline | ended | unreachable
const queue = []
const listeners = new Set()
const statusListeners = new Set()

function setStatus(next) {
  if (next === status) return
  status = next
  statusListeners.forEach(cb => cb(status))
}

function clearTimer() {
  clearTimeout(reconnectTimer)
  reconnectTimer = null
}

function scheduleReconnect() {
  if (stopped || revoked || reconnectTimer) return
  if (attempts >= MAX_ATTEMPTS) return setStatus('unreachable')
  const delay = Math.min(1000 * 2 ** attempts++, MAX_DELAY)
  reconnectTimer = setTimeout(() => {
    reconnectTimer = null
    connect()
  }, delay)
}

function connect() {
  if (stopped || revoked) return
  if (socket && socket.readyState <= WebSocket.OPEN) return // CONNECTING or OPEN
  // no "Connecting…" banner on the first load, only after a working connection dropped
  if (everOpened) setStatus('connecting')

  const connection = new WebSocket(socketUrl())
  socket = connection
  // handlers ignore events from a connection that has since been replaced
  connection.onopen = () => {
    if (socket !== connection) return
    attempts = 0
    everOpened = true
    setStatus('online')
    while (queue.length) connection.send(queue.shift())
  }
  connection.onmessage = e => {
    if (socket !== connection) return
    let data
    try {
      data = JSON.parse(e.data)
    } catch {
      return
    }
    listeners.forEach(cb => cb(data))
  }
  connection.onclose = event => {
    if (socket !== connection) return
    socket = null
    if (stopped) return
    if (event.code === 4401) {
      revoked = true
      queue.length = 0
      setStatus('ended')
      forgetMe()
      return
    }
    setStatus('offline')
    scheduleReconnect()
  }
  connection.onerror = () => connection.close()
}

// Starts again from scratch: after a login, or when the user taps "Try again".
export function retryNow() {
  if (stopped || revoked) return
  clearTimer()
  attempts = 0
  connect()
}

export function ensureSocket() {
  if (!onlineBound) {
    onlineBound = true
    window.addEventListener('online', retryNow) // back from airplane mode: skip the backoff
  }
  stopped = false
  revoked = false
  retryNow()
}

// Sends a payload as JSON, queued until the connection opens. False if it cannot go out.
export function sendWs(payload) {
  if (stopped || revoked) return false
  if (!socket) connect()
  if (!socket) return false
  if (socket.readyState === WebSocket.OPEN) {
    socket.send(JSON.stringify(payload))
    return true
  }
  if (payload.type === 'typing') return true // stale typing is useless, don't queue it
  if (queue.length >= MAX_QUEUE) queue.shift()
  queue.push(JSON.stringify(payload))
  return true
}

export function closeSocket() {
  stopped = true
  queue.length = 0
  clearTimer()
  socket?.close()
  socket = null
  setStatus('closed')
}

export function subscribe(cb) {
  listeners.add(cb)
  return () => listeners.delete(cb)
}

export const getStatus = () => status

export function onStatusChange(cb) {
  statusListeners.add(cb)
  return () => statusListeners.delete(cb)
}
