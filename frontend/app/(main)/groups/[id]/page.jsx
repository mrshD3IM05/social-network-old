'use client'

import { useCallback, useEffect, useState } from 'react'
import { useParams, useRouter } from 'next/navigation'
import Link from 'next/link'
import { apiDelete, apiGet, apiPost } from '@/lib/api'
import { useMe } from '@/lib/useMe'
import usePaged from '@/lib/usePaged'
import { useDebouncedValue } from '@/lib/timing'
import useChat from '@/lib/useChat'
import { LIMITS } from '@/lib/validate'
import Avatar from '@/components/Avatar'
import CharCount from '@/components/CharCount'
import { ChatForm, ChatMessages, MessageBody } from '@/components/Chat'
import Empty from '@/components/Empty'
import EventCard from '@/components/EventCard'
import EventFormModal from '@/components/EventFormModal'
import GroupFormModal from '@/components/GroupFormModal'
import Icon from '@/components/Icon'
import LoadMore from '@/components/LoadMore'
import Modal from '@/components/Modal'
import PersonRow from '@/components/PersonRow'
import PostCard from '@/components/PostCard'
import PostForm from '@/components/PostForm'
import RequestRow from '@/components/RequestRow'

// One group: a header (who, what, the actions) and one tab per thing it holds.
// Outsiders only see the header; the API serves the rest to members only.
export default function GroupDetailPage() {
  const { id } = useParams()
  const router = useRouter()
  const { me } = useMe()
  const [group, setGroup] = useState(null)
  const [tab, setTab] = useState('posts')
  const [opened, setOpened] = useState(['posts']) // a tab's list loads the first time it is opened
  const [notFound, setNotFound] = useState(false)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [modal, setModal] = useState('') // 'invite' | 'edit' | 'event'
  const [answering, setAnswering] = useState(false)

  const isMember = group?.is_member || group?.is_creator
  const wants = key => isMember && (tab === key || opened.includes(key))
  const posts = usePaged(isMember ? `/groups/${id}/posts` : null)
  const members = usePaged(wants('members') ? `/groups/${id}/members` : null)
  const events = usePaged(wants('events') ? `/groups/${id}/events` : null)
  const requests = usePaged(group?.is_creator ? `/groups/${id}/join-requests` : null)

  function openTab(key) {
    setTab(key)
    setOpened(list => (list.includes(key) ? list : [...list, key]))
  }

  const load = useCallback(async () => {
    try {
      setGroup(await apiGet(`/groups/${id}`))
    } catch (err) {
      if (err.status === 404) setNotFound(true)
      else setError(err.message)
    }
  }, [id])

  useEffect(() => {
    load()
  }, [load])

  // every action reports the same way and reloads the group, so counts stay right
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

  function respondJoinRequest(request, accept) {
    return run(async () => {
      await apiPost(`/group-join-requests/${request.id}/${accept ? 'accept' : 'decline'}`)
      requests.setItems(list => list.filter(r => r.id !== request.id))
      if (accept && members.items) members.reload()
    }, accept ? `${request.first_name} is now a member.` : 'Request declined.')
  }

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
    run(async () => {
      await apiDelete(`/groups/${id}/members/${member.user_id}`)
      members.setItems(list => list?.filter(m => m.user_id !== member.user_id))
    }, `${member.first_name} was removed from the group.`)
  }

  // leave = remove yourself; the creator deletes the group instead
  async function leaveOrDelete(question, path) {
    if (!confirm(question)) return
    try {
      await apiDelete(path)
      router.push('/groups')
    } catch (err) {
      setError(err.message)
    }
  }
  const leaveGroup = () => leaveOrDelete(`Leave ${group.title}?`, `/groups/${id}/members/${me.id}`)
  const deleteGroup = () => leaveOrDelete('Delete this group? Its posts, events and messages are deleted too.', `/groups/${id}`)

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
    ['posts', 'Posts'],
    ['events', 'Events', group.event_count],
    ['chat', 'Chat'],
    ['members', 'Members', group.member_count],
    // only the loaded requests are known: "10+" while more are waiting
    ...(group.is_creator ? [['requests', 'Requests', requests.hasMore ? `${requests.items.length}+` : requests.items?.length]] : []),
  ]

  return (
    <>
      <header className="card group-hero">
        <div className="group-hero-top">
          <Avatar user={{ first_name: group.title, avatar: group.avatar }} size={64} />
          <div className="group-hero-title">
            <h1>{group.title}</h1>
            <p className="meta">
              {group.member_count} member{group.member_count === 1 ? '' : 's'}
              {group.creator && <> · created by {group.creator.first_name} {group.creator.last_name}</>}
              {' · '}{new Date(group.created_at).toLocaleDateString()}
            </p>
          </div>
          {group.is_creator ? <span className="chip chip-accent">Creator</span> : group.is_member && <span className="chip">Member</span>}
        </div>

        <p className="group-hero-desc">{group.description || 'No description.'}</p>

        <div className="group-hero-actions">
          <AvatarStack members={group.members} total={group.member_count} />
          {isMember ? (
            <div className="group-hero-buttons">
              {group.is_creator ? (
                <>
                  <button type="button" className="btn btn-light" onClick={() => setModal('edit')}>
                    <Icon name="edit" size={16} /> Edit
                  </button>
                  <button type="button" className="btn btn-light" onClick={deleteGroup}>
                    <Icon name="trash" size={16} /> Delete
                  </button>
                </>
              ) : (
                <button type="button" className="btn btn-light" onClick={leaveGroup}>Leave</button>
              )}
              <button type="button" className="btn" onClick={() => setModal('invite')}>
                <Icon name="plus" size={16} /> Invite people
              </button>
            </div>
          ) : group.pending_join ? (
            <p className="meta group-hero-note">Join request sent — waiting for the creator.</p>
          ) : group.pending_invite ? (
            <div className="group-hero-buttons">
              <p className="meta group-hero-note">You are invited to this group.</p>
              <button type="button" className="btn" disabled={answering} onClick={() => respondInvitation(true)}>Accept</button>
              <button type="button" className="btn btn-light" disabled={answering} onClick={() => respondInvitation(false)}>Decline</button>
            </div>
          ) : (
            <button type="button" className="btn" onClick={() => run(() => apiPost(`/groups/${id}/join-requests`), 'Join request sent. The creator will review it.')}>
              Request to join
            </button>
          )}
        </div>
      </header>

      {error && <p className="error">{error}</p>}
      {notice && <p className="notice">{notice}</p>}

      {!isMember ? (
        <Empty title="Members only">Posts, events and members open up once you join this group.</Empty>
      ) : (
        <>
          <nav className="tabs">
            {tabs.map(([key, label, count]) => (
              <button key={key} className={`tab${tab === key ? ' active' : ''}`} onClick={() => openTab(key)}>
                {label}
                {(typeof count === 'string' || count > 0) && <span className="tab-count">{count}</span>}
              </button>
            ))}
          </nav>

          {tab === 'posts' && (
            <>
              <PostForm groupId={id} onPosted={posts.reload} />
              {posts.items === null && <p className="loading">Loading posts…</p>}
              {posts.items?.length === 0 && <Empty title="No posts yet">Write the first one with the box above.</Empty>}
              {posts.items?.map(post => (
                <PostCard
                  key={post.id}
                  post={post}
                  myId={me.id}
                  currentGroupId={id}
                  isGroupCreator={group.is_creator}
                  onDeleted={postId => posts.setItems(list => list?.filter(p => p.id !== postId))}
                />
              ))}
              <LoadMore list={posts} />
            </>
          )}

          {tab === 'events' && (
            <>
              <div className="section-bar">
                <p className="eyebrow">Upcoming and past events</p>
                <button type="button" className="btn btn-sm" onClick={() => setModal('event')}>
                  <Icon name="plus" size={16} /> Create event
                </button>
              </div>
              {events.error && <p className="error">{events.error.message}</p>}
              {events.items === null && !events.error && <p className="loading">Loading events…</p>}
              {events.items?.length === 0 && <Empty title="No events scheduled">Create one and every member gets notified.</Empty>}
              {events.items?.map(event => <EventCard key={event.id} event={event} />)}
              <LoadMore list={events} />
            </>
          )}

          {tab === 'chat' && <GroupChat groupId={Number(id)} me={me} members={group.members} />}

          {tab === 'members' && (members.items === null ? <p className="loading">Loading members…</p> : (
            <>
              <div className="card list">
                {members.items.map(member => {
                  const action = member.user_id === me.id ? ['Leave', leaveGroup]
                    : group.is_creator ? ['Remove', () => removeMember(member)] : null
                  return (
                    <PersonRow key={member.user_id} person={member} href={`/profile/${member.user_id}`}>
                      {member.user_id === group.creator_id ? (
                        <span className="chip chip-accent">Creator</span>
                      ) : action ? (
                        <button
                          type="button"
                          className="btn btn-light btn-sm"
                          onClick={e => {
                            e.preventDefault() // don't follow the profile link
                            action[1]()
                          }}
                        >
                          {action[0]}
                        </button>
                      ) : (
                        <Icon name="arrow" size={16} />
                      )}
                    </PersonRow>
                  )
                })}
              </div>
              <LoadMore list={members} />
            </>
          ))}

          {tab === 'requests' && (
            requests.items === null ? (
              <p className="loading">Loading requests…</p>
            ) : requests.items.length === 0 ? (
              <Empty title="No pending requests">People asking to join this group land here.</Empty>
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

      {modal === 'invite' && (
        <InviteModal
          groupId={id}
          memberIds={new Set(group.members.map(m => m.user_id))} // the first page; the API catches the rest
          onClose={() => setModal('')}
          onInvited={name => setNotice(`Invitation sent to ${name}.`)}
        />
      )}

      {modal === 'edit' && (
        <GroupFormModal
          group={group}
          onClose={() => setModal('')}
          onSaved={() => {
            setModal('')
            setNotice('Group updated.')
            load()
          }}
        />
      )}

      {modal === 'event' && (
        <EventFormModal
          groupId={id}
          onClose={() => setModal('')}
          onCreated={event => {
            setModal('')
            events.reload() // events come in date order, so re-read
            load() // and the tab count
            setNotice(`Event "${event.title}" created. Members have been notified.`)
          }}
        />
      )}
    </>
  )
}

function GroupChat({ groupId, me, members }) {
  const chat = useChat(`/groups/${groupId}/messages`, { group_id: groupId })

  // id → name for "is typing": the first page of members plus everyone who wrote
  const names = Object.fromEntries(members.map(m => [m.user_id, m.first_name]))
  for (const msg of chat.messages || []) names[msg.from_user_id] ??= msg.from_first_name

  return (
    <section className="card chat chat-group">
      <ChatMessages chat={chat} empty="No messages yet. Say hello!">
        {chat.messages?.map(msg => {
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
                    <Link href={`/profile/${msg.from_user_id}`} className="chat-name">{author.first_name}</Link>
                  </small>
                )}
                <div className={mine ? 'bubble mine' : 'bubble'}>
                  <MessageBody msg={msg} />
                </div>
              </div>
            </div>
          )
        })}
        {chat.typing > 0 && <p className="typing">{names[chat.typing] || 'Someone'} is typing…</p>}
      </ChatMessages>
      <ChatForm chat={chat} placeholder="Write to the group…" />
    </section>
  )
}

function AvatarStack({ members, total, shown = 5 }) {
  const rest = total - Math.min(shown, members.length)
  return (
    <div className="avatar-stack">
      {members.slice(0, shown).map(member => <Avatar key={member.user_id} user={member} size={32} />)}
      {rest > 0 && <span className="avatar-stack-more">+{rest}</span>}
    </div>
  )
}

// Search the people directory and invite them. Members are filtered out; the
// API answers 409 for anyone already invited or already a member.
function InviteModal({ groupId, memberIds, onClose, onInvited }) {
  const [search, setSearch] = useState('')
  const [invited, setInvited] = useState({}) // person id → 'Invited' | 'Member'
  const [busyId, setBusyId] = useState(null)
  const [error, setError] = useState('')
  const query = useDebouncedValue(search, 250).trim()
  const people = usePaged(`/users?q=${encodeURIComponent(query)}`)

  async function invite(person) {
    setError('')
    setBusyId(person.id)
    try {
      await apiPost(`/groups/${groupId}/invitations`, { user_id: person.id })
      setInvited(state => ({ ...state, [person.id]: 'Invited' }))
      onInvited(person.first_name)
    } catch (err) {
      if (err.status === 409) setInvited(state => ({ ...state, [person.id]: /member/i.test(err.message) ? 'Member' : 'Invited' }))
      else setError(err.message)
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
