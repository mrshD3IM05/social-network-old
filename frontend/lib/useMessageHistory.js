'use client'

import { useEffect, useRef, useState } from 'react'
import { apiGet } from './api'
import { useThrottle } from './timing'
import { withLast } from './usePaged'

const MESSAGE_PAGE_SIZE = 10 // MessagePageSize in backend/internal/repository

// Chat history grows upward: the next page is the one before the oldest message shown.
export default function useMessageHistory(path) {
  const [messages, setMessages] = useState(null)
  const [hasMore, setHasMore] = useState(false)
  const [loadingMore, setLoadingMore] = useState(false)
  const [error, setError] = useState(null)
  const oldestID = useRef(0)
  const loading = useRef(false)
  const current = useRef(path)

  useEffect(() => {
    current.current = path
    oldestID.current = 0
    loading.current = false
    setMessages(null)
    setHasMore(false)
    setLoadingMore(false)
    setError(null)

    apiGet(path)
      .then(page => {
        if (current.current !== path) return
        oldestID.current = page[0]?.id || 0
        setMessages(page)
        setHasMore(page.length === MESSAGE_PAGE_SIZE)
      })
      .catch(err => {
        if (current.current !== path) return
        setMessages([])
        setError(err)
      })
  }, [path])

  const loadMore = useThrottle(async () => {
    if (loading.current || !hasMore || !oldestID.current) return
    const before = oldestID.current
    loading.current = true
    setLoadingMore(true)
    try {
      const page = await apiGet(withLast(path, before))
      if (current.current !== path) return
      oldestID.current = page[0]?.id || before
      setMessages(list => [...page, ...(list || [])])
      setHasMore(page.length === MESSAGE_PAGE_SIZE)
    } catch (err) {
      if (current.current === path) setError(err)
    } finally {
      if (current.current === path) {
        loading.current = false
        setLoadingMore(false)
      }
    }
  }, 500)

  return { messages, setMessages, hasMore, loadingMore, error, loadMore }
}
