import { useState } from 'react'
import type { FormEvent } from 'react'
import { Field } from './Field'
import { PasswordInput } from './PasswordInput'
import { useCompactAuthCard } from './AuthCard'

export type SetupFormState = {
  username: string
  password: string
  confirmPassword: string
}

export const emptySetupForm: SetupFormState = { username: '', password: '', confirmPassword: '' }

export function SetupPanel({
  form,
  busy,
  onChange,
  onSubmit,
}: {
  form: SetupFormState
  busy: boolean
  onChange: (f: SetupFormState) => void
  onSubmit: (e: FormEvent<HTMLFormElement>) => void
}) {
  const [showPwd, setShowPwd] = useState(false)
  const [showConfirm, setShowConfirm] = useState(false)
  const compact = useCompactAuthCard()
  return (
    <form className="form-grid" onSubmit={onSubmit} autoComplete="off">
      <p className="summary">当前数据库里还没有管理员。请创建第一个管理员账号，完成后会自动登录。</p>
      <Field label="管理员用户名">
        <input name="username" value={form.username} autoComplete="username" autoCapitalize="none" spellCheck={false} placeholder={compact ? '设置管理员用户名' : ' '} required
          onChange={(e) => onChange({ ...form, username: e.target.value })} />
      </Field>
      <Field label="管理员密码">
        <PasswordInput inputName="password" iconOnly={compact} value={form.password} visible={showPwd} placeholder={compact ? '设置管理员密码' : ' '} autoComplete="new-password"
          onChange={(p) => onChange({ ...form, password: p })} onToggle={() => setShowPwd((v) => !v)} />
      </Field>
      <Field label="确认密码">
        <PasswordInput inputName="confirmPassword" iconOnly={compact} value={form.confirmPassword} visible={showConfirm} placeholder={compact ? '再次输入密码' : ' '} autoComplete="new-password"
          onChange={(p) => onChange({ ...form, confirmPassword: p })} onToggle={() => setShowConfirm((v) => !v)} />
      </Field>
      <p className="form-hint">请尽快注册管理员账号</p>
      <button className="button" disabled={busy} type="submit">{busy ? '正在注册……' : '注册'}</button>
    </form>
  )
}
