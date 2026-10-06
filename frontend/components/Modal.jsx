'use client'

import { useEffect, useRef } from 'react'
import { createPortal } from 'react-dom'
import Icon from '@/components/Icon'

// Centered dialog, closed with X, the backdrop or Escape. Rendered on <body>
// so the page's animations and scroll position do not affect it.
export default function Modal({ title, onClose, children }) {
  // onClose is a new function on every render; the ref lets the effect run once per open
  const onCloseRef = useRef(onClose)
  onCloseRef.current = onClose
  const close = () => onCloseRef.current()

  useEffect(() => {
    const onKey = e => e.key === 'Escape' && onCloseRef.current()
    document.addEventListener('keydown', onKey)

    // lock the page behind on <html> only: hiding <body>'s overflow too would make it
    // its own scroll box and the sticky side columns would jump while the dialog is open.
    // Pad the scrollbar's place so nothing shifts sideways.
    const { body, documentElement: root } = document
    const scrollbar = window.innerWidth - root.clientWidth
    const prev = { padding: body.style.paddingRight, root: root.style.overflow }
    const scrollY = prev.root !== 'hidden' ? window.scrollY : null // null: another modal locked it
    root.style.overflow = 'hidden'
    if (scrollbar > 0) body.style.paddingRight = `${scrollbar}px`

    return () => {
      document.removeEventListener('keydown', onKey)
      body.style.paddingRight = prev.padding
      root.style.overflow = prev.root
      if (scrollY !== null) window.scrollTo(0, scrollY)
    }
  }, [])

  if (typeof document === 'undefined') return null

  return createPortal(
    <div className="modal-backdrop" onClick={close}>
      <div className="modal" role="dialog" aria-modal="true" onClick={e => e.stopPropagation()}>
        <header className="modal-header">
          <h2>{title}</h2>
          <button type="button" className="icon-button" onClick={close} title="Close">
            <Icon name="x" />
          </button>
        </header>
        {children}
      </div>
    </div>,
    document.body,
  )
}
