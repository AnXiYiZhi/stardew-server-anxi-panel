export function CommandOutput({ title, result }: {
  title: string
  result?: { stdout: string; stderr: string; exitCode: number; durationMs: number; timedOut: boolean }
}) {
  if (!result) return null
  return (
    <div className="compose-output">
      <h3>{title}</h3>
      <p>退出码：{result.exitCode}；耗时：{result.durationMs}ms；超时：{result.timedOut ? '是' : '否'}</p>
      {result.stdout ? <pre>{result.stdout}</pre> : null}
      {result.stderr ? <pre className="stderr-output">{result.stderr}</pre> : null}
    </div>
  )
}
