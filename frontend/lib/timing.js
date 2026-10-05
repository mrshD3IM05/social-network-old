import { useEffect, useMemo, useRef, useState } from 'react'

// Throttle that runs the first call at once and the last call of a burst once
// `wait` is over, so the latest call is never lost. Always calls the latest fn,
// and drops a pending call when the component goes away.
export function useThrottle(fn, wait) {
  const fnRef = useRef(fn)
  useEffect(() => {
    fnRef.current = fn
  })

  const throttled = useMemo(() => {
    let last = 0
    let timer = null
    const run = () => {
      last = Date.now()
      timer = null
      fnRef.current()
    }
    const call = () => {
      const remaining = wait - (Date.now() - last)
      if (remaining <= 0) {
        clearTimeout(timer)
        run()
      } else if (!timer) {
        timer = setTimeout(run, remaining)
      }
    }
    call.cancel = () => {
      clearTimeout(timer)
      timer = null
    }
    return call
  }, [wait])

  useEffect(() => throttled.cancel, [throttled])
  return throttled
}

// `value` once it has stopped changing for `delay` ms
export function useDebouncedValue(value, delay) {
  const [debounced, setDebounced] = useState(value)
  useEffect(() => {
    const timer = setTimeout(() => setDebounced(value), delay)
    return () => clearTimeout(timer)
  }, [value, delay])
  return debounced
}
