'use client'

import { useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react'
import Link from 'next/link'
import { useParams } from 'next/navigation'
import { apiGet, apiUpload, imageUrl } from '@/lib/api'
import { useMe } from '@/lib/useMe'
import { sendWs, subscribe } from '@/lib/socket'
import { IMAGE_ACCEPT, LIMITS, checkImageFiles, checkText } from '@/lib/validate'
import { markRead } from '@/lib/unread'
import { useThrottle } from '@/lib/timing'
import useMessageHistory from '@/lib/useMessageHistory'
import CharCount from '@/components/CharCount'
import Avatar from '@/components/Avatar'
import Icon from '@/components/Icon'
import EmojiPicker from '@/components/EmojiPicker'
import MessageContent from '@/components/MessageContent'

// A private conversation with one user, in real time over a WebSocket.
export default function ConversationPage() {
  const { id } = useParams()
  const otherId = Number(id)
  const { me } = useMe()
  const [other, setOther] = useState(null)
  const [text, setText] = useState('')
  const [files, setFiles] = useState([])
  const [typing, setTyping] = useState(false)
  const [blocked, setBlocked] = useState(false)
  const [missing, setMissing] = useState(false)
  const [error, setError] = useState('')
  const [sending, setSending] = useState(false)
  const messageListRef = useRef(null)
  const loadMoreButtonRef = useRef(null)
  const preservedScrollRef = useRef(null)
  const fileRef = useRef(null)
  const history = useMessageHistory(`/messages/${id}`)
  const { messages, setMessages, hasMore, loadingMore, error: historyError, loadMore } = history

  useEffect(() => {
    apiGet(`/user/${id}`)
      .then(setOther)
      // 400 = the id is not a number, 404 = nobody has that id
      .catch(err => {
        if (err.status === 400 || err.status === 404) setMissing(true)
        else setOther({ first_name: 'User', last_name: id })
      })

    // opening the conversation means you read it, so its dot goes away
    markRead(otherId)

    // The one app-wide connection lives in lib/socket; this page only listens.
    let typingTimer = null

    const unsub = subscribe(data => {
      if (data.type === 'message') {
        const msg = data.message
        // keep only the messages of this conversation
        if (msg.from_user_id === otherId || msg.to_user_id === otherId) {
          setMessages(list => ((list || []).some(m => m.id === msg.id) ? list : [...(list || []), msg]))
        }
      }
      // "typing" only means right now, so it fades on its own
      if (data.type === 'typing' && data.from_user_id === otherId) {
        setTyping(true)
        clearTimeout(typingTimer)
        typingTimer = setTimeout(() => setTyping(false), 3000)
      }

      if (data.type === 'error') setError(data.error)

    })

    // stop listening when we leave the page; the connection itself stays up
    return () => {
      clearTimeout(typingTimer)
      unsub()
    }
  }, [id, otherId])

  useEffect(() => {
    if (!historyError) return
    // the API refuses when neither of you follows the other
    if (historyError.status === 403) setBlocked(true)
    else setError(historyError.message)
  }, [historyError])

  // Prepending older history must not move the message currently being read.
  useLayoutEffect(() => {
    const preserved = preservedScrollRef.current
    const list = messageListRef.current
    if (preserved && list) {
      list.scrollTop = preserved.top + list.scrollHeight - preserved.height
      preservedScrollRef.current = null
    } else if (messages !== null && list) {
      // Initial history and new socket messages open directly at newest item.
      list.scrollTop = list.scrollHeight
    }
  }, [messages])

  useEffect(() => {
    if (typing && messageListRef.current) messageListRef.current.scrollTop = messageListRef.current.scrollHeight
  }, [typing])

  const loadOlderMessages = useCallback(() => {
    const list = messageListRef.current
    if (list) preservedScrollRef.current = { top: list.scrollTop, height: list.scrollHeight }
    loadMore()
  }, [loadMore])

  const loadWhenSeen = useCallback(() => {
    if (messageListRef.current?.scrollTop <= 80) loadOlderMessages()
  }, [loadOlderMessages])

  // Reaching the small top button loads the next older page. The same
  // throttled loader remains available through a click.
  useEffect(() => {
    const button = loadMoreButtonRef.current
    if (!hasMore || loadingMore || !button || !('IntersectionObserver' in window)) return
    const observer = new IntersectionObserver(
      entries => {
        if (entries[0].isIntersecting) loadOlderMessages()
      },
      { root: messageListRef.current, rootMargin: '80px 0px 0px' },
    )
    observer.observe(button)
    return () => observer.disconnect()
  }, [hasMore, loadingMore, loadOlderMessages])

  // Tell the other side we are writing, at most once every two seconds.
  // The trailing call keeps "typing…" alive until the last keystroke.
  const sendTyping = useThrottle(() => {
    sendWs({ type: 'typing', to_user_id: otherId })
  }, 2000)

  function onType(e) {
    setText(e.target.value)
    sendTyping()
  }

  async function pickFiles(e) {
    const picked = Array.from(e.target.files)
    const problem = await checkImageFiles(picked)
    setError(problem)
    setFiles(problem ? [] : picked)
    if (problem) e.target.value = ''
  }

  function clearFiles() {
    setFiles([])
    if (fileRef.current) fileRef.current.value = ''
  }

  // Message records use the socket; selected image files use multipart HTTP.
  async function send(e) {
    e.preventDefault()

    // a message needs text, a picture, or both
    const problem = text.trim()
      ? checkText('Your message', text, LIMITS.message)
      : files.length === 0 && 'Write something or add an image.'
    if (problem) {
      setError(problem)
      return
    }

    // the message is on its way, so a late "typing…" would be wrong
    sendTyping.cancel()
    setError('')
    setSending(true)
    try {
      if (files.length > 0) {
        const formData = new FormData()
        formData.append('to_user_id', otherId)
        formData.append('content', text.trim())
        for (const file of files) formData.append('files', file)
        const message = await apiUpload('/messages', formData)
        setMessages(list => {
          const current = list || []
          return current.some(existing => existing.id === message.id) ? current : [...current, message]
        })
      } else {
        const sent = sendWs({ type: 'message', to_user_id: otherId, content: text.trim() })
        if (!sent) throw new Error('Chat connection is not ready. Please try again.')
      }
      setText('')
      clearFiles()
    } catch (err) {
      setError(err.message)
    } finally {
      setSending(false)
    }
  }

  if (missing) {
    return (
      <div className="card locked">
        <span className="locked-icon"><Icon name="lock" size={22} /></span>
        <h2>This user does not exist</h2>
        <p className="subtitle">The link is wrong, or the account was removed.</p>
        <Link href="/chat" className="btn">Back to messages</Link>
      </div>
    )
  }

  if (!me || !other) return <p className="loading">Loading…</p>

  if (blocked) {
    return (
      <div className="card locked">
        <span className="locked-icon"><Icon name="lock" size={22} /></span>
        <h2>{other.first_name} is not in your contacts</h2>
        <p className="subtitle">
          One of you has to follow the other before you can write to each other.
        </p>
        <Link href={`/profile/${otherId}`} className="btn">Open their profile</Link>
      </div>
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
          <p className="meta">{typing ? 'typing…' : 'Live conversation'}</p>
        </div>
      </header>

      <div ref={messageListRef} className="chat-messages" onScroll={loadWhenSeen}>
        {hasMore && (
          <button ref={loadMoreButtonRef} type="button" className="btn btn-light chat-load-more" onClick={loadOlderMessages} disabled={loadingMore}>
            {loadingMore ? 'Loading…' : 'Load older messages'}
          </button>
        )}
        {messages?.length === 0 && <p className="chat-note">No messages yet. Say hello.</p>}
        {messages?.map(msg => (
          <div key={msg.id} className={msg.from_user_id === me.id ? 'bubble mine' : 'bubble'}>
            {msg.content && <MessageContent content={msg.content} />}
            {msg.images?.length > 0 && (
              <span className="bubble-images">
                {msg.images.map(fileId => <img key={fileId} src={imageUrl(fileId)} alt="" />)}
              </span>
            )}
            <time className="message-time" dateTime={msg.created_at}>
              {new Date(msg.created_at).toLocaleString(undefined, { day: 'numeric', month: 'short', year: 'numeric', hour: '2-digit', minute: '2-digit' })}
            </time>
          </div>
        ))}
        {typing && <p className="typing">{other.first_name} is typing…</p>}

      </div>

      {error && <p className="error chat-error">{error}</p>}

      {files.length > 0 && (
        <p className="chat-files">
          {files.length} image{files.length > 1 ? 's' : ''} ready
          <button type="button" className="link-button" onClick={clearFiles}>remove</button>
        </p>
      )}

      <form className="chat-form" onSubmit={send} noValidate>
        <label className="icon-button" title="Add a photo or GIF">
          <Icon name="image" size={16} />
          <input
            ref={fileRef}
            type="file"
            accept={IMAGE_ACCEPT}
            multiple
            hidden
            onChange={pickFiles}
          />
        </label>
        <EmojiPicker onPick={emoji => onType({ target: { value: text + emoji } })} />

        <input
          value={text}
          maxLength={LIMITS.message}
          onChange={onType}
          placeholder="Write a message…"
        />
        <CharCount value={text} max={LIMITS.message} />
        <button className="btn" title="Send" disabled={sending || (!text.trim() && files.length === 0)}>
          <Icon name="send" size={16} />
        </button>
      </form>
    </section>
  )
}
