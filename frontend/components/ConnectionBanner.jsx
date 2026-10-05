'use client'

import { useEffect, useState } from 'react'
import { getStatus, onStatusChange, retryNow } from '@/lib/socket'

// Says when the live connection is down, so a dead page does not look fine.
export default function ConnectionBanner() {
  const [status, setStatus] = useState(getStatus)
  useEffect(() => onStatusChange(setStatus), [])

  if (status === 'closed' || status === 'online') return null

  return (
    <div className="conn-banner" role="status">
      {status === 'unreachable' ? (
        <>
          <span>Cannot reach the server.</span>
          <button type="button" onClick={retryNow}>Try again</button>
        </>
      ) : (
        <span>{status === 'connecting' ? 'Connecting…' : 'Reconnecting…'}</span>
      )}
    </div>
  )
}
