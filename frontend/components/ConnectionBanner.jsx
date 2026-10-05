'use client'

import Link from 'next/link'
import { useEffect, useState } from 'react'
import { getStatus, onStatusChange, retryNow } from '@/lib/socket'

// Says when the shared connection is not live.
//
// A 4401 close is the one failure the server tells us about directly, so it gets
// its own message and a way to fix it. Everything else is a dropped connection:
// back off and try again, and say plainly when we have stopped trying. Without
// this the page looks fine while nothing is arriving, which is how an expired
// session used to go unnoticed.
export default function ConnectionBanner() {
  const [status, setStatus] = useState(getStatus)

  useEffect(() => onStatusChange(setStatus), [])

  if (status === 'closed' || status === 'online') return null

  if (status === 'unreachable') {
    return (
      <div className="conn-banner" role="status">
        <span>Cannot reach the server.</span>
        <button type="button" onClick={retryNow}>Try again</button>
      </div>
    )
  }

  return (
    <div className="conn-banner" role="status">
      <span>{status === 'connecting' ? 'Connecting…' : 'Reconnecting…'}</span>
    </div>
  )
}