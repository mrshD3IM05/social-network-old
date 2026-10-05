'use client'

import usePaged from '@/lib/usePaged'
import LoadMore from './LoadMore'

export const privacyNames = { public: 'Public', almost_private: 'Followers', private: 'Chosen followers' }

export function PrivacySelect({ value, onChange }) {
  return (
    <select className="tool" value={value} onChange={e => onChange(e.target.value)}>
      {Object.entries(privacyNames).map(([key, name]) => <option key={key} value={key}>{name}</option>)}
    </select>
  )
}

// Tick the followers who may see a "Chosen followers" post. `selected` is a list of ids.
export function ViewerPicker({ myId, selected, onChange }) {
  const followers = usePaged(`/users/${myId}/followers`)
  if (followers.items === null) return null

  const toggle = id => onChange(selected.includes(id) ? selected.filter(v => v !== id) : [...selected, id])

  return (
    <div className="viewer-picker">
      <p className="hint">Who can see this post?</p>
      {followers.items.length === 0 && <p className="hint">You have no followers yet.</p>}
      {followers.items.map(person => (
        <label key={person.id} className={selected.includes(person.id) ? 'viewer-chip active' : 'viewer-chip'}>
          <input type="checkbox" checked={selected.includes(person.id)} onChange={() => toggle(person.id)} />
          {person.first_name} {person.last_name}
        </label>
      ))}
      <LoadMore list={followers} />
    </div>
  )
}
