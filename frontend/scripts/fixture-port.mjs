import { createServer } from 'node:net'

// Vite treats port: 0 as its default port. Let the OS choose a real free port
// first so Windows/Hyper-V exclusions cannot silently select 5173.
export async function fixturePort() {
  const probe = createServer()
  await new Promise((resolve, reject) => {
    probe.once('error', reject)
    probe.listen(0, '127.0.0.1', resolve)
  })
  const address = probe.address()
  await new Promise((resolve, reject) => probe.close(error => error ? reject(error) : resolve()))
  if (!address || typeof address === 'string') throw new Error('Fixture port was not allocated')
  return address.port
}
