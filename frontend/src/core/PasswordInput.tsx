import { useState } from 'react'

export function PasswordInput({
  value,
  visible,
  placeholder,
  autoComplete,
  inputName,
  onChange,
  onToggle,
}: {
  value: string
  visible: boolean
  placeholder?: string
  autoComplete: string
  inputName?: string
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
      <button className="password-toggle" type="button"
        aria-label={visible ? '隐藏密码' : '显示密码'}
        onClick={onToggle}>{visible ? '隐藏' : '显示'}</button>
    </div>
  )
}

// Re-export a convenience hook for password visibility toggling
export function usePasswordToggle(initial = false) {
  const [visible, setVisible] = useState(initial)
  return [visible, () => setVisible((v) => !v)] as const
}
