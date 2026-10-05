'use client'

import Link from 'next/link'
import { usePathname, useRouter } from 'next/navigation'
import { apiPost } from '@/lib/api'
import { closeSocket, ensureSocket } from '@/lib/socket'
import { forgetMe } from '@/lib/userStore'
import Avatar from './Avatar'
import Icon from './Icon'
import MessageDot from './MessageDot'
import NotificationBadge from './NotificationBadge'
import ThemeToggle from './ThemeToggle'

const links = [
  { href: '/home', label: 'Feed', icon: 'home' },
  { href: '/people', label: 'People', icon: 'users' },
  { href: '/groups', label: 'Groups', icon: 'grid' },
  { href: '/chat', label: 'Messages', icon: 'chat' },
  { href: '/notifications', label: 'Notifications', icon: 'bell' },
  { href: '/settings', label: 'Settings', icon: 'settings' },
]

// The sidebar; on phones a top bar and a bottom tab bar (see globals.css).
export default function Navbar({ user }) {
  const pathname = usePathname()
  const router = useRouter()

  async function logout() {
    closeSocket() // first, so the server closing it is not read as an ended session
    try {
      await apiPost('/logout')
    } catch {
      ensureSocket() // still logged in: keep receiving
      return
    }
    forgetMe()
    router.push('/login')
  }

  const logoutButton = (
    <button className="icon-button" onClick={logout} title="Log out" aria-label="Log out">
      <Icon name="logout" />
    </button>
  )

  return (
    <>
      <header className="topbar">
        <Link href="/home" className="brand">social-network<span>.</span></Link>
        <div className="topbar-user">
          <Link href={`/profile/${user.id}`} aria-label="Your profile">
            <Avatar user={user} size={32} />
          </Link>
          <ThemeToggle />
          {logoutButton}
        </div>
      </header>

      <aside className="rail">
        <Link href="/home" className="brand">social-network<span>.</span></Link>

        <nav className="menu">
          {links.map(link => {
            const active = pathname.startsWith(link.href)
            return (
              <Link
                key={link.href}
                href={link.href}
                className={active ? 'menu-item active' : 'menu-item'}
                aria-current={active ? 'page' : undefined}
              >
                <Icon name={link.icon} size={20} />
                <span className="menu-label">{link.label}</span>
                {link.href === '/chat' && <MessageDot myId={user.id} />}
                {link.href === '/notifications' && <NotificationBadge />}
              </Link>
            )
          })}
        </nav>

        <div className="rail-user">
          <Link href={`/profile/${user.id}`} className="user-chip">
            <Avatar user={user} size={38} />
            <span>
              <strong>{user.first_name} {user.last_name}</strong>
              <small>@{user.nickname}</small>
            </span>
          </Link>
          <ThemeToggle />
          {logoutButton}
        </div>
      </aside>
    </>
  )
}
