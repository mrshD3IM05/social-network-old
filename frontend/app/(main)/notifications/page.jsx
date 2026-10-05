'use client'

import { Fragment, useEffect, useState } from 'react'
import Link from 'next/link'
import { apiGet, apiPost } from '@/lib/api'
import { subscribe } from '@/lib/socket'
import usePaged, { PAGE_SIZE } from '@/lib/usePaged'
import Avatar from '@/components/Avatar'
import Empty from '@/components/Empty'
import Icon from '@/components/Icon'
import PageHeader from '@/components/PageHeader'
import LoadMore from '@/components/LoadMore'
import RequestRow from '@/components/RequestRow'

// the three kinds of request, as GET /requests names them, and how each row reads
const REQUESTS = {
  follow_requests: {
    path: '/follow-requests',
    row: r => ({
      person: r.user,
      href: `/profile/${r.user.id}`,
      title: `${r.user.first_name} ${r.user.last_name} wants to follow you`,
      subtitle: `@${r.user.nickname}`,
    }),
  },
  group_invitations: {
    path: '/group-invitations',
    row: r => ({
      person: { first_name: r.from_first_name, last_name: r.from_last_name, avatar: r.from_avatar },
      href: `/groups/${r.group_id}`,
      title: `You are invited to join “${r.group_title}”`,
      subtitle: `${r.from_first_name} ${r.from_last_name} invited you`,
    }),
  },
  group_join_requests: {
    path: '/group-join-requests',
    row: r => ({
      person: r,
      href: `/profile/${r.user_id}`,
      title: `${r.first_name} ${r.last_name} wants to join “${r.group_title}”`,
      subtitle: `@${r.nickname}`,
    }),
  },
}
const REQUEST_TYPES = ['follow_request', 'group_invitation', 'group_join_request']

// the small badge on the avatar that says what kind of notification it is
const BADGES = {
  comment_post: ['chat', 'comment'],
  follow_request: ['user-plus', 'follow'],
  new_follower: ['user-plus', 'follow'],
  follow_accepted: ['user-plus', 'follow'],
  group_invitation: ['grid', 'group'],
  group_invite_response: ['grid', 'group'],
  group_join_request: ['grid', 'group'],
  group_join_response: ['grid', 'group'],
  group_removed: ['x', 'removed'],
  event_created: ['calendar', 'event'],
}

const markAllRead = () => apiPost('/notifications/read').catch(() => {})

export default function NotificationsPage() {
  const notifications = usePaged('/notifications')
  const [requests, setRequests] = useState({}) // kind → list
  const [more, setMore] = useState({}) // kind → a full page came, there may be more
  const [loadingMore, setLoadingMore] = useState('')
  const [error, setError] = useState('')

  function loadRequests() {
    apiGet('/requests')
      .then(all => {
        setRequests(all)
        setMore(Object.fromEntries(Object.keys(REQUESTS).map(kind => [kind, all[kind].length === PAGE_SIZE])))
      })
      .catch(() => {})
  }

  async function loadMoreOf(kind) {
    setLoadingMore(kind)
    try {
      const page = await apiGet(`/requests?type=${kind}&last=${requests[kind].at(-1).id}`)
      setRequests(state => ({ ...state, [kind]: [...state[kind], ...page] }))
      setMore(state => ({ ...state, [kind]: page.length === PAGE_SIZE }))
    } catch (err) {
      setError(err.message)
    }
    setLoadingMore('')
  }

  async function respond(kind, id, accept) {
    setError('')
    try {
      await apiPost(`${REQUESTS[kind].path}/${id}/${accept ? 'accept' : 'decline'}`)
      setRequests(state => ({ ...state, [kind]: state[kind].filter(item => item.id !== id) }))
    } catch (err) {
      setError(err.message)
    }
  }

  // every shown request of a kind answered while more are waiting: fetch them
  useEffect(() => {
    if (Object.keys(REQUESTS).some(kind => more[kind] && requests[kind]?.length === 0)) loadRequests()
  }, [more, requests])

  // mark everything read once the first page is shown, so the new ones stay highlighted
  const loaded = notifications.items !== null
  useEffect(() => {
    if (loaded) markAllRead()
  }, [loaded])

  useEffect(() => {
    loadRequests()
    return subscribe(data => {
      if (data.type !== 'notification') return
      notifications.setItems(list => [data.notification, ...(list || [])])
      markAllRead()
      if (REQUEST_TYPES.includes(data.notification.type)) loadRequests()
    })
  }, [])

  const requestCount = Object.keys(REQUESTS).reduce((sum, kind) => sum + (requests[kind]?.length || 0), 0)

  return (
    <>
      <PageHeader title="Notifications" subtitle="Requests to answer and what happened lately." />

      {error && <p className="error">{error}</p>}

      {requestCount > 0 && (
        <section className="card invitations">
          <h2>Requests</h2>
          {Object.entries(REQUESTS).map(([kind, { row }]) => (
            <Fragment key={kind}>
              {(requests[kind] || []).map(r => (
                <RequestRow key={r.id} {...row(r)} onRespond={accept => respond(kind, r.id, accept)} />
              ))}
              {more[kind] && requests[kind].length > 0 && (
                <button type="button" className="btn btn-light btn-sm load-more" disabled={loadingMore === kind} onClick={() => loadMoreOf(kind)}>
                  {loadingMore === kind ? 'Loading…' : 'Load more'}
                </button>
              )}
            </Fragment>
          ))}
        </section>
      )}

      {notifications.error && <p className="error">{notifications.error.message}</p>}
      {notifications.items === null && !notifications.error && <p className="loading">Loading…</p>}

      {notifications.items?.length === 0 && requestCount === 0 && (
        <Empty title="You are all caught up">Follow requests, invitations and group events will show up here.</Empty>
      )}

      {notifications.items?.length > 0 && (
        <div className="card list">
          {notifications.items.map(n => {
            const [icon, tone] = BADGES[n.type] || ['bell', 'other']
            return (
              <Link key={n.id} href={n.group_id ? `/groups/${n.group_id}` : `/profile/${n.actor_id}`} className="list-item">
                <span className="notif-avatar">
                  <Avatar user={{ first_name: n.actor_first_name, last_name: n.actor_last_name, avatar: n.actor_avatar }} size={40} />
                  <span className={`notif-badge notif-${tone}`}><Icon name={icon} size={12} /></span>
                </span>
                <span className="list-text">
                  <strong>{n.content || n.type.replaceAll('_', ' ')}</strong>
                  <small>{new Date(n.created_at).toLocaleString()}</small>
                </span>
                {!n.read && <span className="menu-dot" title="New" />}
              </Link>
            )
          })}
        </div>
      )}
      <LoadMore list={notifications} />
    </>
  )
}
