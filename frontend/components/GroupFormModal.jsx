'use client'

import { useState } from 'react'
import { apiPost, apiPut, apiUpload } from '@/lib/api'
import { IMAGE_ACCEPT, LIMITS, checkImageFile, checkText } from '@/lib/validate'
import Avatar from './Avatar'
import CharCount from './CharCount'
import Icon from './Icon'
import Modal from './Modal'

// Creates a group, or edits `group` (title, description and picture) when given.
export default function GroupFormModal({ group, onClose, onSaved }) {
  const [title, setTitle] = useState(group?.title || '')
  const [description, setDescription] = useState(group?.description || '')
  const [picture, setPicture] = useState(null)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const titleError = checkText('Title', title, LIMITS.groupTitle)
  const descriptionError = checkText('Description', description, LIMITS.groupDescription, { required: false })

  async function pickPicture(e) {
    const file = e.target.files[0]
    if (!file) return
    const problem = await checkImageFile(file)
    setError(problem)
    if (problem) e.target.value = ''
    else setPicture(file)
  }

  async function handleSubmit(e) {
    e.preventDefault()
    const problem = titleError || descriptionError
    setError(problem)
    if (problem) return
    setLoading(true)
    const fields = { title: title.trim(), description: description.trim() }
    try {
      if (group) {
        await apiPut(`/groups/${group.id}`, fields)
        if (picture) await apiUpload(`/groups/${group.id}/avatar`, { avatar: picture })
        onSaved()
      } else {
        onSaved(await apiPost('/groups', fields))
      }
    } catch (err) {
      setError(err.message)
      setLoading(false)
    }
  }

  return (
    <Modal title={group ? 'Edit group' : 'Create a group'} onClose={onClose}>
      <form onSubmit={handleSubmit} noValidate>
        {group && (
          <div className="photo-row">
            {picture ? (
              <img className="avatar" style={{ width: 64, height: 64 }} src={URL.createObjectURL(picture)} alt="" />
            ) : (
              <Avatar user={{ first_name: group.title, avatar: group.avatar }} size={64} />
            )}
            <label className="btn btn-light">
              <Icon name="camera" size={16} /> Change picture
              <input type="file" accept={IMAGE_ACCEPT} hidden onChange={pickPicture} />
            </label>
          </div>
        )}

        <label>Title</label>
        <input
          value={title}
          maxLength={LIMITS.groupTitle}
          className={error && titleError ? 'invalid' : undefined}
          onChange={e => setTitle(e.target.value)}
          autoFocus={!group}
        />

        <label>Description <small>optional</small></label>
        <textarea
          rows={3}
          value={description}
          maxLength={LIMITS.groupDescription}
          className={error && descriptionError ? 'invalid' : undefined}
          onChange={e => setDescription(e.target.value)}
        />

        <div className="composer-bar">
          <CharCount value={description} max={LIMITS.groupDescription} />
          <button className="btn" disabled={loading || Boolean(titleError || descriptionError)}>
            {group ? (loading ? 'Saving…' : 'Save') : (loading ? 'Creating…' : 'Create group')}
          </button>
        </div>

        {error && <p className="error">{error}</p>}
      </form>
    </Modal>
  )
}
