'use client'

import Link from 'next/link'
import Avatar from '@/components/Avatar'
import Icon from '@/components/Icon'

// One group in the browse list: title, description, member count and a
// status chip ("You", "Member") or an action for outsiders — Join to ask for
// access, Cancel request to withdraw one already sent.
export default function GroupCard({ group, onJoin, onCancel, joining }) {
  return (
    <Link href={`/groups/${group.id}`} className="list-item group-item">
      {group.avatar ? (
        <Avatar user={group} size={40} />
      ) : (
        <span className="list-icon"><Icon name="grid" size={18} /></span>
      )}
      <span className="list-text">
        <strong>{group.title}</strong>
        {group.description && <small>{group.description}</small>}
        <small className="meta">{group.member_count} member{group.member_count === 1 ? '' : 's'}</small>
      </span>

      {group.is_creator ? (
        <span className="chip chip-accent">You</span>
      ) : group.is_member ? (
        <span className="chip">Member</span>
      ) : group.pending_join ? (
        <button
          className="btn btn-light btn-sm"
          onClick={e => {
            e.preventDefault() // don't follow the link
            e.stopPropagation()
            onCancel(group)
          }}
          disabled={joining}
          title="Withdraw your join request"
        >
          {joining ? '…' : 'Requested'}
        </button>
      ) : (
        <button
          className="btn btn-light btn-sm"
          onClick={e => {
            e.preventDefault() // don't follow the link
            e.stopPropagation()
            onJoin(group)
          }}
          disabled={joining}
        >
          {joining ? '…' : 'Join'}
        </button>
      )}
    </Link>
  )
}
