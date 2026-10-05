'use client'

import { socketUrl } from './api'
import { forgetMe } from './userStore'

// Retries back off from a second up to MAX_DELAY. After MAX_ATTEMPTS of them we
// stop and say so instead of trying forever: that is what covers a logout that
// happened while this tab was asleep, and so never saw the 4401.
const MAX_ATTEMPTS = 10
const MAX_DELAY = 30000

// A connection that is gone for good will never carry what is typed into it, so
// the queue is bounded rather than left to grow for as long as we are offline.
const MAX_QUEUE = 100

let socket = null
let connecting = false
let stopped = false // the app closed this on purpose (logout)
let revoked = false // the server ended the session behind it
let everOpened = false // has this page seen a working connection yet
let attempts = 0
let onlineBound = false
let reconnectTimer = null
const listeners = new Set()
const statusListeners = new Set()

// closed | connecting | online | offline | ended | unreachable
let status = 'closed'

function setStatus(next) {
  if (next === status) return
  status = next
  for (const cb of statusListeners) {
    try {
      cb(status)
    } catch (err) {
      // ignore listener errors
    }
  }
}

function emit(data) {
  for (const cb of listeners) {
    try {
      cb(data)
    } catch (err) {
      // ignore listener errors
    }
  }
  if (typeof window !== 'undefined') {
    try {
      window.dispatchEvent(new CustomEvent('ws:message', { detail: data }))
    } catch (err) {
      // ignore
    }
  }
}

function scheduleReconnect() {
  if (stopped || revoked || reconnectTimer) return
  if (attempts >= MAX_ATTEMPTS) {
    setStatus('unreachable')
    return
  }
  const delay = Math.min(1000 * 2 ** attempts, MAX_DELAY)
  attempts += 1
  reconnectTimer = setTimeout(() => {
    reconnectTimer = null
    connect()
  }, delay)
}

// Coming back from airplane mode should not have to sit out the backoff. Bound
// on first use rather than at import, so this module stays free of side effects
// when it is evaluated outside a browser.
function bindOnline() {
  if (onlineBound || typeof window === 'undefined') return
  onlineBound = true
  window.addEventListener('online', () => {
    if (stopped || revoked) return
    if (reconnectTimer) {
      clearTimeout(reconnectTimer)
      reconnectTimer = null
    }
    attempts = 0
    connect()
  })
}

const queue = []

function flushQueue() {
  if (!socket || socket.readyState !== WebSocket.OPEN) return
  while (queue.length) socket.send(queue.shift())
}

function connect() {
  if (stopped || revoked) return
  if (socket && (socket.readyState === WebSocket.OPEN || socket.readyState === WebSocket.CONNECTING)) {
    return
  }
  if (connecting) return
  connecting = true
  // Not before the first successful open: on a normal page load the connection
  // is up in a few milliseconds, and saying "Connecting…" for that long would
  // flash a banner on every single page. Once we have seen a working connection
  // go down, though, it is worth saying so straight away.
  if (everOpened) setStatus('connecting')

  // Every handler below works on the connection it was given, not on whatever
  // socket.js points at now. A page can replace the connection (4401 then a new
  // login) while the old one's events are still in flight, and a late event from
  // a socket nobody is waiting on must not close or revive the current one.
  let connection
  try {
    connection = new WebSocket(socketUrl())
  } catch (err) {
    connecting = false
    setStatus('offline')
    scheduleReconnect()
    return
  }
  socket = connection

  connection.onopen = () => {
    if (socket !== connection) return
    connecting = false
    attempts = 0
    everOpened = true
    setStatus('online')
    flushQueue()
  }

  connection.onmessage = (e) => {
    if (socket !== connection) return
    try {
      const data = JSON.parse(e.data)
      emit(data)
    } catch (err) {
      // ignore malformed messages
    }
  }

  connection.onclose = (event) => {
    if (socket !== connection) return
    connecting = false
    socket = null
    if (stopped) return
    // The server ends a connection whose session is gone with this status: 4401 is
    // in the 4000-4999 range the WebSocket standard reserves for applications, so
    // it arrives as event.code and cannot be mistaken for a dropped connection.
    if (event.code === 4401) {
      // The session behind this connection is gone. Reconnecting would only
      // present the same dead cookie, so stop and let the page say so.
      revoked = true
      queue.length = 0
      setStatus('ended')
      forgetMe()
      return
    }
    setStatus('offline')
    scheduleReconnect()
  }

  connection.onerror = () => {
    try {
      connection.close()
    } catch (err) {
      // ignore
    }
  }
}

export function ensureSocket() {
  bindOnline()
  stopped = false
  revoked = false
  attempts = 0
  connect()
}

// Sends one payload as JSON. Anything typed before the connection opened is
// queued and goes out on open, so pages do not have to own a socket.
// False means it did not go out, either because the session ended or because the
// connection is not ready yet.
export function sendWs(payload) {
  if (stopped || revoked) return false
  if (!socket || socket.readyState === WebSocket.CLOSED) connect()
  if (stopped || revoked) return false
  if (!socket || socket.readyState === WebSocket.CLOSED) return false
  if (socket.readyState === WebSocket.OPEN) {
    socket.send(JSON.stringify(payload))
    return true
  }
  // Typing means "right now" and means nothing late, so it is dropped rather
  // than queued. Otherwise an open chat page piles up one of these per keystroke
  // for as long as it stays offline, which is also most of the queue.
  if (payload.type === 'typing') return true
  // Reaching this drops the oldest waiting message, so it is a last resort for a
  // page stuck sending into a connection that is never coming back.
  if (queue.length >= MAX_QUEUE) queue.shift()
  queue.push(JSON.stringify(payload))
  return true
}

export function subscribe(cb) {
  listeners.add(cb)
  return () => listeners.delete(cb)
}

export function closeSocket() {
  stopped = true
  queue.length = 0
  if (reconnectTimer) {
    clearTimeout(reconnectTimer)
    reconnectTimer = null
  }
  if (socket) {
    try {
      socket.close()
    } catch (err) {
      // ignore
    }
    socket = null
  }
  connecting = false
  setStatus('closed')
}

// The state of the one shared connection, for the parts of the page that show
// whether anything is live.
export function getStatus() {
  return status
}

// Returns the function to call when the component goes away.
export function onStatusChange(listener) {
  statusListeners.add(listener)
  return () => statusListeners.delete(listener)
}

// Puts a hand back on the retry loop after the page decided to give up on it.
export function retryNow() {
  if (stopped || revoked) return
  if (reconnectTimer) {
    clearTimeout(reconnectTimer)
    reconnectTimer = null
  }
  attempts = 0
  connect()
}