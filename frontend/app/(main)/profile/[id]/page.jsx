'use client'

import { useEffect, useState } from 'react'
import Link from 'next/link'
import { useParams } from 'next/navigation'
import { apiDelete, apiGet, apiPost, apiPut } from '@/lib/api'
import { useMe } from '@/lib/useMe'
import { setMe } from '@/lib/userStore'
import usePaged from '@/lib/usePaged'
import Avatar from '@/components/Avatar'
import Empty from '@/components/Empty'
import Icon from '@/components/Icon'
import LoadMore from '@/components/LoadMore'
import PersonRow from '@/components/PersonRow'
import PostCard from '@/components/PostCard'
import PostForm from '@/components/PostForm'

// is_followed (model.FollowState) → the status POST /users/{id}/follow answers with
const FOLLOW_STATUS = { 1: 'accepted', 2: 'pending' }

export default function ProfilePage() {
  const { id } = useParams()
  const { me } = useMe()
  const [user, setUser] = useState(null)
  const [followStatus, setFollowStatus] = useState('') // '' | 'pending' | 'accepted'
  const [tab, setTab] = useState('posts') // 'posts' | 'followers' | 'following'
  const [opened, setOpened] = useState([]) // "id:tab" already opened: their list stays loaded
  const [message, setMessage] = useState('')
  const [savingPrivacy, setSavingPrivacy] = useState(false)

  const wants = key => tab === key || opened.includes(`${id}:${key}`)
  const posts = usePaged(`/users/${id}/posts`)
  const followers = usePaged(wants('followers') ? `/users/${id}/followers` : null)
  const following = usePaged(wants('following') ? `/users/${id}/following` : null)

  function openTab(key) {
    setTab(key)
    setOpened(list => (list.includes(`${id}:${key}`) ? list : [...list, `${id}:${key}`]))
  }

  async function load() {
    try {
      const profile = await apiGet(`/user/${id}`)
      setUser(profile)
      setFollowStatus(FOLLOW_STATUS[profile.is_followed] || '')
    } catch (err) {
      setMessage(err.message)
    }
  }

  useEffect(() => {
    load()
  }, [id])

  // after a follow, unfollow or privacy change
  function refresh() {
    load()
    posts.reload()
    followers.reload()
    following.reload()
  }

  async function follow() {
    try {
      const result = await apiPost(`/users/${id}/follow`)
      setFollowStatus(result.status)
      setMessage(result.status === 'pending' ? 'Follow request sent.' : 'You are now following.')
      refresh()
    } catch (err) {
      setMessage(err.message)
    }
  }

  // also cancels a pending request
  async function unfollow() {
    try {
      await apiDelete(`/users/${id}/follow`)
      setMessage(followStatus === 'pending' ? 'Follow request cancelled.' : 'Unfollowed.')
      setFollowStatus('')
      refresh()
    } catch (err) {
      setMessage(err.message)
    }
  }

  async function togglePrivacy() {
    setSavingPrivacy(true)
    try {
      const updated = await apiPut('/me/privacy', { private: !user.private })
      setUser(old => ({ ...old, ...updated })) // keeps the counts
      setMe(updated)
      setMessage(updated.private
        ? 'Your profile is private — only your followers can see it.'
        : 'Your profile is public — everyone can see it.')
      if (!updated.private) refresh() // going public accepts the waiting requests
    } catch (err) {
      setMessage(err.message)
    }
    setSavingPrivacy(false)
  }

  if (!user || !me) return <p className="loading">{message || 'Loading…'}</p>

  const isMe = me.id === user.id
  const locked = user.private && !isMe && followStatus !== 'accepted'
  const canMessage = followStatus === 'accepted' || user.is_following === 1 // one of the two follows the other
  const people = tab === 'followers' ? followers : following

  return (
    <>
      <section className="card profile">
        <div className="profile-cover" />
        <div className="profile-body">
          <Avatar user={user} size={96} />

          <div className="profile-top">
            <div>
              <h1>{user.first_name} {user.last_name}</h1>
              <p className="meta">@{user.nickname} · {user.private ? 'Private' : 'Public'} profile</p>
            </div>

            <div className="profile-buttons">
              {isMe ? (
                <>
                  <button className="btn btn-light" onClick={togglePrivacy} disabled={savingPrivacy}>
                    <Icon name="lock" size={15} />
                    {savingPrivacy ? 'Saving…' : user.private ? 'Make public' : 'Make private'}
                  </button>
                  <Link href="/settings" className="btn btn-light">Edit settings</Link>
                </>
              ) : (
                <>
                  {followStatus === 'accepted' ? (
                    <button className="btn btn-light" onClick={unfollow}>Unfollow</button>
                  ) : followStatus === 'pending' ? (
                    <button className="btn btn-light" onClick={unfollow} title="Cancel the request">Requested</button>
                  ) : (
                    <button className="btn" onClick={follow}>
                      {user.is_following === 1 ? 'Follow back' : user.private ? 'Request to follow' : 'Follow'}
                    </button>
                  )}
                  {canMessage && <Link href={`/chat/${user.id}`} className="btn btn-light">Message</Link>}
                </>
              )}
            </div>
          </div>

          {user.about_me && <p className="profile-about">{user.about_me}</p>}

          <div className="profile-stats">
            <span><strong>{user.post_count}</strong> posts</span>
            {[['followers', user.followers], ['following', user.following]].map(([key, count]) => locked ? (
              <span key={key}><strong>{count}</strong> {key}</span>
            ) : (
              <button key={key} type="button" className="stat-link" onClick={() => openTab(key)}>
                <strong>{count}</strong> {key}
              </button>
            ))}
            <span>Joined {new Date(user.created_at).toLocaleDateString(undefined, { month: 'long', year: 'numeric' })}</span>
            {user.email && <span>{user.email}</span>}
            {user.date_of_birth && <span>Born {new Date(user.date_of_birth + 'T00:00').toLocaleDateString(undefined, { day: 'numeric', month: 'long', year: 'numeric' })}</span>}
          </div>
        </div>
      </section>

      {message && <p className="notice">{message}</p>}

      {locked ? (
        <div className="card locked">
          <span className="locked-icon"><Icon name="lock" size={22} /></span>
          <h2>This account is private</h2>
          <p className="subtitle">Follow this account to see their posts, followers and following.</p>
        </div>
      ) : (
        <>
          <div className="profile-tabs">
            {[['posts', 'Posts', user.post_count], ['followers', 'Followers', user.followers], ['following', 'Following', user.following]].map(([key, label, count]) => (
              <button key={key} className={tab === key ? 'profile-tab active' : 'profile-tab'} onClick={() => openTab(key)}>
                {label} <span>{count}</span>
              </button>
            ))}
          </div>

          {tab === 'posts' && isMe && <PostForm onPosted={refresh} />}

          {tab === 'posts' ? (
            posts.error ? (
              <p className="error">Could not load the posts. {posts.error.message}</p>
            ) : posts.items?.length === 0 ? (
              <Empty title="No posts to show">{isMe ? 'Your posts will appear here.' : 'Posts you are allowed to see will appear here.'}</Empty>
            ) : (
              <>
                {posts.items?.map(post => <PostCard key={post.id} post={post} myId={me.id} onDeleted={refresh} />)}
                <LoadMore list={posts} />
              </>
            )
          ) : people.error ? (
            <p className="error">Could not load this list. {people.error.message}</p>
          ) : people.items === null ? (
            <p className="loading">Loading…</p>
          ) : people.items.length === 0 ? (
            tab === 'followers'
              ? <Empty title="No followers yet">People who follow this profile will appear here.</Empty>
              : <Empty title="Not following anyone yet">People this profile follows will appear here.</Empty>
          ) : (
            <>
              <div className="card list">
                {people.items.map(person => (
                  <PersonRow key={person.id} person={person} href={`/profile/${person.id}`}>
                    <Icon name="arrow" size={16} />
                  </PersonRow>
                ))}
              </div>
              <LoadMore list={people} />
            </>
          )}
        </>
      )}
    </>
  )
}
