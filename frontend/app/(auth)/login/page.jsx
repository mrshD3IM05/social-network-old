'use client'

import { useState } from 'react'
import Link from 'next/link'
import { useRouter } from 'next/navigation'
import { apiPost } from '@/lib/api'
import { setMe } from '@/lib/userStore'
import { LIMITS } from '@/lib/validate'

export default function LoginPage() {
  const router = useRouter()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  async function handleSubmit(e) {
    e.preventDefault()
    // the API answers "invalid credentials" anyway, so only check what is missing
    const problem = !email.trim() ? 'Enter your email or nickname.' : !password ? 'Enter your password.' : ''
    setError(problem)
    if (problem) return
    setLoading(true)
    try {
      // the field also accepts a nickname; the answer is the user
      setMe(await apiPost('/login', { email: email.trim(), password }))
      router.push('/home')
    } catch (err) {
      setError(err.message)
      setLoading(false)
    }
  }

  return (
    <form className="auth-form" onSubmit={handleSubmit} noValidate>
      <h1>Welcome back</h1>
      <p className="subtitle">Log in to continue to your feed.</p>

      <label>Email or nickname</label>
      <input value={email} maxLength={LIMITS.email} onChange={e => setEmail(e.target.value)} autoFocus />

      <label>Password</label>
      <input type="password" value={password} maxLength={LIMITS.password.max} onChange={e => setPassword(e.target.value)} />

      {error && <p className="error">{error}</p>}

      <button className="btn btn-full" disabled={loading}>{loading ? 'Logging in…' : 'Log in'}</button>

      <p className="switch">
        New to social-network? <Link href="/register">Create an account</Link>
      </p>
    </form>
  )
}
