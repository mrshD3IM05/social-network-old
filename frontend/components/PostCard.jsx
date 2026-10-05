'use client'

import { useEffect, useState } from 'react'
import Link from 'next/link'
import { apiDelete, apiGet, apiPost, apiPut, apiUpload, imageUrl } from '@/lib/api'
import { IMAGE_ACCEPT, LIMITS, checkImageFiles, checkImages, checkText } from '@/lib/validate'
import usePaged, { PAGE_SIZE } from '@/lib/usePaged'
import Avatar from './Avatar'
import CharCount from './CharCount'
import Icon from './Icon'
import LoadMore from './LoadMore'
import Modal from './Modal'

const privacyNames = { public: 'Public', almost_private: 'Followers', private: 'Chosen followers' }

// One post in a list. myId is the logged-in user's id; isGroupCreator marks
// the viewer as the group's creator (its admin), who may delete any post in
// the group. onDeleted(postId) runs after a successful delete — the group
// page uses it to drop the post from state, other pages refresh their list.
// Comments load from GET /posts/{id}/comments when expanded — the
// API only answers for viewers who may see the post (group posts included).
export default function PostCard({ post, myId, currentGroupId, isGroupCreator = false, onDeleted }) {
  // likes/dislikes change when you react, so we keep them in state
  const [likes, setLikes] = useState(post.likes)
  const [dislikes, setDislikes] = useState(post.dislikes)
  const [myReaction, setMyReaction] = useState(post.my_reaction)
  const [commentCount, setCommentCount] = useState(post.comment_count ?? 0)
  // content/privacy are editable, so the card shows its own copy
  const [content, setContent] = useState(post.content)
  const [privacy, setPrivacy] = useState(post.privacy)
  const [images, setImages] = useState(post.images ?? [])
  const [editing, setEditing] = useState(false)
  const [editContent, setEditContent] = useState(post.content)
  const [editPrivacy, setEditPrivacy] = useState(post.privacy)
  const [editImages, setEditImages] = useState(post.images ?? [])
  const [editError, setEditError] = useState('')
  // the comment being rewritten, and the text while it is being rewritten
  const [editingComment, setEditingComment] = useState(null)
  const [commentDraft, setCommentDraft] = useState('')
  // who may read this post, while the author is editing a "private" one
  const [editViewers, setEditViewers] = useState([])
  const editFollowers = usePaged(editing && editPrivacy === 'private' && !post.group_id ? `/users/${myId}/followers` : null)
  const [saving, setSaving] = useState(false)
  const [open, setOpen] = useState(false)
  const [comments, setComments] = useState(null) // null = not loaded yet, oldest first
  const [hasOlder, setHasOlder] = useState(false) // more comments before the first shown
  const [loadingOlder, setLoadingOlder] = useState(false)
  const [draft, setDraft] = useState('')
  const [files, setFiles] = useState([])
  const [error, setError] = useState('')
  const [sending, setSending] = useState(false)
  // Delete flow: confirming = the dialog is open, deleting = the request is
  // in flight (the button stays disabled so the request cannot be doubled).
  const [confirming, setConfirming] = useState(false)
  const [deleting, setDeleting] = useState(false)
  const [deleteError, setDeleteError] = useState('')

  // The author can always delete their own post; the group's creator (its
  // admin) can delete any post in the group. The backend enforces the same
  // rule — this only decides whether the button is shown.
  const canDelete = post.author_id === myId || isGroupCreator

  const author = {
    first_name: post.author_first_name,
    last_name: post.author_last_name,
    avatar: post.author_avatar,
  }

  const date = new Date(post.created_at).toLocaleDateString(undefined, {
    month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit',
  })

  // Sending the same reaction again removes it. The API answers with the new counts.
  async function react(reaction) {
    const result = await apiPost(`/posts/${post.id}/reactions`, { reaction })
    setLikes(result.likes)
    setDislikes(result.dislikes)
    setMyReaction(result.my_reaction)
  }

  // Reopening the editor always starts from what is on screen now
  function startEdit() {
    setEditContent(content)
    setEditPrivacy(privacy)
    setEditImages(images)
    setEditError('')
    setEditing(true)
    // a private post already has people chosen: tick them
    if (privacy === 'private' && !post.group_id) {
      apiGet(`/posts/${post.id}/viewers`).then(setEditViewers).catch(() => setEditViewers([]))
    } else {
      setEditViewers([])
    }
  }

  function toggleViewer(id) {
    setEditViewers(list => (list.includes(id) ? list.filter(v => v !== id) : [...list, id]))
  }

  function togglePostImage(imageID) {
    setEditImages(current => (
      current.includes(imageID)
        ? current.filter(existingID => existingID !== imageID)
        : [...current, imageID]
    ))
  }

  async function saveEdit(e) {
    e.preventDefault()
    const problem = checkText('Your post', editContent, LIMITS.post)
    if (problem) {
      setEditError(problem)
      return
    }
    // a private post has to reach somebody
    if (!post.group_id && editPrivacy === 'private' && editViewers.length === 0) {
      setEditError('Choose at least one follower.')
      return
    }
    setEditError('')
    setSaving(true)
    try {
      const imagesChanged = images.length !== editImages.length || images.some(imageID => !editImages.includes(imageID))
      // The API requires a valid privacy on every update. A group post keeps
      // the one it was stored with — the group alone decides who can see it.
      const path = post.group_id
        ? `/groups/${post.group_id}/posts/${post.id}`
        : `/posts/${post.id}`
      const update = {
        content: editContent.trim(),
        privacy: post.group_id ? privacy : editPrivacy,
        viewers: !post.group_id && editPrivacy === 'private' ? editViewers : [],
      }
      if (imagesChanged) update.attachments = editImages.length > 0 ? editImages : ['']
      const updated = await apiPut(path, update)
      setContent(updated.content)
      setPrivacy(updated.privacy)
      setImages(updated.images ?? [])
      setEditing(false)
    } catch (err) {
      setEditError(err.message)
    }
    setSaving(false)
  }

  // A comment can be rewritten by whoever wrote it, and removed by its author
  // or by the author of the post. The API checks the same thing.
  async function saveComment(id) {
    const problem = checkText('Comment', commentDraft, LIMITS.comment)
    if (problem) {
      setError(problem)
      return
    }
    setError('')
    try {
      const updated = await apiPut(`/comments/${id}`, { content: commentDraft.trim() })
      setComments(list => list.map(c => (c.id === id ? { ...c, content: updated.content } : c)))
      setEditingComment(null)
    } catch (err) {
      setError(err.message)
    }
  }

  async function removeComment(id) {
    if (!confirm('Delete this comment?')) return
    setError('')
    try {
      await apiDelete(`/comments/${id}`)
      setComments(list => list.filter(c => c.id !== id))
      setCommentCount(count => Math.max(0, count - 1))
    } catch (err) {
      setError(err.message)
    }
  }

  // Opens the confirmation dialog; the actual delete happens in confirmDelete
  // once the user confirms there.
  function remove() {
    setDeleteError('')
    setConfirming(true)
  }

  async function confirmDelete() {
    if (deleting) return // one request at a time
    setDeleting(true)
    setDeleteError('')
    try {
      // Group posts go through the group-scoped route (the only one that
      // lets a group creator delete other people's posts); normal posts
      // keep the author-only /posts/{id} route.
      const path = post.group_id
        ? `/groups/${post.group_id}/posts/${post.id}`
        : `/posts/${post.id}`
      await apiDelete(path)
      setConfirming(false)
      onDeleted?.(post.id)
    } catch (err) {
      setDeleteError(err.message)
    }
    setDeleting(false)
  }

  // The API answers newest first, 10 at a time; the list shows oldest first.
  const commentsPath = `/posts/${post.id}/comments`

  // First open loads the 10 newest comments; afterwards they are kept in state.
  function toggle() {
    const next = !open
    setOpen(next)
    if (next && comments === null) {
      apiGet(commentsPath)
        .then(page => {
          setComments([...page].reverse())
          setHasOlder(page.length === PAGE_SIZE)
        })
        .catch(err => {
          setError(err.message)
          setComments([])
        })
    }
  }

  // the 10 before the oldest one shown, added on top
  async function loadOlder() {
    if (!comments?.length) return
    setLoadingOlder(true)
    try {
      const page = await apiGet(`${commentsPath}?last=${comments[0].id}`)
      setComments(list => [...[...page].reverse(), ...list])
      setHasOlder(page.length === PAGE_SIZE)
    } catch (err) {
      setError(err.message)
    }
    setLoadingOlder(false)
  }

  async function submitComment(e) {
    e.preventDefault()
    const problem = checkText('Comment', draft, LIMITS.comment, { required: files.length === 0 }) || checkImages(files)
    if (problem) {
      setError(problem)
      return
    }
    setError('')
    setSending(true)
    try {
      const formData = new FormData()
      formData.append('content', draft.trim())
      for (const file of files) formData.append('files', file)
      const comment = await apiUpload(`/posts/${post.id}/comments`, formData)
      setComments(list => [...(list || []), comment])
      setDraft('')
      setFiles([])
      setCommentCount(count => count + 1)
    } catch (err) {
      setError(err.message)
    }
    setSending(false)
  }

  async function pickFiles(e) {
    const picked = Array.from(e.target.files)
    const imageError = await checkImageFiles(picked)
    setError(imageError)
    setFiles(imageError ? [] : picked)
    if (imageError) e.target.value = ''
  }

  return (
    <article className="card post">
      <header className="post-header">
        <Avatar user={author} size={42} />
        <div className="post-who">
          <Link href={`/profile/${post.author_id}`} className="post-author">
            {author.first_name} {author.last_name}
          </Link>
          <span className="meta">
            {post.group_id && post.group_name && String(post.group_id) !== String(currentGroupId) && (
              <>
                <Link href={`/groups/${post.group_id}`}>{post.group_name}</Link> ·{' '}
              </>
            )}
            {date}{!post.group_id && <> · {privacyNames[privacy]}</>}
          </span>
        </div>
        {canDelete && !editing && (
          <>
            {post.author_id === myId && (
              <button className="icon-button" onClick={startEdit} title="Edit post">
                <Icon name="edit" size={16} />
              </button>
            )}
            <button className="icon-button" onClick={remove} title="Delete post">
              <Icon name="trash" size={16} />
            </button>
          </>
        )}
      </header>

      {editing ? (
        <form className="post-edit" onSubmit={saveEdit} noValidate>
          <textarea
            value={editContent}
            maxLength={LIMITS.post}
            onChange={e => setEditContent(e.target.value)}
            autoFocus
          />
          <div className="post-edit-bar">
            {!post.group_id && (
              <select className="tool" value={editPrivacy} onChange={e => setEditPrivacy(e.target.value)}>
                <option value="public">Public</option>
                <option value="almost_private">Followers</option>
                <option value="private">Chosen followers</option>
              </select>
            )}
            <CharCount value={editContent} max={LIMITS.post} />
            <button type="button" className="btn btn-sm btn-light" onClick={() => setEditing(false)}>
              Cancel
            </button>
            <button className="btn btn-sm" disabled={saving || !editContent.trim()}>
              {saving ? 'Saving…' : 'Save'}
            </button>
          </div>
          {!post.group_id && editPrivacy === 'private' && editFollowers.items !== null && (
            <div className="viewer-picker">
              <p className="hint">Who can see this post?</p>
              {editFollowers.items.length === 0 && <p className="hint">You have no followers yet.</p>}
              {editFollowers.items.map(person => (
                <label key={person.id} className={editViewers.includes(person.id) ? 'viewer-chip active' : 'viewer-chip'}>
                  <input
                    type="checkbox"
                    checked={editViewers.includes(person.id)}
                    onChange={() => toggleViewer(person.id)}
                  />
                  {person.first_name} {person.last_name}
                </label>
              ))}
              <LoadMore list={editFollowers} />
            </div>
          )}

          {editError && <p className="error">{editError}</p>}
        </form>
      ) : (
        <p className="post-content">{content}</p>
      )}

      {images.length > 0 && (
        <div className={editing ? 'post-images post-images-editing' : 'post-images'}>
          {images.map(imageID => {
            const keeping = editImages.includes(imageID)
            return (
              <div key={imageID} className={keeping ? 'post-image' : 'post-image post-image-removed'}>
                <img src={imageUrl(imageID)} alt="" />
                {editing && (
                  <button
                    type="button"
                    className="btn btn-sm btn-light post-image-toggle"
                    aria-label={`${keeping ? 'Remove' : 'Keep'} image`}
                    onClick={() => togglePostImage(imageID)}
                  >
                    {keeping ? 'Remove' : 'Keep'}
                  </button>
                )}
              </div>
            )
          })}
        </div>
      )}

      <footer className="post-actions">
        <button className={myReaction === 'like' ? 'reaction active' : 'reaction'} onClick={() => react('like')}>
          <Icon name="like" size={16} /> {likes}
        </button>
        <button className={myReaction === 'dislike' ? 'reaction active' : 'reaction'} onClick={() => react('dislike')}>
          <Icon name="dislike" size={16} /> {dislikes}
        </button>
        <button className={open ? 'reaction active' : 'reaction'} onClick={toggle}>
          <Icon name="chat" size={16} /> {commentCount}
        </button>
      </footer>

      {open && (
        <div className="comments">
          {comments === null && <p className="meta">Loading comments…</p>}

          {comments !== null && comments.length === 0 && (
            <p className="meta comments-empty">No comments yet.</p>
          )}

          {hasOlder && (
            <button type="button" className="btn btn-light btn-sm load-more" onClick={loadOlder} disabled={loadingOlder}>
              {loadingOlder ? 'Loading…' : 'Show older comments'}
            </button>
          )}

          {(comments || []).map(comment => (
            <div key={comment.id} className="comment">
              <Avatar user={{
                first_name: comment.author_first_name,
                last_name: comment.author_last_name,
                avatar: comment.author_avatar,
              }} size={30} />
              <div className="comment-body">
                <div className="comment-head">
                  <Link href={`/profile/${comment.author_id}`} className="comment-author">
                    {comment.author_first_name} {comment.author_last_name}
                  </Link>
                  <span className="meta">
                    {new Date(comment.created_at).toLocaleDateString(undefined, {
                      month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit',
                    })}
                  </span>
                  {editingComment !== comment.id && (comment.author_id === myId || post.author_id === myId) && (
                    <div className="comment-actions">
                      {comment.author_id === myId && (
                        <button
                          type="button"
                          className="icon-button"
                          onClick={() => { setEditingComment(comment.id); setCommentDraft(comment.content) }}
                          title="Edit comment"
                        >
                          <Icon name="edit" size={16} />
                        </button>
                      )}
                      <button type="button" className="icon-button" onClick={() => removeComment(comment.id)} title="Delete comment">
                        <Icon name="trash" size={16} />
                      </button>
                    </div>
                  )}
                </div>
                {editingComment === comment.id ? (
                  <form
                    className="comment-edit"
                    onSubmit={e => { e.preventDefault(); saveComment(comment.id) }}
                    noValidate
                  >
                    <textarea
                      value={commentDraft}
                      maxLength={LIMITS.comment}
                      onChange={e => setCommentDraft(e.target.value)}
                      autoFocus
                    />
                    <div className="comment-bar">
                      <CharCount value={commentDraft} max={LIMITS.comment} />
                      <button type="button" className="btn btn-sm btn-light" onClick={() => setEditingComment(null)}>
                        Cancel
                      </button>
                      <button className="btn btn-sm" disabled={!commentDraft.trim()}>Save</button>
                    </div>
                  </form>
                ) : (
                  <p className="comment-content">{comment.content}</p>
                )}
                {comment.images?.length > 0 && (
                  <div className="comment-images">
                    {comment.images.map(id => <img key={id} src={imageUrl(id)} alt="" />)}
                  </div>
                )}
              </div>
            </div>
          ))}

          <form className="comment-form" onSubmit={submitComment} noValidate>
            <textarea
              placeholder="Write a comment…"
              value={draft}
              maxLength={LIMITS.comment}
              onChange={e => setDraft(e.target.value)}
            />
            <div className="comment-bar">
              <label className="tool">
                <Icon name="image" size={14} />
                {files.length > 0 ? `${files.length}` : 'Photo'}
                <input type="file" accept={IMAGE_ACCEPT} hidden onChange={pickFiles} />
              </label>
              {files.length > 0 && (
                <button type="button" className="tool" onClick={() => setFiles([])}>Remove</button>
              )}
              <CharCount value={draft} max={LIMITS.comment} />
              <button className="btn btn-sm" disabled={sending || (!draft.trim() && files.length === 0)}>
                {sending ? '…' : 'Reply'}
              </button>
            </div>
          </form>

          {error && <p className="error">{error}</p>}
        </div>
      )}

      {confirming && (
        <Modal title="Delete post" onClose={() => setConfirming(false)}>
          <p>
            Delete this post by {author.first_name} {author.last_name}?
            {content.length > 0 &&
              (content.length > 120 ? ` “${content.slice(0, 120)}…”` : ` “${content}”`)}
          </p>
          <p className="meta">This cannot be undone. Comments on it are removed too.</p>
          {deleteError && <p className="error">{deleteError}</p>}
          <div className="composer-bar">
            <button
              type="button"
              className="btn btn-light"
              onClick={() => setConfirming(false)}
              disabled={deleting}
            >
              Cancel
            </button>
            <button type="button" className="btn btn-danger" onClick={confirmDelete} disabled={deleting}>
              {deleting ? 'Deleting…' : 'Delete post'}
            </button>
          </div>
        </Modal>
      )}
    </article>
  )
}
