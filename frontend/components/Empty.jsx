export default function Empty({ title, children }) {
  return (
    <div className="empty">
      <p className="empty-title">{title}</p>
      <p>{children}</p>
    </div>
  )
}
