// Who is logged in: asked once (GET /me), then kept in memory and shared by
// every page. Mutations that answer with the updated user call setMe().
import { apiGet } from './api'

let me = null
let pending = null // the GET /me in flight
let generation = 0 // bumped on logout, so a late answer cannot bring the old session back
const listeners = new Set()

const tellEveryone = () => listeners.forEach(cb => cb(me))

export const getMe = () => me

// Never rejects: an invalid session answers null.
export function fetchMe() {
  if (me) return Promise.resolve(me)
  if (!pending) {
    const asked = ++generation
    const done = user => {
      if (asked !== generation) return
      me = user
      tellEveryone()
    }
    pending = apiGet('/me')
      .then(done, () => done(null))
      .finally(() => (pending = null))
  }
  return pending.then(() => me)
}

export function setMe(user) {
  me = user
  tellEveryone()
}

export function forgetMe() {
  generation++
  setMe(null)
}

export function onMeChange(cb) {
  listeners.add(cb)
  return () => listeners.delete(cb)
}
