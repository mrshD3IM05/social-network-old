'use client'

import { useEffect, useState } from 'react'
import { apiDelete, apiPost } from '@/lib/api'
import usePaged from '@/lib/usePaged'
import { LIMITS, checkText } from '@/lib/validate'
import Modal from '@/components/Modal'
import GroupCard from '@/components/GroupCard'
import Icon from '@/components/Icon'
import LoadMore from '@/components/LoadMore'
import PageHeader from '@/components/PageHeader'
import RequestRow from '@/components/RequestRow'
import CharCount from '@/components/CharCount'

// Groups hub: your invitations first, then the groups you belong to, then the
// ones left to discover — so the list you act on is never mixed with the rest.
export default function GroupsPage() {
  // two lists, 10 at a time each: the groups you are in, and the rest
  const mine = usePaged('/groups?joined=true')
  const others = usePaged('/groups?joined=false')
  const invitations = usePaged('/group-invitations') // the inbox, 10 at a time
  // all shown invitations answered while more are waiting: fetch them
  useEffect(() => {
    if (invitations.items?.length === 0 && invitations.hasMore) invitations.reload()
  }, [invitations.items?.length, invitations.hasMore])
  const [error, setError] = useState('')
  const [showCreate, setShowCreate] = useState(false)
  const [joiningId, setJoiningId] = useState(null) // id of the group being joined

  async function respondInvitation(id, accept) {
    setError('')
    try {
      await apiPost(`/group-invitations/${id}/${accept ? 'accept' : 'decline'}`)
      invitations.setItems(list => list.filter(inv => inv.id !== id))
      if (accept) {
        // membership changed → the group moves to "Your groups"
        mine.reload()
        others.reload()
      }
    } catch (err) {
      setError(err.message)
    }
  }

  async function join(group) {
    setError('')
    setJoiningId(group.id)
    try {
      await apiPost(`/groups/${group.id}/join-requests`)
      others.setItems(list =>
        list.map(g => (g.id === group.id ? { ...g, pending_join: true } : g)),
      )
    } catch (err) {
      setError(err.message)
    }
    setJoiningId(null)
  }

  // Withdrawing only changes that one row: the group stays where it is in the
  // list and its Join button comes back, so there is nothing to refetch.
  async function cancelJoin(group) {
    setError('')
    setJoiningId(group.id)
    try {
      await apiDelete(`/groups/${group.id}/cancel-join-request`)
      others.setItems(list =>
        list.map(g => (g.id === group.id ? { ...g, pending_join: false } : g)),
      )
    } catch (err) {
      setError(err.message)
    }
    setJoiningId(null)
  }

  return (
    <>
      <PageHeader title="Groups" subtitle="Find your people, or start a space of your own." />

      <div className="section-bar">
        <p className="eyebrow">Your groups and the ones to discover</p>
        <button type="button" className="btn" onClick={() => setShowCreate(true)}>
          <Icon name="plus" size={16} /> Create a group
        </button>
      </div>

      {error && <p className="error">{error}</p>}

      {invitations.items?.length > 0 && (
        <section className="card invitations">
          <h2>Group invitations</h2>
          {invitations.items.map(inv => (
            <RequestRow
              key={inv.id}
              person={{ first_name: inv.from_first_name, last_name: inv.from_last_name, avatar: inv.from_avatar }}
              href={`/groups/${inv.group_id}`}
              title={`You are invited to join “${inv.group_title}”`}
              subtitle={`${inv.from_first_name} ${inv.from_last_name} invited you`}
              onRespond={accept => respondInvitation(inv.id, accept)}
            />
          ))}
          <LoadMore list={invitations} />
        </section>
      )}

      {(mine.error || others.error) && <p className="error">{(mine.error || others.error).message}</p>}

      {mine.items !== null && others.items !== null && (
        <>
          <GroupSection
            label="Your groups"
            list={mine}
            empty="You have not joined a group yet. Pick one below, or create your own."
            onJoin={join}
            onCancel={cancelJoin}
            joiningId={joiningId}
          />
          <GroupSection
            label="Discover"
            list={others}
            empty="Nothing left to discover — you are in every group."
            onJoin={join}
            onCancel={cancelJoin}
            joiningId={joiningId}
          />
        </>
      )}

      {showCreate && (
        <CreateGroupModal
          onClose={() => setShowCreate(false)}
          onCreated={() => {
            setShowCreate(false)
            mine.reload() // the new group shows up under "Your groups" (as creator)
          }}
        />
      )}
    </>
  )
}

// One titled list of groups (10 at a time), or a one-line reason why it is empty.
function GroupSection({ label, list, empty, onJoin, onCancel, joiningId }) {
  return (
    <>
      <p className="eyebrow section-label">{label}</p>
      {list.items.length === 0 ? (
        <p className="meta section-empty">{empty}</p>
      ) : (
        <>
          <div className="card list">
            {list.items.map(group => (
              <GroupCard
                key={group.id}
                group={group}
                onJoin={onJoin}
                onCancel={onCancel}
                joining={joiningId === group.id}
              />
            ))}
          </div>
          <LoadMore list={list} />
        </>
      )}
    </>
  )
}

// Create form in a modal. The API answers 201 with the new group.
function CreateGroupModal({ onClose, onCreated }) {
  const [title, setTitle] = useState('')
  const [description, setDescription] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const titleError = checkText('Title', title, LIMITS.groupTitle)
  const descriptionError = checkText('Description', description, LIMITS.groupDescription, { required: false })

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
      const group = await apiPost('/groups', { title: title.trim(), description: description.trim() })
      onCreated(group)
    } catch (err) {
      setError(err.message)
      setLoading(false)
    }
  }

  return (
    <Modal title="Create a group" onClose={onClose}>
      <form onSubmit={handleSubmit} noValidate>
        <label>Title</label>
        <input
          value={title}
          maxLength={LIMITS.groupTitle}
          className={error && titleError ? 'invalid' : undefined}
          onChange={e => setTitle(e.target.value)}
          autoFocus
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
            {loading ? 'Creating…' : 'Create group'}
          </button>
        </div>

        {error && <p className="error">{error}</p>}
      </form>
    </Modal>
  )
}
