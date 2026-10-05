'use client'

import Link from 'next/link'
import Avatar from '@/components/Avatar'
import Icon from '@/components/Icon'

// One group in a list, with a status chip or a Join button.
export default function GroupCard({ group, onJoin, joining }) {
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
        <span className="chip">Requested</span>
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
