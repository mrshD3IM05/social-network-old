'use client'

import { useEffect, useRef } from 'react'
import { createPortal } from 'react-dom'
import Icon from '@/components/Icon'

// Simple centered dialog. Close with the X button, the backdrop, or Escape.
//
// It is mounted in a portal on <body> instead of inside the page that opened
// it. That keeps it out of the page's own animation/stacking context (the
// `.page > *` entrance animation would otherwise fade and delay the dialog on
// every render) and keeps it centered on the viewport no matter how far the
// page behind it is scrolled.
export default function Modal({ title, onClose, children }) {
  // Callers pass an inline arrow (`() => setShow(false)`), so onClose is a new
  // function on every parent render. Reading it through a ref lets the effect
  // below run exactly once per open: re-running it would toggle the body
  // scroll lock off and on, which shifts the layout and looks like a refresh.
  const onCloseRef = useRef(onClose)
  onCloseRef.current = onClose

  useEffect(() => {
    function onKey(e) {
      if (e.key === 'Escape') onCloseRef.current()
    }
    document.addEventListener('keydown', onKey)

    // Lock the page behind the dialog while keeping the scroll position.
    // Overflow is hidden on <html> only: that stops the window from scrolling.
    // Hiding it on <body> too would turn <body> into its own scroll box, and
    // the sticky side columns would then stick to it instead of the window,
    // jumping up and getting cut off while the dialog is open. Hiding the
    // scrollbar would shift the layout sideways, so pad the freed gap back.
    const body = document.body
    const root = document.documentElement
    const scrollbar = window.innerWidth - root.clientWidth
    const prevBodyPadding = body.style.paddingRight
    const prevRootOverflow = root.style.overflow
    const locked = prevRootOverflow !== 'hidden'
    const scrollY = locked ? window.scrollY : null

    root.style.overflow = 'hidden'
    if (scrollbar > 0) body.style.paddingRight = `${scrollbar}px`

    return () => {
      document.removeEventListener('keydown', onKey)
      body.style.paddingRight = prevBodyPadding
      root.style.overflow = prevRootOverflow
      // Only restore the position if this modal did the locking: restoring
      // blindly could fight another lock (e.g. a stacked second modal).
      if (scrollY !== null) window.scrollTo(0, scrollY)
    }
  }, [])

  // The server render has no document; the dialog only ever opens on the
  // client anyway (from a click), so nothing is lost.
  if (typeof document === 'undefined') return null

  return createPortal(
    <div className="modal-backdrop" onClick={() => onCloseRef.current()}>
      <div
        className="modal"
        role="dialog"
        aria-modal="true"
        onClick={e => e.stopPropagation()} // clicks inside stay open
      >
        <header className="modal-header">
          <h2>{title}</h2>
          <button
            type="button"
            className="icon-button"
            onClick={() => onCloseRef.current()}
            title="Close"
          >
            <Icon name="x" />
          </button>
        </header>
        {children}
      </div>
    </div>,
    document.body,
  )
}
