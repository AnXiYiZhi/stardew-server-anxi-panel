export function StatusPill({ label, ok, emptyLabel }: { label: string; ok: boolean | undefined; emptyLabel: string }) {
  const text = ok === undefined ? emptyLabel : ok ? '可用' : '不可用'
  const className = ok === undefined ? 'docker-status-pill' : ok ? 'docker-status-pill ok' : 'docker-status-pill bad'
  return (
    <div className={className}>
      <span>{label}</span>
      <strong>{text}</strong>
    </div>
  )
}
