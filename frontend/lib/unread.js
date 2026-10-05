// Ids of the people whose messages are not read yet. Memory only.
const unread = new Set()
const listeners = new Set()

const tellEveryone = () => listeners.forEach(cb => cb(new Set(unread)))

export function markUnread(userId) {
  unread.add(userId)
  tellEveryone()
}

export function markRead(userId) {
  if (unread.delete(userId)) tellEveryone()
}

export const getUnread = () => new Set(unread)

export function onUnreadChange(cb) {
  listeners.add(cb)
  return () => listeners.delete(cb)
}
