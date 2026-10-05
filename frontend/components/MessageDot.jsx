'use client'

import { useEffect, useState } from 'react'
import { getUnread, markUnread, onUnreadChange } from '@/lib/unread'
import { subscribe } from '@/lib/socket'

// A dot next to Messages while a conversation has unread messages
// (kept apart from the notification count on purpose).
export default function MessageDot({ myId }) {
  const [unread, setUnread] = useState(getUnread)

  useEffect(() => onUnreadChange(setUnread), [])

  useEffect(() => subscribe(data => {
    if (data.type !== 'message') return
    const msg = data.message
    const sentToMe = msg.to_user_id === myId && msg.from_user_id !== myId
    if (sentToMe && window.location.pathname !== `/chat/${msg.from_user_id}`) markUnread(msg.from_user_id)
  }), [myId])

  if (unread.size === 0) return null
  return <span className="menu-dot" title={`${unread.size} unread conversation(s)`} />
}
