'use client'

import { useState } from 'react'
import { apiPost, apiUpload } from '@/lib/api'
import { useMe } from '@/lib/useMe'
import { IMAGE_ACCEPT, LIMITS, checkText, pickImages } from '@/lib/validate'
import CharCount from './CharCount'
import Icon from './Icon'
import { PrivacySelect, ViewerPicker } from './Privacy'

// Writes a new post. With `groupId` the post goes to that group (members only, no privacy).
export default function PostForm({ onPosted, groupId }) {
  const { me } = useMe()
  const [content, setContent] = useState('')
  const [privacy, setPrivacy] = useState('public')
  const [viewers, setViewers] = useState([]) // followers who may see a "Chosen followers" post
  const [files, setFiles] = useState([])
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const contentError = checkText('Your post', content, LIMITS.post)
  const chooseViewers = !groupId && privacy === 'private'

  async function pickFiles(e) {
    const picked = await pickImages(e)
    setError(picked.error)
    setFiles(picked.files)
  }

  async function handleSubmit(e) {
    e.preventDefault()
    const problem = contentError || (chooseViewers && viewers.length === 0 ? 'Choose at least one follower.' : '')
    setError(problem)
    if (problem) return
    setLoading(true)
    try {
      const post = groupId
        ? await apiPost(`/groups/${groupId}/posts`, { content: content.trim() })
        : await apiPost('/posts', { content: content.trim(), privacy, viewers: chooseViewers ? viewers : [] })
      if (files.length > 0) await apiUpload('/files', { files, post_id: post.id })
      onPosted()
      setContent('')
      setFiles([])
      setViewers([])
    } catch (err) {
      setError(err.message)
    }
    setLoading(false)
  }

  return (
    <form className="card composer" onSubmit={handleSubmit} noValidate>
      <textarea
        placeholder={groupId ? 'Share something with the group…' : 'Share something with your followers…'}
        value={content}
        maxLength={LIMITS.post}
        onChange={e => setContent(e.target.value)}
      />

      <div className="composer-bar">
        <label className="tool">
          <Icon name="image" />
          {files.length > 0 ? `${files.length} photo${files.length > 1 ? 's' : ''}` : 'Photo'}
          <input type="file" accept={IMAGE_ACCEPT} multiple hidden onChange={pickFiles} />
        </label>
        {files.length > 0 && (
          <button type="button" className="tool" onClick={() => { setFiles([]); setError('') }}>Remove</button>
        )}
        {!groupId && <PrivacySelect value={privacy} onChange={setPrivacy} />}
        <CharCount value={content} max={LIMITS.post} />
        <button className="btn" disabled={loading || Boolean(contentError)}>
          {loading ? 'Publishing…' : 'Publish'}
        </button>
      </div>

      {chooseViewers && me && <ViewerPicker myId={me.id} selected={viewers} onChange={setViewers} />}

      <p className="hint">Up to {LIMITS.images} images, JPEG, PNG or GIF, 10 MB each.</p>
      {error && <p className="error">{error}</p>}
    </form>
  )
}
