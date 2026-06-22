function App() {
  return (
    <main className="shell">
      <section className="panel-card">
        <p className="eyebrow">Milestone 0 · Repo Skeleton</p>
        <h1>Stardew Anxi Panel</h1>
        <p className="summary">
          当前项目只包含前后端基础骨架，后续会逐步接入管理员初始化、Junimo 安装向导、服务器状态和管理入口。
        </p>
        <div className="entry-list" aria-label="后续开发入口">
          <span>管理员初始化</span>
          <span>安装向导</span>
          <span>状态面板</span>
          <span>存档与 Mod 管理</span>
        </div>
      </section>
    </main>
  )
}

export default App
