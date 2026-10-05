'use client'

import { useEffect, useState } from 'react'
import { fetchMe, getMe, onMeChange } from './userStore'

// The logged-in user, redrawn whenever the store changes. `loading` tells
// "still asking" apart from "not logged in" (me is null in both cases).
export function useMe() {
  const [me, setMeState] = useState(getMe)
  const [loading, setLoading] = useState(() => getMe() === null)

  useEffect(() => {
    let here = true
    const stop = onMeChange(user => {
      setMeState(user)
      setLoading(false)
    })
    fetchMe().finally(() => here && setLoading(false))
    return () => {
      here = false
      stop()
    }
  }, [])

  return { me, loading }
}
