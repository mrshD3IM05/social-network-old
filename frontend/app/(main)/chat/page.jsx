'use client'

import { useEffect, useRef, useState } from 'react'
import usePaged from '@/lib/usePaged'
import { useMe } from '@/lib/useMe'
import { getUnread, onUnreadChange } from '@/lib/unread'
import { subscribe } from '@/lib/socket'
import Empty from '@/components/Empty'
import Icon from '@/components/Icon'
import LoadMore from '@/components/LoadMore'
import PageHeader from '@/components/PageHeader'
import PersonRow from '@/components/PersonRow'

// The people you can message: one of you follows the other. Newest conversation first.
export default function ChatListPage() {
  const contacts = usePaged('/contacts')
  const [unread, setUnread] = useState(getUnread)
  const { me } = useMe()
  const contactsRef = useRef(contacts) // the socket handler reads the current list
  contactsRef.current = contacts

  useEffect(() => onUnreadChange(setUnread), [])

  // a new message moves that person to the top
  useEffect(() => {
    if (!me) return
    return subscribe(data => {
      if (data.type !== 'message') return
      const msg = data.message
      if (msg.group_id || (msg.to_user_id !== me.id && msg.from_user_id !== me.id)) return
      const otherId = msg.from_user_id === me.id ? msg.to_user_id : msg.from_user_id
      const { items, setItems, reload } = contactsRef.current
      const person = items?.find(p => p.id === otherId)
      if (person) setItems([person, ...items.filter(p => p.id !== otherId)])
      else reload() // not shown yet: the first page now starts with them
    })
  }, [me])

  return (
    <>
      <PageHeader title="Messages" subtitle="Pick someone you follow, or who follows you." />

      {contacts.error && <p className="error">{contacts.error.message}</p>}
      {contacts.items === null && !contacts.error && <p className="loading">Loading…</p>}
      {contacts.items?.length === 0 && (
        <Empty title="No one to message yet">Follow someone, or get them to follow you, to start a conversation.</Empty>
      )}

      <div className="card list">
        {contacts.items?.map(person => (
          <PersonRow key={person.id} person={person} href={`/chat/${person.id}`}>
            {unread.has(person.id) && <span className="menu-dot" title="New message" />}
            <Icon name="chat" size={16} />
          </PersonRow>
        ))}
      </div>
      <LoadMore list={contacts} />
    </>
  )
}
