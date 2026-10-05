'use client'

import { useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react'
import { useParams, useRouter } from 'next/navigation'
import Link from 'next/link'
import { apiDelete, apiGet, apiPost, apiPut, apiUpload, imageUrl } from '@/lib/api'
import { sendWs, subscribe } from '@/lib/socket'
import { useMe } from '@/lib/useMe'
import usePaged from '@/lib/usePaged'
import { useDebouncedValue, useThrottle } from '@/lib/timing'
import useMessageHistory from '@/lib/useMessageHistory'
import Modal from '@/components/Modal'
import Avatar from '@/components/Avatar'
import Icon from '@/components/Icon'
import CharCount from '@/components/CharCount'
import LoadMore from '@/components/LoadMore'
import PersonRow from '@/components/PersonRow'
import RequestRow from '@/components/RequestRow'
import PostForm from '@/components/PostForm'
import PostCard from '@/components/PostCard'
import EventCard from '@/components/EventCard'
import EventFormModal from '@/components/EventFormModal'
import { IMAGE_ACCEPT, LIMITS, checkImageFile, checkImageFiles, checkText } from '@/lib/validate'
import EmojiPicker from '@/components/EmojiPicker'
import MessageContent from '@/components/MessageContent'

// One group: an identity header (who, what, how many, the actions) and one
// tab per thing the group holds — posts, events, chat, members, and the
// creator's join requests. Outsiders only see the header; the API gates
// posts, events, chat and members to members.
export default function GroupDetailPage() {
  const { id } = useParams()
  const router = useRouter()
  const { me } = useMe()
  const [group, setGroup] = useState(null)
  const [eventsOpened, setEventsOpened] = useState(false) // events load on first visit of their tab
  const [tab, setTab] = useState('posts')
  const [notFound, setNotFound] = useState(false)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [showInvite, setShowInvite] = useState(false)
  const [showEventForm, setShowEventForm] = useState(false)
  const [showEdit, setShowEdit] = useState(false)
  const [answering, setAnswering] = useState(false) // answering an invitation

  const isMember = group?.is_member || group?.is_creator

  const load = useCallback(async () => {
    try {
      setGroup(await apiGet(`/groups/${id}`))
    } catch (err) {
      if (err.status === 404) setNotFound(true)
      else setError(err.message)
    }
  }, [id])

  // The member-only lists. Posts come 10 at a time, and only for members.
  const posts = usePaged(isMember ? `/groups/${id}/posts` : null)
  // members 10 at a time once their tab is opened (the group only carries the first page)
  const [membersOpened, setMembersOpened] = useState(false)
  const members = usePaged(isMember && (membersOpened || tab === 'members') ? `/groups/${id}/members` : null)
  useEffect(() => {
    if (tab === 'members') setMembersOpened(true)
  }, [tab])

  // events too, once their tab was opened (the tab count comes with the group)
  const events = usePaged(isMember && (eventsOpened || tab === 'events') ? `/groups/${id}/events` : null)
  // join requests, for the creator only, 10 at a time
  const requests = usePaged(group?.is_creator ? `/groups/${id}/join-requests` : null)
  // every shown request answered while more are waiting: fetch them
  useEffect(() => {
    if (requests.items?.length === 0 && requests.hasMore) requests.reload()
  }, [requests.items?.length, requests.hasMore])

  useEffect(() => {
    load()
  }, [load])

  useEffect(() => {
    if (tab === 'events') setEventsOpened(true)
  }, [tab])


  // Every action reports through the same notice/error pair and reloads the
  // group, so the counts and the status chip stay in sync.
  async function run(action, message) {
    setError('')
    setNotice('')
    try {
      await action()
      setNotice(message)
      load()
    } catch (err) {
      setError(err.message)
    }
  }

  // Deleting a post only touches that one card: drop it from state instead
  // of refetching, so the page keeps its scroll position and the tab count
  // stays correct without a reload.
  const postDeleted = postId => posts.setItems(list => (list ? list.filter(p => p.id !== postId) : list))

  function respondJoinRequest(request, accept) {
    return run(
      async () => {
        await apiPost(`/group-join-requests/${request.id}/${accept ? 'accept' : 'decline'}`)
        requests.setItems(list => list.filter(r => r.id !== request.id))
        if (accept && members.items) members.reload() // the new member joins the list
      },
      accept ? `${request.first_name} is now a member.` : 'Request declined.',
    )
  }

  // an invitation to this group, answered right here instead of on /groups
  async function respondInvitation(accept) {
    setAnswering(true)
    await run(
      () => apiPost(`/group-invitations/${group.invitation_id}/${accept ? 'accept' : 'decline'}`),
      accept ? `Welcome to ${group.title}!` : 'Invitation declined.',
    )
    setAnswering(false)
  }

  function removeMember(member) {
    if (!confirm(`Remove ${member.first_name} ${member.last_name} from the group?`)) return
    return run(
      async () => {
        await apiDelete(`/groups/${id}/members/${member.user_id}`)
        members.setItems(list => list?.filter(m => m.user_id !== member.user_id))
      },
      `${member.first_name} was removed from the group.`,
    )
  }

  // a member leaves by removing themselves; the creator deletes the group instead
  async function leaveGroup() {
    if (!confirm(`Leave ${group.title}?`)) return
    try {
      await apiDelete(`/groups/${id}/members/${me.id}`)
      router.push('/groups')
    } catch (err) {
      setError(err.message)
    }
  }

  async function deleteGroup() {
    if (!confirm('Delete this group? Its posts, events and messages are deleted too.')) return
    try {
      await apiDelete(`/groups/${id}`)
      router.push('/groups')
    } catch (err) {
      setError(err.message)
    }
  }

  function requestJoin() {
    return run(
      () => apiPost(`/groups/${id}/join-requests`),
      'Join request sent. The creator will review it.',
    )
  }

  if (notFound) {
    return (
      <div className="empty">
        <p className="empty-title">Group not found</p>
        <p>It does not exist, or you have no relation to it.</p>
        <Link href="/groups" className="btn">Back to groups</Link>
      </div>
    )
  }

  if (!group || !me) return <p className="loading">{error || 'Loading…'}</p>

  const tabs = [
    { key: 'posts', label: 'Posts' },
    { key: 'events', label: 'Events', count: group.event_count },
    { key: 'chat', label: 'Chat' },
    { key: 'members', label: 'Members', count: group.member_count },
    // only the loaded ones are known: "10+" while more pages are waiting
    ...(group.is_creator
      ? [{ key: 'requests', label: 'Requests', count: requests.hasMore ? `${requests.items.length}+` : requests.items?.length }]
      : []),
  ]

  return (
    <>
      {/* ------------------------------------------- identity header */}
      <header className="card group-hero">
        <div className="group-hero-top">
          <Avatar user={{ first_name: group.title, last_name: '', avatar: group.avatar }} size={64} />
          <div className="group-hero-title">
            <h1>{group.title}</h1>
            <p className="meta">
              {group.member_count} member{group.member_count === 1 ? '' : 's'}
              {group.creator && <> · created by {group.creator.first_name} {group.creator.last_name}</>}
              {' · '}{new Date(group.created_at).toLocaleDateString()}
            </p>
          </div>
          {group.is_creator ? (
            <span className="chip chip-accent">Creator</span>
          ) : group.is_member ? (
            <span className="chip">Member</span>
          ) : null}
        </div>

        <p className="group-hero-desc">{group.description || 'No description.'}</p>

        <div className="group-hero-actions">
          <AvatarStack members={group.members} total={group.member_count} />
          {isMember ? (
            <div className="group-hero-buttons">
              {group.is_creator && (
                <>
                  <button type="button" className="btn btn-light" onClick={() => setShowEdit(true)}>
                    <Icon name="edit" size={16} /> Edit
                  </button>
                  <button type="button" className="btn btn-light" onClick={deleteGroup}>
                    <Icon name="trash" size={16} /> Delete
                  </button>
                </>
              )}
              {!group.is_creator && (
                <button type="button" className="btn btn-light" onClick={leaveGroup}>
                  Leave
                </button>
              )}
              <button type="button" className="btn" onClick={() => setShowInvite(true)}>
                <Icon name="plus" size={16} /> Invite people
              </button>
            </div>
          ) : group.pending_join ? (
            <p className="meta group-hero-note">Join request sent — waiting for the creator.</p>
          ) : group.pending_invite ? (
            <div className="group-hero-buttons">
              <p className="meta group-hero-note">You are invited to this group.</p>
              <button type="button" className="btn" disabled={answering} onClick={() => respondInvitation(true)}>
                Accept
              </button>
              <button type="button" className="btn btn-light" disabled={answering} onClick={() => respondInvitation(false)}>
                Decline
              </button>
            </div>
          ) : (
            <button type="button" className="btn" onClick={requestJoin}>Request to join</button>
          )}
        </div>
      </header>

      {error && <p className="error">{error}</p>}
      {notice && <p className="notice">{notice}</p>}

      {/* Outsiders stop here: the API serves nothing else to them. */}
      {!isMember ? (
        <Empty title="Members only">
          Posts, events and members open up once you join this group.
        </Empty>
      ) : (
        <>
          <nav className="tabs">
            {tabs.map(({ key, label, count }) => (
              <button
                key={key}
                className={`tab${tab === key ? ' active' : ''}`}
                onClick={() => setTab(key)}
              >
                {label}
                {(typeof count === 'string' || count > 0) && <span className="tab-count">{count}</span>}
              </button>
            ))}
          </nav>

          {/* ------------------------------------------------- posts */}
          {tab === 'posts' && (
            <>
              <PostForm groupId={id} onPosted={posts.reload} />
              {posts.items === null && <p className="loading">Loading posts…</p>}
              {posts.items?.length === 0 && (
                <Empty title="No posts yet">Write the first one with the box above.</Empty>
              )}
              {posts.items?.map(post => (
                <PostCard
                  key={post.id}
                  post={post}
                  myId={me.id}
                  currentGroupId={id}
                  isGroupCreator={group.is_creator}
                  onDeleted={postDeleted}
                />
              ))}
              <LoadMore list={posts} />
            </>
          )}

          {/* ------------------------------------------------ events */}
          {tab === 'events' && (
            <>
              <div className="section-bar">
                <p className="eyebrow">Upcoming and past events</p>
                <button type="button" className="btn btn-sm" onClick={() => setShowEventForm(true)}>
                  <Icon name="plus" size={16} /> Create event
                </button>
              </div>
              {events.error && <p className="error">{events.error.message}</p>}
              {events.items === null && !events.error && <p className="loading">Loading events…</p>}
              {events.items?.length === 0 && (
                <Empty title="No events scheduled">
                  Create one and every member gets notified.
                </Empty>
              )}
              {events.items?.map(event => (
                <EventCard key={event.id} event={event} />
              ))}
              <LoadMore list={events} />
            </>
          )}

          {/* -------------------------------------------------- chat */}
          {tab === 'chat' && <GroupChat groupId={Number(id)} me={me} members={group.members} />}

          {/* ----------------------------------------------- members */}
          {tab === 'members' && members.items === null && <p className="loading">Loading members…</p>}
          {tab === 'members' && members.items !== null && (
            <>
            <div className="card list">
              {members.items.map(member => (
                <PersonRow key={member.user_id} person={member} href={`/profile/${member.user_id}`}>
                  {member.user_id === group.creator_id ? (
                    <span className="chip chip-accent">Creator</span>
                  ) : member.user_id === me.id ? (
                    <button
                      type="button"
                      className="btn btn-light btn-sm"
                      onClick={e => {
                        e.preventDefault() // don't follow the profile link
                        leaveGroup()
                      }}
                    >
                      Leave
                    </button>
                  ) : group.is_creator ? (
                    <button
                      type="button"
                      className="btn btn-light btn-sm"
                      onClick={e => {
                        e.preventDefault() // don't follow the profile link
                        removeMember(member)
                      }}
                    >
                      Remove
                    </button>
                  ) : (
                    <Icon name="arrow" size={16} />
                  )}
                </PersonRow>
              ))}
            </div>
            <LoadMore list={members} />
            </>
          )}

          {/* ---------------------------------------------- requests */}
          {tab === 'requests' && (
            requests.items === null ? (
              <p className="loading">Loading requests…</p>
            ) : requests.items.length === 0 ? (
              <Empty title="No pending requests">
                People asking to join this group land here.
              </Empty>
            ) : (
              <>
              <div className="card list">
                {requests.items.map(request => (
                  <RequestRow
                    key={request.id}
                    person={request}
                    href={`/profile/${request.user_id}`}
                    title={`${request.first_name} ${request.last_name}`}
                    subtitle={`@${request.nickname} · wants to join`}
                    onRespond={accept => respondJoinRequest(request, accept)}
                  />
                ))}
              </div>
              <LoadMore list={requests} />
              </>
            )
          )}
        </>
      )}

      {showInvite && (
        <InviteModal
          groupId={id}
          memberIds={new Set(group.members.map(m => m.user_id))} // the first page; the API catches the rest
          onClose={() => setShowInvite(false)}
          onInvited={name => setNotice(`Invitation sent to ${name}.`)}
        />
      )}

      {showEdit && (
        <EditGroupModal
          group={group}
          onClose={() => setShowEdit(false)}
          onSaved={() => {
            setShowEdit(false)
            setNotice('Group updated.')
            load()
          }}
        />
      )}

      {showEventForm && (
        <EventFormModal
          groupId={id}
          onClose={() => setShowEventForm(false)}
          onCreated={event => {
            setShowEventForm(false)
            events.reload() // the API returns events in date order, so re-read
            load() // and the tab count
            setNotice(`Event "${event.title}" created. Members have been notified.`)
          }}
        />
      )}
    </>
  )
}

// The group chat: the history from the API, then new messages live over the
// WebSocket (the server sends each group message to every member).
function GroupChat({ groupId, me, members }) {
  const [text, setText] = useState('')
  const [files, setFiles] = useState([])
  const [typing, setTyping] = useState('')
  const [error, setError] = useState('')
  const [sending, setSending] = useState(false)
  const messageListRef = useRef(null)
  const loadMoreButtonRef = useRef(null)
  const preservedScrollRef = useRef(null)
  const fileRef = useRef(null)
  const history = useMessageHistory(`/groups/${groupId}/messages`)
  const { messages, setMessages, hasMore, loadingMore, error: historyError, loadMore } = history

  // user id → person, for the "is typing" line: the members we have (the first
  // page) plus everyone who wrote a loaded message. Each message carries its
  // sender's name and photo itself.
  const people = Object.fromEntries(members.map(m => [m.user_id, m]))
  for (const msg of messages || []) {
    people[msg.from_user_id] ??= { first_name: msg.from_first_name, last_name: msg.from_last_name, avatar: msg.from_avatar }
  }

  useEffect(() => {
    // The one app-wide connection lives in lib/socket; this page only listens.
    let typingTimer = null

    const unsub = subscribe(data => {
      if (data.type === 'message' && data.message.group_id === groupId) {
        const msg = data.message
        setMessages(list => ((list || []).some(m => m.id === msg.id) ? list : [...(list || []), msg]))
      }
      // "someone is writing" only means right now, so it fades on its own
      if (data.type === 'typing' && data.group_id === groupId) {
        setTyping(data.from_user_id)
        clearTimeout(typingTimer)
        typingTimer = setTimeout(() => setTyping(''), 3000)
      }

      if (data.type === 'error') setError(data.error)

    })

    // stop listening when we leave the page; the connection itself stays up
    return () => {
      clearTimeout(typingTimer)
      unsub()
    }
  }, [groupId])

  useEffect(() => {
    if (historyError) setError(historyError.message)
  }, [historyError])

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

  // Tell the group we are writing, at most once every two seconds.
  // The trailing call keeps "typing…" alive until the last keystroke.
  const sendTyping = useThrottle(() => {
    sendWs({ type: 'typing', group_id: groupId })
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
        formData.append('group_id', groupId)
        formData.append('content', text.trim())
        for (const file of files) formData.append('files', file)
        const message = await apiUpload('/messages', formData)
        setMessages(list => {
          const current = list || []
          return current.some(existing => existing.id === message.id) ? current : [...current, message]
        })
      } else {
        const sent = sendWs({ type: 'message', group_id: groupId, content: text.trim() })
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

  return (
    <section className="card chat chat-group">
      <div ref={messageListRef} className="chat-messages" onScroll={loadWhenSeen}>
        {messages === null && <p className="loading">Loading messages…</p>}
        {hasMore && (
          <button ref={loadMoreButtonRef} type="button" className="btn btn-light chat-load-more" onClick={loadOlderMessages} disabled={loadingMore}>
            {loadingMore ? 'Loading…' : 'Load older messages'}
          </button>
        )}
        {messages?.length === 0 && <p className="chat-note">No messages yet. Say hello!</p>}
        {messages?.map(msg => {
          const mine = msg.from_user_id === me.id
          const author = { first_name: msg.from_first_name, last_name: msg.from_last_name, avatar: msg.from_avatar }
          return (
            <div key={msg.id} className={mine ? 'chat-line mine' : 'chat-line'}>
              {!mine && (
                <Link href={`/profile/${msg.from_user_id}`} aria-label={`${author.first_name}'s profile`}>
                  <Avatar user={author} size={28} />
                </Link>
              )}
              <div>
                {!mine && (
                  <small className="meta">
                    <Link href={`/profile/${msg.from_user_id}`} className="chat-name">
                      {author.first_name}
                    </Link>
                  </small>
                )}
                <div className={mine ? 'bubble mine' : 'bubble'}>
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
              </div>
            </div>
          )
        })}
        {typing && (
          <p className="typing">
            {people[typing] ? people[typing].first_name : 'Someone'} is typing…
          </p>
        )}

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
          placeholder="Write to the group…"
        />
        <CharCount value={text} max={LIMITS.message} />
        <button className="btn" title="Send" disabled={sending || (!text.trim() && files.length === 0)}>
          <Icon name="send" size={16} />
        </button>
      </form>
    </section>
  )
}

// Creator only: change the picture, the title and the description.
function EditGroupModal({ group, onClose, onSaved }) {
  const [title, setTitle] = useState(group.title)
  const [description, setDescription] = useState(group.description)
  const [picture, setPicture] = useState(null) // the new file, if one was picked
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const titleError = checkText('Title', title, LIMITS.groupTitle)
  const descriptionError = checkText('Description', description, LIMITS.groupDescription, { required: false })

  async function pickPicture(e) {
    const file = e.target.files[0]
    if (!file) return
    const problem = await checkImageFile(file)
    if (problem) {
      setError(problem)
      e.target.value = ''
      return
    }
    setError('')
    setPicture(file)
  }

  async function handleSubmit(e) {
    e.preventDefault()
    const problem = titleError || descriptionError
    if (problem) {
      setError(problem)
      return
    }
    setError('')
    setLoading(true)
    try {
      await apiPut(`/groups/${group.id}`, { title: title.trim(), description: description.trim() })
      if (picture) {
        const formData = new FormData()
        formData.append('avatar', picture)
        await apiUpload(`/groups/${group.id}/avatar`, formData)
      }
      onSaved()
    } catch (err) {
      setError(err.message)
      setLoading(false)
    }
  }

  return (
    <Modal title="Edit group" onClose={onClose}>
      <form onSubmit={handleSubmit} noValidate>
        <div className="photo-row">
          {picture ? (
            <img className="avatar" style={{ width: 64, height: 64 }} src={URL.createObjectURL(picture)} alt="" />
          ) : (
            <Avatar user={{ first_name: group.title, last_name: '', avatar: group.avatar }} size={64} />
          )}
          <label className="btn btn-light">
            <Icon name="camera" size={16} /> Change picture
            <input type="file" accept={IMAGE_ACCEPT} hidden onChange={pickPicture} />
          </label>
        </div>

        <label>Title</label>
        <input
          value={title}
          maxLength={LIMITS.groupTitle}
          className={error && titleError ? 'invalid' : undefined}
          onChange={e => setTitle(e.target.value)}
        />

        <label>Description <small>optional</small></label>
        <textarea
          rows={3}
          value={description}
          maxLength={LIMITS.groupDescription}
          className={error && descriptionError ? 'invalid' : undefined}
          onChange={e => setDescription(e.target.value)}
        />

        <div className="composer-bar">
          <CharCount value={description} max={LIMITS.groupDescription} />
          <button className="btn" disabled={loading || Boolean(titleError) || Boolean(descriptionError)}>
            {loading ? 'Saving…' : 'Save'}
          </button>
        </div>

        {error && <p className="error">{error}</p>}
      </form>
    </Modal>
  )
}

// The faces of the group, so you see who is in it without opening anything.
function AvatarStack({ members, total, shown = 5 }) {
  const rest = total - Math.min(shown, members.length)
  return (
    <div className="avatar-stack">
      {members.slice(0, shown).map(member => (
        <Avatar key={member.user_id} user={member} size={32} />
      ))}
      {rest > 0 && <span className="avatar-stack-more">+{rest}</span>}
    </div>
  )
}

// The same empty state the group page shows in half a dozen places.
function Empty({ title, children }) {
  return (
    <div className="empty">
      <p className="empty-title">{title}</p>
      <p>{children}</p>
    </div>
  )
}

// Pick someone from the people directory (GET /users, searched by the server,
// 10 at a time) and invite them. Members are filtered out; the API answers 409
// for anyone already invited.
function InviteModal({ groupId, memberIds, onClose, onInvited }) {
  const [search, setSearch] = useState('')
  const [invited, setInvited] = useState({}) // person id → 'Invited' | 'Member' once the API answered
  const [busyId, setBusyId] = useState(null)
  const [error, setError] = useState('')

  const query = useDebouncedValue(search, 250).trim() // search once typing pauses
  const people = usePaged(`/users?q=${encodeURIComponent(query)}`)

  async function invite(person) {
    setError('')
    setBusyId(person.id)
    try {
      await apiPost(`/groups/${groupId}/invitations`, { user_id: person.id })
      setInvited(state => ({ ...state, [person.id]: 'Invited' }))
      onInvited(person.first_name)
    } catch (err) {
      // 409: already invited, or already a member (only the first members are
      // known here to filter out) — show which instead of an error
      if (err.status === 409) {
        const status = /member/i.test(err.message) ? 'Member' : 'Invited'
        setInvited(state => ({ ...state, [person.id]: status }))
      } else setError(err.message)
    }
    setBusyId(null)
  }

  const shown = (people.items || []).filter(person => !memberIds.has(person.id))

  return (
    <Modal title="Invite people" onClose={onClose}>
      <div className="search">
        <Icon name="search" />
        <input
          placeholder="Search by name or nickname"
          value={search}
          maxLength={LIMITS.search}
          onChange={e => setSearch(e.target.value)}
          autoFocus
        />
      </div>

      {(error || people.error) && <p className="error">{error || people.error.message}</p>}

      {people.items === null && !people.error && <p className="loading">Loading…</p>}

      {people.items !== null && shown.length === 0 && !people.hasMore && (
        <Empty title="No one to invite">
          {query ? 'No one matches that search.' : 'Everyone on the network is already a member.'}
        </Empty>
      )}

      <div className="invite-list">
        {shown.map(person => (
          <PersonRow key={person.id} person={person} size={40}>
            {invited[person.id] ? (
              <span className="chip">{invited[person.id]}</span>
            ) : (
              <button type="button" className="btn btn-light btn-sm" onClick={() => invite(person)} disabled={busyId === person.id}>
                {busyId === person.id ? '…' : 'Invite'}
              </button>
            )}
          </PersonRow>
        ))}
      </div>
      <LoadMore list={people} />

      <div className="composer-bar">
        <CharCount value={search} max={LIMITS.search} />
        <button type="button" className="btn btn-light" onClick={onClose}>Done</button>
      </div>
    </Modal>
  )
}
