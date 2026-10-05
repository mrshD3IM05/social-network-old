'use client'

import { useState } from 'react'
import { apiPost } from '@/lib/api'
import Icon from './Icon'

// One group event with its Going / Not going buttons. Clicking the chosen one
// again removes the answer. The change shows at once and is undone if the API refuses.
export default function EventCard({ event }) {
  const [state, setState] = useState({
    going_count: event.going_count,
    not_going_count: event.not_going_count,
    my_choice: event.my_choice || '',
  })
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  const when = new Date(event.date_time)
  const day = when.toLocaleDateString(undefined, { weekday: 'short', year: 'numeric', month: 'short', day: 'numeric' })
  const time = when.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' })
  const past = when.getTime() < Date.now()

  async function respond(choice) {
    const previous = state
    const count = (key, value) => state[key] + (choice === value) - (state.my_choice === value)
    setError('')
    setBusy(true)
    setState({ going_count: count('going_count', 'going'), not_going_count: count('not_going_count', 'not_going'), my_choice: choice })
    try {
      setState(await apiPost(`/events/${event.id}/response`, { choice }))
    } catch (err) {
      setState(previous)
      setError(err.message)
    }
    setBusy(false)
  }

  const button = (value, label) => {
    const chosen = state.my_choice === value
    return (
      <button
        className={chosen ? 'btn btn-sm' : 'btn btn-light btn-sm'}
        disabled={busy}
        onClick={() => respond(chosen ? '' : value)}
        title={chosen ? 'Click again to remove your answer' : undefined}
      >
        {label}
      </button>
    )
  }

  return (
    <article className="card event">
      <header className="event-head">
        <span className="list-icon"><Icon name="bell" size={18} /></span>
        <div className="event-who">
          <h3>{event.title}</h3>
          <small className="meta">by {event.creator_first_name} {event.creator_last_name}</small>
        </div>
        <div className="event-when">
          <strong>{day}</strong>
          <span className="meta">{time}{past ? ' · past' : ''}</span>
        </div>
      </header>

      {event.description && <p className="event-description">{event.description}</p>}

      <footer className="event-footer">
        <span className="meta">
          Going: <strong>{state.going_count}</strong> · Not going: <strong>{state.not_going_count}</strong>
          {state.my_choice && <> · you: <strong>{state.my_choice === 'going' ? 'Going' : 'Not going'}</strong></>}
        </span>
        <div className="event-actions">
          {button('going', 'Going')}
          {button('not_going', 'Not going')}
        </div>
      </footer>

      {error && <p className="error">{error}</p>}
    </article>
  )
}
