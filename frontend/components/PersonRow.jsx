'use client'

import Link from 'next/link'
import Avatar from '@/components/Avatar'

// One person in a list, with the page's action on the right (children). `href` makes it a link.
export default function PersonRow({ person, href, size = 44, children }) {
  const body = (
    <>
      <Avatar user={person} size={size} />
      <span className="list-text">
        <strong>{person.first_name} {person.last_name}</strong>
        <small>@{person.nickname}</small>
      </span>
      {children}
    </>
  )

  return href ? <Link href={href} className="list-item">{body}</Link> : <div className="list-item">{body}</div>
}
