// Every input rule of the app, so all forms check the same thing.
// The Go API checks again; this only answers the user right away.
// Each check returns an error message, or '' when the value is fine.

export const LIMITS = {
  firstName: 50,
  lastName: 50,
  email: 254,
  nickname: { min: 4, max: 15 },
  password: { min: 8, max: 72 }, // bcrypt ignores anything past 72
  aboutMe: 500,
  post: 1000,
  comment: 2000,
  message: 1000,
  search: 50,
  groupTitle: 100,
  groupDescription: 1000,
  minAge: 13,
  images: 3,
  imageBytes: 10 * 1024 * 1024,
  imageSide: 8000,
  imageTypes: ['image/jpeg', 'image/png', 'image/gif'],
}

export const IMAGE_ACCEPT = LIMITS.imageTypes.join(',')

// same rules as internal/service/authsvc/service.go
const emailRegex = /^[a-z0-9._%+-]+@[a-z0-9.-]+\.[a-z]{2,}$/
const nicknameRegex = /^[a-z0-9]{4,15}$/

export function checkText(label, value, max, { required = true } = {}) {
  const text = (value || '').trim()
  if (required && !text) return `${label} is required.`
  if (text.length > max) return `${label} must be ${max} characters or less.`
  return ''
}

export function checkEmail(value) {
  const email = (value || '').trim().toLowerCase()
  if (!email) return 'Email is required.'
  if (email.length > LIMITS.email) return `Email must be ${LIMITS.email} characters or less.`
  if (!emailRegex.test(email)) return 'Enter a valid email address.'
  return ''
}

export function checkNickname(value) {
  const nickname = (value || '').trim().toLowerCase()
  if (!nickname) return '' // optional: the server makes one from the name
  if (!nicknameRegex.test(nickname) || !/[a-z]/.test(nickname)) {
    const { min, max } = LIMITS.nickname
    return `Nickname must be ${min}–${max} letters or numbers, with at least one letter.`
  }
  return ''
}

export function checkPassword(password = '') {
  const { min, max } = LIMITS.password
  if (!password) return 'Password is required.'
  if (password.length < min) return `Password must be at least ${min} characters.`
  if (password.length > max) return `Password must be ${max} characters or less.`
  return ''
}

export function checkDateOfBirth(value) {
  if (!value) return 'Date of birth is required.'
  const birth = new Date(`${value}T00:00:00`)
  if (Number.isNaN(birth.getTime())) return 'Enter a valid date of birth.'
  const today = new Date()
  if (birth > today) return 'Date of birth cannot be in the future.'

  let age = today.getFullYear() - birth.getFullYear()
  const beforeBirthday = today.getMonth() < birth.getMonth() ||
    (today.getMonth() === birth.getMonth() && today.getDate() < birth.getDate())
  if (beforeBirthday) age--

  if (age < LIMITS.minAge) return `You must be at least ${LIMITS.minAge} years old.`
  if (age > 120) return 'Enter a valid date of birth.'
  return ''
}

// for the max="" of the date of birth input
export function maxBirthDate() {
  const date = new Date()
  date.setFullYear(date.getFullYear() - LIMITS.minAge)
  return date.toISOString().slice(0, 10)
}

// Format, bytes, and pixels: opening the picture also catches a fake image.
export async function checkImageFile(file) {
  if (!LIMITS.imageTypes.includes(file.type) || file.size === 0) {
    return `"${file.name}" is not a JPEG, PNG or GIF image.`
  }
  if (file.size > LIMITS.imageBytes) {
    return `"${file.name}" is larger than ${LIMITS.imageBytes / (1024 * 1024)} MB.`
  }
  const url = URL.createObjectURL(file)
  try {
    const image = new Image()
    image.src = url
    await image.decode()
    if (image.naturalWidth > LIMITS.imageSide || image.naturalHeight > LIMITS.imageSide) {
      return `"${file.name}" is larger than ${LIMITS.imageSide}×${LIMITS.imageSide} pixels.`
    }
  } catch {
    return `"${file.name}" is not a valid image.`
  } finally {
    URL.revokeObjectURL(url)
  }
  return ''
}

export async function checkImageFiles(files) {
  if (files.length > LIMITS.images) return `You can add up to ${LIMITS.images} images per post.`
  for (const file of files) {
    const error = await checkImageFile(file)
    if (error) return error
  }
  return ''
}

// onChange of a multiple file input: { files, error }. A bad pick clears the input.
export async function pickImages(e) {
  const files = Array.from(e.target.files)
  const error = await checkImageFiles(files)
  if (error) e.target.value = ''
  return { files: error ? [] : files, error }
}
