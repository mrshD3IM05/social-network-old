'use client'

import { useState } from 'react'
import Link from 'next/link'
import Avatar from './Avatar'

// A follow request, group invitation or join request with Accept / Decline.
// Both buttons stay disabled while onRespond(accept) runs, so it cannot be answered twice.
export default function RequestRow({ person, href, title, subtitle, onRespond }) {
  const [busy, setBusy] = useState(null) // 'accept' | 'decline' while answering

  async function answer(accept) {
    setBusy(accept ? 'accept' : 'decline')
    try {
      await onRespond(accept)
    } finally {
      setBusy(null)
    }
  }

  const avatar = <Avatar user={person} size={40} />

  return (
    <div className="list-item request-item">
      {href ? <Link href={href} className="request-avatar">{avatar}</Link> : <span className="request-avatar">{avatar}</span>}
      <span className="list-text">
        <strong>{title}</strong>
        {subtitle && <small>{subtitle}</small>}
      </span>
      <div className="invitation-actions">
        <button type="button" className="btn btn-sm" disabled={busy !== null} onClick={() => answer(true)}>
          {busy === 'accept' ? 'Accepting…' : 'Accept'}
        </button>
        <button type="button" className="btn btn-light btn-sm" disabled={busy !== null} onClick={() => answer(false)}>
          {busy === 'decline' ? 'Declining…' : 'Decline'}
        </button>
      </div>
    </div>
  )
}
