'use client'

import { useState } from 'react'
import Link from 'next/link'
import { useRouter } from 'next/navigation'
import { apiPost, apiUpload } from '@/lib/api'
import { setMe } from '@/lib/userStore'
import Icon from '@/components/Icon'
import {
  IMAGE_ACCEPT, LIMITS, checkDateOfBirth, checkEmail, checkImageFile, checkNickname, checkPassword, checkText, maxBirthDate,
} from '@/lib/validate'

export default function RegisterPage() {
  const router = useRouter()
  const [errors, setErrors] = useState({}) // field name → message
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const [avatar, setAvatar] = useState(null)
  const [preview, setPreview] = useState('')

  async function pickAvatar(e) {
    const file = e.target.files[0]
    e.target.value = '' // so the same file can be picked again
    if (!file) return
    const problem = await checkImageFile(file)
    setErrors(rest => ({ ...rest, avatar: problem }))
    if (problem) return
    if (preview) URL.revokeObjectURL(preview)
    setAvatar(file)
    setPreview(URL.createObjectURL(file))
  }

  async function handleSubmit(e) {
    e.preventDefault()
    setError('')
    const form = new FormData(e.target)
    const values = {
      first_name: form.get('first_name').trim(),
      last_name: form.get('last_name').trim(),
      email: form.get('email').trim().toLowerCase(),
      nickname: form.get('nickname').trim().toLowerCase(),
      password: form.get('password'),
      date_of_birth: form.get('date_of_birth'),
      about_me: form.get('about_me').trim(),
    }
    const found = {
      first_name: checkText('First name', values.first_name, LIMITS.firstName),
      last_name: checkText('Last name', values.last_name, LIMITS.lastName),
      email: checkEmail(values.email),
      nickname: checkNickname(values.nickname),
      password: checkPassword(values.password),
      date_of_birth: checkDateOfBirth(values.date_of_birth),
      about_me: checkText('About me', values.about_me, LIMITS.aboutMe, { required: false }),
    }
    setErrors(found)
    if (Object.values(found).some(Boolean)) return

    setLoading(true)
    try {
      // registering logs you in, so the photo goes right after; if it fails it can be set in settings
      let user = await apiPost('/register', values)
      if (avatar) user = await apiUpload('/avatar', { avatar }).catch(() => user)
      setMe(user)
      router.push('/home')
    } catch (err) {
      setError(err.message)
      setLoading(false)
    }
  }

  function clearError(e) {
    const { name } = e.target
    if (errors[name]) setErrors(rest => ({ ...rest, [name]: '' }))
  }

  // the props and message of one input
  const field = name => ({ name, className: errors[name] ? 'invalid' : undefined })
  const fieldError = name => errors[name] && <p className="field-error">{errors[name]}</p>

  return (
    <form className="auth-form" onSubmit={handleSubmit} onInput={clearError} noValidate>
      <h1>Create your account</h1>
      <p className="subtitle">It takes less than a minute.</p>

      <label>Photo <small>optional, JPEG, PNG or GIF</small></label>
      <div className="photo-row">
        {preview
          ? <img className="avatar" style={{ width: 56, height: 56 }} src={preview} alt="" />
          : <span className="avatar" style={{ width: 56, height: 56 }}><Icon name="camera" size={20} /></span>}
        <label className="btn btn-light">
          {avatar ? 'Change photo' : 'Add photo'}
          <input type="file" accept={IMAGE_ACCEPT} hidden onChange={pickAvatar} />
        </label>
      </div>
      {fieldError('avatar')}

      <div className="row">
        <div>
          <label>First name</label>
          <input {...field('first_name')} maxLength={LIMITS.firstName} autoFocus />
          {fieldError('first_name')}
        </div>
        <div>
          <label>Last name</label>
          <input {...field('last_name')} maxLength={LIMITS.lastName} />
          {fieldError('last_name')}
        </div>
      </div>

      <label>Email</label>
      <input {...field('email')} type="email" maxLength={LIMITS.email} />
      {fieldError('email')}

      <div className="row">
        <div>
          <label>Nickname <small>optional</small></label>
          <input {...field('nickname')} placeholder="made from your name if empty" maxLength={LIMITS.nickname.max} />
          {fieldError('nickname')}
        </div>
        <div>
          <label>Date of birth</label>
          <input {...field('date_of_birth')} type="date" max={maxBirthDate()} />
          {fieldError('date_of_birth')}
        </div>
      </div>

      <label>Password <small>at least {LIMITS.password.min} characters</small></label>
      <input {...field('password')} type="password" maxLength={LIMITS.password.max} />
      {fieldError('password')}

      <label>About me <small>optional, up to {LIMITS.aboutMe} characters</small></label>
      <textarea {...field('about_me')} rows={3} maxLength={LIMITS.aboutMe} />
      {fieldError('about_me')}

      {error && <p className="error">{error}</p>}

      <button className="btn btn-full" disabled={loading}>{loading ? 'Creating account…' : 'Create account'}</button>

      <p className="switch">
        Already have an account? <Link href="/login">Log in</Link>
      </p>
    </form>
  )
}
