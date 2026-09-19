
export function PasswordInput({
  value,
  visible,
  placeholder,
  autoComplete,
  inputName,
  iconOnly = false,
  onChange,
  onToggle,
}: {
  value: string
  visible: boolean
  placeholder?: string
  autoComplete: string
  inputName?: string
  iconOnly?: boolean
  onChange: (v: string) => void
  onToggle: () => void
}) {
  return (
    <div className="password-input">
      <input
        name={inputName}
        type={visible ? 'text' : 'password'}
        value={value}
        placeholder={placeholder}
        autoComplete={autoComplete}
        onChange={(e) => onChange(e.target.value)}
        required
      />
      <button className={`password-toggle${iconOnly ? ' password-toggle--icon' : ''}`} type="button"
        aria-label={visible ? '隐藏密码' : '显示密码'}
        aria-pressed={visible}
        onClick={onToggle}>
        {iconOnly ? (
          <svg viewBox="0 0 24 24" aria-hidden="true">
            <path d="M2 10h2V8h3V6h10v2h3v2h2v4h-2v2h-3v2H7v-2H4v-2H2z" />
            <circle cx="12" cy="12" r="3" />
            {visible ? <path d="m4 3 16 18" /> : null}
          </svg>
        ) : visible ? '隐藏' : '显示'}
      </button>
    </div>
  )
}

// Re-export a convenience hook for password visibility toggling
