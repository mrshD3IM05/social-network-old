// next.config.js forwards /api/v1/... to the Go API on http://localhost:8080/...
const API = '/api/v1'

async function request(path, options = {}) {
  const res = await fetch(API + path, { credentials: 'include', ...options })
  const text = await res.text()
  if (!res.ok) {
    const error = new Error(text || 'Something went wrong')
    error.status = res.status
    throw error
  }
  return text ? JSON.parse(text) : null
}

export const apiGet = path => request(path)

// An array becomes the same field repeated: { viewers: [2, 5] } → viewers=2&viewers=5
function fill(body, data) {
  for (const [name, value] of Object.entries(data)) {
    for (const item of [].concat(value)) body.append(name, item)
  }
  return body
}

export const apiPost = (path, data = {}) => request(path, { method: 'POST', body: fill(new URLSearchParams(), data) })
export const apiPut = (path, data = {}) => request(path, { method: 'PUT', body: fill(new URLSearchParams(), data) })
export const apiDelete = path => request(path, { method: 'DELETE' })
// multipart, for files: apiUpload('/files', { files, post_id: 3 })
export const apiUpload = (path, data) => request(path, { method: 'POST', body: fill(new FormData(), data) })

export const imageUrl = id => `${API}/fs/${id}`

// In dev (next on :3000) connect straight to the Go server; behind Caddy go through /api/v1.
export function socketUrl() {
  const { hostname, host, port } = window.location
  return port === '3000' ? `ws://${hostname}:8080/ws` : `ws://${host}${API}/ws`
}
