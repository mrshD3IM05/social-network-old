'use client'

import { Fragment, useState } from 'react'
import { apiPost } from '@/lib/api'
import usePaged from '@/lib/usePaged'
import GroupCard from '@/components/GroupCard'
import GroupFormModal from '@/components/GroupFormModal'
import Icon from '@/components/Icon'
import LoadMore from '@/components/LoadMore'
import PageHeader from '@/components/PageHeader'
import RequestRow from '@/components/RequestRow'

// Your invitations first, then the groups you belong to, then the ones to discover.
export default function GroupsPage() {
  const mine = usePaged('/groups?joined=true')
  const others = usePaged('/groups?joined=false')
  const invitations = usePaged('/group-invitations')
  const [error, setError] = useState('')
  const [showCreate, setShowCreate] = useState(false)
  const [joiningId, setJoiningId] = useState(null)

  async function respondInvitation(id, accept) {
    setError('')
    try {
      await apiPost(`/group-invitations/${id}/${accept ? 'accept' : 'decline'}`)
      invitations.setItems(list => list.filter(inv => inv.id !== id))
      if (accept) {
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
      others.setItems(list => list.map(g => (g.id === group.id ? { ...g, pending_join: true } : g)))
    } catch (err) {
      setError(err.message)
    }
    setJoiningId(null)
  }

  const listError = mine.error || others.error

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

      {listError && <p className="error">{listError.message}</p>}

      {mine.items !== null && others.items !== null && [
        ['Your groups', mine, 'You have not joined a group yet. Pick one below, or create your own.'],
        ['Discover', others, 'Nothing left to discover — you are in every group.'],
      ].map(([label, list, empty]) => (
        <Fragment key={label}>
          <p className="eyebrow section-label">{label}</p>
          {list.items.length === 0 ? (
            <p className="meta section-empty">{empty}</p>
          ) : (
            <>
              <div className="card list">
                {list.items.map(group => (
                  <GroupCard key={group.id} group={group} onJoin={join} joining={joiningId === group.id} />
                ))}
              </div>
              <LoadMore list={list} />
            </>
          )}
        </Fragment>
      ))}

      {showCreate && (
        <GroupFormModal
          onClose={() => setShowCreate(false)}
          onSaved={() => {
            setShowCreate(false)
            mine.reload()
          }}
        />
      )}
    </>
  )
}
