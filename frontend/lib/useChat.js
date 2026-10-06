'use client'

import { useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react'
import { apiUpload } from './api'
import { sendWs, subscribe } from './socket'
import { useThrottle } from './timing'
import useMessageHistory from './useMessageHistory'
import { LIMITS, checkText, pickImages } from './validate'

// Everything a chat needs: the history, live messages and "typing…" over the
// socket, scrolling, and the composer. `target` is { to_user_id } or { group_id }.
export default function useChat(path, target) {
  const { group_id: groupId, to_user_id: userId } = target
  const history = useMessageHistory(path)
  const { messages, setMessages, hasMore, loadingMore, loadMore } = history
  const [text, setText] = useState('')
  const [files, setFiles] = useState([])
  const [typing, setTyping] = useState(0) // id of who is typing, 0 for nobody
  const [error, setError] = useState('')
  const [sending, setSending] = useState(false)
  const listRef = useRef(null)
  const loadMoreRef = useRef(null)
  const fileRef = useRef(null)
  const preservedScroll = useRef(null)

  useEffect(() => {
    let typingTimer = null
    const unsub = subscribe(data => {
      if (data.type === 'message') {
        const msg = data.message
        const here = groupId
          ? msg.group_id === groupId
          : !msg.group_id && (msg.from_user_id === userId || msg.to_user_id === userId)
        if (here) setMessages(list => (list?.some(m => m.id === msg.id) ? list : [...(list || []), msg]))
      }
      if (data.type === 'typing' && (groupId ? data.group_id === groupId : !data.group_id && data.from_user_id === userId)) {
        setTyping(data.from_user_id)
        clearTimeout(typingTimer)
        typingTimer = setTimeout(() => setTyping(0), 3000)
      }
      if (data.type === 'error') setError(data.error)
    })
    return () => {
      clearTimeout(typingTimer)
      unsub()
    }
  }, [groupId, userId])

  useEffect(() => {
    if (history.error) setError(history.error.message)
  }, [history.error])

  // stay at the bottom, except when older messages are added on top
  useLayoutEffect(() => {
    const list = listRef.current
    if (!list) return
    const preserved = preservedScroll.current
    if (preserved) {
      list.scrollTop = preserved.top + list.scrollHeight - preserved.height
      preservedScroll.current = null
    } else if (messages !== null) {
      list.scrollTop = list.scrollHeight
    }
  }, [messages])

  useEffect(() => {
    if (typing && listRef.current) listRef.current.scrollTop = listRef.current.scrollHeight
  }, [typing])

  const loadOlder = useCallback(() => {
    const list = listRef.current
    if (list) preservedScroll.current = { top: list.scrollTop, height: list.scrollHeight }
    loadMore()
  }, [loadMore])

  const onScroll = () => {
    if (listRef.current?.scrollTop <= 80) loadOlder()
  }

  // the "Load older messages" button coming into view loads them
  useEffect(() => {
    const button = loadMoreRef.current
    if (!hasMore || loadingMore || !button) return
    const observer = new IntersectionObserver(
      entries => entries[0].isIntersecting && loadOlder(),
      { root: listRef.current, rootMargin: '80px 0px 0px' },
    )
    observer.observe(button)
    return () => observer.disconnect()
  }, [hasMore, loadingMore, loadOlder])

  const sendTyping = useThrottle(() => sendWs({ type: 'typing', ...target }), 2000)

  function type(value) {
    setText(value)
    sendTyping()
  }

  async function pickFiles(e) {
    const picked = await pickImages(e)
    setError(picked.error)
    setFiles(picked.files)
  }

  function clearFiles() {
    setFiles([])
    if (fileRef.current) fileRef.current.value = ''
  }

  // a text-only message goes over the socket; one with images is a single multipart request
  async function send(e) {
    e.preventDefault()
    if (sending) return
    const problem = text.trim()
      ? checkText('Your message', text, LIMITS.message)
      : files.length === 0 && 'Write something or add an image.'
    if (problem) return setError(problem)

    sendTyping.cancel()
    setError('')
    setSending(true)
    try {
      if (files.length > 0) {
        const msg = await apiUpload('/messages', { ...target, content: text.trim(), files })
        setMessages(list => (list?.some(m => m.id === msg.id) ? list : [...(list || []), msg]))
      } else if (!sendWs({ type: 'message', ...target, content: text.trim() })) {
        throw new Error('Chat connection is not ready. Please try again.')
      }
      setText('')
      clearFiles()
    } catch (err) {
      setError(err.message)
    }
    setSending(false)
  }

  return {
    ...history, historyError: history.error, text, type, files, pickFiles, clearFiles, send, sending, typing, error,
    listRef, loadMoreRef, fileRef, loadOlder, onScroll,
  }
}
