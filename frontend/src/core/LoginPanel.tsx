import { useState } from 'react'
import type { FormEvent } from 'react'
import { Field } from './Field'
import { PasswordInput } from './PasswordInput'

export type LoginFormState = {
  username: string
  password: string
}

export const emptyLoginForm: LoginFormState = { username: '', password: '' }

export function LoginPanel({
  form,
  busy,
  onChange,
  onSubmit,
}: {
  form: LoginFormState
  busy: boolean
  onChange: (f: LoginFormState) => void
  onSubmit: (e: FormEvent<HTMLFormElement>) => void
}) {
  const [showPwd, setShowPwd] = useState(false)
  return (
    <form className="form-grid" onSubmit={onSubmit} autoComplete="on">
      <p className="summary">请输入面板账号登录。登录状态会通过 HttpOnly Cookie 保存。</p>
      <Field label="用户名">
        <input value={form.username} autoComplete="username" required
          onChange={(e) => onChange({ ...form, username: e.target.value })} />
      </Field>
      <Field label="密码">
        <PasswordInput value={form.password} visible={showPwd} autoComplete="current-password"
          onChange={(p) => onChange({ ...form, password: p })} onToggle={() => setShowPwd((v) => !v)} />
      </Field>
      <button className="button" disabled={busy} type="submit">{busy ? '正在登录……' : '登录'}</button>
    </form>
  )
}
