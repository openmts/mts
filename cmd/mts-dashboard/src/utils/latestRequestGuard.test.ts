import assert from 'node:assert/strict'
import test from 'node:test'

type Deferred<T> = {
  promise: Promise<T>
  resolve: (value: T) => void
  reject: (reason: unknown) => void
}

function deferred<T>(): Deferred<T> {
  let resolve!: (value: T) => void
  let reject!: (reason: unknown) => void
  const promise = new Promise<T>((onResolve, onReject) => {
    resolve = onResolve
    reject = onReject
  })
  return { promise, resolve, reject }
}

test('旧请求晚于新请求完成时不能提交数据、错误或 loading', async () => {
  let module: typeof import('./latestRequestGuard.ts')
  try {
    module = await import('./latestRequestGuard.ts')
  } catch {
    assert.fail('最新请求提交门卫尚未实现')
  }

  const guard = module.createLatestRequestGuard()
  const requestA = deferred<string>()
  const requestB = deferred<string>()
  const state = { data: '', error: '', loading: false }

  async function load(request: Promise<string>) {
    const token = guard.begin()
    state.loading = true
    try {
      const value = await request
      token.commit(() => { state.data = value })
    } catch (error) {
      token.commit(() => { state.error = String(error) })
    } finally {
      token.commit(() => { state.loading = false })
    }
  }

  const stale = load(requestA.promise)
  const current = load(requestB.promise)
  requestB.resolve('database-b')
  await current
  requestA.reject(new Error('database-a failed'))
  await stale

  assert.deepEqual(state, { data: 'database-b', error: '', loading: false })
})

test('清空当前目标时立即结束 loading 并使旧请求失效', async () => {
  const module = await import('./latestRequestGuard.ts')
  const guard = module.createLatestRequestGuard()
  const staleRequest = deferred<string>()
  let loading = false
  let data = ''

  const staleToken = module.beginLatestLoad(guard, true, (value) => { loading = value })
  assert.ok(staleToken)
  const stale = staleRequest.promise.then((value) => {
    staleToken.commit(() => { data = value })
  }).finally(() => {
    staleToken.commit(() => { loading = false })
  })

  const emptyToken = module.beginLatestLoad(guard, false, (value) => { loading = value })
  assert.equal(emptyToken, null)
  assert.equal(loading, false)

  staleRequest.resolve('stale')
  await stale
  assert.equal(data, '')
  assert.equal(loading, false)
})
