'use client'

import { useEffect } from 'react'
import { useRouter } from 'next/navigation'
import { useMe } from '@/lib/useMe'
import { ensureSocket } from '@/lib/socket'
import ConnectionBanner from '@/components/ConnectionBanner'
import Navbar from '@/components/Navbar'
import SidePanel from '@/components/SidePanel'

// Every page inside (main): logged in only, with the nav rail and the side panel.
export default function MainLayout({ children }) {
  const router = useRouter()
  const { me, loading } = useMe()
  const myId = me?.id

  useEffect(() => {
    if (!loading && !me) router.push('/login')
  }, [loading, me, router])

  useEffect(() => {
    if (myId) ensureSocket()
  }, [myId])

  if (!me) return <p className="loading">Loading…</p>

  return (
    <>
      <ConnectionBanner />
      <div className="app">
        <Navbar user={me} />
        <main className="main">
          <div className="page">{children}</div>
        </main>
        <SidePanel />
      </div>
    </>
  )
}
