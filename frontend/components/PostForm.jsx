'use client'

import { useState } from 'react'
import { apiUpload } from '@/lib/api'
import { useMe } from '@/lib/useMe'
import usePaged from '@/lib/usePaged'
import { IMAGE_ACCEPT, LIMITS, checkImageFiles, checkImages, checkText } from '@/lib/validate'
import CharCount from './CharCount'
import Icon from './Icon'
import LoadMore from './LoadMore'

// Form to write a new post. onPosted() is called after it is saved.
// Inside a group, `groupId` is set: the post goes to the group (members only)
// and the privacy selector disappears — group posts are member-only by design.
export default function PostForm({ onPosted, groupId }) {
  const [content, setContent] = useState('')
  const [privacy, setPrivacy] = useState('public')
  const [files, setFiles] = useState([])
  const { me } = useMe()
  // The follower list is only needed for "Chosen followers", so it is not asked
  // for until that privacy is picked.
  const [pickingViewers, setPickingViewers] = useState(false)
  const followers = usePaged(pickingViewers && me ? `/users/${me.id}/followers` : null) // 10 at a time
  const [viewers, setViewers] = useState([]) // ids of the followers who can see a private post
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  // Checked while typing so the Publish button knows if the post is valid
  const contentError = checkText('Your post', content, LIMITS.post, { required: files.length === 0 })

  // Keep the picked images only if there are at most 3 valid ones
  async function pickFiles(e) {
    const picked = Array.from(e.target.files)
    const imageError = await checkImageFiles(picked)

    setError(imageError)
    setFiles(imageError ? [] : picked)
    if (imageError) e.target.value = '' // let the user pick again
  }

  function changePrivacy(e) {
    setPrivacy(e.target.value)
    setPickingViewers(e.target.value === 'private')
  }

  function toggleViewer(id) {
    setViewers(list => (list.includes(id) ? list.filter(v => v !== id) : [...list, id]))
  }

  function clearFiles() {
    setFiles([])
    setError('')
  }

  async function handleSubmit(e) {
    e.preventDefault()

    // check everything once more before calling the API
    const noViewers = !groupId && privacy === 'private' && viewers.length === 0
    const problem = contentError || checkImages(files) || (noViewers ? 'Choose at least one follower.' : '')
    if (problem) {
      setError(problem)
      return
    }

    setError('')
    setLoading(true)

    try {
      const formData = new FormData()
      formData.append('content', content.trim())
      if (!groupId) {
        formData.append('privacy', privacy)
        for (const viewerID of privacy === 'private' ? viewers : []) formData.append('viewers', viewerID)
      }
      for (const file of files) formData.append('files', file)

      const path = groupId ? `/groups/${groupId}/posts` : '/posts'
      const post = await apiUpload(path, formData)
      onPosted(post)

      // 3. reset the form
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
        {/* the real file input is hidden, the label acts as the button */}
        <label className="tool">
          <Icon name="image" />
          {files.length > 0 ? `${files.length} photo${files.length > 1 ? 's' : ''}` : 'Photo'}
          <input
            type="file"
            accept={IMAGE_ACCEPT}
            multiple
            hidden
            onChange={pickFiles}
          />
        </label>

        {files.length > 0 && (
          <button type="button" className="tool" onClick={clearFiles}>Remove</button>
        )}

        {!groupId && (
          <select className="tool" value={privacy} onChange={changePrivacy}>
            <option value="public">Public</option>
            <option value="almost_private">Followers</option>
            <option value="private">Chosen followers</option>
          </select>
        )}

        <CharCount value={content} max={LIMITS.post} />

        <button className="btn" disabled={loading || Boolean(contentError)}>
          {loading ? 'Publishing…' : 'Publish'}
        </button>
      </div>

      {!groupId && privacy === 'private' && followers.items !== null && (
        <div className="viewer-picker">
          <p className="hint">Who can see this post?</p>
          {followers.items.length === 0 && <p className="hint">You have no followers yet.</p>}
          {followers.items.map(person => (
            <label key={person.id} className={viewers.includes(person.id) ? 'viewer-chip active' : 'viewer-chip'}>
              <input type="checkbox" checked={viewers.includes(person.id)} onChange={() => toggleViewer(person.id)} />
              {person.first_name} {person.last_name}
            </label>
          ))}
          <LoadMore list={followers} />
        </div>
      )}

      <p className="hint">Up to {LIMITS.images} images, JPEG, PNG or GIF, 10 MB each.</p>

      {error && <p className="error">{error}</p>}
    </form>
  )
}
