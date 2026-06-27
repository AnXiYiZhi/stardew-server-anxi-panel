import type { ComposePsResponse, DockerStatusResponse } from '../../types'
import { StatusPill } from '../../core/StatusPill'
import { CommandOutput } from '../../core/CommandOutput'

export function DockerSection({
  status,
  composePs,
  checkedAt,
  message,
  busy,
  onCheckDocker,
  onLoadComposePs,
}: {
  status: DockerStatusResponse | null
  composePs: ComposePsResponse | null
  checkedAt: string
  message: string
  busy: boolean
  onCheckDocker: () => void
  onLoadComposePs: () => void
}) {
  return (
    <section className="docker-section">
      <div className="section-heading">
        <div>
          <h2>Docker 状态</h2>
          <p>本区域只做本机联调检查，不会拉取镜像或启动 Junimo 容器。</p>
        </div>
        <div className="docker-actions">
          <button className="button button-small" disabled={busy} onClick={onCheckDocker} type="button">
            {busy ? '正在检查……' : '检查 Docker'}
          </button>
          <button className="button button-small button-secondary" disabled={busy} onClick={onLoadComposePs} type="button">
            {busy ? '正在读取……' : '查看 Compose PS'}
          </button>
        </div>
      </div>
      {message ? <div className="error-banner docker-error">{message}</div> : null}
      <div className="docker-status-grid">
        <StatusPill label="Docker" ok={status?.docker.available} emptyLabel="未检查" />
        <StatusPill label="Compose" ok={status?.compose.available} emptyLabel="未检查" />
        <StatusPill label="Compose 目录" ok={status?.composeProject.ready} emptyLabel="未检查" />
        <div className="docker-status-pill">
          <span>最近检查</span>
          <strong>{checkedAt || '未检查'}</strong>
        </div>
      </div>
      {status ? (
        <div className="compose-output-grid">
          <CommandOutput title="Docker version" result={status.docker.result} />
          <CommandOutput title="Compose version" result={status.compose.result} />
          <div className="compose-output">
            <h3>Compose 工作目录</h3>
            <p>{status.composeProject.workDir}</p>
            <p>
              目录：{status.composeProject.workDirExists ? '存在' : '不存在'}；Compose 文件：
              {status.composeProject.composeFileExists ? '存在' : '不存在'}。
            </p>
          </div>
        </div>
      ) : null}
      {composePs ? (
        <div className="compose-output">
          <h3>Compose PS</h3>
          <p>工作目录：{composePs.workDir}</p>
          {composePs.services.length > 0 ? (
            <div className="compose-table" role="table" aria-label="Compose 服务列表">
              <div className="compose-row compose-row-head" role="row">
                <span>服务</span><span>容器</span><span>状态</span><span>健康</span>
              </div>
              {composePs.services.map((svc, i) => (
                <div className="compose-row" key={`${svc.name}-${svc.service}-${i}`} role="row">
                  <span>{svc.service || '-'}</span>
                  <span>{svc.name || '-'}</span>
                  <span>{svc.state || svc.status || '-'}</span>
                  <span>{svc.health || '-'}</span>
                </div>
              ))}
            </div>
          ) : (
            <p>当前没有 Compose 服务，或当前 Compose 版本没有返回结构化服务列表。</p>
          )}
          <CommandOutput title="原始输出" result={composePs.result} />
        </div>
      ) : null}
    </section>
  )
}
