'use client'

import { useCallback, useEffect, useRef, useState } from 'react'
import { apiGet } from './api'

export const PAGE_SIZE = 10 // PageSize in backend/internal/repository

export const withLast = (path, last) => `${path}${path.includes('?') ? '&' : '?'}last=${last}`

// Loads a list 10 by 10; the next page is asked with ?last=<id of the last item>.
// Pass null as path to wait.
export default function usePaged(path) {
  const [items, setItems] = useState(null) // null until the first page arrives
  const [hasMore, setHasMore] = useState(false)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState(null)
  const current = useRef(path) // answers for an older path are ignored

  const load = useCallback(async last => {
    if (!path) return
    current.current = path
    setLoading(true)
    try {
      const page = await apiGet(last ? withLast(path, last) : path)
      if (current.current !== path) return
      setItems(list => (last ? [...list, ...page] : page))
      setHasMore(page.length === PAGE_SIZE)
      setError(null)
    } catch (err) {
      if (current.current === path) setError(err)
    }
    if (current.current === path) setLoading(false)
  }, [path])

  useEffect(() => {
    load(0)
  }, [load])

  // every shown item removed (requests answered) while more are waiting: fetch them
  useEffect(() => {
    if (items?.length === 0 && hasMore) load(0)
  }, [items, hasMore, load])

  return {
    items,
    setItems,
    hasMore,
    loading,
    error,
    loadMore: () => load(items?.at(-1)?.id || 0),
    reload: () => load(0),
  }
}
