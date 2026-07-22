import { expect, test, type Page } from '@playwright/test'

const NEW_PASSWORD = 'AdminChanged!2026'

async function login(page: Page, user: string, password: string) {
  await page.goto('/login')
  await expect(page.getByTestId('login-panel')).toBeVisible()
  await page.getByTestId('login-username').fill(user)
  await page.getByTestId('login-password').fill(password)
  await page.getByTestId('login-submit').click()
}

test.describe.configure({ mode: 'serial' })


async function expectFailAdminLast(
  page: Page,
  pageTestId: string,
  lastTestId: string,
) {
  await expect(page.getByTestId(pageTestId)).toBeVisible()
  await expect(page.getByTestId(lastTestId)).toHaveAttribute('data-ok', 'false')
  await expect(page.getByTestId(`${lastTestId}-error`)).toContainText(/e2e disk full/i)
  await expect(page.getByTestId(`${lastTestId}-copy`)).toBeVisible()
}

test('commercial browser smoke path', async ({ page }) => {
  test.setTimeout(210_000)
  // 脏表单离开确认：冒烟路径自动接受，避免深链被 confirm 卡住
  page.on('dialog', async (dialog) => {
    await dialog.accept()
  })
  // 0) 登录表单校验：空密码 -> alert 错误区（本地校验不可重试，仅 dismiss）
  await page.goto('/login')
  await expect(page.getByTestId('login-toggle-password')).toBeVisible()
  await expect(page.getByTestId('login-remember-user')).toBeVisible()
  await page.getByTestId('login-username').fill('admin')
  await page.getByTestId('login-password').fill('')
  await page.getByTestId('login-submit').click()
  await expect(page.getByTestId('login-error')).toBeVisible()
  await expect(page.getByTestId('login-error')).toHaveAttribute('role', 'alert')
  await expect(page.getByTestId('login-error-retry')).toHaveCount(0)
  await expect(page.getByTestId('login-error-dismiss')).toBeVisible()
  await expect(page.getByTestId('login-password')).toHaveAttribute('aria-invalid', 'true')
  await page.getByTestId('login-error-dismiss').click()
  await expect(page.getByTestId('login-error')).toHaveCount(0)
  await expect(page.getByTestId('login-ttl')).toBeVisible()

  // P279/P424: 错误密码 -> 服务端失败可 retry/dismiss，文案对齐密码不正确
  await page.getByTestId('login-password').fill('definitely-wrong-password')
  await page.getByTestId('login-submit').click()
  await expect(page.getByTestId('login-error')).toBeVisible()
  await expect(page.getByTestId('login-error')).toContainText(/密码不正确|Incorrect password|用户名或密码/i)
  await expect(page.getByTestId('login-error-retry')).toBeVisible()
  await expect(page.getByTestId('login-error-dismiss')).toBeVisible()
  await page.getByTestId('login-error-dismiss').click()
  await expect(page.getByTestId('login-error')).toHaveCount(0)

  // P187: 登录页展示深链 redirect 目标
  await page.goto('/login?redirect=%2Fquery%3Fdatabase%3Ddefault')
  await expect(page.getByTestId('login-redirect-hint')).toBeVisible()
  await expect(page.getByTestId('login-redirect-path')).toContainText('/query')

  // 1) bootstrap 默认密码 -> 强制改密
  await login(page, 'admin', 'admin')
  await expect(page).toHaveURL(/force-change-password/)
  await expect(page.getByTestId('password-hints')).toBeVisible()
  // P280: 强改密密码可见性切换
  await expect(page.getByTestId('force-toggle-old')).toBeVisible()
  await expect(page.getByTestId('force-toggle-new')).toBeVisible()
  await expect(page.getByTestId('force-toggle-confirm')).toBeVisible()
  await page.getByTestId('force-toggle-new').click()
  await expect(page.getByTestId('force-new')).toHaveAttribute('type', 'text')
  await page.getByTestId('force-toggle-new').click()
  await expect(page.getByTestId('force-new')).toHaveAttribute('type', 'password')
  // P279: 本地策略失败仅 dismiss、不可 retry
  await page.getByTestId('force-old').fill('admin')
  await page.getByTestId('force-new').fill('short')
  await page.getByTestId('force-confirm').fill('short')
  await page.getByTestId('force-password-submit').click()
  await expect(page.getByTestId('force-password-error')).toBeVisible()
  await expect(page.getByTestId('force-password-error')).toHaveAttribute('role', 'alert')
  await expect(page.getByTestId('force-password-error-retry')).toHaveCount(0)
  await expect(page.getByTestId('force-password-error-dismiss')).toBeVisible()
  await page.getByTestId('force-password-error-dismiss').click()
  await expect(page.getByTestId('force-password-error')).toHaveCount(0)
  // P421: 强制改密旧密码错误 — 保持强制改密页，不清会话
  await page.getByTestId('force-old').fill('definitely-wrong-old')
  await page.getByTestId('force-new').fill(NEW_PASSWORD)
  await page.getByTestId('force-confirm').fill(NEW_PASSWORD)
  await page.getByTestId('force-password-submit').click()
  await expect(page.getByTestId('force-password-error')).toBeVisible({ timeout: 10_000 })
  await expect(page).toHaveURL(/force-change-password/)
  await expect(page.getByTestId('force-password-panel')).toBeVisible()

  await page.getByTestId('force-old').fill('admin')
  await page.getByTestId('force-new').fill(NEW_PASSWORD)
  await page.getByTestId('force-confirm').fill(NEW_PASSWORD)
  await page.getByTestId('force-password-submit').click()
  await expect(page).toHaveURL(/login/)
  await expect(page.getByText(/密码已更新|new password/i)).toBeVisible()
  await expect(page.getByTestId('login-reason')).toBeVisible()
  await expect(page.getByTestId('login-username')).toHaveValue(/admin/)

  // 2) 新密码登录
  await login(page, 'admin', NEW_PASSWORD)
  await expect(page).not.toHaveURL(/login|force-change/)
  await expect(page.getByText(/概览|健康|Healthy|Ready/i).first()).toBeVisible()
  await expect(page.getByTestId('overview-page')).toBeVisible()
  if (await page.getByTestId('overview-health-virtual-list').count()) {
    await expect(page.getByTestId('overview-health-virtual-list')).toBeVisible()
  }
  if (await page.getByTestId('overview-doctor-virtual-list').count()) {
    await expect(page.getByTestId('overview-doctor-virtual-list')).toBeVisible()
  }
  await expect(page.getByTestId('overview-summary')).toBeVisible()
  await expect(page.getByTestId('overview-load-error')).toHaveCount(0)
  await expect(page.getByTestId('overview-refresh-error')).toHaveCount(0)
  await expect(page.getByTestId('overview-partial-error')).toHaveCount(0)
  await expect(page.getByTestId('overview-partial-sections')).toHaveCount(0)
  await expect(page.getByTestId('overview-auto-refresh')).toBeVisible()
  await expect(page.getByTestId('overview-refresh')).toBeVisible()
  // P187: 登出后访问受保护深链应带 redirect 提示，登录后回到原路径
  await page.getByTestId('topbar-logout').click()
  await expect(page).toHaveURL(/login/)
  await page.goto('/query?database=default')
  await expect(page).toHaveURL(/login/)
  await expect(page.getByTestId('login-redirect-hint')).toBeVisible()
  await expect(page.getByTestId('login-redirect-path')).toContainText('/query')
  await page.getByTestId('login-username').fill('admin')
  await page.getByTestId('login-password').fill(NEW_PASSWORD)
  await page.getByTestId('login-submit').click()
  await expect(page).toHaveURL(/\/query(?:\?|$)/)
  await expect(page).not.toHaveURL(/login/)
  await expect(page.getByTestId('query-page').or(page.getByRole('main')).first()).toBeVisible()
  await page.goto('/')
  await expect(page.getByTestId('overview-page')).toBeVisible()
  await expect(page.getByTestId('sidebar')).toBeVisible()
  await expect(page.getByTestId('sidebar-collapse-toggle')).toBeVisible()
  await expect(page.getByTestId('sidebar-section-workspace')).toBeVisible()
  await expect(page.getByTestId('sidebar-section-label-workspace')).toBeVisible()
  // 组内排序控件（工作区至少 3 项）
  await expect(page.getByTestId('sidebar-order-up-query')).toBeVisible()
  await expect(page.getByTestId('sidebar-order-down-query')).toBeVisible()
  await expect(page.getByTestId('sidebar-drag-query')).toBeVisible()
  await expect(page.getByTestId('sidebar-drag-write')).toBeVisible()
  await page.getByTestId('sidebar-order-up-query').click()
  // P107: 拖拽手柄可见；拖拽 write 到 home 行后顺序持久化
  const navKeySmoke = 'mts.dashboard.nav-order.prefs.v1'
  await page.getByTestId('sidebar-drag-write').dragTo(page.getByTestId('sidebar-nav-row-home'))
  await expect.poll(async () => {
    const raw = await page.evaluate((k) => localStorage.getItem(k), navKeySmoke)
    if (!raw) return false
    try {
      const parsed = JSON.parse(raw) as { order?: { workspace?: string[] } }
      const ws = parsed.order?.workspace || []
      return ws.includes('/write') && ws.indexOf('/write') < ws.indexOf('/')
    } catch {
      return false
    }
  }).toBe(true)
  // 过滤时隐藏排序（避免与搜索结果冲突）
  await page.getByTestId('sidebar-filter').fill('query')
  await expect(page.getByTestId('sidebar-order-up-query')).toHaveCount(0)
  await expect(page.getByTestId('sidebar-drag-query')).toHaveCount(0)
  await page.getByTestId('sidebar-filter-clear').click()
  await expect(page.getByTestId('sidebar-order-up-query')).toBeVisible()
  await expect(page.getByTestId('sidebar-drag-query')).toBeVisible()
  // P424: 分组重置侧栏排序（workspace）；重置后重建自定义序，供后续账户偏好导出断言
  await expect(page.getByTestId('sidebar-order-reset-workspace')).toBeVisible()
  await page.getByTestId('sidebar-order-reset-workspace').click()
  await expect.poll(async () => {
    const raw = await page.evaluate((k) => localStorage.getItem(k), navKeySmoke)
    if (!raw) return true
    try {
      const parsed = JSON.parse(raw) as { order?: { workspace?: string[] } }
      const ws = parsed.order?.workspace
      return !ws || ws.length === 0
    } catch {
      return false
    }
  }).toBe(true)
  // 命令面板 section 重置入口
  await page.getByTestId('sidebar-order-up-query').click()
  await expect.poll(async () => {
    const raw = await page.evaluate((k) => localStorage.getItem(k), navKeySmoke)
    if (!raw) return false
    try {
      const parsed = JSON.parse(raw) as { order?: { workspace?: string[] } }
      return Array.isArray(parsed.order?.workspace) && (parsed.order?.workspace?.length || 0) > 0
    } catch {
      return false
    }
  }).toBe(true)
  await page.getByTestId('topbar-command-palette').click()
  await expect(page.getByTestId('command-palette')).toBeVisible()
  await page.getByTestId('command-palette-input').fill('重置工作区')
  await expect(page.getByTestId('command-item-action-reset-nav-section-workspace')).toBeVisible()
  await page.getByTestId('command-item-action-reset-nav-section-workspace').click()
  await expect(page.getByTestId('command-palette')).toHaveCount(0)
  // P425: 英文 locale 下 section 重置可检索（随后恢复中文，避免干扰后续文案断言）
  await page.getByTestId('topbar-lang').click()
  await page.getByTestId('sidebar-order-up-query').click()
  await page.getByTestId('topbar-command-palette').click()
  await expect(page.getByTestId('command-palette')).toBeVisible()
  await page.getByTestId('command-palette-input').fill('reset workspace')
  await expect(page.getByTestId('command-item-action-reset-nav-section-workspace')).toBeVisible()
  await page.getByTestId('command-item-action-reset-nav-section-workspace').click()
  await expect(page.getByTestId('command-palette')).toHaveCount(0)
  await page.getByTestId('topbar-lang').click()
  // 再次写入自定义序，避免清空后账户页 orderBefore 为空
  await page.getByTestId('sidebar-order-up-query').click()
  await expect.poll(async () => page.evaluate((k) => localStorage.getItem(k), navKeySmoke)).toBeTruthy()
  await expect(page.getByTestId('overview-export-json')).toBeVisible()
  await expect(page.getByTestId('overview-copy-snapshot')).toBeVisible()
  await expect(page.getByTestId('overview-share-link')).toBeVisible()
  await expect(page.getByTestId('offline-banner')).toHaveCount(0)
  // P285: 标签页可见性恢复不打断会话
  await page.evaluate(() => {
    Object.defineProperty(document, 'visibilityState', { configurable: true, get: () => 'visible' })
    document.dispatchEvent(new Event('visibilitychange'))
  })
  await expect(page.getByTestId('overview-page').or(page.getByTestId('topbar-account')).first()).toBeVisible()
  await expect(page.getByTestId('server-unreachable-banner')).toHaveCount(0)
  await expect(page.getByTestId('overview-connectivity')).toBeVisible()
  await expect(page.getByTestId('overview-connectivity-kind')).toBeVisible()
  await expect(page.getByTestId('topbar-account')).toBeVisible()
  await expect(page.getByTestId('topbar-connectivity')).toBeVisible()

    // account session card + P191/P192 续期引导
  await page.goto('/account')
  await expect(page.getByTestId('account-page')).toBeVisible()
  await expect(page.getByTestId('account-password-error')).toHaveCount(0)
  await expect(page.getByTestId('account-session-renew-error')).toHaveCount(0)
  await expect(page.getByTestId('account-share-link')).toBeVisible()
  await expect(page.getByTestId('account-session')).toBeVisible()
  await expect(page.getByTestId('account-session-renew-toggle')).toBeVisible()
  await expect(page.getByTestId('account-toggle-old')).toBeVisible()
  await expect(page.getByTestId('account-toggle-new')).toBeVisible()
  await expect(page.getByTestId('account-toggle-confirm')).toBeVisible()
  await expect(page.getByTestId('account-session-remaining')).toBeVisible()
  await expect(page.getByTestId('account-session-relogin')).toBeVisible()
  await expect(page.getByTestId('account-session-hint')).toBeVisible()
  // 顶栏会话徽章跳转账户会话区
  await page.goto('/')
  await expect(page.getByTestId('session-badge')).toBeVisible()
  await page.getByTestId('session-badge').click()
  await expect(page).toHaveURL(/\/account/)
  await expect(page.getByTestId('account-session')).toBeVisible()
  await page.goto('/')

  await expect(page.getByTestId('skip-to-main')).toHaveCount(1)
  await expect(page).toHaveTitle(/仪表盘|概览|Overview/)

  // 3) Line Protocol 写入
  await page.goto('/write')
  await expect(page.getByTestId('write-page')).toBeVisible()
  await expect(page.getByTestId('write-action-error')).toHaveCount(0)
  await expect(page.getByTestId('write-export-draft')).toBeVisible()
  await expect(page.getByTestId('write-share-link')).toBeVisible()
  await expect(page.getByTestId('write-export-result')).toBeVisible()
  await page.getByRole('main').getByRole('button', { name: 'Line Protocol' }).click()
  await page.getByRole('main').locator('textarea').first().fill('cpu,host=e2e-playwright usage=0.42 1000')
  await page.getByTestId('write-submit').click()
  await expect(page.getByTestId('write-result-ok')).toBeVisible({ timeout: 20_000 })
  // P236: 成功结果 live region
  await expect(page.getByTestId('write-result-ok')).toHaveAttribute('role', 'status')
  await expect(page.getByRole('main').getByText(/写入成功/).first()).toBeVisible({ timeout: 20_000 })
  await expect(page.getByRole('main').getByRole('button', { name: /表单写入|Form write/i })).toBeVisible()
  await expect(page.getByTestId('write-mode-tabs')).toBeVisible()
  await expect(page.getByTestId('write-mode-typed')).toBeVisible()
  await expect(page.getByTestId('write-prefs-hint')).toBeVisible()
  // P138: 表单写行上限指示（切到 form 模式可见）
  await page.getByTestId('write-mode-form').click()
  await expect(page.getByTestId('write-form-row-count')).toBeVisible()
  await expect(page.getByTestId('write-meta-panel')).toBeVisible()
  await expect(page.getByTestId('write-add-row')).toBeVisible()
  await expect(page.getByTestId('write-retention-policy')).toBeVisible()
  // P245: 健康路径写入元数据错误区不出现
  await expect(page.getByTestId('write-meta-error')).toHaveCount(0)

  // P240/P241: 管理页可达与错误恢复入口存在
  await page.goto('/api-spec')
  await expect(page.getByTestId('api-spec-page').or(page.getByRole('main')).first()).toBeVisible()
  await page.goto('/storage')
  await expect(page.getByTestId('storage-page')).toBeVisible()
  await expect(page.getByTestId('storage-list-error')).toHaveCount(0)
  await expect(page.getByTestId('storage-snapshots-refresh-error')).toHaveCount(0)
  await expect(page.getByTestId('storage-data-refresh-error')).toHaveCount(0)
  await expect(page.getByTestId('storage-list-error')).toHaveCount(0)

  // 4) 查询页可达 + 执行一次 rows 查询以验证结果虚拟列表
  await page.goto('/query')
  await expect(page.getByTestId('query-page')).toBeVisible()
  await expect(page.getByTestId('query-action-error')).toHaveCount(0)
  // 等库/表元数据预填后再查（写入后通常有 default + cpu）
  await expect.poll(async () => {
    const db = await page.getByTestId('query-database').inputValue()
    return db.trim().length > 0
  }, { timeout: 15_000 }).toBe(true)
  const measInput = page.getByTestId('query-measurement')
  if (!(await measInput.inputValue()).trim()) {
    await measInput.fill('cpu')
  }
  await page.getByTestId('query-run').click()
  await expect(page.getByTestId('query-run')).toBeEnabled({ timeout: 20_000 })
  if (await page.getByTestId('query-results-virtual-list').count()) {
    await expect(page.getByTestId('query-results-virtual-list')).toBeVisible()
    await expect(page.getByTestId('query-results-virtual-hint')).toBeVisible()
  }
  // P134: 查询历史面板虚拟列表（有历史时）
  await page.goto('/query#query-history')
  await expect(page.getByTestId('query-page')).toBeVisible()
  // 打开历史（hash 会自动 showHistory）
  if (await page.getByTestId('query-history-panel').count()) {
    await expect(page.getByTestId('query-history-panel')).toBeVisible()
    await expect(page.getByTestId('query-history-filter')).toBeVisible()
    if (await page.getByTestId('query-history-virtual-list').count()) {
      await expect(page.getByTestId('query-history-virtual-list')).toBeVisible()
      await expect(page.getByTestId('query-history-virtual-hint')).toBeVisible()
    }
  }
  // P120: 只读时间预填深链
  await page.goto('/query?range=1h#query-form')
  await expect(page.getByTestId('query-page')).toBeVisible()
  await expect(page.getByTestId('query-start-time')).not.toHaveValue('')
  await expect(page.getByTestId('query-end-time')).not.toHaveValue('')
  const startV = await page.getByTestId('query-start-time').inputValue()
  const endV = await page.getByTestId('query-end-time').inputValue()
  expect(Number(endV) - Number(startV)).toBeGreaterThanOrEqual(3500_000)

  await expect(page.getByTestId('breadcrumb-bar')).toBeVisible()
  await expect(page.getByTestId('global-progress')).toBeAttached()
  await expect(page.getByTestId('breadcrumb-current')).toBeVisible()
  await expect(page.getByTestId('breadcrumb-copy-path')).toBeVisible()
  await expect(page.getByRole('main').getByText(/查询|Query/i).first()).toBeVisible()
  await expect(page.getByRole('main').getByText(/数据库|Database/).first()).toBeVisible()
  await expect(page.getByRole('main').getByText(/开始时间|Start/).first()).toBeVisible()
  await expect(page.getByTestId('query-export-csv')).toBeVisible()
  await expect(page.getByTestId('query-share-link')).toBeVisible()
  await expect(page.getByTestId('query-predicates')).toBeVisible()
  await expect(page.getByTestId('query-series-meta')).toBeVisible()
  await expect(page.getByTestId('query-series-refresh')).toBeVisible()
  await expect(page.getByTestId('query-series-select')).toBeVisible()
  await expect(page.getByTestId('query-series-filter')).toBeVisible()
  await expect(page.getByTestId('query-fields')).toBeVisible()
  await expect(page.getByTestId('query-stats-panel')).toBeVisible()
  await expect(page.getByTestId('query-engine-stats')).toBeVisible()
  await expect(page.getByTestId('query-range-delete')).toBeVisible()
  await page.getByTestId('query-range-delete').click()
  await expect(page.getByTestId('confirm-dialog')).toBeVisible()
  await expect(page.getByTestId('confirm-dialog-confirm')).toBeDisabled()
  await page.getByTestId('confirm-dialog-input').fill('DELETE')
  await expect(page.getByTestId('confirm-dialog-confirm')).toBeEnabled()
  await page.getByTestId('confirm-dialog-cancel').click()
  await page.getByTestId('query-engine-stats').click()
  // P244: 引擎统计错误区默认不出现
  await expect(page.getByTestId('query-engine-stats-error')).toHaveCount(0)
  await expect(page.getByTestId('query-stats-source-engine')).toBeVisible({ timeout: 15_000 })
  // 若有结果则校验虚拟列表；冷启动无结果时跳过
  if (await page.getByTestId('query-results-virtual-list').count()) {
    await expect(page.getByTestId('query-results-virtual-list')).toBeVisible()
    await expect(page.getByTestId('query-results-virtual-hint')).toBeVisible()
  }
  if (await page.getByTestId('query-columns-virtual-list').count()) {
    await expect(page.getByTestId('query-columns-virtual-list')).toBeVisible()
    await expect(page.getByTestId('query-columns-virtual-hint')).toBeVisible()
  }

  // 5) 运维 Flush（确认按钮文案为「执行」）
  await page.goto('/operations')
  await expect(page.getByTestId('ops-page')).toBeVisible()
  await expect(page.getByTestId('ops-share-link')).toBeVisible()

  await expect(page.getByTestId('ops-status-strip')).toBeVisible()
  await expect(page.getByTestId('ops-status-refresh-busy')).toBeVisible()
  // P317/P321: 空闲时无全局 admin busy / 轮询失败横幅
  await expect(page.getByTestId('admin-op-busy-banner')).toHaveCount(0)
  await expect(page.getByTestId('admin-op-busy-poll-error-banner')).toHaveCount(0)
  await expect(page.getByTestId('ops-partial-error')).toHaveCount(0)
  await expect(page.getByTestId('ops-load-error')).toHaveCount(0)
  await expect(page.getByTestId('ops-status-connectivity')).toBeVisible()
  await expect(page.getByTestId('ops-status-stats-at')).toBeVisible()
  await expect(page.getByRole('main').getByRole('heading', { name: /^(运维|Operations)$/ })).toBeVisible()
  await expect(page.getByTestId('ops-export-stats')).toBeVisible()
  await expect(page.getByTestId('ops-copy-stats')).toBeVisible()
  await expect(page.getByTestId('ops-flush')).toContainText(/Flush/)
  await expect(page.getByTestId('ops-retention')).toContainText(/保留策略|Retention/i)
  await page.getByRole('main').getByRole('button', { name: /Flush/ }).first().click()
  await page.getByRole('button', { name: '执行', exact: true }).click()
  await expect(page.getByRole('main').getByText('Flush 已完成').first()).toBeVisible({ timeout: 20_000 })
  // P350–P351: flush 后展示最近一次管理重操作
  await expect(page.getByTestId('ops-status-last')).toBeVisible({ timeout: 15_000 })
  await expect(page.getByTestId('ops-status-last')).toContainText(/flush|Flush|刷盘/i)
  await expect(page.getByTestId('admin-op-last-banner')).toBeVisible()
  await expect(page.getByTestId('admin-op-last-summary')).toContainText(/flush|Flush|刷盘|ok/i)
  await expect(page.getByTestId('ops-status-last')).toHaveAttribute('data-ok', 'true')
  await expect(page.getByTestId('admin-op-last-banner')).toHaveAttribute('data-ok', 'true')
  await expect(page.getByTestId('admin-op-last-copy')).toBeVisible()
  await page.getByTestId('admin-op-last-copy').click()
  await page.getByTestId('admin-op-last-dismiss').click()
  await expect(page.getByTestId('admin-op-last-banner')).toHaveCount(0)
  // 运维条仍保留最近一次
  await expect(page.getByTestId('ops-status-last')).toBeVisible()
  await page.getByTestId('ops-status-refresh-busy').click()
  await expect(page.getByTestId('ops-status-last')).toContainText(/flush|Flush|刷盘/i)

  // P360: 其它管理页可看到 last 摘要芯片
  await page.goto('/databases')
  await expect(page.getByTestId('databases-page')).toBeVisible()
  await expect(page.getByTestId('databases-admin-last')).toBeVisible()
  await expect(page.getByTestId('databases-admin-last')).toContainText(/flush|Flush|刷盘|ok/i)
  await expect(page.getByTestId('databases-admin-last-copy')).toBeVisible()
  await page.goto('/users')
  await expect(page.getByTestId('users-page')).toBeVisible()
  await expect(page.getByTestId('users-admin-last')).toBeVisible()
  await expect(page.getByTestId('users-admin-last-copy')).toBeVisible()

  await page.goto('/observability/metrics')
  await expect(page.getByTestId('metrics-page')).toBeVisible()
  await expect(page.getByTestId('metrics-admin-last')).toBeVisible()
  await expect(page.getByTestId('metrics-admin-last-copy')).toBeVisible()
  await page.goto('/audit')
  await expect(page.getByTestId('audit-page')).toBeVisible()
  await expect(page.getByTestId('audit-admin-last')).toBeVisible()
  await expect(page.getByTestId('audit-admin-last-copy')).toBeVisible()

  // P366: About / ApiSpec last 芯片（不在 h1 内）
  await page.goto('/about')
  await expect(page.getByTestId('about-page')).toBeVisible()
  await expect(page.getByTestId('about-admin-last')).toBeVisible()
  await expect(page.getByTestId('about-admin-last')).toContainText(/flush|Flush|刷盘|ok/i)
  await expect(page.getByTestId('about-admin-last-copy')).toBeVisible()
  await page.goto('/api-spec')
  await expect(page.getByTestId('api-spec-page')).toBeVisible()
  await expect(page.getByTestId('api-spec-admin-last')).toBeVisible()
  await expect(page.getByTestId('api-spec-admin-last')).toContainText(/flush|Flush|刷盘|ok/i)
  await expect(page.getByTestId('api-spec-admin-last-copy')).toBeVisible()

  // P368: Operations 复制最近一次
  await page.goto('/operations')
  await expect(page.getByTestId('ops-status-last')).toBeVisible()
  await expect(page.getByTestId('ops-status-last-copy')).toBeVisible()
  await page.getByTestId('ops-status-last-copy').click()

  // P369: Access last 芯片
  await page.goto('/access')
  await expect(page.getByTestId('access-matrix-page')).toBeVisible()
  await expect(page.getByTestId('access-matrix-admin-last')).toBeVisible()
  await expect(page.getByTestId('access-matrix-admin-last-copy')).toBeVisible()
  await page.goto('/access/grants')
  await expect(page.getByTestId('access-grants-page')).toBeVisible()
  await expect(page.getByTestId('access-grants-admin-last')).toBeVisible()
  await expect(page.getByTestId('access-grants-admin-last-copy')).toBeVisible()

  await page.goto('/account')
  await expect(page.getByTestId('account-page')).toBeVisible()
  await expect(page.getByTestId('account-admin-last')).toBeVisible()
  await expect(page.getByTestId('account-admin-last-copy')).toBeVisible()

  await page.goto('/query')
  await expect(page.getByTestId('query-page')).toBeVisible()
  await expect(page.getByTestId('query-admin-last')).toBeVisible()
  await expect(page.getByTestId('query-admin-last-copy')).toBeVisible()
  await page.goto('/write')
  await expect(page.getByTestId('write-page')).toBeVisible()
  await expect(page.getByTestId('write-admin-last')).toBeVisible()
  await expect(page.getByTestId('write-admin-last-copy')).toBeVisible()

  // P383: mock ops-status / stats/maintenance 失败 last → 横幅 error + Overview 明细
  const failLastPayload = {
    admin_op_busy: false,
    last: {
      op: 'compact',
      ok: false,
      error: 'e2e disk full',
      started_at_unix: 1700000000,
      finished_at_unix: Math.floor(Date.now() / 1000),
      duration_ms: 42,
    },
  }
  const fulfillFailLast = async (route: import('@playwright/test').Route) => {
    const url = route.request().url()
    if (url.includes('/stats/maintenance')) {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          stats: {
            compaction_active: 0,
            retention_active: 0,
            flush_active: 0,
          },
          ...failLastPayload,
        }),
      })
      return
    }
    if (url.includes('/stats/compaction')) {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          stats: { active: 0, backlog: 0, total: 0, success: 0, failure: 0, last_error: '' },
          ...failLastPayload,
        }),
      })
      return
    }
    if (url.includes('/stats/storage-memory')) {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          snapshot: {},
          ...failLastPayload,
        }),
      })
      return
    }
    if (url.includes('/maintenance/errors')) {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          errors: [],
          ...failLastPayload,
        }),
      })
      return
    }
    if (url.includes('/storage/export')) {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          export: { generated_at: '2026-01-01T00:00:00Z', config: {}, health: {} },
          ...failLastPayload,
        }),
      })
      return
    }
    if (url.includes('/storage/data-snapshots') || url.includes('/storage/snapshots')) {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          snapshots: [],
          ...failLastPayload,
        }),
      })
      return
    }
    if (url.includes('/config/schema')) {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ fields: [], ...failLastPayload }),
      })
      return
    }
    if (url.includes('/config/effective') || url.endsWith('/admin/config') || url.includes('/admin/config?')) {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ config: {}, ...failLastPayload }),
      })
      return
    }
    if (url.includes('/api-spec')) {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ version: 'v1', namespaces: [], ...failLastPayload }),
      })
      return
    }
    if (url.includes('/error-codes')) {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ codes: [], ...failLastPayload }),
      })
      return
    }
    if (url.includes('/admin/audit') || (url.includes('/users/') && url.includes('/audit'))) {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ events: [], total: 0, ...failLastPayload }),
      })
      return
    }
    if (url.includes('/downsample/policies')) {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ policies: [], ...failLastPayload }),
      })
      return
    }
    if (url.includes('/downsample/statuses')) {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ statuses: [], ...failLastPayload }),
      })
      return
    }
    if (url.includes('/api/v1/users') && !url.includes('/users/')) {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ users: [], ...failLastPayload }),
      })
      return
    }
    // 用户库权限列表/单项：AccessGrants 并发拉取时也会 apply
    if (url.includes('/database-permissions')) {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ grants: [], ok: true, ...failLastPayload }),
      })
      return
    }
    if (url.includes('/admin/databases') || url.includes('/data/databases')) {
      // 避免误伤 databases/{name}/... 子资源：仅列表路径
      const pathOnly = url.split('?')[0]
      if (pathOnly.endsWith('/admin/databases') || pathOnly.endsWith('/data/databases')) {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ databases: [], measurements: [], ...failLastPayload }),
        })
        return
      }
    }
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(failLastPayload),
    })
  }
  // P418: 统一 ** 匹配，避免 query/trailing 导致 mock 未命中而真实 last 覆盖 fail 场景
  await page.route('**/api/v1/admin/ops-status**', fulfillFailLast)
  await page.route('**/api/v1/admin/stats/maintenance**', fulfillFailLast)
  await page.route('**/api/v1/admin/stats/compaction**', fulfillFailLast)
  await page.route('**/api/v1/admin/stats/storage-memory**', fulfillFailLast)
  await page.route('**/api/v1/admin/storage/snapshots**', fulfillFailLast)
  await page.route('**/api/v1/admin/storage/data-snapshots**', fulfillFailLast)
  await page.route('**/api/v1/admin/storage/export**', fulfillFailLast)
  await page.route('**/api/v1/admin/config/effective**', fulfillFailLast)
  await page.route('**/api/v1/admin/config/schema**', fulfillFailLast)
  await page.route('**/api/v1/admin/config**', fulfillFailLast)
  await page.route('**/api/v1/admin/api-spec**', fulfillFailLast)
  await page.route('**/api/v1/admin/error-codes**', fulfillFailLast)
  await page.route('**/api/v1/admin/audit**', fulfillFailLast)
  await page.route('**/api/v1/users/**/audit**', fulfillFailLast)
  await page.route('**/api/v1/admin/downsample/policies**', fulfillFailLast)
  await page.route('**/api/v1/admin/downsample/statuses**', fulfillFailLast)
  // users 列表：用 ** 但 fulfill 内排除 /users/{name}/...
  await page.route('**/api/v1/users**', fulfillFailLast)
  await page.route('**/api/v1/admin/databases**', fulfillFailLast)
  await page.route('**/api/v1/data/databases**', fulfillFailLast)
  await page.route('**/api/v1/admin/maintenance/errors**', fulfillFailLast)
  // doctor / admin health 加载也会 applyAdminOpStatus，需与 fail last 一致
  await page.route('**/api/v1/admin/doctor**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        ok: true,
        http_tls_enabled: false,
        checks: [{ level: 'ok', code: 'data_dir', message: 'data_dir ready' }],
        lines: ['ok: data_dir ready'],
        ...failLastPayload,
      }),
    })
  })
  await page.route('**/api/v1/admin/health**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        health: { healthy: true, ready: true, reasons: [], checks: [] },
        ...failLastPayload,
      }),
    })
  })
  await page.route('**/api/v1/admin/version**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        version: 'e2e',
        commit: 'deadbeef',
        built_at: '2026-01-01T00:00:00Z',
        ...failLastPayload,
      }),
    })
  })
  // session / query stats / storage validate 也会 applyAdminOpStatus，需与 fail last 一致
  await page.route('**/api/v1/auth/session**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        ok: true,
        user_name: 'admin',
        role: 'admin',
        expires_at: new Date(Date.now() + 3600_000).toISOString(),
        remaining_seconds: 3600,
        server_time_unix: Math.floor(Date.now() / 1000),
        ...failLastPayload,
      }),
    })
  })
  await page.route('**/api/v1/data/query/stats**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        stats: {},
        ...failLastPayload,
      }),
    })
  })
  await page.route('**/api/v1/admin/storage/validate**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        ok: true,
        data_dir: '/tmp/e2e',
        health: { healthy: true, ready: true },
        ...failLastPayload,
      }),
    })
  })
  await page.route('**/api/v1/admin/config/validate**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        ok: true,
        ...failLastPayload,
      }),
    })
  })
  // write/delete 成功响应也会 apply；fail-last 场景需覆盖以免真实 last 覆盖
  await page.route('**/api/v1/data/write', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ ok: true, ...failLastPayload }),
    })
  })
  await page.route('**/api/v1/data/write/**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ ok: true, ...failLastPayload }),
    })
  })
  await page.route('**/api/v1/data/delete', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ ok: true, ...failLastPayload }),
    })
  })
  // Users grant/revoke/batch 成功响应也会 apply；fail-last 场景需覆盖以免真实 last 覆盖
  await page.route('**/api/v1/users/**/database-permissions/**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ ok: true, ...failLastPayload }),
    })
  })
  await page.route('**/api/v1/users/batch-disabled**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/x-ndjson',
      body: [
        JSON.stringify({ type: 'item', index: 1, total: 1, name: 'u1', status: 'ok' }),
        JSON.stringify({
          type: 'summary',
          ok: true,
          ok_count: 1,
          skip_count: 0,
          fail_count: 0,
          total: 1,
          items: [{ name: 'u1', status: 'ok' }],
          ...failLastPayload,
        }),
      ].join('\n') + '\n',
    })
  })
  // P419: snapshots GET 已由 fulfillFailLast 覆盖；此路由仅保证 DELETE 也带 fail last
  await page.route('**/api/v1/admin/storage/snapshots**', async (route) => {
    const req = route.request()
    const url = req.url()
    if (req.method() === 'DELETE') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ ok: true, ...failLastPayload }),
      })
      return
    }
    // GET list 由 fulfillFailLast 覆盖；此处仅兜底 DELETE 以外未匹配场景
    await route.fallback()
  })
  await page.goto('/operations')
  await page.getByTestId('ops-status-refresh-busy').click()
  await expect(page.getByTestId('ops-status-last')).toContainText(/fail|失败|compact|压缩/i)
  await expect(page.getByTestId('admin-op-last-banner')).toBeVisible()
  await expect(page.getByTestId('admin-op-last-banner')).toHaveAttribute('data-ok', 'false')
  await expect(page.getByTestId('admin-op-last-error')).toContainText(/e2e disk full/i)
  await expect(page.getByTestId('ops-status-last-error')).toContainText(/e2e disk full/i)
  // 运维页对失败 last 自动 ack 后可关闭
  await expect(page.getByTestId('admin-op-last-dismiss')).toBeEnabled({ timeout: 5_000 })
  await page.getByTestId('admin-op-last-dismiss').click()
  await expect(page.getByTestId('admin-op-last-banner')).toHaveCount(0)
  await page.goto('/')
  await expect(page.getByTestId('overview-admin-last')).toBeVisible()
  await expect(page.getByTestId('overview-admin-last')).toHaveAttribute('data-ok', 'false')
  await expect(page.getByTestId('overview-admin-last-error')).toContainText(/e2e disk full/i)
  await expect(page.getByTestId('overview-admin-last-copy')).toBeVisible()
  await expect(page.getByTestId('overview-admin-last')).toHaveAttribute('aria-label', /.+/)
  await expect(page.getByTestId('overview-admin-last-copy')).toHaveAttribute('aria-label', /.+/)
  // P397: last 芯片可跳转运维状态条
  await page.getByTestId('overview-admin-last').click()
  await expect(page).toHaveURL(/\/operations/)
  await expect(page.getByTestId('ops-status-strip')).toBeVisible()
  await page.goto('/storage')
  await expect(page.getByTestId('storage-page')).toBeVisible()
  await expect(page.getByTestId('storage-admin-last')).toHaveAttribute('data-ok', 'false')
  await expect(page.getByTestId('storage-admin-last-error')).toContainText(/e2e disk full/i)
  await expect(page.getByTestId('storage-admin-last-copy')).toBeVisible()
  // P413: Write/Query 加载 meta 时也会 apply fail last
  await page.goto('/write')
  await expect(page.getByTestId('write-page')).toBeVisible()
  await expect(page.getByTestId('write-admin-last')).toHaveAttribute('data-ok', 'false')
  await expect(page.getByTestId('write-admin-last-error')).toContainText(/e2e disk full/i)
  await expect(page.getByTestId('write-admin-last-copy')).toBeVisible()
  await page.goto('/query')
  await expect(page.getByTestId('query-page')).toBeVisible()
  await expect(page.getByTestId('query-admin-last')).toHaveAttribute('data-ok', 'false')
  await expect(page.getByTestId('query-admin-last-error')).toContainText(/e2e disk full/i)
  await expect(page.getByTestId('query-admin-last-copy')).toBeVisible()
  // P415: Users 页 last 芯片（列表加载 apply；grant/revoke/batch 路径已 mock）
  await page.goto('/users')
  await expectFailAdminLast(page, 'users-page', 'users-admin-last')
  // P417: 管理页 fail-last 芯片覆盖（列表/配置加载 apply；Metrics/AccessMatrix 走 refresh）
  await page.goto('/config')
  await expectFailAdminLast(page, 'config-page', 'config-admin-last')
  await page.goto('/downsample')
  await expectFailAdminLast(page, 'downsample-page', 'downsample-admin-last')
  await page.goto('/databases')
  await expectFailAdminLast(page, 'databases-page', 'databases-admin-last')
  await page.goto('/audit')
  await expectFailAdminLast(page, 'audit-page', 'audit-admin-last')
  await page.goto('/access')
  await expectFailAdminLast(page, 'access-matrix-page', 'access-matrix-admin-last')
  await page.goto('/access/grants')
  await expectFailAdminLast(page, 'access-grants-page', 'access-grants-admin-last')
  await page.goto('/observability/metrics')
  await expectFailAdminLast(page, 'metrics-page', 'metrics-admin-last')
  // P418: 指标页深链 hash 可定位筛选区（侧栏/命令面板一致性）
  // 注意：浏览器全页 /metrics 是服务端 Prometheus 抓取路径，不会进 SPA；UI 路由为 /observability/metrics
  await page.goto('/observability/metrics#metrics-filter')
  await expect(page.getByTestId('metrics-page')).toBeVisible()
  await expect(page.getByTestId('metrics-filter').or(page.locator('#metrics-filter'))).toBeVisible()
  await expect(page).toHaveURL(/\/observability\/metrics/)
  await page.goto('/about')
  await expectFailAdminLast(page, 'about-page', 'about-admin-last')
  await page.goto('/api-spec')
  await expectFailAdminLast(page, 'api-spec-page', 'api-spec-admin-last')
  await page.goto('/account')
  await expectFailAdminLast(page, 'account-page', 'account-admin-last')
  // P386: 失败 last 进入就绪评分原因（本地化文案）
  await page.goto('/ops/readiness')
  await expect(page.getByRole('main').getByRole('heading', { name: /就绪中心|Commercial readiness|可商用就绪/ })).toBeVisible()
  await expect(page.getByTestId('readiness-admin-last')).toHaveAttribute('data-ok', 'false')
  await expect(page.getByTestId('readiness-admin-last-error')).toContainText(/e2e disk full/i)
  await expect(page.getByTestId('readiness-score-reasons')).toBeVisible()
  await expect(page.getByTestId('readiness-score-reasons')).toContainText(/最近管理重操作失败|Last admin heavy op failed/)
  await page.unroute('**/api/v1/admin/ops-status**', fulfillFailLast).catch(() => {})
  await page.unroute('**/api/v1/admin/stats/maintenance**', fulfillFailLast).catch(() => {})
  await page.unroute('**/api/v1/admin/stats/compaction**', fulfillFailLast).catch(() => {})
  await page.unroute('**/api/v1/admin/stats/storage-memory**', fulfillFailLast).catch(() => {})
  await page.unroute('**/api/v1/admin/storage/snapshots**', fulfillFailLast).catch(() => {})
  await page.unroute('**/api/v1/admin/storage/data-snapshots**', fulfillFailLast).catch(() => {})
  await page.unroute('**/api/v1/admin/storage/export**', fulfillFailLast).catch(() => {})
  await page.unroute('**/api/v1/admin/config/effective**', fulfillFailLast).catch(() => {})
  await page.unroute('**/api/v1/admin/config/schema**', fulfillFailLast).catch(() => {})
  await page.unroute('**/api/v1/admin/config**', fulfillFailLast).catch(() => {})
  await page.unroute('**/api/v1/admin/api-spec**', fulfillFailLast).catch(() => {})
  await page.unroute('**/api/v1/admin/error-codes**', fulfillFailLast).catch(() => {})
  await page.unroute('**/api/v1/admin/audit**', fulfillFailLast).catch(() => {})
  await page.unroute('**/api/v1/users/**/audit**', fulfillFailLast).catch(() => {})
  await page.unroute('**/api/v1/admin/downsample/policies**', fulfillFailLast).catch(() => {})
  await page.unroute('**/api/v1/admin/downsample/statuses**', fulfillFailLast).catch(() => {})
  await page.unroute('**/api/v1/users**', fulfillFailLast).catch(() => {})
  await page.unroute('**/api/v1/admin/databases**', fulfillFailLast).catch(() => {})
  await page.unroute('**/api/v1/data/databases**', fulfillFailLast).catch(() => {})
  await page.unroute('**/api/v1/admin/maintenance/errors**', fulfillFailLast).catch(() => {})
  await page.unroute('**/api/v1/admin/doctor**').catch(() => {})
  await page.unroute('**/api/v1/admin/health**').catch(() => {})
  await page.unroute('**/api/v1/admin/version**').catch(() => {})
  await page.unroute('**/api/v1/auth/session**').catch(() => {})
  await page.unroute('**/api/v1/data/query/stats**').catch(() => {})
  await page.unroute('**/api/v1/admin/storage/validate**').catch(() => {})
  await page.unroute('**/api/v1/admin/config/validate**').catch(() => {})
  await page.unroute('**/api/v1/data/write').catch(() => {})
  await page.unroute('**/api/v1/data/write/**').catch(() => {})
  await page.unroute('**/api/v1/data/delete').catch(() => {})
  await page.unroute('**/api/v1/users/**/database-permissions/**').catch(() => {})
  await page.unroute('**/api/v1/users/batch-disabled**').catch(() => {})
  // snapshots** 已在上面 unroute；避免重复依赖精确路径

  // P381: mock 写入命中 admin busy → 结果条可跳转运维
  await page.route('**/api/v1/data/write', async (route) => {
    await route.fulfill({
      status: 429,
      contentType: 'application/json',
      body: JSON.stringify({
        ok: false,
        code: 'resource_exhausted',
        message: 'admin heavy op already in progress: flush',
        admin_op_busy: true,
        op: 'flush',
      }),
    })
  })
  await page.route('**/api/v1/data/write/**', async (route) => {
    await route.fulfill({
      status: 429,
      contentType: 'application/json',
      body: JSON.stringify({
        ok: false,
        code: 'resource_exhausted',
        message: 'admin heavy op already in progress: flush',
        admin_op_busy: true,
        op: 'flush',
      }),
    })
  })
  await page.goto('/write')
  await expect(page.getByTestId('write-page')).toBeVisible()
  await page.getByTestId('write-mode-line').click()
  await page.getByRole('main').locator('textarea').first().fill('cpu,host=e2e-busy usage=1 1000')
  await page.getByTestId('write-submit').click()
  await expect(page.getByTestId('write-action-error')).toBeVisible({ timeout: 10_000 })
  await expect(page.getByTestId('action-result-action')).toBeVisible()
  await page.getByTestId('action-result-action').click()
  await expect(page).toHaveURL(/\/operations/)
  await expect(page.getByTestId('ops-status-strip')).toBeVisible()
  await page.unroute('**/api/v1/data/write').catch(() => {})
  await page.unroute('**/api/v1/data/write/**').catch(() => {})

  // 6) 权限矩阵 / 实时授权 / 指标
  await page.goto('/access')
  await expect(page.getByTestId('access-matrix-page')).toBeVisible()
  await expect(page.getByRole('main').getByRole('heading', { name: /权限能力矩阵|Capability matrix/ })).toBeVisible()
  // a11y 树中 th 可能暴露为 cell
  await expect(page.getByRole('main').getByText(/管理员|Admin/).first()).toBeVisible()
  await expect(page.getByTestId('access-matrix-export')).toBeVisible()
  await expect(page.getByTestId('access-matrix-table')).toBeVisible()
  await expect(page.getByTestId('access-matrix-table-header')).toBeVisible()
  await expect(page.getByTestId('access-matrix-virtual-list')).toBeVisible()
  await expect(page.getByTestId('access-matrix-virtual-hint')).toBeVisible()
  await page.goto('/access/grants')
  await expect(page.getByTestId('access-grants-page')).toBeVisible()
  await expect(page.getByTestId('access-grants-load-error')).toHaveCount(0)
  await expect(page.getByTestId('access-grants-partial-error')).toHaveCount(0)
  await expect(page.getByRole('main').getByRole('heading', { name: /实时授权|Live grants/ })).toBeVisible()
  await expect(page.getByTestId('access-grants-export-json')).toBeVisible()
  await expect(page.getByTestId('access-grants-export-csv')).toBeVisible()
  await expect(page.getByTestId('access-grants-share-link')).toBeVisible()
  // 有授权数据时虚拟列表；无数据时 EmptyState
  if (await page.getByTestId('access-grants-table').count()) {
    await expect(page.getByTestId('access-grants-table')).toBeVisible()
    await expect(page.getByTestId('access-grants-table-header')).toBeVisible()
    await expect(page.getByTestId('access-grants-virtual-list')).toBeVisible()
    await expect(page.getByTestId('access-grants-virtual-hint')).toBeVisible()
  }
  await page.goto('/api-spec')
  await expect(page.getByTestId('api-spec-page').or(page.getByRole('main')).first()).toBeVisible()
  // P137: 任一 namespace 有端点时校验 virtual-list
  if (await page.locator('[data-testid^="api-spec-ep-virtual-list-"]').count()) {
    await expect(page.locator('[data-testid^="api-spec-ep-virtual-list-"]').first()).toBeVisible()
  }
  await expect(page.getByTestId('api-spec-page')).toBeVisible()
  await expect(page.getByTestId('api-spec-load-error')).toHaveCount(0)
  await expect(page.getByTestId('api-spec-refresh-error')).toHaveCount(0)
  await expect(page.getByTestId('api-spec-export-json')).toBeVisible()
  await expect(page.getByTestId('api-spec-export-md')).toBeVisible()
  await expect(page.getByTestId('api-spec-share-link')).toBeVisible()
  // P427: API Spec 响应说明可检索/可见（先清空 ns 过滤，避免默认首个 namespace 漏掉 users/admin）
  const nsSelect = page.getByTestId('api-spec-ns-filter')
  if (await nsSelect.count()) {
    await nsSelect.selectOption('')
  }
  await page.getByTestId('api-spec-search').fill('batchMutationResponse')
  await expect(
    page.locator('[data-testid^="api-spec-ep-response-"]').filter({ hasText: /batchMutationResponse/i }).first(),
  ).toBeVisible({ timeout: 10_000 })
  await page.getByTestId('api-spec-search').fill('okResponse')
  await expect(
    page.locator('[data-testid^="api-spec-ep-response-"]').filter({ hasText: /okResponse/i }).first(),
  ).toBeVisible({ timeout: 10_000 })
  await page.getByTestId('api-spec-search').fill('setPasswordResponse')
  await expect(
    page.locator('[data-testid^="api-spec-ep-row-"]').filter({ hasText: /setPasswordResponse/i }).first(),
  ).toBeVisible({ timeout: 10_000 })
  await page.getByTestId('api-spec-search').fill('')
  await page.goto('/observability/metrics')
  await expect(page.getByTestId('metrics-page')).toBeVisible()
  await expect(page.getByTestId('metrics-load-error')).toHaveCount(0)
  await expect(page.getByTestId('metrics-refresh-error')).toHaveCount(0)
  await expect(page.getByTestId('metrics-share-link')).toBeVisible()
  await expect(page.getByTestId('metrics-filter')).toBeVisible()
  await expect(page.getByTestId('metrics-export-raw')).toBeVisible()
  await expect(page.getByTestId('metrics-auto-refresh')).toBeVisible()
  if (await page.getByTestId('metrics-virtual-list').count()) {
    await expect(page.getByTestId('metrics-virtual-list')).toBeVisible()
    await expect(page.getByTestId('metrics-virtual-hint')).toBeVisible()
  }
  await expect(page.getByRole('main').getByRole('heading', { name: /指标浏览|Metrics explorer/ })).toBeVisible()
  await expect(page.getByRole('main').getByText(/指标族|Families|样本|Samples/i).first()).toBeVisible()

  // 6b) 库/用户/降采样清单导出
  await page.goto('/databases')
  await expect(page.getByTestId('databases-page')).toBeVisible()
  await expect(page.getByTestId('databases-load-error')).toHaveCount(0)
  await expect(page.getByTestId('databases-refresh-error')).toHaveCount(0)
  await expect(page.getByTestId('databases-detail-error')).toHaveCount(0)
  // measurement-level errors only appear after expand; healthy path has none mounted
  await expect(page.locator('[data-testid^="databases-meas-error-"]')).toHaveCount(0)
  await expect(page.getByTestId('databases-export-json')).toBeVisible()
  await expect(page.getByTestId('databases-export-csv')).toBeVisible()
  await expect(page.getByTestId('databases-share-link')).toBeVisible()
  // 导航对登录用户开放（admin 冒烟可见侧栏 + 创建控件）
  await expect(page.getByTestId('sidebar-nav-row-databases')).toBeVisible()
  await expect(page.getByTestId('databases-create-input')).toBeVisible()
  await expect(page.getByTestId('databases-create-btn')).toBeVisible()
  // 库列表虚拟滚动（空库时仍有列表容器与 hint；有数据时 virtual-list 可见）
  const dbEmpty = page.getByTestId('databases-empty-filter')
  const dbList = page.getByTestId('databases-virtual-list')
  if (await dbList.count()) {
    await expect(dbList).toBeVisible()
    await expect(page.getByTestId('databases-virtual-hint')).toBeVisible()
  } else {
    await expect(dbEmpty).toBeVisible()
  }
  await page.goto('/users')
  await expect(page.getByTestId('users-page')).toBeVisible()
  await expect(page.getByTestId('users-export-json')).toBeVisible()
  await expect(page.getByTestId('users-export-csv')).toBeVisible()
  await expect(page.getByTestId('users-table')).toBeVisible()
  await expect(page.getByTestId('users-table-header')).toBeVisible()
  await expect(page.getByTestId('users-virtual-list')).toBeVisible()
  await expect(page.getByTestId('users-virtual-hint')).toBeVisible()
  // 打开首个用户授权面板（admin 默认有 admin 用户）
  const openGrant = page.locator('[data-testid^="users-open-grant-"]').first()
  await expect(openGrant).toBeVisible()
  await openGrant.click()
  await expect(page.getByTestId('user-grant-panel')).toBeVisible()
  await expect(page.getByTestId('user-grant-db-filter')).toBeVisible()
  await expect(page.getByTestId('user-grant-submit')).toBeDisabled()
  await page.getByTestId('user-grant-close').click()
  await expect(page.getByTestId('user-grant-panel')).toHaveCount(0)
  await page.goto('/downsample')
  await expect(page.getByTestId('downsample-page')).toBeVisible()
  await expect(page.getByTestId('downsample-load-error')).toHaveCount(0)
  await expect(page.getByTestId('downsample-policies-error')).toHaveCount(0)
  await expect(page.getByTestId('downsample-statuses-error')).toHaveCount(0)
  await expect(page.getByTestId('downsample-export-json')).toBeVisible()
  await expect(page.getByTestId('downsample-export-csv')).toBeVisible()
  await page.getByTestId('downsample-open-create').click()
  await expect(page.getByTestId('downsample-create-dialog')).toBeVisible()
  await expect(page.getByTestId('downsample-source-db')).toBeVisible()
  await expect(page.getByTestId('downsample-source-retention')).toBeVisible()
  await expect(page.getByTestId('downsample-create-refresh')).toBeVisible()
  await expect(page.getByTestId('downsample-create-lookback')).toBeVisible()
  await expect(page.getByTestId('downsample-create-batch-size')).toBeVisible()
  await expect(page.getByTestId('downsample-create-meta')).toBeVisible()
  await page.keyboard.press('Escape')
  if (await page.getByTestId('downsample-virtual-list').count()) {
    await expect(page.getByTestId('downsample-virtual-list')).toBeVisible()
    await expect(page.getByTestId('downsample-virtual-hint')).toBeVisible()
  }

  // 7) 存储与 data-snapshot 入口
  await page.goto('/storage')
  await expect(page.getByTestId('storage-page')).toBeVisible()
  await expect(page.getByTestId('storage-share-link')).toBeVisible()
  await expect(page.getByTestId('storage-export-fetch')).toBeVisible()
  await expect(page.getByTestId('storage-drill-source')).toBeVisible()
  await expect(page.getByTestId('storage-drill-source-select')).toBeVisible()
  // 快照虚拟列表：有数据时可见
  if (await page.getByTestId('storage-snapshots-virtual-list').count()) {
    await expect(page.getByTestId('storage-snapshots-virtual-list')).toBeVisible()
    await expect(page.getByTestId('storage-snapshots-virtual-hint')).toBeVisible()
  }
  if (await page.getByTestId('storage-data-virtual-list').count()) {
    await expect(page.getByTestId('storage-data-virtual-list')).toBeVisible()
    await expect(page.getByTestId('storage-data-virtual-hint')).toBeVisible()
  }
  await expect(page.getByRole('main').getByRole('heading', { name: /^(存储|Storage)$/ })).toBeVisible()
  await expect(page.getByTestId('storage-data-snapshot')).toBeVisible()

  // Config 表头 i18n（表头在空数据时仍可见；a11y 树中 th 可能暴露为 cell）
  await page.goto('/config')
  await expect(page.getByTestId('config-page')).toBeVisible()
  await expect(page.getByTestId('config-schema-error')).toHaveCount(0)
  await expect(page.getByTestId('config-error-codes-error')).toHaveCount(0)
  await expect(page.getByTestId('config-share-link')).toBeVisible()
  await expect(page.getByTestId('config-export-effective')).toBeVisible()
  await expect(page.getByTestId('config-copy-effective')).toBeVisible()
  await expect(page.getByTestId('config-export-schema')).toBeVisible()
  await expect(page.getByTestId('config-export-error-codes')).toBeVisible()
  if (await page.getByTestId('config-schema-virtual-list').count()) {
    await expect(page.getByTestId('config-schema-virtual-list')).toBeVisible()
    await expect(page.getByTestId('config-schema-virtual-hint')).toBeVisible()
  }
  if (await page.getByTestId('config-error-codes-virtual-list').count()) {
    await expect(page.getByTestId('config-error-codes-virtual-list')).toBeVisible()
    await expect(page.getByTestId('config-error-codes-virtual-hint')).toBeVisible()
  }
  await expect(page.getByTestId('config-error-codes-filter')).toBeVisible()
  await expect(page.getByRole('main').getByRole('heading', { level: 1, name: /^(配置|Config)$/ })).toBeVisible()
  await expect(page.getByTestId('config-schema-table').getByText(/^(名称|Name)$/)).toBeVisible()
  await expect(page.getByTestId('config-error-codes-table').getByText(/^(错误码|Code)$/)).toBeVisible()

  // 8) 就绪中心：勾选持久化 + 导出/归档入口
  await page.goto('/ops/readiness')
  await expect(page.getByRole('main').getByRole('heading', { name: /就绪中心|Commercial readiness|可商用就绪/ })).toBeVisible()
  const firstCheckbox = page.locator('[data-testid^="readiness-prod-"]').first()
  await firstCheckbox.check()
  await expect(firstCheckbox).toBeChecked()
  await page.reload()
  await expect(page.locator('[data-testid^="readiness-prod-"]').first()).toBeChecked()
  await expect(page.getByTestId('readiness-export-preflight')).toBeVisible()
  // P325: 空闲时无 admin busy chip / 刷新钮
  await expect(page.getByTestId('readiness-admin-busy')).toHaveCount(0)
  await expect(page.getByTestId('readiness-admin-busy-refresh')).toHaveCount(0)
  await expect(page.getByTestId('readiness-preflight-summary')).toBeVisible()
  await expect(page.getByTestId('readiness-copy-preflight')).toBeVisible()
  await expect(page.getByTestId('readiness-next-steps')).toBeVisible()
  await expect(page.locator('[data-testid^="next-step-"]').first()).toBeVisible()
  await expect(page.getByTestId('readiness-doctor-panel')).toBeVisible()
  await expect(page.getByTestId('readiness-production-checklist')).toBeVisible()
  await expect(page.getByTestId('readiness-prod-jump-admin-op-visibility')).toBeVisible()
  await expect(page.getByTestId('readiness-prod-jump-user-disable-revokes-tokens')).toBeVisible()
  await page.getByTestId('readiness-prod-jump-user-disable-revokes-tokens').click()
  await expect(page).toHaveURL(/\/users\?status=disabled/)
  await expect(page.getByTestId('users-status-filter')).toHaveValue('disabled')
  await page.goto('/ops/readiness')
  await expect(page.getByTestId('readiness-production-checklist')).toBeVisible()
  await page.getByTestId('readiness-prod-jump-admin-op-visibility').click()
  await expect(page).toHaveURL(/operations/)
  await expect(page.getByTestId('ops-page')).toBeVisible({ timeout: 15_000 })
  await expect(page.getByTestId('ops-status-strip')).toBeVisible()
  await page.goto('/ops/readiness')
  await expect(page.getByTestId('readiness-production-checklist')).toBeVisible()
  if (await page.getByTestId('readiness-doctor-virtual-list').count()) {
    await expect(page.getByTestId('readiness-doctor-virtual-list')).toBeVisible()
    await expect(page.getByTestId('readiness-doctor-virtual-hint')).toBeVisible()
  }
  await expect(page.getByTestId('preflight-item-signoff')).toBeVisible()
  await expect(page.getByTestId('preflight-jump-signoff')).toBeVisible()
  await page.getByTestId('preflight-jump-signoff').click()
  await expect(page.getByTestId('readiness-signoff-notes')).toBeVisible()
  await expect(page.getByTestId('preflight-jump-deployKit')).toBeVisible()
  await page.getByTestId('preflight-jump-deployKit').click()
  await expect(page.getByTestId('readiness-deploy-kit')).toBeVisible()
  await expect(page.getByTestId('deploy-acceptance-boundary')).toBeVisible()
  await expect(page.getByTestId('deploy-runbook-drill')).toBeVisible()
  await expect(page.getByTestId('deploy-drill-download')).toBeVisible()
  await expect(page.getByTestId('deploy-drill-step-edge-cert-present')).toBeVisible()
  await expect(page.getByTestId('deploy-runbook-links')).toBeVisible()
  await expect(page.getByTestId('deploy-accept-step-1')).toBeVisible()
  await expect(page.getByTestId('readiness-export')).toBeVisible()
  await expect(page.getByTestId('readiness-doctor-error')).toHaveCount(0)
  await expect(page.getByTestId('readiness-doctor-refresh-error')).toHaveCount(0)
  await expect(page.getByTestId('readiness-version-error')).toHaveCount(0)
  await expect(page.getByTestId('readiness-share-link')).toBeVisible()
  await expect(page.getByTestId('readiness-archive')).toBeVisible()
  await expect(page.getByTestId('readiness-acceptance-pack')).toBeVisible()
  await expect(page.getByTestId('readiness-deploy-kit')).toBeVisible()
  await expect(page.getByTestId('deploy-kit-local-hints')).toBeVisible()
  await expect(page.getByTestId('deploy-kit-hint-reviewed')).toBeVisible()
  await expect(page.getByTestId('readiness-deploy-kit-download')).toBeVisible()
  await expect(page.getByTestId('readiness-signoff-notes')).toBeVisible()
  await expect(page.getByTestId('signoff-completeness')).toBeVisible()
  await expect(page.getByTestId('signoff-progress')).toBeVisible()
  await expect(page.getByTestId('signoff-progress-bar')).toBeVisible()
  await expect(page.getByTestId('signoff-progress-bar')).toHaveAttribute('aria-valuenow', '0')
  await expect(page.getByTestId('signoff-edge-https')).toBeVisible()
  await expect(page.getByTestId('signoff-backup-offsite')).toBeVisible()
  // P117: 实时保存 + 进度
  await page.getByTestId('signoff-edge-https').fill('e2e edge evidence')
  await expect.poll(async () => page.getByTestId('signoff-field-status-edgeHttps').innerText()).toMatch(/已填|Filled/)
  await expect(page.getByTestId('signoff-progress-bar')).toHaveAttribute('aria-valuenow', '33')
  await expect(page.getByTestId('signoff-jump-backupOffsite').or(page.getByTestId('signoff-jump-backupAlert')).first()).toBeVisible()
  await expect(page.getByTestId('signoff-copy-missing')).toBeVisible()
  await expect(page.getByTestId('signoff-related-edgeHttps')).toBeVisible()
  await expect(page.getByTestId('readiness-signoff-guide-banner')).toBeVisible()
  await expect(page.getByTestId('signoff-guide-edgeHttps')).toBeVisible()
  // 空白备份字段可一键示例
  await page.getByTestId('signoff-guide-summary-backupOffsite').click()
  await page.getByTestId('signoff-guide-fill-backupOffsite').click()
  await expect.poll(async () => (await page.getByTestId('signoff-backup-offsite').inputValue()).length).toBeGreaterThan(20)
  await expect.poll(async () => page.getByTestId('signoff-field-status-backupOffsite').innerText()).toMatch(/已填|Filled/)
  await expect(page.getByTestId('deploy-jump-signoff-nginx-https')).toBeVisible()
  await page.getByTestId('deploy-jump-signoff-nginx-https').click()
  await expect(page.getByTestId('signoff-edge-https')).toBeFocused()
  await expect(page.getByTestId('signoff-open-tpl-nginx-https')).toBeVisible()
  await expect(page.getByTestId('deploy-tpl-rsync-offsite')).toBeVisible()
  await expect(page.getByTestId('deploy-tpl-backup-alert-hook')).toBeVisible()
  await expect(page.getByTestId('deploy-tpl-nginx-https')).toBeVisible()
  await expect(page.getByTestId('deploy-tpl-cron-backup')).toBeVisible()
  await expect(page.getByTestId('deploy-copy-nginx-https')).toBeVisible()

  // 就绪清单数据层双语（生产清单标题）
  await expect(page.getByRole('main').getByText('边缘 HTTPS / TLS').first()).toBeVisible()
  const localeBtnReadiness = page.locator('header button').filter({ has: page.locator('.sr-only', { hasText: /^(zh|en)$/ }) })
  await localeBtnReadiness.click()
  await expect(page.getByRole('main').getByText('Edge HTTPS / TLS').first()).toBeVisible()
  await localeBtnReadiness.click()

  // 9) About 页
  await page.goto('/about')
  await expect(page.getByRole('main').getByText(/关于|About/i).first()).toBeVisible()
  await expect(page.getByRole('main').getByText(/mts-dashboard/i).first()).toBeVisible()

  // 10) 账户页改密入口 + 会话徽章
  await page.goto('/account')
  await expect(page.getByTestId('account-password-form')).toBeVisible()
  await expect(page.getByTestId('account-landing-select')).toBeVisible()
  await expect(page.getByTestId('account-landing-list')).toBeVisible()
  await expect(page.getByTestId('account-landing-filter')).toBeVisible()
  await page.getByTestId('account-landing-filter').fill('___no_match___')
  await expect(page.getByTestId('account-landing-empty')).toBeVisible()
  await page.getByTestId('account-landing-filter').fill('')
  await expect(page.getByTestId('account-landing-list')).toBeVisible()
  await expect(page.getByTestId('account-density-select')).toBeVisible()
  await expect(page.getByTestId('account-prefs-tools')).toBeVisible()
  await expect(page.getByTestId('account-prefs-export')).toBeVisible()
  await expect(page.getByTestId('account-prefs-copy')).toBeVisible()
  await expect(page.getByTestId('account-prefs-reset')).toBeVisible()
  await expect(page.getByTestId('account-prefs-import-submit')).toBeVisible()
  // P98：偏好导出/重置应覆盖侧栏导航排序（本机 localStorage）
  const navKey = 'mts.dashboard.nav-order.prefs.v1'
  const orderBefore = await page.evaluate((k) => localStorage.getItem(k), navKey)
  expect(orderBefore).toBeTruthy()
  await page.getByTestId('account-prefs-reset').click()
  await expect.poll(async () => page.evaluate((k) => localStorage.getItem(k), navKey)).toBeNull()
  await page.getByTestId('account-density-select').selectOption('compact')
  await expect(page.locator('html')).toHaveAttribute('data-density', 'compact')
  await page.getByTestId('account-density-select').selectOption('comfortable')
  await expect(page.locator('html')).not.toHaveAttribute('data-density', 'compact')
  await expect(page.getByTestId('session-badge')).toBeVisible()

  // 11) 命令面板跳转 + 快捷键帮助 + 最近访问
  await expect(page.getByTestId('topbar-shortcuts')).toBeVisible()
  await expect(page.getByTestId('topbar-notify-history')).toBeVisible()
  await page.getByTestId('topbar-shortcuts').click()
  await expect(page.getByTestId('shortcuts-help')).toBeVisible()
  await page.getByTestId('shortcuts-help-close').click()
  await page.keyboard.press('Control+KeyK')
  await expect(page.getByTestId('command-palette')).toBeVisible()
  await page.keyboard.press('Escape')
  await expect(page.getByTestId('command-palette')).toHaveCount(0)
  await expect(page.getByTestId('recent-routes')).toBeVisible()
  // 侧栏导航过滤（展开态）
  await expect(page.getByTestId('sidebar-filter')).toBeVisible()
  await page.getByTestId('sidebar-filter').fill('audit')
  await expect(page.getByTestId('sidebar-nav-audit')).toBeVisible()
  await expect(page.getByTestId('sidebar-nav-home')).toHaveCount(0)
  await page.getByTestId('sidebar-filter-clear').click()
  await expect(page.getByTestId('sidebar-nav-home')).toBeVisible()
  // / 聚焦侧栏过滤（非输入态；先 blur 输入框）
  await page.locator('#main-content').focus()
  await page.keyboard.press('/')
  await expect(page.getByTestId('sidebar-filter')).toBeFocused()
  // 命令面板跳转（在清空最近访问前，避免额外 goto 干扰）
  await page.getByTestId('topbar-command-palette').click()
  await expect(page.getByTestId('command-palette')).toBeVisible()
  await expect(page.getByTestId('command-palette-panel')).toBeVisible()
  await expect(page.getByTestId('command-palette-result-count')).toBeVisible()
  await expect(page.getByTestId('command-palette-listbox')).toHaveAttribute('data-density', /comfortable|compact/)
  await expect(page.getByTestId('command-palette-recent-label')).toBeVisible()
  await page.getByTestId('command-palette-input').fill('audit')
  await expect(page.getByTestId('command-item-audit')).toBeVisible()
  await page.getByTestId('command-palette-input').fill('刷新占用')
  await expect(page.getByTestId('command-item-action-refresh-admin-op-busy')).toBeVisible()
  // P421: 命令面板重置侧栏排序（独立筛选；执行后面板关闭）
  await page.getByTestId('command-palette-input').fill('nav order')
  await expect(page.getByTestId('command-item-action-reset-nav-order')).toBeVisible()
  await page.getByTestId('command-item-action-reset-nav-order').click()
  await expect(page.getByTestId('command-palette')).toHaveCount(0)
  // P426: 命令面板跳转禁用用户筛选
  await page.getByTestId('topbar-command-palette').click()
  await expect(page.getByTestId('command-palette')).toBeVisible()
  await page.getByTestId('command-palette-input').fill('disabled users')
  await expect(page.getByTestId('command-item-users-status-disabled')).toBeVisible()
  await page.getByTestId('command-item-users-status-disabled').click()
  await expect(page).toHaveURL(/status=disabled/)
  await expect(page.getByTestId('users-status-filter')).toHaveValue('disabled')
  // P429: 命令面板启用/active 用户筛选对称入口
  await page.getByTestId('topbar-command-palette').click()
  await expect(page.getByTestId('command-palette')).toBeVisible()
  await page.getByTestId('command-palette-input').fill('enable users')
  await expect(page.getByTestId('command-item-users-status-active')).toBeVisible()
  await page.getByTestId('command-item-users-status-active').click()
  await expect(page).toHaveURL(/status=active/)
  await expect(page.getByTestId('users-status-filter')).toHaveValue('active')
  // P430: 命令面板批量 last 就绪入口
  await page.getByTestId('topbar-command-palette').click()
  await expect(page.getByTestId('command-palette')).toBeVisible()
  await page.getByTestId('command-palette-input').fill('批量 last')
  await expect(page.getByTestId('command-item-readiness-batch-admin-last')).toBeVisible()
  await page.getByTestId('command-item-readiness-batch-admin-last').click()
  await expect(page).toHaveURL(/\/users/)
  await expect(page.getByTestId('users-page')).toBeVisible()
  // P428: 命令面板就绪清单「禁用用户撤销会话」深链
  await page.getByTestId('topbar-command-palette').click()
  await expect(page.getByTestId('command-palette')).toBeVisible()
  await page.getByTestId('command-palette-input').fill('禁用用户撤销会话')
  await expect(page.getByTestId('command-item-readiness-user-disable-revokes-tokens')).toBeVisible()
  await page.getByTestId('command-item-readiness-user-disable-revokes-tokens').click()
  await expect(page).toHaveURL(/status=disabled/)
  await expect(page.getByTestId('users-status-filter')).toHaveValue('disabled')
  await page.getByTestId('topbar-command-palette').click()
  await expect(page.getByTestId('command-palette')).toBeVisible()
  await page.getByTestId('command-palette-input').fill('复制最近一次')
  await expect(page.getByTestId('command-item-action-copy-admin-op-last')).toBeVisible()
  await page.getByTestId('command-item-action-copy-admin-op-last').click()
  await expect(page.getByTestId('command-palette')).toHaveCount(0)
  // 动作执行后面板关闭，后续检索需重新打开
  await page.getByTestId('topbar-command-palette').click()
  await expect(page.getByTestId('command-palette')).toBeVisible()
  await page.getByTestId('command-palette-input').fill('audit')
  await page.getByTestId('command-item-audit').click()
  await expect(page).toHaveURL(/\/audit/)
  await expect(page.getByTestId('audit-quick-ranges')).toBeVisible()
  await expect(page.getByTestId('audit-export-json')).toBeVisible()
  await expect(page.getByTestId('audit-export-csv')).toBeVisible()
  await expect(page.getByTestId('audit-share-link')).toBeVisible()
  await expect(page.getByTestId('audit-limit')).toBeVisible()
  await expect(page.getByTestId('audit-merged-hint')).toBeVisible()
  // P112: Audit 多选/排序入口（空表也可验证控件）
  await expect(page.getByTestId('audit-selection-toolbar')).toBeVisible()
  await expect(page.getByTestId('audit-select-all')).toBeVisible()
  await expect(page.getByTestId('audit-table')).toBeVisible()
  await expect(page.getByTestId('audit-sort-time-col')).toHaveAttribute('aria-sort', /none|ascending|descending/)
  await expect(page.getByTestId('audit-table-header')).toBeVisible()
  // 空结果时空状态；有事件时虚拟列表
  const emptyBody = page.getByTestId('audit-empty-body')
  if (await emptyBody.count()) {
    await expect(emptyBody).toBeVisible()
  } else {
    await expect(page.getByTestId('audit-virtual-list')).toBeVisible()
    await expect(page.getByTestId('audit-virtual-hint')).toBeVisible()
  }
  await expect(page.getByTestId('audit-sort-time')).toBeVisible()
  await page.getByTestId('audit-sort-time').click()
  await expect.poll(async () => page.evaluate(() => localStorage.getItem('mts.dashboard.audit-sort.prefs.v1'))).toBeTruthy()

  // 命令面板运维深链：签核备注 / 部署材料
  await page.getByTestId('topbar-command-palette').click()
  await expect(page.getByTestId('command-palette')).toBeVisible()
  await page.getByTestId('command-palette-input').fill('signoff')
  await page.getByTestId('command-item-readiness-signoff').click()
  await expect(page).toHaveURL(/ops\/readiness/)
  await expect(page.getByTestId('readiness-signoff-notes')).toBeVisible()
  await page.getByTestId('topbar-command-palette').click()
  await page.getByTestId('command-palette-input').fill('deploy kit')
  await page.getByTestId('command-item-readiness-deploy-kit').click()
  await expect(page).toHaveURL(/ops\/readiness/)
  await expect(page.getByTestId('readiness-deploy-kit')).toBeVisible()

  // P99: 运维页深链 flush / action-log
  await page.getByTestId('topbar-command-palette').click()
  await expect(page.getByTestId('command-palette')).toBeVisible()
  await page.getByTestId('command-palette-input').fill('memtable')
  await page.getByTestId('command-item-operations-flush').click()
  await expect(page).toHaveURL(/\/operations(?:#ops-flush)?/)
  await expect(page.getByTestId('ops-flush')).toBeVisible()
  await page.getByTestId('topbar-command-palette').click()
  await page.getByTestId('command-palette-input').fill('ops log')
  await page.getByTestId('command-item-operations-action-log').click()
  await expect(page).toHaveURL(/\/operations/)
  await expect(page.getByTestId('ops-action-log')).toBeVisible()

  // P100: 配置/审计深链
  await page.getByTestId('topbar-command-palette').click()
  await page.getByTestId('command-palette-input').fill('effective config')
  await page.getByTestId('command-item-config-effective').click()
  await expect(page).toHaveURL(/\/config/)
  await expect(page.getByTestId('config-effective-json')).toBeVisible()
  await page.getByTestId('topbar-command-palette').click()
  await page.getByTestId('command-palette-input').fill('audit filters')
  await page.keyboard.press('Escape')
  await page.goto('/audit?range=1h#audit-filters')
  await expect(page.getByTestId('audit-since')).not.toHaveValue('')
  await expect(page.getByTestId('audit-until')).not.toHaveValue('')
  await page.getByTestId('topbar-command-palette').click()
  await page.getByTestId('command-palette-input').fill('audit filters')
  await page.getByTestId('command-item-audit-filters').click()
  await expect(page).toHaveURL(/\/audit/)
  await expect(page.getByTestId('audit-quick-ranges')).toBeVisible()

  // P101: 查询/写入工作台深链
  await page.getByTestId('topbar-command-palette').click()
  await page.getByTestId('command-palette-input').fill('query history')
  await page.getByTestId('command-item-query-history').click()
  await expect(page).toHaveURL(/\/query/)
  await expect(page.getByTestId('query-export-history')).toBeVisible()
  // P120: 查询预填命令（只读，不执行写）
  await page.getByTestId('topbar-command-palette').click()
  await page.getByTestId('command-palette-input').fill('prefill query')
  await expect(page.getByTestId('command-item-query-range-1h')).toBeVisible()
  await page.getByTestId('command-item-query-range-1h').click()
  await expect(page).toHaveURL(/\/query\?range=1h/)
  await expect(page.getByTestId('query-start-time')).not.toHaveValue('')
  await expect(page.getByTestId('query-end-time')).not.toHaveValue('')
  await page.getByTestId('topbar-command-palette').click()
  await page.getByTestId('command-palette-input').fill('typed batch')
  await page.getByTestId('command-item-write-mode-typed').click()
  await expect(page).toHaveURL(/\/write/)
  await expect(page.getByTestId('write-mode-typed')).toBeVisible()
  // typed 模式已选中（深色底）
  await expect(page.getByTestId('write-mode-typed')).toHaveClass(/bg-slate-800/)
  // P419: 写入模式深链 hash 切换（命令面板路径 /write#write-mode-*）
  await page.goto('/write#write-mode-form')
  await expect(page.getByTestId('write-mode-form')).toHaveClass(/bg-slate-800/)
  await page.goto('/write#write-mode-line')
  await expect(page.getByTestId('write-mode-line')).toHaveClass(/bg-slate-800/)
  await page.goto('/write#write-mode-prometheus')
  await expect(page.getByTestId('write-mode-prometheus')).toHaveClass(/bg-slate-800/)
  await page.goto('/write#write-mode-typed')
  await expect(page.getByTestId('write-mode-typed')).toHaveClass(/bg-slate-800/)

  // P104: 命令面板分组（空查询可见导航+动作）
  await page.getByTestId('topbar-command-palette').click()
  await expect(page.getByTestId('command-palette')).toBeVisible()
  await expect(page.getByTestId('command-palette-group-nav')).toBeVisible()
  await expect(page.getByTestId('command-palette-group-action')).toBeVisible()
  await expect(page.getByTestId('command-palette-group-nav-count')).toBeVisible()
  await expect(page.getByTestId('command-palette-group-action-count')).toBeVisible()
  // P108: 空查询默认折叠深链导航
  await expect(page.getByTestId('command-palette-nav-expand')).toBeVisible()
  await expect(page.getByTestId('command-item-query')).toBeVisible()
  await expect(page.getByTestId('command-item-query-history')).toHaveCount(0)
  await page.getByTestId('command-palette-nav-expand').click()
  await expect(page.getByTestId('command-item-query-history')).toBeVisible()
  await page.getByTestId('command-palette-nav-expand').click()
  await expect(page.getByTestId('command-item-query-history')).toHaveCount(0)
  // P105: Home/End 跨组键盘定位
  await page.keyboard.press('End')
  await expect(page.locator('[role="option"][aria-selected="true"]')).toHaveAttribute('data-testid', /command-item-action-/)
  await page.keyboard.press('Home')
  await expect(page.locator('[role="option"][aria-selected="true"]')).toHaveAttribute('data-testid', /command-item-/)
  await page.keyboard.press('Escape')

  // P102: 命令面板页内动作（切换主题）
  const darkBefore = await page.locator('html').evaluate((el) => el.classList.contains('dark'))
  await page.getByTestId('topbar-command-palette').click()
  await expect(page.getByTestId('command-palette')).toBeVisible()
  await page.getByTestId('command-palette-input').fill('theme')
  await expect(page.getByTestId('command-palette-group-action')).toBeVisible()
  await expect(page.getByTestId('command-palette-group-nav')).toHaveCount(0)
  await page.getByTestId('command-item-action-toggle-theme').click()
  await expect(page.getByTestId('command-palette')).toHaveCount(0)
  await expect.poll(async () => page.locator('html').evaluate((el) => el.classList.contains('dark'))).not.toBe(darkBefore)

  // P109: 复制当前页链接动作入口
  await page.getByTestId('topbar-command-palette').click()
  await expect(page.getByTestId('command-palette')).toBeVisible()
  await page.getByTestId('command-palette-input').fill('copy url')
  await expect(page.getByTestId('command-item-action-copy-page-url')).toBeVisible()
  await page.getByTestId('command-item-action-copy-page-url').click()
  await expect(page.getByTestId('command-palette')).toHaveCount(0)
  // focus main / reload 仅校验入口可达，避免 reload 打断后续断言
  await page.getByTestId('topbar-command-palette').click()
  await page.getByTestId('command-palette-input').fill('focus main')
  await expect(page.getByTestId('command-item-action-focus-main')).toBeVisible()
  await page.keyboard.press('Escape')
  await page.getByTestId('topbar-command-palette').click()
  await page.getByTestId('command-palette-input').fill('reload')
  await expect(page.getByTestId('command-item-action-reload-page')).toBeVisible()
  await page.keyboard.press('Escape')

  // P103: 主内容返回顶部（就绪页足够长）
  await page.goto('/ops/readiness')
  await expect(page.getByTestId('readiness-signoff-notes').or(page.getByTestId('readiness-deploy-kit')).first()).toBeVisible()
  await page.evaluate(() => {
    const main = document.getElementById('main-content') as HTMLElement | null
    if (!main) return
    // 强制可滚高度，避免短视口不出现按钮
    main.style.minHeight = '200px'
    const pad = document.createElement('div')
    pad.setAttribute('data-testid', 'e2e-scroll-pad')
    pad.style.height = '1200px'
    main.appendChild(pad)
    main.scrollTop = 900
    main.dispatchEvent(new Event('scroll'))
  })
  await expect(page.getByTestId('back-to-top')).toBeVisible()
  await page.getByTestId('back-to-top').click()
  await expect.poll(async () => page.evaluate(() => document.getElementById('main-content')?.scrollTop ?? -1)).toBe(0)
  await expect(page.getByTestId('back-to-top')).toHaveCount(0)

  // P106: 路由切换自动回顶
  await page.evaluate(() => {
    const main = document.getElementById('main-content') as HTMLElement | null
    if (!main) return
    const pad = document.createElement('div')
    pad.style.height = '1200px'
    main.appendChild(pad)
    main.scrollTop = 700
    main.dispatchEvent(new Event('scroll'))
  })
  await expect.poll(async () => page.evaluate(() => document.getElementById('main-content')?.scrollTop ?? 0)).toBeGreaterThan(100)
  await page.goto('/about')
  await expect(page.getByTestId('about-page')).toBeVisible()
  await expect(page.getByTestId('about-refresh-error')).toHaveCount(0)
  await expect(page.getByTestId('about-share-link')).toBeVisible()
  await expect.poll(async () => page.evaluate(() => document.getElementById('main-content')?.scrollTop ?? -1)).toBe(0)

  // 最近访问清空：多页后 clear，仅剩当前页（>1 才显示清空）
  await page.goto('/write')
  await page.goto('/query')
  await expect(page.getByTestId('recent-routes')).toBeVisible()
  await expect(page.getByTestId('recent-routes-clear')).toBeVisible()
  await page.getByTestId('recent-routes-clear').click()
  await expect(page.getByTestId('recent-routes-clear')).toHaveCount(0)

  // 固定最近访问：直接注入 session 条目，避免 fail-last 深路径干扰
  await page.goto('/query')
  await page.evaluate(() => {
    const now = Date.now()
    sessionStorage.setItem(
      'mts.dashboard.recent-routes.v1',
      JSON.stringify({
        version: 1,
        items: [
          { path: '/query', name: 'Query', at: now },
          { path: '/write', name: 'Write', at: now - 1 },
        ],
      }),
    )
  })
  await page.reload()
  await expect(page.getByTestId('query-page')).toBeVisible()
  await expect(page.getByTestId('recent-routes')).toBeVisible()
  await expect(page.getByTestId('recent-route-pin-/write')).toBeVisible({ timeout: 15_000 })
  await page.getByTestId('recent-route-pin-/write').click()
  await expect(page.getByTestId('recent-route-pin-/write')).toHaveAttribute('aria-pressed', 'true')
  // 命令面板最近访问展示固定
  await page.getByTestId('topbar-command-palette').click()
  await expect(page.getByTestId('command-palette')).toBeVisible()
  await expect(page.getByTestId('command-recent-/write')).toHaveAttribute('data-pinned', 'true')
  await page.keyboard.press('Escape')

  // 通知历史面板 + 导出入口 + 快捷键
  // 通知历史 action：预置一条可跳转运维的 error，再打开面板点击
  await page.evaluate(() => {
    const item = {
      id: 'e2e-admin-busy',
      kind: 'error',
      message: 'e2e admin busy toast',
      count: 1,
      at: Date.now(),
      actionLabel: '打开运维',
      actionPath: '/operations#ops-status-strip',
    }
    sessionStorage.setItem(
      'mts.dashboard.notify-history.v1',
      JSON.stringify({ version: 1, items: [item] }),
    )
  })
  await page.getByTestId('topbar-notify-history').click()
  await expect(page.getByTestId('notify-history-panel')).toBeVisible()
  await expect(page.getByTestId('notify-history-list')).toBeVisible()
  await expect(page.getByTestId('notify-history-action')).toBeVisible()
  await page.getByTestId('notify-history-action').click()
  await expect(page).toHaveURL(/\/operations/)
  await expect(page.getByTestId('ops-status-strip')).toBeVisible()
  // 回到概览后再次打开历史面板做筛选/导出冒烟
  await page.goto('/')
  await page.waitForLoadState('networkidle')
  await page.getByTestId('topbar-notify-history').click()
  await expect(page.getByTestId('notify-history-panel')).toBeVisible()
  await expect(page.getByTestId('notify-history-list')).toBeVisible()
  // 空历史时仅 empty；有条目时 virtual-list 可见
  if (await page.getByTestId('notify-history-virtual-list').count()) {
    await expect(page.getByTestId('notify-history-virtual-list')).toBeVisible()
    await expect(page.getByTestId('notify-history-virtual-hint')).toBeVisible()
  } else {
    await expect(page.getByTestId('notify-history-empty')).toBeVisible()
  }
  await expect(page.getByTestId('notify-history-share-link')).toBeVisible()
  await expect(page.getByTestId('notify-history-export-json')).toBeVisible()
  await expect(page.getByTestId('notify-history-export-csv')).toBeVisible()
  await expect(page.getByTestId('notify-history-copy')).toBeVisible()
  await expect(page.getByTestId('notify-history-filter')).toBeVisible()
  await expect(page.getByTestId('notify-history-search')).toBeVisible()
  await expect(page.getByTestId('notify-history-time-range')).toBeVisible()
  await expect(page.getByTestId('notify-history-since')).toBeVisible()
  await expect(page.getByTestId('notify-history-until')).toBeVisible()
  await page.getByTestId('notify-history-filter').selectOption('error')
  await page.getByTestId('notify-history-search').fill('x')
  await page.getByTestId('notify-history-time-range').selectOption('24h')
  await page.getByTestId('notify-history-time-clear').click()
  await expect(page.getByTestId('notify-history-time-range')).toHaveValue('all')
  await page.getByTestId('notify-history-search').fill('')
  await page.getByTestId('notify-history-filter').selectOption('all')
  await page.getByTestId('notify-history-close').click()
  await expect(page.getByTestId('notify-history-panel')).toHaveCount(0)
  await expect(page.getByTestId('notify-host')).toBeAttached()

  // P193–P194: 通知历史筛选深链自动打开并预填
  await page.goto('/?notify=1&nh_kind=error&nh_q=fail&nh_range=24h#notify-history')
  await expect(page.getByTestId('notify-history-panel')).toBeVisible()
  await expect(page.getByTestId('notify-history-share-link')).toBeVisible()
  await expect(page.getByTestId('notify-history-filter')).toHaveValue('error')
  await expect(page.getByTestId('notify-history-search')).toHaveValue('fail')
  await expect(page.getByTestId('notify-history-time-range')).toHaveValue('24h')
  await page.getByTestId('notify-history-close').click()
  await expect(page.getByTestId('notify-history-panel')).toHaveCount(0)
  // 命令面板跳转错误通知深链
  await page.getByTestId('topbar-command-palette').click()
  await expect(page.getByTestId('command-palette')).toBeVisible()
  await page.getByTestId('command-palette-input').fill('错误通知')
  await expect(page.getByTestId('command-item-notify-history-errors')).toBeVisible()
  await page.getByTestId('command-item-notify-history-errors').click()
  await expect(page.getByTestId('notify-history-panel')).toBeVisible()
  await expect(page.getByTestId('notify-history-filter')).toHaveValue('error')
  await page.getByTestId('notify-history-close').click()

  // P196: 快捷键帮助深链
  await page.goto('/?shortcuts=1#shortcuts-help')
  await expect(page.getByTestId('shortcuts-help')).toBeVisible()
  await page.getByTestId('shortcuts-help-close').click()
  await expect(page.getByTestId('shortcuts-help')).toHaveCount(0)
  await page.getByTestId('topbar-command-palette').click()
  await page.getByTestId('command-palette-input').fill('快捷键帮助')
  await expect(page.getByTestId('command-item-shortcuts-help-deeplink')).toBeVisible()
  await page.getByTestId('command-item-shortcuts-help-deeplink').click()
  await expect(page.getByTestId('shortcuts-help')).toBeVisible()
  await page.getByTestId('shortcuts-help-close').click()

  // P195: 离线时写提交按钮禁用（模拟 navigator.onLine=false）
  await page.goto('/write')
  await expect(page.getByTestId('write-page')).toBeVisible()
  await page.evaluate(() => {
    Object.defineProperty(window.navigator, 'onLine', { configurable: true, get: () => false })
    window.dispatchEvent(new Event('offline'))
  })
  await expect(page.getByTestId('offline-banner')).toBeVisible()
  await expect(page.getByTestId('offline-banner-retry')).toBeVisible()
  await expect(page.getByTestId('write-submit')).toBeDisabled()
  await page.evaluate(() => {
    Object.defineProperty(window.navigator, 'onLine', { configurable: true, get: () => true })
    window.dispatchEvent(new Event('online'))
  })
  await expect(page.getByTestId('offline-banner')).toHaveCount(0)
  await page.locator('#main-content').focus()
  // P197: Users 创建草稿脏标记
  await page.goto('/users')
  await expect(page.getByTestId('users-create-open')).toBeVisible()
  await page.getByTestId('users-create-open').click()
  await expect(page.getByTestId('users-create-password-toggle')).toBeVisible()
  await page.getByTestId('users-create-name').fill('draft-user-e2e')
  // P423: 创建用户弱密码前端拦截
  await page.getByTestId('users-create-password').fill('admin')
  await page.getByTestId('users-create-submit').click()
  await expect(page.getByTestId('users-action-result')).toBeVisible({ timeout: 10_000 })
  await expect(page.getByTestId('users-action-result')).toContainText(/默认|admin|至少|password|8/i)
  await expect(page.getByTestId('users-dirty-badge')).toBeVisible()
  await page.getByTestId('users-create-cancel').click()
  // P422: Users 设密弱密码前端拦截
  const setPwdBtn = page.locator('[data-testid^="users-set-password-"]').first()
  if (await setPwdBtn.count()) {
    await setPwdBtn.click()
    await expect(page.getByTestId('users-set-password-input')).toBeVisible()
    await page.getByTestId('users-set-password-input').fill('admin')
    await page.getByTestId('users-set-password-confirm').fill('admin')
    await page.getByTestId('users-set-password-submit').click()
    await expect(page.getByTestId('users-action-result')).toBeVisible({ timeout: 10_000 })
    await expect(page.getByTestId('users-action-result')).toContainText(/默认|admin|至少|password|8/i)
    // P425: 确认密码不一致前端拦截
    await page.getByTestId('users-set-password-input').fill('StrongPass!2026')
    await page.getByTestId('users-set-password-confirm').fill('StrongPass!2026x')
    await page.getByTestId('users-set-password-submit').click()
    await expect(page.getByTestId('users-action-result')).toBeVisible({ timeout: 10_000 })
    await expect(page.getByTestId('users-action-result')).toContainText(/不一致|does not match|confirm/i)
    await page.getByTestId('users-set-password-cancel').click()
  // P423: Users 自改密弱密码前端拦截
  await page.getByTestId('users-change-self-open').click()
  await expect(page.getByTestId('users-self-old-password')).toBeVisible()
  await page.getByTestId('users-self-old-password').fill(NEW_PASSWORD)
  await page.getByTestId('users-self-new-password').fill('admin')
  await page.getByTestId('users-self-confirm-password').fill('admin')
  await page.getByTestId('users-change-self-submit').click()
  await expect(page.getByTestId('users-action-result')).toBeVisible({ timeout: 10_000 })
  await expect(page.getByTestId('users-action-result')).toContainText(/默认|admin|至少|password|8/i)
  await expect(page).toHaveURL(/\/users/)
  await page.getByTestId('users-change-self-cancel').click()

  }

  // P198/P213: Users 创建入口离线禁用（不再打开弹层）
  await page.evaluate(() => {
    Object.defineProperty(window.navigator, 'onLine', { configurable: true, get: () => false })
    window.dispatchEvent(new Event('offline'))
  })
  await expect(page.getByTestId('offline-banner')).toBeVisible()
  await expect(page.getByTestId('users-create-open')).toBeDisabled()
  await expect(page.getByTestId('users-grant-db-error')).toHaveCount(0)
  await expect(page.getByTestId('users-refresh-error')).toHaveCount(0)
  await expect(page.getByTestId('users-load-error')).toHaveCount(0)
  await expect(page.getByTestId('users-batch-enable')).toBeDisabled()
  await page.evaluate(() => {
    Object.defineProperty(window.navigator, 'onLine', { configurable: true, get: () => true })
    window.dispatchEvent(new Event('online'))
  })

  // P200: Databases 创建草稿脏标记
  await page.goto('/databases')
  await expect(page.getByTestId('databases-create-input')).toBeVisible()
  await page.getByTestId('databases-create-input').fill('draft-db-e2e')
  await expect(page.getByTestId('databases-dirty-badge')).toBeVisible()
  await page.getByTestId('databases-create-input').fill('')
  await expect(page.getByTestId('databases-dirty-badge')).toHaveCount(0)

  // P201: 审计导出进度 banner
  await page.goto('/audit')
  await expect(page.getByTestId('audit-page')).toBeVisible()
  await expect(page.getByTestId('audit-load-error')).toHaveCount(0)
  await expect(page.getByTestId('audit-refresh-error')).toHaveCount(0)
  await expect(page.getByTestId('audit-users-load-error')).toHaveCount(0)
  await page.getByTestId('audit-export-json').click()
  await expect(page.getByTestId('export-job-banner')).toBeVisible({ timeout: 10000 })
  // P230: banner 暴露 status 属性（done/cancelled/error/running）
  await expect(page.getByTestId('export-job-banner')).toHaveAttribute(
    'data-export-status',
    /^(running|done|cancelled|error)$/,
  )
  // 小数据通常瞬间完成，dismiss 完成态
  const dismiss = page.getByTestId('export-job-dismiss')
  if (await dismiss.count()) {
    await dismiss.click()
    await expect(page.getByTestId('export-job-banner')).toHaveCount(0)
  }

  // P202: write-cancel 非 loading 时 disabled
  // P228: query-cancel 空闲 disabled；取消态文案与 info 样式
  await page.goto('/write')
  await expect(page.getByTestId('write-page')).toBeVisible()
  await expect(page.getByTestId('write-cancel')).toBeDisabled()
  await page.goto('/query')
  await expect(page.getByTestId('query-page')).toBeVisible()
  await expect(page.getByTestId('query-cancel')).toBeDisabled()

  // P231: 查询取消 — 延迟 mock 后取消，info 文案 + loading 恢复
  await page.route('**/api/v1/data/query/**', async (route) => {
    // 挂起请求直到 unroute/abort；取消后不再 fulfill，避免 "Route is already handled"
    await new Promise(() => {})
  })
  try {
    const meas = page.getByTestId('query-measurement')
    if (!(await meas.inputValue()).trim()) {
      await meas.fill('cpu')
    }
    await page.getByTestId('query-run').click()
    await expect(page.getByTestId('query-cancel')).toBeEnabled({ timeout: 5_000 })
    await page.getByTestId('query-cancel').click()
    await expect(page.getByTestId('query-action-error')).toBeVisible({ timeout: 10_000 })
    await expect(page.getByTestId('query-action-error')).toContainText(/查询已取消|Query cancelled/i)
    await expect(page.getByTestId('query-action-error')).toHaveClass(/mts-alert-info/)
    await expect(page.getByTestId('query-action-error')).toHaveAttribute('role', 'status')
    await expect(page.getByTestId('query-cancel')).toBeDisabled({ timeout: 5_000 })
    await expect(page.getByTestId('query-run')).toBeEnabled({ timeout: 5_000 })
  } finally {
    await page.unroute('**/api/v1/data/query/**').catch(() => {})
  }

  // P233: 查询超时 — mock 408 timeout 错误码友好文案
  await page.route('**/api/v1/data/query/rows', async (route) => {
    await route.fulfill({
      status: 408,
      contentType: 'application/json',
      body: JSON.stringify({ ok: false, code: 'timeout', message: 'request timeout' }),
    })
  })
  try {
    await page.goto('/query')
    await expect(page.getByTestId('query-page')).toBeVisible()
    const meas = page.getByTestId('query-measurement')
    if (!(await meas.inputValue()).trim()) {
      await meas.fill('cpu')
    }
    await page.getByTestId('query-run').click()
    await expect(page.getByTestId('query-action-error')).toBeVisible({ timeout: 10_000 })
    await expect(page.getByTestId('query-action-error')).toContainText(/超时|timeout/i)
    await expect(page.getByTestId('query-action-error')).toHaveClass(/mts-alert-error/)
    // P235: 错误态 live region
    await expect(page.getByTestId('query-action-error')).toHaveAttribute('role', 'alert')
    await expect(page.getByTestId('query-run')).toBeEnabled({ timeout: 5_000 })
  } finally {
    await page.unroute('**/api/v1/data/query/rows').catch(() => {})
  }

  // P233b: 写入取消 — 挂起 write 后取消，info 文案
  await page.goto('/write')
  await expect(page.getByTestId('write-page')).toBeVisible()
  const hangWrite = async (route: import('@playwright/test').Route) => {
    await new Promise(() => {})
  }
  await page.route('**/api/v1/data/write', hangWrite)
  await page.route('**/api/v1/data/write/**', hangWrite)
  try {
    await page.getByTestId('write-mode-line').click()
    await page.getByRole('main').locator('textarea').first().fill('cpu,host=e2e-cancel usage=1 2000')
    await page.getByTestId('write-submit').click()
    await expect(page.getByTestId('write-cancel')).toBeEnabled({ timeout: 10_000 })
    await page.getByTestId('write-cancel').click()
    await expect(page.getByTestId('write-action-error')).toBeVisible({ timeout: 10_000 })
    await expect(page.getByTestId('write-action-error')).toContainText(/写入已取消|Write cancelled/i)
    await expect(page.getByTestId('write-action-error')).toHaveClass(/mts-alert-info/)
    await expect(page.getByTestId('write-cancel')).toBeDisabled({ timeout: 5_000 })
  } finally {
    await page.unroute('**/api/v1/data/write', hangWrite).catch(() => {})
    await page.unroute('**/api/v1/data/write/**', hangWrite).catch(() => {})
  }

  // P234: 写入超时 mock — 408 timeout 友好文案（error 样式）
  await page.goto('/write')
  await expect(page.getByTestId('write-page')).toBeVisible()
  const fulfillTimeout = async (route: import('@playwright/test').Route) => {
    await route.fulfill({
      status: 408,
      contentType: 'application/json',
      body: JSON.stringify({ ok: false, code: 'timeout', message: 'request timeout' }),
    })
  }
  await page.route('**/api/v1/data/write', fulfillTimeout)
  await page.route('**/api/v1/data/write/**', fulfillTimeout)
  try {
    await page.getByTestId('write-mode-line').click()
    await page.getByRole('main').locator('textarea').first().fill('cpu,host=e2e-timeout usage=2 3000')
    await page.getByTestId('write-submit').click()
    await expect(page.getByTestId('write-action-error')).toBeVisible({ timeout: 10_000 })
    await expect(page.getByTestId('write-action-error')).toContainText(/超时|timeout/i)
    await expect(page.getByTestId('write-action-error')).toHaveClass(/mts-alert-error/)
  } finally {
    await page.unroute('**/api/v1/data/write', fulfillTimeout).catch(() => {})
    await page.unroute('**/api/v1/data/write/**', fulfillTimeout).catch(() => {})
  }

  // P288: 写入取消按钮 idle 可访问性
  await page.goto('/write')
  await expect(page.getByTestId('write-page')).toBeVisible()
  await expect(page.getByTestId('write-cancel')).toBeDisabled()
  const cancelTitle = await page.getByTestId('write-cancel').getAttribute('title')
  if (cancelTitle && !/无进行中|No write|idle|当前无/i.test(cancelTitle)) {
    // title 应提示无进行中写入
    throw new Error(`unexpected write-cancel idle title: ${cancelTitle}`)
  }
  await expect(page.getByTestId('write-submit')).toHaveAttribute('aria-busy', 'false')

  // P420: 写入路径 admin busy 429 友好文案（mock，避免真实写入）
  {
    const fulfillBusy = async (route: import('@playwright/test').Route) => {
      await route.fulfill({
        status: 429,
        contentType: 'application/json',
        body: JSON.stringify({
          ok: false,
          code: 'resource_exhausted',
          message: 'admin heavy operation already in progress: flush',
          admin_op_busy: true,
          op: 'flush',
        }),
      })
    }
    await page.route('**/api/v1/data/write', fulfillBusy)
    await page.route('**/api/v1/data/write/**', fulfillBusy)
    try {
      await page.goto('/write')
      await expect(page.getByTestId('write-page')).toBeVisible()
      await page.getByTestId('write-mode-line').click()
      await page.getByRole('main').locator('textarea').first().fill('cpu,host=e2e-busy usage=1 3000')
      await page.getByTestId('write-submit').click()
      await expect(page.getByTestId('write-action-error')).toBeVisible({ timeout: 10_000 })
      await expect(page.getByTestId('write-action-error')).toContainText(/管理重操作|Admin operation busy|flush/i)
    } finally {
      await page.unroute('**/api/v1/data/write', fulfillBusy).catch(() => {})
      await page.unroute('**/api/v1/data/write/**', fulfillBusy).catch(() => {})
    }
  }


  // P234b: 导出取消 — 慢导出窗口 + export-job-cancel
  await page.goto('/query')
  await expect(page.getByTestId('query-page')).toBeVisible()
  await page.evaluate(() => {
    const items = Array.from({ length: 12 }, (_, i) => ({
      id: `e2e-hist-${i}`,
      at: Date.now() - i * 1000,
      mode: 'rows',
      form: {
        database: 'default',
        measurement: 'cpu',
        retention_policy: 'autogen',
        start_time: '0',
        end_time: '0',
        tags: '',
        fields: '',
        predicates: '',
        aggregates: '',
        window: '',
        offset: '0',
        limit: '100',
      },
      pinned: false,
    }))
    localStorage.setItem('mts_query_history', JSON.stringify(items))
    ;(window as unknown as { __MTS_E2E_SLOW_EXPORT_MS?: number }).__MTS_E2E_SLOW_EXPORT_MS = 1200
  })
  await page.reload()
  await expect(page.getByTestId('query-page')).toBeVisible()
  await page.evaluate(() => {
    ;(window as unknown as { __MTS_E2E_SLOW_EXPORT_MS?: number }).__MTS_E2E_SLOW_EXPORT_MS = 1200
  })
  await page.goto('/query#query-history')
  await expect(page.getByTestId('query-export-history')).toBeVisible({ timeout: 10_000 })
  await page.getByTestId('query-export-history').click()
  await expect(page.getByTestId('export-job-banner')).toBeVisible({ timeout: 5_000 })
  await expect(page.getByTestId('export-job-banner')).toHaveAttribute('data-export-status', 'running')
  await page.getByTestId('export-job-cancel').click()
  await expect(page.getByTestId('export-job-banner')).toHaveAttribute(
    'data-export-status',
    /^(cancelled|done)$/,
    { timeout: 10_000 },
  )
  // 取消成功应为 cancelled；若极端竞态 done 也可接受但优先 cancelled
  const st = await page.getByTestId('export-job-banner').getAttribute('data-export-status')
  if (st === 'cancelled') {
    await expect(page.getByTestId('export-job-title')).toContainText(/取消|cancel/i)
  }
  await page.evaluate(() => {
    delete (window as unknown as { __MTS_E2E_SLOW_EXPORT_MS?: number }).__MTS_E2E_SLOW_EXPORT_MS
  })

  // P287: 导出失败可重试
  await page.evaluate(() => {
    ;(window as unknown as { __MTS_E2E_FAIL_EXPORT?: boolean }).__MTS_E2E_FAIL_EXPORT = true
  })
  await page.getByTestId('query-export-history').click()
  await expect(page.getByTestId('export-job-banner')).toBeVisible({ timeout: 10_000 })
  await expect(page.getByTestId('export-job-banner')).toHaveAttribute('data-export-status', 'error')
  await expect(page.getByTestId('export-job-retry')).toBeVisible()
  await page.evaluate(() => {
    delete (window as unknown as { __MTS_E2E_FAIL_EXPORT?: boolean }).__MTS_E2E_FAIL_EXPORT
  })
  await page.getByTestId('export-job-retry').click()
  await expect(page.getByTestId('export-job-banner')).toHaveAttribute(
    'data-export-status',
    /^(done|running|error)$/,
    { timeout: 10_000 },
  )
  // 清除失败注入后重试应成功（允许短暂 running）
  await expect.poll(async () => page.getByTestId('export-job-banner').getAttribute('data-export-status'), {
    timeout: 10_000,
  }).toMatch(/^(done|cancelled)$/)
  {
    const dismiss = page.getByTestId('export-job-dismiss')
    if (await dismiss.count()) await dismiss.click()
  }

  // P203: Databases 导出进度 banner
  await page.goto('/databases')
  await expect(page.getByTestId('databases-export-json')).toBeVisible()
  // 可能无库数据时 disabled；有数据则点导出
  if (await page.getByTestId('databases-export-json').isEnabled()) {
    await page.getByTestId('databases-export-json').click()
    await expect(page.getByTestId('export-job-banner')).toBeVisible({ timeout: 10000 })
    const dismiss = page.getByTestId('export-job-dismiss')
    if (await dismiss.count()) await dismiss.click()
  }
  // P204: Access matrix 导出进度 banner（有行时）
  await page.goto('/access')
  await expect(page.getByTestId('access-matrix-page')).toBeVisible()
  const matrixExport = page.getByTestId('access-matrix-export')
  await expect(matrixExport).toBeVisible()
  if (await matrixExport.isEnabled()) {
    await matrixExport.click()
    await expect(page.getByTestId('export-job-banner')).toBeVisible({ timeout: 10000 })
    const dismiss = page.getByTestId('export-job-dismiss')
    if (await dismiss.count()) await dismiss.click()
  }
  // P205: Readiness 导出 banner
  await page.goto('/ops/readiness')
  await expect(page.getByTestId('readiness-export')).toBeVisible()
  await page.getByTestId('readiness-export').click()
  await expect(page.getByTestId('export-job-banner')).toBeVisible({ timeout: 10000 })
  const readinessDismiss = page.getByTestId('export-job-dismiss')
  if (await readinessDismiss.count()) await readinessDismiss.click()

  // P206: 账户会话续期表单
  await page.goto('/account#account-session')
  await expect(page.getByTestId('account-session')).toBeVisible()
  await expect(page.getByTestId('account-session-renew-form')).toBeVisible()
  await expect(page.getByTestId('account-session-renew-submit')).toBeDisabled()
  // P423: 会话续期错误密码 — 保持账户页与会话
  await page.getByTestId('account-session-renew-password').fill('definitely-wrong-renew')
  await expect(page.getByTestId('account-session-renew-submit')).toBeEnabled()
  await page.getByTestId('account-session-renew-submit').click()
  await expect(page.getByTestId('account-session-renew-error')).toBeVisible({ timeout: 10_000 })
  await expect(page.getByTestId('account-session-renew-error')).toContainText(/密码|incorrect|credentials|会话/i)
  await expect(page).toHaveURL(/\/account/)
  await expect(page.getByTestId('account-page')).toBeVisible()
  // P422/P423: 会话续期成功路径
  await page.getByTestId('account-session-renew-password').fill(NEW_PASSWORD)
  await expect(page.getByTestId('account-session-renew-submit')).toBeEnabled()
  await page.getByTestId('account-session-renew-submit').click()
  await expect(page.getByTestId('account-session-renew-error')).toHaveCount(0, { timeout: 10_000 })
  await expect(page.getByTestId('account-session-renew-submit')).toBeDisabled({ timeout: 10_000 })


  // P217: Readiness 清单/签核会话脏标记（相对进页基线；改动会自动落 localStorage）
  await page.goto('/ops/readiness')
  await expect(page.getByTestId('readiness-signoff-notes')).toBeVisible()
  await expect(page.getByTestId('readiness-dirty-badge')).toHaveCount(0)
  const prodBox = page.locator('[data-testid^="readiness-prod-"]').first()
  if (await prodBox.count()) {
    const was = await prodBox.isChecked()
    await prodBox.click()
    await expect(page.getByTestId('readiness-dirty-badge')).toBeVisible()
    await prodBox.click()
    if (was) await expect(prodBox).toBeChecked()
    else await expect(prodBox).not.toBeChecked()
    await expect(page.getByTestId('readiness-dirty-badge')).toHaveCount(0)
  }
  const signoffEdge = page.getByTestId('signoff-edge-https')
  const originalSignoff = await signoffEdge.inputValue()
  const mutated = originalSignoff === 'e2e-signoff-edge' ? 'e2e-signoff-edge-2' : 'e2e-signoff-edge'
  await signoffEdge.fill(mutated)
  await expect(page.getByTestId('readiness-dirty-badge')).toBeVisible()
  await signoffEdge.fill(originalSignoff)
  await expect(page.getByTestId('readiness-dirty-badge')).toHaveCount(0)

  // P208: Account 改密脏标记
  await page.goto('/account')
  await expect(page.getByTestId('account-password-form')).toBeVisible()
  await page.getByTestId('account-old-password').fill('draft-old')
  await expect(page.getByTestId('account-password-dirty-badge')).toBeVisible()
  await page.getByTestId('account-old-password').fill('')
  await expect(page.getByTestId('account-password-dirty-badge')).toHaveCount(0)

  // P420: 账户改密旧密码错误不得清会话 / 不得踢回登录
  await page.getByTestId('account-old-password').fill('definitely-wrong-old')
  await page.getByTestId('account-new-password').fill('NewPass-e2e-420')
  await page.getByTestId('account-confirm-password').fill('NewPass-e2e-420')
  await page.getByTestId('account-password-submit').click()
  await expect(page.getByTestId('account-password-error')).toBeVisible({ timeout: 10_000 })
  await expect(page).toHaveURL(/\/account/)
  await expect(page.getByTestId('account-page')).toBeVisible()
  await page.getByTestId('account-old-password').fill('')
  await page.getByTestId('account-new-password').fill('')
  await page.getByTestId('account-confirm-password').fill('')

  // P207: Overview / About 导出入口与 Write 导出
  await page.goto('/')
  await expect(page.getByTestId('overview-export-json')).toBeVisible()
  await page.getByTestId('overview-export-json').click()
  await expect(page.getByTestId('export-job-banner')).toBeVisible({ timeout: 10000 })
  {
    const dismiss = page.getByTestId('export-job-dismiss')
    if (await dismiss.count()) await dismiss.click()
  }
  await page.goto('/about')
  await expect(page.getByTestId('about-export-json')).toBeVisible()
  await page.getByTestId('about-export-json').click()
  await expect(page.getByTestId('export-job-banner')).toBeVisible({ timeout: 10000 })
  {
    const dismiss = page.getByTestId('export-job-dismiss')
    if (await dismiss.count()) await dismiss.click()
  }
  await page.goto('/write')
  await expect(page.getByTestId('write-export-draft')).toBeVisible()
  await page.getByTestId('write-export-draft').click()
  await expect(page.getByTestId('export-job-banner')).toBeVisible({ timeout: 10000 })
  {
    const dismiss = page.getByTestId('export-job-dismiss')
    if (await dismiss.count()) await dismiss.click()
  }

  // P210: Config Token 脏标记
  await page.goto('/config')
  await expect(page.getByTestId('config-token-panel')).toBeVisible()
  await expect(page.getByTestId('config-token-admin-toggle')).toBeVisible()
  await expect(page.getByTestId('config-token-data-toggle')).toBeVisible()
  await page.getByTestId('config-token-admin').fill('draft-token-e2e')
  await expect(page.getByTestId('config-token-dirty-badge')).toBeVisible()
  await page.getByTestId('config-token-clear').click()
  await expect(page.getByTestId('config-token-dirty-badge')).toHaveCount(0)

  // P199: Config offline 时 validate/reload disabled
  await page.goto('/config')
  await expect(page.getByTestId('config-page')).toBeVisible()
  await page.evaluate(() => {
    Object.defineProperty(window.navigator, 'onLine', { configurable: true, get: () => false })
    window.dispatchEvent(new Event('offline'))
  })
  await expect(page.getByTestId('offline-banner')).toBeVisible()
  await expect(page.getByTestId('config-validate')).toBeDisabled()
  await expect(page.getByTestId('config-reload')).toBeDisabled()
  await page.evaluate(() => {
    Object.defineProperty(window.navigator, 'onLine', { configurable: true, get: () => true })
    window.dispatchEvent(new Event('online'))
  })

  // P211: Storage 管理写/导出离线禁用
  await page.goto('/storage')
  await expect(page.getByTestId('storage-export-fetch')).toBeVisible()
  await page.evaluate(() => {
    Object.defineProperty(window.navigator, 'onLine', { configurable: true, get: () => false })
    window.dispatchEvent(new Event('offline'))
  })
  await expect(page.getByTestId('offline-banner')).toBeVisible()
  await expect(page.getByTestId('storage-export-fetch')).toBeDisabled()
  await expect(page.getByTestId('storage-validate')).toBeDisabled()
  await page.evaluate(() => {
    Object.defineProperty(window.navigator, 'onLine', { configurable: true, get: () => true })
    window.dispatchEvent(new Event('online'))
  })

  // P212: Operations 卡片离线禁用
  await page.goto('/operations')
  await expect(page.getByTestId('ops-flush')).toBeVisible()
  await page.evaluate(() => {
    Object.defineProperty(window.navigator, 'onLine', { configurable: true, get: () => false })
    window.dispatchEvent(new Event('offline'))
  })
  await expect(page.getByTestId('offline-banner')).toBeVisible()
  await expect(page.getByTestId('ops-flush')).toBeDisabled()
  await expect(page.getByTestId('ops-compact')).toBeDisabled()
  await expect(page.getByTestId('ops-retention')).toBeDisabled()
  await page.evaluate(() => {
    Object.defineProperty(window.navigator, 'onLine', { configurable: true, get: () => true })
    window.dispatchEvent(new Event('online'))
  })

  // P213: Databases 创建按钮离线禁用
  await page.goto('/databases')
  await expect(page.getByTestId('databases-create-btn')).toBeVisible()
  await page.evaluate(() => {
    Object.defineProperty(window.navigator, 'onLine', { configurable: true, get: () => false })
    window.dispatchEvent(new Event('offline'))
  })
  await expect(page.getByTestId('offline-banner')).toBeVisible()
  await expect(page.getByTestId('databases-create-btn')).toBeDisabled()
  await page.evaluate(() => {
    Object.defineProperty(window.navigator, 'onLine', { configurable: true, get: () => true })
    window.dispatchEvent(new Event('online'))
  })

  // P214: Downsample 创建入口离线禁用
  await page.goto('/downsample')
  await expect(page.getByTestId('downsample-open-create')).toBeVisible()
  await page.evaluate(() => {
    Object.defineProperty(window.navigator, 'onLine', { configurable: true, get: () => false })
    window.dispatchEvent(new Event('offline'))
  })
  await expect(page.getByTestId('offline-banner')).toBeVisible()
  await expect(page.getByTestId('downsample-open-create')).toBeDisabled()
  await page.evaluate(() => {
    Object.defineProperty(window.navigator, 'onLine', { configurable: true, get: () => true })
    window.dispatchEvent(new Event('online'))
  })

  // P215/P216: Storage 删除按钮离线禁用
  await page.goto('/storage')
  await expect(page.getByTestId('storage-export-fetch')).toBeVisible()
  // 若有快照行则校验删除按钮；无则至少校验校验按钮离线禁用
  await page.evaluate(() => {
    Object.defineProperty(window.navigator, 'onLine', { configurable: true, get: () => false })
    window.dispatchEvent(new Event('offline'))
  })
  await expect(page.getByTestId('offline-banner')).toBeVisible()
  await expect(page.getByTestId('storage-validate')).toBeDisabled()
  const delBtn = page.locator('[data-testid^="storage-delete-"]').first()
  if (await delBtn.count()) {
    await expect(delBtn).toBeDisabled()
  }
  await page.evaluate(() => {
    Object.defineProperty(window.navigator, 'onLine', { configurable: true, get: () => true })
    window.dispatchEvent(new Event('online'))
  })

  // P220: 危险确认框在离线时禁用确认并展示阻断提示
  await page.goto('/query')
  await expect(page.getByTestId('query-range-delete')).toBeVisible()
  await page.getByTestId('query-range-delete').click()
  await expect(page.getByTestId('confirm-dialog')).toBeVisible()
  await page.getByTestId('confirm-dialog-input').fill('DELETE')
  await expect(page.getByTestId('confirm-dialog-confirm')).toBeEnabled()
  await page.evaluate(() => {
    Object.defineProperty(window.navigator, 'onLine', { configurable: true, get: () => false })
    window.dispatchEvent(new Event('offline'))
  })
  await expect(page.getByTestId('confirm-dialog-blocked')).toBeVisible()
  await expect(page.getByTestId('confirm-dialog-confirm')).toBeDisabled()
  await page.evaluate(() => {
    Object.defineProperty(window.navigator, 'onLine', { configurable: true, get: () => true })
    window.dispatchEvent(new Event('online'))
  })
  await page.getByTestId('confirm-dialog-cancel').click()
  await expect(page.getByTestId('confirm-dialog')).toHaveCount(0)

  // P286: 会话 warn 横幅（未禁用写）
  await page.evaluate(() => {
    const soon = new Date(Date.now() + 5 * 60_000).toISOString()
    localStorage.setItem('mts_token_expires_at', soon)
  })
  await page.reload()
  await expect(page.getByTestId('session-warn-banner')).toBeVisible({ timeout: 15000 })
  await expect(page.getByTestId('session-warn-remaining')).toBeVisible()
  await expect(page.getByTestId('session-warn-renew')).toBeVisible()
  await expect(page.getByTestId('session-critical-banner')).toHaveCount(0)
  await page.getByTestId('session-warn-renew').click()
  await expect(page).toHaveURL(/\/account/)
  await expect(page.getByTestId('account-session').or(page.getByTestId('account-session-renew-form')).first()).toBeVisible({ timeout: 10000 })

  // P215: 会话 critical 时写操作禁用 + 顶栏 banner
  await page.evaluate(() => {
    const soon = new Date(Date.now() + 30_000).toISOString()
    localStorage.setItem('mts_token_expires_at', soon)
  })
  await page.reload()
  await expect(page.getByTestId('session-critical-banner')).toBeVisible({ timeout: 15000 })
  await expect(page.getByTestId('session-critical-remaining')).toBeVisible()
  await expect(page.getByTestId('session-critical-renew')).toBeVisible()
  await expect(page.getByTestId('session-critical-relogin')).toBeVisible()
  await page.getByTestId('session-critical-renew').click()
  await expect(page).toHaveURL(/\/account/)
  await expect(page.getByTestId('account-session').or(page.getByTestId('account-session-renew-form')).first()).toBeVisible({ timeout: 10000 })
  // 回到 critical 态校验写按钮与 Users title
  await page.evaluate(() => {
    const soon = new Date(Date.now() + 30_000).toISOString()
    localStorage.setItem('mts_token_expires_at', soon)
  })
  await page.goto('/write')
  await expect(page.getByTestId('session-critical-banner')).toBeVisible({ timeout: 15000 })
  await expect(page.getByTestId('session-critical-remaining')).toBeVisible()
  await expect(page.getByTestId('write-submit')).toBeDisabled()
  {
    const writeTitle = await page.getByTestId('write-submit').getAttribute('title')
    if (writeTitle) {
      if (!/会话|Session|session|过期|expired|critical|续期|Renew/i.test(writeTitle)) {
        throw new Error(`unexpected write-submit title under critical: ${writeTitle}`)
      }
      if (/离线|offline/i.test(writeTitle)) {
        throw new Error(`write-submit should not show offline title under session critical: ${writeTitle}`)
      }
    }
  }
  // P218: Users 弹窗入口 title 为会话文案（非离线）
  await page.goto('/users')
  await expect(page.getByTestId('users-create-open')).toBeDisabled()
  const createTitle = await page.getByTestId('users-create-open').getAttribute('title')
  if (createTitle) {
    if (!/会话|Session|session|过期|expired|critical|续期|Renew/i.test(createTitle)) {
      throw new Error(`unexpected users-create-open title under critical: ${createTitle}`)
    }
    if (/离线|offline/i.test(createTitle)) {
      throw new Error(`users-create-open should not show offline title under session critical: ${createTitle}`)
    }
  }
  // P221: 重新登录会先 logout 并进入登录页
  await page.getByTestId('session-critical-relogin').click()
  await expect(page.getByTestId('login-panel')).toBeVisible({ timeout: 15000 })
  // 重新登录以继续后续用例
  await page.getByTestId('login-username').fill('admin')
  await page.getByTestId('login-password').fill(NEW_PASSWORD)
  await page.getByTestId('login-submit').click()
  await expect(page.getByTestId('overview-page').or(page.getByRole('main')).first()).toBeVisible({ timeout: 20000 })
  await page.evaluate(() => {
    const later = new Date(Date.now() + 12 * 3600_000).toISOString()
    localStorage.setItem('mts_token_expires_at', later)
  })

  // P209: 登录离线门禁
  await page.getByTestId('topbar-logout').click()
  await expect(page.getByTestId('login-panel')).toBeVisible()
  await page.evaluate(() => {
    Object.defineProperty(window.navigator, 'onLine', { configurable: true, get: () => false })
    window.dispatchEvent(new Event('offline'))
  })
  await expect(page.getByTestId('login-submit')).toBeDisabled()
  await page.evaluate(() => {
    Object.defineProperty(window.navigator, 'onLine', { configurable: true, get: () => true })
    window.dispatchEvent(new Event('online'))
  })
  // 重新登录以继续后续用例
  await page.getByTestId('login-username').fill('admin')
  await page.getByTestId('login-password').fill(NEW_PASSWORD)
  await page.getByTestId('login-submit').click()
  await expect(page.getByTestId('overview-page').or(page.getByRole('main')).first()).toBeVisible({ timeout: 20000 })

  await page.keyboard.press('Control+Shift+KeyH')
  await expect(page.getByTestId('notify-history-panel')).toBeVisible()
  await page.keyboard.press('Escape')
  await expect(page.getByTestId('notify-history-panel')).toHaveCount(0)

  // 12) 跳过链接 + 运维操作历史导出入口
  await page.goto('/operations')
  await expect(page.getByTestId('ops-action-log')).toBeVisible()
  await expect(page.getByTestId('ops-export-log')).toBeVisible()
  await expect(page.getByTestId('ops-action-filter-bar')).toBeVisible()
  await expect(page.getByTestId('ops-export-stats')).toBeVisible()
  await expect(page.getByTestId('ops-copy-stats')).toBeVisible()
  await expect(page.getByTestId('ops-export-maint-errors')).toBeVisible()
  await expect(page.getByTestId('ops-copy-maint-errors')).toBeVisible()
  await expect(page.getByTestId('ops-maint-errors-panel')).toBeVisible()
  if (await page.getByTestId('ops-maint-errors-virtual-list').count()) {
    await expect(page.getByTestId('ops-maint-errors-virtual-list')).toBeVisible()
    await expect(page.getByTestId('ops-maint-errors-virtual-hint')).toBeVisible()
    await expect(page.getByTestId('ops-maint-errors-filter')).toBeVisible()
  }
  // P113: Retention 需输入口令；清空日志需确认
  await page.getByTestId('ops-retention').click()
  await expect(page.getByTestId('confirm-dialog')).toBeVisible()
  await expect(page.getByTestId('confirm-dialog-input')).toBeVisible()
  await expect(page.getByTestId('confirm-dialog-confirm')).toBeDisabled()
  await page.getByTestId('confirm-dialog-input').fill('RETENTION')
  await expect(page.getByTestId('confirm-dialog-confirm')).toBeEnabled()
  await page.getByTestId('confirm-dialog-cancel').click()
  await expect(page.getByTestId('confirm-dialog')).toHaveCount(0)
  await page.evaluate(() => {
    sessionStorage.setItem('mts.dashboard.ops-actions.v1', JSON.stringify({
      version: 1,
      items: [{ id: 'e2e', kind: 'flush', status: 'ok', message: 'seed', at: Date.now() }],
    }))
  })
  await page.reload()
  await expect(page.getByTestId('ops-action-virtual-list')).toBeVisible()
  await expect(page.getByTestId('ops-action-virtual-hint')).toBeVisible()
  await expect(page.getByTestId('ops-action-filter-bar')).toBeVisible()
  await expect(page.getByTestId('ops-clear-log')).toBeEnabled()
  await page.getByTestId('ops-clear-log').click()
  await expect(page.getByTestId('confirm-dialog')).toBeVisible()
  await page.getByTestId('confirm-dialog-confirm').click()
  await expect(page.getByTestId('ops-clear-log')).toBeDisabled()
  await page.locator('[data-testid="skip-to-main"]').evaluate((el: HTMLElement) => el.focus())
  await expect(page.getByTestId('skip-to-main')).toBeFocused()

  // 13) 用户/数据库筛选入口
  await page.goto('/users')
  await expect(page.getByTestId('users-filter')).toBeVisible()
  await expect(page.getByTestId('users-share-link')).toBeVisible()
  await expect(page.getByTestId('users-role-filter')).toBeVisible()
  await expect(page.getByTestId('users-role-filter')).toContainText(/管理员|Admin|普通用户|User/)
  // P110: 用户多选 / 批量工具条
  await expect(page.getByTestId('users-selection-toolbar')).toBeVisible()
  await expect(page.getByTestId('users-select-all')).toBeVisible()
  await expect(page.getByTestId('users-batch-enable')).toBeDisabled()
  await expect(page.getByTestId('users-batch-disable')).toBeDisabled()
  await page.getByTestId('users-select-all').click()
  await expect(page.getByTestId('users-selected-count')).toBeVisible()
  await expect(page.getByTestId('users-batch-enable')).toBeEnabled()
  await page.getByTestId('users-clear-selection').click()
  await expect(page.getByTestId('users-selected-count')).toHaveCount(0)
  await page.goto('/databases')
  await expect(page.getByTestId('databases-filter')).toBeVisible()
  await expect(page.getByTestId('databases-selection-toolbar')).toBeVisible()
  await expect(page.getByTestId('databases-select-all')).toBeVisible()
  if (await page.getByTestId('databases-virtual-list').count()) {
    await expect(page.getByTestId('databases-virtual-list')).toBeVisible()
    await expect(page.getByTestId('databases-virtual-hint')).toBeVisible()
  }
  await page.getByTestId('databases-select-all').click()
  await expect(page.getByTestId('databases-selected-count')).toBeVisible()
  await page.getByTestId('databases-clear-selection').click()
  // P111: 列排序
  await page.goto('/users')
  await expect(page.getByTestId('users-sort-name')).toBeVisible()
  await page.getByTestId('users-sort-name').click()
  await expect.poll(async () => page.evaluate(() => localStorage.getItem('mts.dashboard.users-sort.prefs.v1'))).toBeTruthy()
  await page.goto('/databases')
  await expect(page.getByTestId('databases-sort-name')).toBeVisible()
  await page.getByTestId('databases-sort-name').click()
  await expect.poll(async () => page.evaluate(() => localStorage.getItem('mts.dashboard.databases-sort.prefs.v1'))).toBeTruthy()
  // 展开第一个库时 measurement 筛选可见
  const expandDb = page.locator('[data-testid^="databases-expand-"]').first()
  if (await expandDb.count()) {
    await expandDb.click()
    if (await page.getByTestId('databases-detail-panel').count()) {
      await expect(page.getByTestId('databases-detail-panel')).toBeVisible()
      await expect(page.getByTestId('databases-open-query')).toBeVisible()
      await expect(page.getByTestId('databases-open-write')).toBeVisible()
      // RP 列表或空态
      const rpList = page.getByTestId('databases-rp-list')
      const rpEmpty = page.getByTestId('databases-rp-empty')
      if (await rpList.count()) {
        await expect(rpList).toBeVisible()
      } else {
        await expect(rpEmpty).toBeVisible()
      }
      if (await page.getByTestId('databases-meas-filter').count()) {
        await expect(page.getByTestId('databases-meas-filter')).toBeVisible()
        await expect(page.getByTestId('databases-meas-count')).toBeVisible()
      }
      const measQuery = page.locator('[data-testid^="databases-meas-query-"]').first()
      if (await measQuery.count()) {
        await measQuery.click()
        await expect(page).toHaveURL(/\/query\?/)
        await expect(page.getByTestId('query-page')).toBeVisible()
        await expect(page.getByTestId('query-database')).not.toHaveValue('')
        // 回到库页继续后续用例
        await page.goto('/databases')
        await expect(page.getByTestId('databases-page')).toBeVisible()
      }
    }
  }

  await expect(page.getByRole('main').getByRole('heading', { name: /数据库|Databases/i })).toBeVisible()

  // P112: Access Grants 多选/排序
  await page.goto('/access/grants')
  await expect(page.getByTestId('access-grants-page')).toBeVisible()
  await expect(page.getByTestId('access-grants-selection-toolbar')).toBeVisible()
  await expect(page.getByTestId('access-grants-select-all')).toBeVisible()
  await expect(page.getByTestId('access-grants-sort-user')).toBeVisible()
  await page.getByTestId('access-grants-sort-user').click()
  await expect.poll(async () => page.evaluate(() => localStorage.getItem('mts.dashboard.access-grants-sort.prefs.v1'))).toBeTruthy()

  // 14) 降采样筛选/批量/状态/区间操作入口
  await page.goto('/downsample')
  await expect(page.getByTestId('downsample-filter-bar')).toBeVisible()
  await expect(page.getByTestId('downsample-filter')).toBeVisible()
  await expect(page.getByTestId('downsample-share-link')).toBeVisible()
  await expect(page.getByTestId('downsample-selection-toolbar')).toBeVisible()
  await expect(page.getByTestId('downsample-select-all')).toBeVisible()
  await expect(page.getByTestId('downsample-clear-select')).toBeVisible()
  await expect(page.getByTestId('downsample-batch-enable')).toBeVisible()
  await expect(page.getByTestId('downsample-status-panel')).toBeVisible()
  // P433: 创建策略并真实批量禁用，核对 downsample-admin-last
  await page.getByTestId('downsample-open-create').click()
  await expect(page.getByTestId('downsample-create-dialog')).toBeVisible()
  await page.getByTestId('downsample-create-name').fill('e2e-batch-ds')
  await page.getByTestId('downsample-create-interval').fill('1m')
  await page.getByTestId('downsample-source-db').fill('default')
  await page.getByTestId('downsample-source-measurement').fill('cpu')
  await page.getByTestId('downsample-target-db').fill('default')
  await page.getByTestId('downsample-target-measurement').fill('cpu_1m_e2e')
  // P434: 显式 retention/refresh/lookback（可商用表单）
  await expect(page.getByTestId('downsample-source-retention')).toHaveValue('autogen')
  await page.getByTestId('downsample-target-retention').fill('autogen')
  await page.getByTestId('downsample-create-refresh').fill('1m')
  await page.getByTestId('downsample-create-lookback').fill('1m')
  await page.getByTestId('downsample-create-batch-size').fill('100')
  await page.getByTestId('downsample-fn-field').fill('usage')
  await page.getByTestId('downsample-create-submit').click()
  await expect(page.getByTestId('downsample-select-e2e-batch-ds')).toBeVisible({ timeout: 20_000 })
  await expect(page.getByTestId('downsample-interval-e2e-batch-ds')).toBeVisible()
  await expect(page.getByTestId('downsample-path-e2e-batch-ds')).toBeVisible()
  // P436: 策略详情面板核对高级字段
  await page.getByTestId('downsample-open-detail-e2e-batch-ds').click()
  await expect(page.getByTestId('downsample-detail-panel')).toBeVisible()
  await expect(page.getByTestId('downsample-detail-name')).toContainText('e2e-batch-ds')
  await expect(page.getByTestId('downsample-detail-field-refresh')).toBeVisible()
  await expect(page.getByTestId('downsample-detail-field-lookback')).toBeVisible()
  await expect(page.getByTestId('downsample-detail-field-batch_size')).toBeVisible()
  // P437: 复制 JSON/链接 + 深链 policy=
  await expect(page.getByTestId('downsample-detail-copy-json')).toBeVisible()
  await expect(page.getByTestId('downsample-detail-copy-link')).toBeVisible()
  // P438: refresh 列 + functions 清单 + Markdown 复制
  await expect(page.getByTestId('downsample-refresh-e2e-batch-ds')).toBeVisible()
  await expect(page.getByTestId('downsample-detail-functions')).toBeVisible()
  await expect(page.getByTestId('downsample-detail-copy-md')).toBeVisible()
  await page.getByTestId('downsample-detail-copy-json').click()
  await expect(page).toHaveURL(/policy=e2e-batch-ds/)
  await page.getByTestId('downsample-detail-close').click()
  await expect(page.getByTestId('downsample-detail-panel')).toHaveCount(0)
  await page.goto('/downsample?policy=e2e-batch-ds#downsample-detail')
  await expect(page.getByTestId('downsample-detail-panel')).toBeVisible({ timeout: 15_000 })
  await expect(page.getByTestId('downsample-detail-name')).toContainText('e2e-batch-ds')
  await expect(page.getByTestId('downsample-detail-functions-list')).toBeVisible()
  await expect(page.getByTestId('downsample-detail-status-extra')).toBeVisible()
  // P440: 状态筛选条 + 状态行点开详情
  await expect(page.getByTestId('downsample-status-next-e2e-batch-ds')).toBeVisible()
  await expect(page.getByTestId('downsample-status-health-filter')).toBeVisible()
  await page.getByTestId('downsample-detail-close').click()
  await expect(page.getByTestId('downsample-detail-panel')).toHaveCount(0)
  await page.getByTestId('downsample-status-row-e2e-batch-ds').click()
  await expect(page.getByTestId('downsample-detail-panel')).toBeVisible()
  await expect(page.getByTestId('downsample-detail-name')).toContainText('e2e-batch-ds')
  await page.getByTestId('downsample-detail-close').click()
  await expect(page.getByTestId('downsample-detail-panel')).toHaveCount(0)
  await page.getByTestId('downsample-select-e2e-batch-ds').check()
  await expect(page.getByTestId('downsample-batch-disable')).toBeEnabled()
  await page.getByTestId('downsample-batch-disable').click()
  await expect(page.getByTestId('confirm-dialog')).toBeVisible()
  await page.getByTestId('confirm-dialog-confirm').click()
  await expect(page.getByTestId('downsample-admin-last')).toBeVisible({ timeout: 15_000 })
  await expect(page.getByTestId('downsample-admin-last-copy')).toBeVisible()
  if (await page.getByTestId('downsample-virtual-list').count()) {
    await expect(page.getByTestId('downsample-virtual-list')).toBeVisible()
  }
  if (await page.getByTestId('downsample-status-virtual-list').count()) {
    await expect(page.getByTestId('downsample-status-virtual-list')).toBeVisible()
    await expect(page.getByTestId('downsample-status-virtual-hint')).toBeVisible()
  }

  // 15) Overview 就绪评分入口
  await page.goto('/')
  await expect(page.getByTestId('overview-readiness-score')).toBeVisible()
  await expect(page.getByTestId('overview-readiness-total')).toBeVisible()
  await expect(page.getByTestId('overview-go-deploy-kit')).toBeVisible()
  await expect(page.getByTestId('overview-signoff-completeness')).toBeVisible()
  await expect(page.getByTestId('overview-go-signoff')).toBeVisible()

  await expect(page.getByTestId('overview-signoff-panel')).toBeVisible()
  await expect(page.getByTestId('overview-signoff-progress-bar')).toBeVisible()
  // 冒烟路径前序已填写 edge 签核，进度应 >=33 且仍有缺失跳转
  await expect.poll(async () => Number(await page.getByTestId('overview-signoff-progress-bar').getAttribute('aria-valuenow'))).toBeGreaterThanOrEqual(33)
  await expect(page.getByTestId('overview-signoff-missing-jumps')).toBeVisible()
  await expect(page.getByTestId('overview-signoff-jump-backupOffsite').or(page.getByTestId('overview-signoff-jump-backupAlert')).first()).toBeVisible()
  await expect(page.getByTestId('overview-export-preflight')).toBeVisible()
  await expect(page.getByTestId('overview-next-steps')).toBeVisible()
  await expect(page.locator('[data-testid^="overview-next-step-"]').first()).toBeVisible()
  await expect(page.getByTestId('overview-go-preflight')).toBeVisible()
  await page.getByTestId('overview-go-preflight').click()
  await expect(page).toHaveURL(/ops\/readiness/)
  await expect(page.getByTestId('readiness-export-preflight')).toBeVisible()
  await page.goto('/')
  await expect(page.getByTestId('overview-go-signoff')).toBeVisible()
  await page.getByTestId('overview-go-signoff').click()
  await expect(page).toHaveURL(/ops\/readiness/)
  await expect(page.getByTestId('readiness-signoff-notes')).toBeVisible()
  await page.goto('/')
  await expect(page.getByTestId('overview-go-deploy-kit')).toBeVisible()
  await page.getByTestId('overview-go-deploy-kit').click()
  await expect(page).toHaveURL(/ops\/readiness/)
  await expect(page.getByTestId('readiness-deploy-kit')).toBeVisible()
  await page.goto('/')
  await page.getByTestId('overview-go-readiness').click()
  await expect(page).toHaveURL(/ops\/readiness/)


  // 键盘可达：命令面板 + 快捷键帮助 + skip-link 聚焦主内容
  await page.goto('/')
  await expect(page.getByTestId('overview-page')).toBeVisible()
  // 先 blur 任何可编辑焦点，再发 Ctrl/Meta+K（部分环境 Control+k 可能被吞）
  await page.locator('body').click({ position: { x: 8, y: 8 } })
  await page.keyboard.press('Control+k')
  if (!(await page.getByTestId('command-palette').isVisible().catch(() => false))) {
    await page.keyboard.press('Meta+k')
  }
  if (!(await page.getByTestId('command-palette').isVisible().catch(() => false))) {
    await page.getByTestId('topbar-command-palette').click()
  }
  await expect(page.getByTestId('command-palette')).toBeVisible()
  await expect(page.getByTestId('command-palette-input')).toBeFocused()
  await page.keyboard.press('Escape')
  await expect(page.getByTestId('command-palette')).toHaveCount(0)
  await page.getByTestId('topbar-shortcuts').click()
  await expect(page.getByTestId('shortcuts-help')).toBeVisible()
  await page.getByTestId('shortcuts-help-close').click()
  await expect(page.getByTestId('shortcuts-help')).toHaveCount(0)
  await page.getByTestId('skip-to-main').focus()
  await page.keyboard.press('Enter')
  await expect(page.locator('#main-content')).toBeFocused()

  // 账户页改密表单可达
  await page.goto('/account')
  await expect(page.getByTestId('account-page')).toBeVisible()
  await expect(page.getByTestId('account-export-json')).toBeVisible()
  await expect(page.getByTestId('account-copy-snapshot')).toBeVisible()
  await expect(page.getByTestId('account-password-form')).toBeVisible()
  await expect(page.getByTestId('password-hints')).toBeVisible()
  await page.getByTestId('account-password-submit').click()
  await expect(page.getByTestId('account-password-error')).toBeVisible()
  await expect(page.getByTestId('account-old-password')).toHaveAttribute('aria-invalid', 'true')

  // About 字段标签
  await page.goto('/about')
  await expect(page.getByTestId('about-page')).toBeVisible()
  await expect(page.getByTestId('about-export-json')).toBeVisible()
  await expect(page.getByTestId('about-copy')).toBeVisible()
  await expect(page.getByRole('main').getByText(/BASE_URL|API/).first()).toBeVisible()

  // 16) 权限矩阵 / 实时授权 / 指标 / 404（含矩阵行双语）
  await page.goto('/access')
  await expect(page.getByTestId('access-matrix-page')).toBeVisible()
  await expect(page.getByTestId('access-matrix-export')).toBeEnabled()
  await expect(page.getByTestId('access-matrix-export-csv')).toBeEnabled()
  await expect(page.getByTestId('access-matrix-search')).toBeVisible()
  await expect(page.getByTestId('access-matrix-selection-toolbar')).toBeVisible()
  await page.getByTestId('access-matrix-select-all').click()
  await expect(page.getByTestId('access-matrix-selected-count')).toBeVisible()
  await page.getByTestId('access-matrix-clear-selection').click()
  await page.getByTestId('access-matrix-sort-area').click()
  await expect.poll(async () => page.evaluate(() => localStorage.getItem('mts.dashboard.access-matrix-sort.prefs.v1'))).toBeTruthy()
  await page.getByTestId('access-matrix-search').fill('query')
  await expect(page.getByTestId('access-matrix-filter-count')).toBeVisible()
  await expect(page.getByRole('main').getByRole('heading', { name: /权限能力矩阵|Capability matrix/ })).toBeVisible()
  // 虚拟列表为 div 行（非 table cell）；限定在列表内，避免命中 select option
  const matrixList = page.getByTestId('access-matrix-virtual-list')
  await expect(matrixList).toBeVisible()
  await expect(matrixList.getByText('数据面', { exact: true }).first()).toBeVisible()
  await expect(matrixList.getByText(/查询 rows/i).first()).toBeVisible()
  // 切换语言后 capability 行与区域标签应为英文
  const localeBtn = page.locator('header button').filter({ has: page.locator('.sr-only', { hasText: /^(zh|en)$/ }) })
  await localeBtn.click()
  await expect(page.getByRole('main').getByRole('heading', { name: /Capability matrix/ })).toBeVisible()
  await expect(matrixList.getByText('Data plane', { exact: true }).first()).toBeVisible()
  await expect(matrixList.getByText(/Query rows\/columns\/stream\/explain/i).first()).toBeVisible()
  await expect(matrixList.getByText(/Non-admin needs read grant/i).first()).toBeVisible()
  // 切回中文，避免后续步骤依赖中文文案
  await localeBtn.click()
  await expect(page.getByRole('main').getByRole('heading', { name: /权限能力矩阵/ })).toBeVisible()
  await page.getByTestId('access-matrix-search').fill('')
  await page.getByTestId('access-matrix-search').fill('元数据')
  await expect(matrixList.getByText(/数据库元数据只读浏览/i).first()).toBeVisible()
  await page.getByTestId('access-matrix-search').fill('')

  await page.goto('/access/grants')
  await expect(page.getByTestId('access-grants-page')).toBeVisible()
  await expect(page.getByRole('main').getByRole('heading', { name: /实时授权|Live grants/ })).toBeVisible()
  await expect(page.getByTestId('access-grants-export-json')).toBeVisible()
  await page.goto('/api-spec')
  await expect(page.getByTestId('api-spec-page')).toBeVisible()
  await expect(page.getByTestId('api-spec-title')).toBeVisible()
  await expect(page.getByTestId('api-spec-export-json')).toBeVisible()
  await page.goto('/observability/metrics')
  await expect(page.getByRole('main').getByRole('heading', { name: /指标浏览|Metrics explorer/ })).toBeVisible()
  await page.goto('/this-route-should-404')
  await expect(page.getByTestId('not-found-page')).toBeVisible()
  await expect(page.getByTestId('not-found-go-overview')).toBeVisible()
  await expect(page.getByText(/页面不存在|Page not found|404/).first()).toBeVisible()


  // P173: 库页深链展开 + 分享按钮
  await page.goto('/databases?database=default#databases-detail')
  await expect(page.getByTestId('databases-page')).toBeVisible()
  // 若 default 存在则详情可见
  if (await page.getByTestId('databases-detail-panel').count()) {
    await expect(page.getByTestId('databases-detail-panel')).toBeVisible()
  }
  await expect(page.getByTestId('databases-share-link')).toBeVisible()


  // P175/P426: 用户筛选深链（角色 + 状态）
  await page.goto('/users?role=user#users-filter-bar')
  await expect(page.getByTestId('users-page')).toBeVisible()
  await expect(page.getByTestId('users-role-filter')).toHaveValue('user')
  await expect(page.getByTestId('users-status-filter')).toBeVisible()
  await expect(page.getByTestId('users-share-link')).toBeVisible()
  await page.goto('/users?status=disabled#users-filter-bar')
  await expect(page.getByTestId('users-status-filter')).toHaveValue('disabled')
  await expect(page.getByTestId('users-filter-bar')).toBeVisible()
  // P176: 能力矩阵筛选深链
  await page.goto('/access?role=admin&q=audit#access-matrix-filter-bar')
  await expect(page.getByTestId('access-matrix-role-filter')).toHaveValue('admin')
  await expect(page.getByTestId('access-matrix-search')).toHaveValue('audit')
  await expect(page.getByTestId('access-matrix-share-link')).toBeVisible()


  // P177: 授权总览筛选深链
  await page.goto('/access/grants?q=read#access-grants-filters')
  await expect(page.getByTestId('access-grants-page')).toBeVisible()
  await expect(page.getByTestId('access-grants-search')).toHaveValue('read')
  await expect(page.getByTestId('access-grants-share-link')).toBeVisible()
  // P178: 降采样筛选深链
  await page.goto('/downsample?enabled=enabled#downsample-filters')
  await expect(page.getByTestId('downsample-page')).toBeVisible()
  await expect(page.getByTestId('downsample-enabled-filter')).toHaveValue('enabled')
  await expect(page.getByTestId('downsample-share-link')).toBeVisible()


  // P179: 运维筛选深链
  await page.goto('/operations?action_kind=flush&action_status=ok#ops-action-filter-bar')
  await expect(page.getByTestId('ops-page')).toBeVisible()
  await expect(page.getByTestId('ops-action-filter-kind')).toHaveValue('flush')
  await expect(page.getByTestId('ops-action-filter-status')).toHaveValue('ok')
  await expect(page.getByTestId('ops-share-link')).toBeVisible()
  // P180: 存储区块深链
  await page.goto('/storage#data-restore')
  await expect(page.getByTestId('storage-page')).toBeVisible()
  await expect(page.getByTestId('storage-share-link')).toBeVisible()


  // P181: 就绪区块分享
  await page.goto('/ops/readiness#deploy-kit')
  await expect(page.getByTestId('readiness-deploy-kit')).toBeVisible()
  await expect(page.getByTestId('readiness-share-link')).toBeVisible()
  // P182: 配置/指标筛选深链
  await page.goto('/config?schema_q=http#config-schema')
  await expect(page.getByTestId('config-page')).toBeVisible()
  await expect(page.getByTestId('config-schema-filter')).toHaveValue('http')
  await expect(page.getByTestId('config-share-link')).toBeVisible()
  await page.goto('/observability/metrics?q=go_#metrics-list')
  await expect(page.getByTestId('metrics-page')).toBeVisible()
  await expect(page.getByTestId('metrics-filter')).toHaveValue('go_')
  await expect(page.getByTestId('metrics-share-link')).toBeVisible()

  // P183–P184: ApiSpec/Account 筛选深链
  await page.goto('/api-spec?q=flush#api-spec-filters')
  await expect(page.getByTestId('api-spec-page')).toBeVisible()
  await expect(page.getByTestId('api-spec-search')).toHaveValue('flush')
  await expect(page.getByTestId('api-spec-share-link')).toBeVisible()
  await page.goto('/account?landing_q=query#account-landing')
  await expect(page.getByTestId('account-page')).toBeVisible()
  await expect(page.getByTestId('account-landing-filter')).toHaveValue('query')
  await expect(page.getByTestId('account-share-link')).toBeVisible()

  // P185–P186: Overview/About 区块分享
  await page.goto('/#overview-health-checks')
  await expect(page.getByTestId('overview-page')).toBeVisible()
  await expect(page.getByTestId('overview-share-link')).toBeVisible()
  await page.goto('/about#about-server')
  await expect(page.getByTestId('about-page')).toBeVisible()
  await expect(page.getByTestId('about-share-link')).toBeVisible()
  await expect(page.getByTestId('about-server')).toBeVisible()

  // P188: NotFound 路径展示与快捷入口
  await page.goto('/this-route-does-not-exist-xyz')
  await expect(page.getByTestId('not-found-page')).toBeVisible()
  await expect(page.getByTestId('not-found-path')).toBeVisible()
  await expect(page.getByTestId('not-found-go-overview')).toBeVisible()
  await expect(page.getByTestId('not-found-go-query')).toBeVisible()
  await page.getByTestId('not-found-go-overview').click()
  await expect(page.getByTestId('overview-page')).toBeVisible()

  // P191: 从业务页登出应携带 redirect
  await page.goto('/write')
  await expect(page.getByTestId('write-page')).toBeVisible()
  await page.getByTestId('topbar-logout').click()
  await expect(page).toHaveURL(/login/)
  await expect(page.getByTestId('login-redirect-hint')).toBeVisible()
  await expect(page.getByTestId('login-redirect-path')).toContainText('/write')
  await page.getByTestId('login-username').fill('admin')
  await page.getByTestId('login-password').fill(NEW_PASSWORD)
  await page.getByTestId('login-submit').click()
  await expect(page).toHaveURL(/\/write(?:\?|$|#)/)
  await expect(page).not.toHaveURL(/login/)

  // P189: 命令面板「复制当前筛选深链」
  await page.goto('/audit?range=1h')
  await expect(page.getByTestId('audit-page')).toBeVisible()
  await expect(page.getByTestId('audit-share-link')).toBeVisible()
  await page.getByTestId('topbar-command-palette').click()
  await expect(page.getByTestId('command-palette')).toBeVisible()
  await page.getByTestId('command-palette-input').fill('复制筛选链接')
  await expect(page.getByTestId('command-item-action-click-share-deep-link')).toBeVisible()
  await page.getByTestId('command-item-action-click-share-deep-link').click()
  await expect(page.getByTestId('command-palette')).toHaveCount(0)

  // 17) 非 admin 端到端：创建 reader、授权 default 读、只读库浏览
  await page.goto('/users')
  await expect(page.getByTestId('users-page')).toBeVisible()
  await page.getByTestId('users-create-open').click()
  await expect(page.locator('[data-modal="create-user"]')).toBeVisible()
  await page.getByTestId('users-create-name').fill('reader-e2e')
  await page.getByTestId('users-create-role').selectOption('user')
  await page.getByTestId('users-create-password').fill('ReaderPass!2026')
  await page.getByTestId('users-create-submit').click()
  await expect(page.getByTestId('users-row-reader-e2e')).toBeVisible({ timeout: 15_000 })
  // P424: 对非当前用户设密成功，admin 会话保持在 /users
  await page.getByTestId('users-set-password-reader-e2e').click()
  await expect(page.getByTestId('users-set-password-input')).toBeVisible()
  await page.getByTestId('users-set-password-input').fill('ReaderPass!2026b')
  await page.getByTestId('users-set-password-confirm').fill('ReaderPass!2026b')
  await page.getByTestId('users-set-password-submit').click()
  await expect(page.getByTestId('users-action-result')).toBeVisible({ timeout: 15_000 })
  await expect(page.getByTestId('users-action-result')).toContainText(/密码已设置|Password set/i)
  await expect(page).toHaveURL(/\/users/)
  await expect(page.getByTestId('users-page')).toBeVisible()
  // P426: 禁用用户后登录失败（安全统一 invalid credentials 友好文案），再启用并继续授权路径
  await page.getByTestId('users-toggle-reader-e2e').click()
  await expect(page.getByTestId('confirm-dialog')).toBeVisible()
  await expect(page.getByTestId('confirm-dialog')).toContainText(/撤销|token|会话|revoke/i)
  await page.getByTestId('confirm-dialog-confirm').click()
  await expect(page.getByTestId('users-action-result')).toBeVisible({ timeout: 15_000 })
  await expect(page.getByTestId('users-action-result')).toContainText(/禁用|disabled|Disabled/i)
  // P431: 单条禁用写入轻量 last，Users 芯片可见
  await expect(page.getByTestId('users-admin-last')).toBeVisible({ timeout: 10_000 })
  await expect(page.getByTestId('users-admin-last-copy')).toBeVisible()
  await page.getByTestId('topbar-logout').click()
  await expect(page).toHaveURL(/login/)
  await page.getByTestId('login-username').fill('reader-e2e')
  await page.getByTestId('login-password').fill('ReaderPass!2026b')
  await page.getByTestId('login-submit').click()
  await expect(page.getByTestId('login-error')).toBeVisible({ timeout: 10_000 })
  await expect(page.getByTestId('login-error')).toContainText(/密码不正确|Incorrect password|用户名或密码/i)
  await login(page, 'admin', NEW_PASSWORD)
  await expect(page).not.toHaveURL(/login|force-change/)
  await page.goto('/users')
  await expect(page.getByTestId('users-row-reader-e2e')).toBeVisible({ timeout: 15_000 })
  await page.getByTestId('users-toggle-reader-e2e').click()
  await expect(page.getByTestId('confirm-dialog')).toBeVisible()
  await page.getByTestId('confirm-dialog-confirm').click()
  await expect(page.getByTestId('users-action-result')).toBeVisible({ timeout: 15_000 })
  await expect(page.getByTestId('users-action-result')).toContainText(/启用|enabled|Enabled|active/i)
  // P432: 单条启用同样写入轻量 last
  await expect(page.getByTestId('users-admin-last')).toBeVisible({ timeout: 10_000 })
  await expect(page.getByTestId('users-admin-last-copy')).toBeVisible()
  // P428: 批量禁用 reader-e2e（真实路径，非 fail-last mock）并再启用
  const clearSel = page.getByTestId('users-clear-selection')
  if ((await clearSel.count()) && (await clearSel.isEnabled())) {
    await clearSel.click()
  }
  // 确保筛选不过滤掉 reader
  await page.getByTestId('users-filter').fill('')
  await page.getByTestId('users-status-filter').selectOption('')
  await page.getByTestId('users-role-filter').selectOption('')
  await expect(page.getByTestId('users-row-reader-e2e')).toBeVisible({ timeout: 10_000 })
  await page.getByTestId('users-select-reader-e2e').check()
  await expect(page.getByTestId('users-batch-disable')).toBeEnabled()
  await page.getByTestId('users-batch-disable').click()
  await expect(page.getByTestId('confirm-dialog')).toBeVisible()
  await expect(page.getByTestId('confirm-dialog')).toContainText(/撤销|token|会话|revoke/i)
  await page.getByTestId('confirm-dialog-confirm').click()
  await expect(page.getByTestId('users-action-result')).toBeVisible({ timeout: 15_000 })
  await expect(page.getByTestId('users-action-result')).toContainText(/禁用|disabled|batch|批量|ok/i)
  // P429: 批量禁用后 Users 页 last 芯片可见/可复制（op 文案不硬绑 batch 专属）
  await expect(page.getByTestId('users-admin-last')).toBeVisible({ timeout: 15_000 })
  await expect(page.getByTestId('users-admin-last-copy')).toBeVisible()
  await page.getByTestId('users-admin-last-copy').click()
  await page.getByTestId('users-status-filter').selectOption('disabled')
  await expect(page.getByTestId('users-row-reader-e2e')).toBeVisible({ timeout: 10_000 })
  await page.getByTestId('users-status-filter').selectOption('')
  await expect(page.getByTestId('users-row-reader-e2e')).toBeVisible({ timeout: 10_000 })
  await page.getByTestId('users-select-reader-e2e').check()
  await expect(page.getByTestId('users-batch-enable')).toBeEnabled()
  await page.getByTestId('users-batch-enable').click()
  await expect(page.getByTestId('confirm-dialog')).toBeVisible()
  await page.getByTestId('confirm-dialog-confirm').click()
  await expect(page.getByTestId('users-action-result')).toBeVisible({ timeout: 15_000 })
  await expect(page.getByTestId('users-action-result')).toContainText(/启用|enabled|batch|批量|ok/i)
  // P430: 批量启用后 last 芯片仍可见（batch_user_enable 或先前 batch_user_disable）
  await expect(page.getByTestId('users-admin-last')).toBeVisible({ timeout: 10_000 })
  await expect(page.getByTestId('users-admin-last-copy')).toBeVisible()
  await page.getByTestId('users-open-grant-reader-e2e').click()
  await expect(page.getByTestId('user-grant-panel')).toBeVisible()
  // 仅匹配库名 checkbox，排除 count/filter 等
  const grantDbBoxes = page.locator('input[type="checkbox"][data-testid^="user-grant-db-"]')
  const grantDefault = page.getByTestId('user-grant-db-default')
  if (await grantDefault.count()) {
    await grantDefault.check()
  } else if (await grantDbBoxes.count()) {
    await grantDbBoxes.first().check()
  }
  if (await grantDbBoxes.count()) {
    await page.getByTestId('user-grant-perm-read').check()
    await expect(page.getByTestId('user-grant-submit')).toBeEnabled()
    await page.getByTestId('user-grant-submit').click()
  }
  await page.getByTestId('topbar-logout').click()
  await expect(page).toHaveURL(/login/)
  await login(page, 'reader-e2e', 'ReaderPass!2026b')
  await expect(page).not.toHaveURL(/login|force-change/)
  // 侧栏可见 databases，不可见 operations
  await expect(page.getByTestId('sidebar-nav-row-databases')).toBeVisible()
  await expect(page.getByTestId('sidebar-nav-row-operations')).toHaveCount(0)
  // 非 admin Overview 工作区入口
  await page.goto('/')
  await expect(page.getByTestId('overview-workspace-panel')).toBeVisible()
  await expect(page.getByTestId('overview-go-query')).toBeVisible()
  await expect(page.getByTestId('overview-go-databases')).toBeVisible()
  await expect(page.getByTestId('overview-go-audit')).toBeVisible()
  await page.getByTestId('overview-go-databases').click()
  await expect(page).toHaveURL(/\/databases/)
  await page.goto('/databases')
  await expect(page.getByTestId('databases-page')).toBeVisible()
  await expect(page.getByTestId('databases-readonly-hint')).toBeVisible()
  await expect(page.getByTestId('databases-create-input')).toHaveCount(0)
  await expect(page.getByTestId('databases-create-btn')).toHaveCount(0)
  // 管理页应权限空态
  await page.goto('/operations')
  await expect(page.getByText(/无权限访问|权限不足|Permission denied|没有权限/i).first()).toBeVisible()
  // 非 admin 自身审计
  await expect(page.getByTestId('sidebar-nav-row-audit')).toBeVisible()
  await page.goto('/audit')
  await expect(page.getByTestId('audit-page')).toBeVisible()
  await expect(page.getByTestId('audit-self-hint')).toBeVisible()
  await expect(page.getByTestId('audit-user')).toBeDisabled()
  await expect(page.getByTestId('audit-reload')).toBeVisible()
  await expect(page.getByTestId('audit-share-link')).toBeVisible()
  // 查询分享链接
  await page.goto('/query')
  await expect(page.getByTestId('query-page')).toBeVisible()
  await expect(page.getByTestId('query-share-link')).toBeVisible()

  // P225: 非 admin 会话 critical — 写禁用 + 横幅续期入口（账户续期仍可用）
  await page.evaluate(() => {
    const soon = new Date(Date.now() + 30_000).toISOString()
    localStorage.setItem('mts_token_expires_at', soon)
  })
  await page.goto('/write')
  await expect(page.getByTestId('write-page')).toBeVisible()
  await expect(page.getByTestId('session-critical-banner')).toBeVisible({ timeout: 15000 })
  await expect(page.getByTestId('session-critical-remaining')).toBeVisible()
  await expect(page.getByTestId('session-critical-renew')).toBeVisible()
  await expect(page.getByTestId('write-submit')).toBeDisabled()
  await page.getByTestId('session-critical-renew').click()
  await expect(page).toHaveURL(/\/account/)
  await expect(page.getByTestId('account-session-renew-form')).toBeVisible({ timeout: 10000 })
  // 恢复 TTL 以免后续 reader 路径被强制登出
  await page.evaluate(() => {
    const later = new Date(Date.now() + 12 * 3600_000).toISOString()
    localStorage.setItem('mts_token_expires_at', later)
  })
  await page.goto('/query')
  await expect(page.getByTestId('session-critical-banner')).toHaveCount(0)
  await expect(page.getByTestId('session-critical-remaining')).toHaveCount(0)

  // P190: 非 admin 深链预填 + 分享 + 登录 redirect 回跳
  await page.goto('/query?database=default&range=1h')
  await expect(page.getByTestId('query-page')).toBeVisible()
  await expect(page.getByTestId('query-share-link')).toBeVisible()
  await page.goto('/write?database=default')
  await expect(page.getByTestId('write-page')).toBeVisible()
  await expect(page.getByTestId('write-share-link')).toBeVisible()
  await page.goto('/databases?q=def')
  await expect(page.getByTestId('databases-page')).toBeVisible()
  await expect(page.getByTestId('databases-share-link')).toBeVisible()
  // 筛选输入若存在则应被预填
  const dbFilter = page.getByTestId('databases-filter')
  if (await dbFilter.count()) {
    await expect(dbFilter).toHaveValue(/def/i)
  }
  await page.goto('/access?q=read')
  await expect(page.getByTestId('access-matrix-page').or(page.getByTestId('access-page')).first()).toBeVisible()
  // 非 admin 也可复制页面筛选深链（命令面板）
  await page.goto('/audit')
  await expect(page.getByTestId('audit-share-link')).toBeVisible()
  await page.getByTestId('topbar-command-palette').click()
  await page.getByTestId('command-palette-input').fill('share deep link')
  await expect(page.getByTestId('command-item-action-click-share-deep-link')).toBeVisible()
  await page.keyboard.press('Escape')
  await expect(page.getByTestId('command-palette')).toHaveCount(0)
  // 登出后访问 query 深链，登录 reader 应回到 query
  await page.getByTestId('topbar-logout').click()
  await expect(page).toHaveURL(/login/)
  await page.goto('/query?database=default')
  await expect(page).toHaveURL(/login/)
  await expect(page.getByTestId('login-redirect-hint')).toBeVisible()
  await expect(page.getByTestId('login-redirect-path')).toContainText('/query')
  await page.getByTestId('login-username').fill('reader-e2e')
  await page.getByTestId('login-password').fill('ReaderPass!2026b')
  await page.getByTestId('login-submit').click()
  await expect(page).toHaveURL(/\/query(?:\?|$)/)
  await expect(page).not.toHaveURL(/login/)
  await expect(page.getByTestId('query-page')).toBeVisible()
})
