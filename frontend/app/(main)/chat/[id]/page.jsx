'use client'

import { useEffect, useState } from 'react'
import Link from 'next/link'
import { useParams } from 'next/navigation'
import { apiGet } from '@/lib/api'
import { useMe } from '@/lib/useMe'
import { markRead } from '@/lib/unread'
import useChat from '@/lib/useChat'
import Avatar from '@/components/Avatar'
import { ChatForm, ChatMessages, MessageBody } from '@/components/Chat'
import Icon from '@/components/Icon'

// A private conversation with one user, live over the WebSocket.
export default function ConversationPage() {
  const { id } = useParams()
  const otherId = Number(id)
  const { me } = useMe()
  const [other, setOther] = useState(null)
  const [missing, setMissing] = useState(false)
  const chat = useChat(`/messages/${id}`, { to_user_id: otherId })

  useEffect(() => {
    apiGet(`/user/${id}`)
      .then(setOther)
      .catch(err => {
        // 400 = not a number, 404 = nobody has that id
        if (err.status === 400 || err.status === 404) setMissing(true)
        else setOther({ first_name: 'User', last_name: id })
      })
    markRead(otherId)
  }, [id, otherId])

  if (missing) {
    return (
      <Locked title="This user does not exist" text="The link is wrong, or the account was removed.">
        <Link href="/chat" className="btn">Back to messages</Link>
      </Locked>
    )
  }

  if (!me || !other) return <p className="loading">Loading…</p>

  // the API refuses when neither of you follows the other
  if (chat.historyError?.status === 403) {
    return (
      <Locked title={`${other.first_name} is not in your contacts`} text="One of you has to follow the other before you can write to each other.">
        <Link href={`/profile/${otherId}`} className="btn">Open their profile</Link>
      </Locked>
    )
  }

  return (
    <section className="card chat">
      <header className="chat-header">
        <Link href="/chat" className="icon-button" title="Back"><Icon name="back" /></Link>
        <Link href={`/profile/${otherId}`} aria-label="Open their profile">
          <Avatar user={other} size={38} />
        </Link>
        <div>
          <Link href={`/profile/${otherId}`} className="chat-name">
            {other.first_name} {other.last_name}
          </Link>
          <p className="meta">{chat.typing ? 'typing…' : 'Live conversation'}</p>
        </div>
      </header>

      <ChatMessages chat={chat} empty="No messages yet. Say hello.">
        {chat.messages?.map(msg => (
          <div key={msg.id} className={msg.from_user_id === me.id ? 'bubble mine' : 'bubble'}>
            <MessageBody msg={msg} />
          </div>
        ))}
        {chat.typing > 0 && <p className="typing">{other.first_name} is typing…</p>}
      </ChatMessages>

      <ChatForm chat={chat} placeholder="Write a message…" />
    </section>
  )
}

function Locked({ title, text, children }) {
  return (
    <div className="card locked">
      <span className="locked-icon"><Icon name="lock" size={22} /></span>
      <h2>{title}</h2>
      <p className="subtitle">{text}</p>
      {children}
    </div>
  )
}
