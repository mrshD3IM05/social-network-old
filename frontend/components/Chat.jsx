'use client'

import { imageUrl } from '@/lib/api'
import { IMAGE_ACCEPT, LIMITS } from '@/lib/validate'
import CharCount from './CharCount'
import EmojiPicker from './EmojiPicker'
import Icon from './Icon'
import MessageContent from './MessageContent'

// The scrolling list of a chat; `children` renders the messages.
export function ChatMessages({ chat, empty, children }) {
  return (
    <div ref={chat.listRef} className="chat-messages" onScroll={chat.onScroll}>
      {chat.messages === null && <p className="loading">Loading messages…</p>}
      {chat.hasMore && (
        <button ref={chat.loadMoreRef} type="button" className="btn btn-light chat-load-more" onClick={chat.loadOlder} disabled={chat.loadingMore}>
          {chat.loadingMore ? 'Loading…' : 'Load older messages'}
        </button>
      )}
      {chat.messages?.length === 0 && <p className="chat-note">{empty}</p>}
      {children}
    </div>
  )
}

// What goes inside a bubble: text, pictures and time.
export function MessageBody({ msg }) {
  return (
    <>
      {msg.content && <MessageContent content={msg.content} />}
      {msg.images?.length > 0 && (
        <span className="bubble-images">
          {msg.images.map(id => <img key={id} src={imageUrl(id)} alt="" />)}
        </span>
      )}
      <time className="message-time" dateTime={msg.created_at}>
        {new Date(msg.created_at).toLocaleString(undefined, { day: 'numeric', month: 'short', year: 'numeric', hour: '2-digit', minute: '2-digit' })}
      </time>
    </>
  )
}

// The error line, the picked images and the input row under a chat.
export function ChatForm({ chat, placeholder }) {
  const { text, files } = chat
  return (
    <>
      {chat.error && <p className="error chat-error">{chat.error}</p>}

      {files.length > 0 && (
        <p className="chat-files">
          {files.length} image{files.length > 1 ? 's' : ''} ready
          <button type="button" className="link-button" onClick={chat.clearFiles}>remove</button>
        </p>
      )}

      <form className="chat-form" onSubmit={chat.send} noValidate>
        <label className="icon-button" title="Add a photo or GIF">
          <Icon name="image" size={16} />
          <input ref={chat.fileRef} type="file" accept={IMAGE_ACCEPT} multiple hidden onChange={chat.pickFiles} />
        </label>
        <EmojiPicker onPick={emoji => chat.type(text + emoji)} />
        <input value={text} maxLength={LIMITS.message} onChange={e => chat.type(e.target.value)} placeholder={placeholder} />
        <CharCount value={text} max={LIMITS.message} />
        <button className="btn" title="Send" disabled={chat.sending || (!text.trim() && files.length === 0)}>
          <Icon name="send" size={16} />
        </button>
      </form>
    </>
  )
}
