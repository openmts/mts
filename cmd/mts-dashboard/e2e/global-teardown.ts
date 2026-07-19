import { existsSync, readFileSync, rmSync } from 'node:fs'
import { join } from 'node:path'
import { setTimeout as sleep } from 'node:timers/promises'

const STATE_DIR = join(process.cwd(), '.e2e-runtime')
const PID_PATH = join(STATE_DIR, 'server.pid')

export default async function globalTeardown() {
  if (existsSync(PID_PATH)) {
    const pid = Number(readFileSync(PID_PATH, 'utf8').trim())
    if (Number.isFinite(pid) && pid > 0) {
      try { process.kill(-pid, 'SIGTERM') } catch { /* ignore */ }
      try { process.kill(pid, 'SIGTERM') } catch { /* ignore */ }
      await sleep(300)
      try { process.kill(pid, 'SIGKILL') } catch { /* ignore */ }
    }
  }
  try {
    rmSync(STATE_DIR, { recursive: true, force: true })
  } catch {
    // ignore
  }
}
