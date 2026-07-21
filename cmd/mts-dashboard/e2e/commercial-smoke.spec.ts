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

test('commercial browser smoke path', async ({ page }) => {
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

  // P279: 错误密码 -> 服务端失败可 retry/dismiss
  await page.getByTestId('login-password').fill('definitely-wrong-password')
  await page.getByTestId('login-submit').click()
  await expect(page.getByTestId('login-error')).toBeVisible()
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
  await page.getByTestId('force-old').fill('admin')
  await page.getByTestId('force-new').fill(NEW_PASSWORD)
  await page.getByTestId('force-confirm').fill(NEW_PASSWORD)
  await page.getByTestId('force-password-submit').click()
  await expect(page).toHaveURL(/login/)
  await expect(page.getByText(/密码已更新|new password/i)).toBeVisible()

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
  await expect(page.getByTestId('readiness-preflight-summary')).toBeVisible()
  await expect(page.getByTestId('readiness-copy-preflight')).toBeVisible()
  await expect(page.getByTestId('readiness-next-steps')).toBeVisible()
  await expect(page.locator('[data-testid^="next-step-"]').first()).toBeVisible()
  await expect(page.getByTestId('readiness-doctor-panel')).toBeVisible()
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

  // 固定最近访问
  await page.goto('/write')
  await page.goto('/query')
  await expect(page.getByTestId('recent-route-pin-/write')).toBeVisible()
  await page.getByTestId('recent-route-pin-/write').click()
  await expect(page.getByTestId('recent-route-pin-/write')).toHaveAttribute('aria-pressed', 'true')
  // 命令面板最近访问展示固定
  await page.getByTestId('topbar-command-palette').click()
  await expect(page.getByTestId('command-palette')).toBeVisible()
  await expect(page.getByTestId('command-recent-/write')).toHaveAttribute('data-pinned', 'true')
  await page.keyboard.press('Escape')

  // 通知历史面板 + 导出入口 + 快捷键
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
  await expect(page.getByTestId('users-dirty-badge')).toBeVisible()
  await page.getByTestId('users-create-cancel').click()
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
  await page.keyboard.press('Control+k')
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


  // P175: 用户筛选深链
  await page.goto('/users?role=user#users-filter-bar')
  await expect(page.getByTestId('users-page')).toBeVisible()
  await expect(page.getByTestId('users-role-filter')).toHaveValue('user')
  await expect(page.getByTestId('users-share-link')).toBeVisible()
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
  await login(page, 'reader-e2e', 'ReaderPass!2026')
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
  await page.getByTestId('login-password').fill('ReaderPass!2026')
  await page.getByTestId('login-submit').click()
  await expect(page).toHaveURL(/\/query(?:\?|$)/)
  await expect(page).not.toHaveURL(/login/)
  await expect(page.getByTestId('query-page')).toBeVisible()
})
