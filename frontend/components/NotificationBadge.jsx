'use client'

import { useEffect, useState } from 'react'
import { usePathname } from 'next/navigation'
import { apiGet } from '@/lib/api'
import { subscribe } from '@/lib/socket'

// Unread notifications count beside Notifications. The notifications page marks them read.
export default function NotificationBadge() {
  const onPage = usePathname() === '/notifications'
  const [count, setCount] = useState(0)

  useEffect(() => {
    const away = () => window.location.pathname !== '/notifications'
    apiGet('/notifications/unread')
      .then(result => away() && setCount(result.count))
      .catch(() => {})
    return subscribe(data => {
      if (data.type === 'notification' && away()) setCount(c => c + 1)
    })
  }, [])

  useEffect(() => {
    if (onPage) setCount(0)
  }, [onPage])

  if (count === 0) return null
  return <span className="menu-count">{count > 99 ? '99+' : count}</span>
}
