'use client'

import { useState } from 'react'
import Link from 'next/link'
import { apiDelete, apiGet, apiPost, apiPut, apiUpload, imageUrl } from '@/lib/api'
import { IMAGE_ACCEPT, LIMITS, checkText, pickImages } from '@/lib/validate'
import { PAGE_SIZE } from '@/lib/usePaged'
import Avatar from './Avatar'
import CharCount from './CharCount'
import Icon from './Icon'
import Modal from './Modal'
import { PrivacySelect, ViewerPicker, privacyNames } from './Privacy'

const formatDate = date => new Date(date).toLocaleDateString(undefined, { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })

// One post with its reactions and comments. The author may edit it; the author
// or the group's creator may delete it. onDeleted(postId) runs after a delete.
export default function PostCard({ post, myId, currentGroupId, isGroupCreator = false, onDeleted }) {
  const [reactions, setReactions] = useState({ likes: post.likes, dislikes: post.dislikes, my_reaction: post.my_reaction })
  const [commentCount, setCommentCount] = useState(post.comment_count ?? 0)
  const [content, setContent] = useState(post.content)
  const [privacy, setPrivacy] = useState(post.privacy)
  // editing the post
  const [editing, setEditing] = useState(false)
  const [editContent, setEditContent] = useState('')
  const [editPrivacy, setEditPrivacy] = useState('')
  const [editViewers, setEditViewers] = useState([])
  const [editError, setEditError] = useState('')
  const [saving, setSaving] = useState(false)
  // comments
  const [open, setOpen] = useState(false)
  const [comments, setComments] = useState(null) // null = not loaded yet, oldest first
  const [hasOlder, setHasOlder] = useState(false)
  const [loadingOlder, setLoadingOlder] = useState(false)
  const [editingComment, setEditingComment] = useState(null)
  const [commentDraft, setCommentDraft] = useState('')
  const [draft, setDraft] = useState('')
  const [files, setFiles] = useState([])
  const [error, setError] = useState('')
  const [sending, setSending] = useState(false)
  // deleting
  const [confirming, setConfirming] = useState(false)
  const [deleting, setDeleting] = useState(false)
  const [deleteError, setDeleteError] = useState('')

  const isAuthor = post.author_id === myId
  const author = { first_name: post.author_first_name, last_name: post.author_last_name, avatar: post.author_avatar }
  const commentsPath = `/posts/${post.id}/comments` // newest first, 10 at a time

  // sending the same reaction again removes it
  async function react(reaction) {
    setReactions(await apiPost(`/posts/${post.id}/reactions`, { reaction }))
  }

  function startEdit() {
    setEditContent(content)
    setEditPrivacy(privacy)
    setEditError('')
    setEditViewers([])
    setEditing(true)
    if (privacy === 'private' && !post.group_id) {
      apiGet(`/posts/${post.id}/viewers`).then(setEditViewers).catch(() => {})
    }
  }

  async function saveEdit(e) {
    e.preventDefault()
    const chooseViewers = !post.group_id && editPrivacy === 'private'
    const problem = checkText('Your post', editContent, LIMITS.post) ||
      (chooseViewers && editViewers.length === 0 ? 'Choose at least one follower.' : '')
    setEditError(problem)
    if (problem) return
    setSaving(true)
    try {
      // a group post keeps its stored privacy: the group decides who sees it
      const updated = await apiPut(`/posts/${post.id}`, {
        content: editContent.trim(),
        privacy: post.group_id ? privacy : editPrivacy,
        viewers: chooseViewers ? editViewers : [],
      })
      setContent(updated.content)
      setPrivacy(updated.privacy)
      setEditing(false)
    } catch (err) {
      setEditError(err.message)
    }
    setSaving(false)
  }

  async function confirmDelete() {
    setDeleting(true)
    setDeleteError('')
    try {
      // group posts use the group route, the only one that lets the creator delete others' posts
      await apiDelete(post.group_id ? `/groups/${post.group_id}/posts/${post.id}` : `/posts/${post.id}`)
      setConfirming(false)
      onDeleted?.(post.id)
    } catch (err) {
      setDeleteError(err.message)
    }
    setDeleting(false)
  }

  // shown oldest first; `last` asks for the page before that comment
  async function loadComments(last) {
    try {
      const page = await apiGet(last ? `${commentsPath}?last=${last}` : commentsPath)
      setComments(list => [...page.reverse(), ...(last ? list : [])])
      setHasOlder(page.length === PAGE_SIZE)
    } catch (err) {
      setError(err.message)
      setComments(list => list || [])
    }
  }

  function toggle() {
    setOpen(!open)
    if (!open && comments === null) loadComments(0)
  }

  async function loadOlder() {
    setLoadingOlder(true)
    await loadComments(comments[0].id)
    setLoadingOlder(false)
  }

  async function submitComment(e) {
    e.preventDefault()
    const problem = checkText('Comment', draft, LIMITS.comment)
    setError(problem)
    if (problem) return
    setSending(true)
    try {
      let comment = await apiPost(commentsPath, { content: draft.trim() })
      if (files.length > 0) {
        await apiUpload('/files', { files, comment_id: comment.id })
        // the images were attached afterwards: take the comment again, with them
        comment = (await apiGet(commentsPath)).find(c => c.id === comment.id) || comment
      }
      setComments(list => [...(list || []), comment])
      setDraft('')
      setFiles([])
      setCommentCount(count => count + 1)
    } catch (err) {
      setError(err.message)
    }
    setSending(false)
  }

  async function saveComment(id) {
    const problem = checkText('Comment', commentDraft, LIMITS.comment)
    setError(problem)
    if (problem) return
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

  async function pickFiles(e) {
    const picked = await pickImages(e)
    setError(picked.error)
    setFiles(picked.files)
  }

  const reactionButton = (name, count) => (
    <button className={reactions.my_reaction === name ? 'reaction active' : 'reaction'} onClick={() => react(name)}>
      <Icon name={name} size={16} /> {count}
    </button>
  )

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
              <><Link href={`/groups/${post.group_id}`}>{post.group_name}</Link> ·{' '}</>
            )}
            {formatDate(post.created_at)}{!post.group_id && <> · {privacyNames[privacy]}</>}
          </span>
        </div>
        {!editing && isAuthor && (
          <button className="icon-button" onClick={startEdit} title="Edit post"><Icon name="edit" size={16} /></button>
        )}
        {!editing && (isAuthor || isGroupCreator) && (
          <button className="icon-button" onClick={() => { setDeleteError(''); setConfirming(true) }} title="Delete post">
            <Icon name="trash" size={16} />
          </button>
        )}
      </header>

      {editing ? (
        <form className="post-edit" onSubmit={saveEdit} noValidate>
          <textarea value={editContent} maxLength={LIMITS.post} onChange={e => setEditContent(e.target.value)} autoFocus />
          <div className="post-edit-bar">
            {!post.group_id && <PrivacySelect value={editPrivacy} onChange={setEditPrivacy} />}
            <CharCount value={editContent} max={LIMITS.post} />
            <button type="button" className="btn btn-sm btn-light" onClick={() => setEditing(false)}>Cancel</button>
            <button className="btn btn-sm" disabled={saving || !editContent.trim()}>{saving ? 'Saving…' : 'Save'}</button>
          </div>
          {!post.group_id && editPrivacy === 'private' && (
            <ViewerPicker myId={myId} selected={editViewers} onChange={setEditViewers} />
          )}
          {editError && <p className="error">{editError}</p>}
        </form>
      ) : (
        <p className="post-content">{content}</p>
      )}

      {post.images?.length > 0 && (
        <div className="post-images">
          {post.images.map(id => <img key={id} src={imageUrl(id)} alt="" />)}
        </div>
      )}

      <footer className="post-actions">
        {reactionButton('like', reactions.likes)}
        {reactionButton('dislike', reactions.dislikes)}
        <button className={open ? 'reaction active' : 'reaction'} onClick={toggle}>
          <Icon name="chat" size={16} /> {commentCount}
        </button>
      </footer>

      {open && (
        <div className="comments">
          {comments === null && <p className="meta">Loading comments…</p>}
          {comments?.length === 0 && <p className="meta comments-empty">No comments yet.</p>}
          {hasOlder && (
            <button type="button" className="btn btn-light btn-sm load-more" onClick={loadOlder} disabled={loadingOlder}>
              {loadingOlder ? 'Loading…' : 'Show older comments'}
            </button>
          )}

          {comments?.map(comment => (
            <div key={comment.id} className="comment">
              <Avatar user={{ first_name: comment.author_first_name, last_name: comment.author_last_name, avatar: comment.author_avatar }} size={30} />
              <div className="comment-body">
                <div className="comment-head">
                  <Link href={`/profile/${comment.author_id}`} className="comment-author">
                    {comment.author_first_name} {comment.author_last_name}
                  </Link>
                  <span className="meta">{formatDate(comment.created_at)}</span>
                  {/* the comment's author may edit it; they or the post's author may delete it */}
                  {editingComment !== comment.id && (comment.author_id === myId || isAuthor) && (
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
                  <form className="comment-edit" onSubmit={e => { e.preventDefault(); saveComment(comment.id) }} noValidate>
                    <textarea value={commentDraft} maxLength={LIMITS.comment} onChange={e => setCommentDraft(e.target.value)} autoFocus />
                    <div className="comment-bar">
                      <CharCount value={commentDraft} max={LIMITS.comment} />
                      <button type="button" className="btn btn-sm btn-light" onClick={() => setEditingComment(null)}>Cancel</button>
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
            <textarea placeholder="Write a comment…" value={draft} maxLength={LIMITS.comment} onChange={e => setDraft(e.target.value)} />
            <div className="comment-bar">
              <label className="tool">
                <Icon name="image" size={14} />
                {files.length > 0 ? `${files.length}` : 'Photo'}
                <input type="file" accept={IMAGE_ACCEPT} hidden onChange={pickFiles} />
              </label>
              {files.length > 0 && <button type="button" className="tool" onClick={() => setFiles([])}>Remove</button>}
              <CharCount value={draft} max={LIMITS.comment} />
              <button className="btn btn-sm" disabled={sending || !draft.trim()}>{sending ? '…' : 'Reply'}</button>
            </div>
          </form>

          {error && <p className="error">{error}</p>}
        </div>
      )}

      {confirming && (
        <Modal title="Delete post" onClose={() => setConfirming(false)}>
          <p>
            Delete this post by {author.first_name} {author.last_name}?
            {content && ` “${content.length > 120 ? `${content.slice(0, 120)}…` : content}”`}
          </p>
          <p className="meta">This cannot be undone. Comments on it are removed too.</p>
          {deleteError && <p className="error">{deleteError}</p>}
          <div className="composer-bar">
            <button type="button" className="btn btn-light" onClick={() => setConfirming(false)} disabled={deleting}>Cancel</button>
            <button type="button" className="btn btn-danger" onClick={confirmDelete} disabled={deleting}>
              {deleting ? 'Deleting…' : 'Delete post'}
            </button>
          </div>
        </Modal>
      )}
    </article>
  )
}
