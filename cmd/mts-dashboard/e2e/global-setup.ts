import { spawn } from 'node:child_process'
import { createWriteStream, mkdirSync, writeFileSync, chmodSync, existsSync, rmSync } from 'node:fs'
import { join } from 'node:path'
import { setTimeout as sleep } from 'node:timers/promises'

const ROOT = join(process.cwd(), '../..')
const STATE_DIR = join(process.cwd(), '.e2e-runtime')
const DATA_DIR = join(STATE_DIR, 'data')
const BACKUP_DIR = join(STATE_DIR, 'backups')
const CONFIG_PATH = join(STATE_DIR, 'mts-server.yaml')
const BIN_PATH = join(STATE_DIR, 'mts-server')
const LOG_PATH = join(STATE_DIR, 'server.log')
const PID_PATH = join(STATE_DIR, 'server.pid')
const BASE_URL = process.env.MTS_E2E_BASE_URL || 'http://127.0.0.1:18086'
const HTTP_ADDR = process.env.MTS_E2E_HTTP_ADDR || '127.0.0.1:18086'
const GRPC_ADDR = process.env.MTS_E2E_GRPC_ADDR || '127.0.0.1:19096'

function writeConfig() {
  // 每次冒烟使用干净 data_dir，避免上次强制改密残留导致 admin/admin 失败
  try {
    rmSync(STATE_DIR, { recursive: true, force: true })
  } catch {
    // ignore
  }
  mkdirSync(STATE_DIR, { recursive: true, mode: 0o700 })
  mkdirSync(DATA_DIR, { recursive: true, mode: 0o700 })
  mkdirSync(BACKUP_DIR, { recursive: true, mode: 0o700 })
  const yaml = `data_dir: ${JSON.stringify(DATA_DIR)}
http:
  enabled: true
  addr: ${JSON.stringify(HTTP_ADDR)}
  dashboard_base: /
  read_timeout: 30s
  read_header_timeout: 5s
  write_timeout: 30s
  idle_timeout: 2m0s
grpc:
  enabled: true
  addr: ${JSON.stringify(GRPC_ADDR)}
  max_recv_msg_bytes: 16777216
  max_send_msg_bytes: 16777216
auth:
  admin_token: ""
  data_tokens: []
  require_user: false
user:
  endpoint: local
  password_auth_disabled: false
limits:
  max_request_body_bytes: 16777216
  max_write_points: 10000
  default_query_limit: 10000
  max_query_limit: 100000
  request_timeout: 30s
  max_concurrent_http: 1024
  max_concurrent_grpc: 1024
observability:
  access_log: false
  pprof:
    enabled: false
backup:
  dir: ${JSON.stringify(BACKUP_DIR)}
log:
  level: info
  format: text
engine:
  default_database: default
  default_retention_policy: autogen
  shard_duration: 1h0m0s
  retention: 0s
  memtable_max_samples: 10000
  flush_sync: false
  compaction:
    enabled: true
    level0_part_limit: 4
    max_cascade_steps: 8
`
  writeFileSync(CONFIG_PATH, yaml, { mode: 0o600 })
}

async function buildServer() {
  await new Promise<void>((resolve, reject) => {
    const child = spawn('go', ['build', '-o', BIN_PATH, './cmd/mts-server'], {
      cwd: ROOT,
      stdio: ['ignore', 'pipe', 'pipe'],
      env: process.env,
    })
    let err = ''
    child.stderr.on('data', (d) => { err += String(d) })
    child.on('exit', (code) => {
      if (code === 0) resolve()
      else reject(new Error(`go build failed: ${code}\n${err}`))
    })
  })
  chmodSync(BIN_PATH, 0o700)
}

async function waitReady(url: string, timeoutMs: number) {
  const start = Date.now()
  while (Date.now() - start < timeoutMs) {
    try {
      const resp = await fetch(`${url}/healthz`)
      if (resp.ok) return
    } catch {
      // retry
    }
    await sleep(250)
  }
  throw new Error(`server not ready: ${url}`)
}

export default async function globalSetup() {
  writeConfig()
  if (!existsSync(join(ROOT, 'cmd/mts-server/dashboard-dist/index.html'))) {
    throw new Error('dashboard-dist missing; run npm run build first')
  }
  await buildServer()

  const log = createWriteStream(LOG_PATH, { flags: 'w', mode: 0o600 })
  const child = spawn(BIN_PATH, ['serve', '--config', CONFIG_PATH], {
    cwd: ROOT,
    stdio: ['ignore', 'pipe', 'pipe'],
    env: process.env,
    detached: true,
  })
  child.stdout.pipe(log)
  child.stderr.pipe(log)
  child.unref()
  if (!child.pid) {
    throw new Error('failed to start mts-server (no pid)')
  }
  writeFileSync(PID_PATH, String(child.pid), { mode: 0o600 })

  try {
    await waitReady(BASE_URL, 45_000)
  } catch (e) {
    try { process.kill(-child.pid, 'SIGTERM') } catch { /* ignore */ }
    try { process.kill(child.pid, 'SIGTERM') } catch { /* ignore */ }
    throw e
  }
}
