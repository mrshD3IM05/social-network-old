'use client'

import { useEffect, useState } from 'react'
import Link from 'next/link'
import { useParams } from 'next/navigation'
import { apiDelete, apiGet, apiPost, apiPut } from '@/lib/api'
import { useMe } from '@/lib/useMe'
import { setMe } from '@/lib/userStore'
import usePaged from '@/lib/usePaged'
import Avatar from '@/components/Avatar'
import Icon from '@/components/Icon'
import LoadMore from '@/components/LoadMore'
import PersonRow from '@/components/PersonRow'
import PostForm from '@/components/PostForm'
import PostCard from '@/components/PostCard'

// is_followed on a user (model.FollowState) → the status POST /users/{id}/follow answers with
const FOLLOW_STATUS = { 1: 'accepted', 2: 'pending' }

export default function ProfilePage() {
  const { id } = useParams() // the [id] from the URL, e.g. /profile/3
  const { me } = useMe()
  const [user, setUser] = useState(null)
  const [followStatus, setFollowStatus] = useState('') // '' | 'pending' | 'accepted'
  const [tab, setTab] = useState('posts') // 'posts' | 'followers' | 'following'
  // tabs opened on this profile, as "id:tab": a list loads the first time its
  // tab is opened (the counts come with the profile), then stays loaded
  const [opened, setOpened] = useState([])
  const wants = key => tab === key || opened.includes(`${id}:${key}`)
  function openTab(key) {
    setTab(key)
    setOpened(list => (list.includes(`${id}:${key}`) ? list : [...list, `${id}:${key}`]))
  }
  // the three lists come 10 at a time; a private profile answers 403 to them
  const posts = usePaged(`/users/${id}/posts`)
  const followers = usePaged(wants('followers') ? `/users/${id}/followers` : null)
  const following = usePaged(wants('following') ? `/users/${id}/following` : null)
  const [message, setMessage] = useState('')
  const [savingPrivacy, setSavingPrivacy] = useState(false)

  async function load() {
    try {
      // the profile with its counts (posts, followers, following), always
      // answered: a private one just leaves out the contact details. It also
      // carries your follow state (is_followed), so no second request is needed.
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

  // after a follow, unfollow or privacy change: the profile and its lists
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
      refresh() // a new follower may now see more posts
    } catch (err) {
      setMessage(err.message)
    }
  }

  // also used to cancel a pending request
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

  // The switch on your own profile. The API answers with the saved user, so the
  // label always matches what is stored.
  async function togglePrivacy() {
    setSavingPrivacy(true)
    try {
      const updated = await apiPut('/me/privacy', { private: !user.private })
      setUser(old => ({ ...old, ...updated })) // keeps the counts
      setMe(updated)
      setMessage(updated.private
        ? 'Your profile is private — only your followers can see it.'
        : 'Your profile is public — everyone can see it.')
      // going public accepts the waiting follow requests, so reload the lists
      if (!updated.private) refresh()
    } catch (err) {
      setMessage(err.message)
    }
    setSavingPrivacy(false)
  }

  // One button that changes with the relation: Follow (or Follow back when they
  // already follow you) → Requested / Unfollow
  const followButton =
    followStatus === 'accepted' ? (
      <button className="btn btn-light" onClick={unfollow}>Unfollow</button>
    ) : followStatus === 'pending' ? (
      <button className="btn btn-light" onClick={unfollow} title="Cancel the request">Requested</button>
    ) : (
      <button className="btn" onClick={follow}>
        {user?.is_following === 1 ? 'Follow back' : user?.private ? 'Request to follow' : 'Follow'}
      </button>
    )

  if (!user || !me) return <p className="loading">{message || 'Loading…'}</p>

  const isMe = me.id === user.id
  // like Instagram: a private profile you don't follow shows only its header
  // and counts, the posts and the lists stay locked
  const locked = user.private && !isMe && followStatus !== 'accepted'
  // chat needs one of the two to follow the other
  const canMessage = followStatus === 'accepted' || user.is_following === 1

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
                  {followButton}
                  {canMessage && <Link href={`/chat/${user.id}`} className="btn btn-light">Message</Link>}
                </>
              )}
            </div>
          </div>

          {user.about_me && <p className="profile-about">{user.about_me}</p>}

          <div className="profile-stats">
            <span><strong>{user.post_count}</strong> posts</span>
            {locked ? (
              <>
                <span><strong>{user.followers}</strong> followers</span>
                <span><strong>{user.following}</strong> following</span>
              </>
            ) : (
              <>
                {/* the counts open their list below */}
                <button type="button" className="stat-link" onClick={() => openTab('followers')}>
                  <strong>{user.followers}</strong> followers
                </button>
                <button type="button" className="stat-link" onClick={() => openTab('following')}>
                  <strong>{user.following}</strong> following
                </button>
              </>
            )}
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
      ) : (<>
      <div className="profile-tabs">
        {[
          ['posts', 'Posts', user.post_count],
          ['followers', 'Followers', user.followers],
          ['following', 'Following', user.following],
        ].map(([key, label, count]) => (
          <button
            key={key}
            className={tab === key ? 'profile-tab active' : 'profile-tab'}
            onClick={() => openTab(key)}
          >
            {label} <span>{count}</span>
          </button>
        ))}
      </div>

      {tab === 'posts' && isMe && <PostForm onPosted={refresh} />}

      {tab === 'posts' && posts.error && (
        <p className="error">Could not load the posts. {posts.error.message}</p>
      )}

      {tab === 'posts' && !posts.error && (
        posts.items?.length === 0 ? (
          <div className="empty">
            <p className="empty-title">No posts to show</p>
            <p>{isMe ? 'Your posts will appear here.' : 'Posts you are allowed to see will appear here.'}</p>
          </div>
        ) : (
          <>
            {posts.items?.map(post => (
              <PostCard key={post.id} post={post} myId={me.id} onDeleted={refresh} />
            ))}
            <LoadMore list={posts} />
          </>
        )
      )}

      {tab !== 'posts' && (() => {
        const people = tab === 'followers' ? followers : following
        if (people.error) {
          return <p className="error">Could not load this list. {people.error.message}</p>
        }
        if (people.items === null) return <p className="loading">Loading…</p>
        if (people.items.length === 0) {
          return (
            <div className="empty">
              <p className="empty-title">
                {tab === 'followers' ? 'No followers yet' : 'Not following anyone yet'}
              </p>
              <p>
                {tab === 'followers'
                  ? 'People who follow this profile will appear here.'
                  : 'People this profile follows will appear here.'}
              </p>
            </div>
          )
        }
        return (
          <>
            <div className="card list">
              {people.items?.map(person => (
                <PersonRow key={person.id} person={person} href={`/profile/${person.id}`}>
                  <Icon name="arrow" size={16} />
                </PersonRow>
              ))}
            </div>
            <LoadMore list={people} />
          </>
        )
      })()}
      </>)}
    </>
  )
}
