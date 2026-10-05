'use client'

import { useEffect, useState } from 'react'

// 'light', 'dark' or 'system'. Saved in localStorage and written on <html> as
// data-theme, which globals.css reads ('system' removes the attribute).
const KEY = 'theme'

// Runs in <head> before the first paint, so a dark theme never flashes light (app/layout.jsx).
export const themeScript =
  `try{var t=localStorage.getItem('${KEY}');if(t==='light'||t==='dark')document.documentElement.setAttribute('data-theme',t)}catch(e){}`

function savedChoice() {
  try {
    const value = localStorage.getItem(KEY)
    return value === 'light' || value === 'dark' ? value : 'system'
  } catch {
    return 'system' // storage blocked (private window)
  }
}

function apply(choice) {
  if (choice === 'system') document.documentElement.removeAttribute('data-theme')
  else document.documentElement.setAttribute('data-theme', choice)
}

export function setTheme(choice) {
  try {
    if (choice === 'system') localStorage.removeItem(KEY)
    else localStorage.setItem(KEY, choice)
  } catch {}
  apply(choice)
  window.dispatchEvent(new Event('themechange'))
}

// { choice, theme }: what was picked, and what is on screen ('light' | 'dark').
export function useTheme() {
  const [state, setState] = useState({ choice: 'system', theme: 'light' })

  useEffect(() => {
    const media = window.matchMedia('(prefers-color-scheme: dark)')
    function update() {
      const choice = savedChoice()
      setState({ choice, theme: choice === 'system' ? (media.matches ? 'dark' : 'light') : choice })
    }
    function fromOtherTab(e) {
      if (e.key !== KEY) return
      apply(savedChoice())
      update()
    }
    update()
    media.addEventListener('change', update)
    window.addEventListener('themechange', update)
    window.addEventListener('storage', fromOtherTab)
    return () => {
      media.removeEventListener('change', update)
      window.removeEventListener('themechange', update)
      window.removeEventListener('storage', fromOtherTab)
    }
  }, [])

  return state
}
