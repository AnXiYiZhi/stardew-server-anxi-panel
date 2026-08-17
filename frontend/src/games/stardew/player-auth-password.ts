export function playerAuthPasswordError(password: string, label: string): string | null {
  const length = [...password].length
  if (length === 0 || length > 128) return `${label}必须为 1 到 128 个字符`
  if (/\p{Cc}/u.test(password)) return `${label}不能包含控制字符`
  if (password.startsWith(' ') || password.endsWith(' ') || password.includes('  ')) {
    return `${label}的空格不能位于首尾或连续出现`
  }
  return null
}
