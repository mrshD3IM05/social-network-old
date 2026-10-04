'use client'

import { useEffect } from 'react'
import { useRouter } from 'next/navigation'
import { useMe } from '@/lib/useMe'

// Login and register share this two-column layout: the form on the left,
// a presentation panel on the right.
//
// Both pages are guest-only: the API answers 403 to /login and /register when a
// session cookie is already valid, so someone who is logged in and lands here
// would only get "already authenticated" after filling in a form. This sends
// them to the feed before the form is ever shown.
export default function AuthLayout({ children }) {
  const router = useRouter()
  const { me, loading } = useMe()

  useEffect(() => {
    if (!loading && me) router.replace('/home')
  }, [loading, me, router])

  // blank while we ask, and while the redirect is on its way
  if (loading || me) return <p className="loading">Loading…</p>

  return (
    <div className="auth">
      <div className="auth-main">
        <p className="brand">social-network<span>.</span></p>
        {children}
      </div>

      <aside className="auth-aside">
        <p className="auth-quote">Share what matters with the people who matter.</p>
        <ul className="auth-list">
          <li>Just posted the photos from Saturday's hike.</li>
          <li>Great shots. Are you joining the photo walk next week?</li>
          <li>Already signed up. See you there.</li>
        </ul>
      </aside>
    </div>
  )
}

