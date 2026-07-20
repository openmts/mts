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
  // 0) 登录表单校验：空密码 -> alert 错误区
  await page.goto('/login')
  await page.getByTestId('login-username').fill('admin')
  await page.getByTestId('login-password').fill('')
  await page.getByTestId('login-submit').click()
  await expect(page.getByTestId('login-error')).toBeVisible()
  await expect(page.getByTestId('login-error')).toHaveAttribute('role', 'alert')
  await expect(page.getByTestId('login-password')).toHaveAttribute('aria-invalid', 'true')
  await expect(page.getByTestId('login-ttl')).toBeVisible()

  // 1) bootstrap 默认密码 -> 强制改密
  await login(page, 'admin', 'admin')
  await expect(page).toHaveURL(/force-change-password/)
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
  await expect(page.getByTestId('overview-summary')).toBeVisible()
  await expect(page.getByTestId('offline-banner')).toHaveCount(0)
  await expect(page.getByTestId('server-unreachable-banner')).toHaveCount(0)
  await expect(page.getByTestId('overview-connectivity')).toBeVisible()
  await expect(page.getByTestId('overview-connectivity-kind')).toBeVisible()
  await expect(page.getByTestId('topbar-account')).toBeVisible()
  await expect(page.getByTestId('topbar-connectivity')).toBeVisible()

  // account session card
  await page.goto('/account')
  await expect(page.getByTestId('account-session')).toBeVisible()
  await expect(page.getByTestId('account-session-remaining')).toBeVisible()
  await page.goto('/')
  await expect(page.getByTestId('skip-to-main')).toHaveCount(1)
  await expect(page).toHaveTitle(/仪表盘|概览|Overview/)

  // 3) Line Protocol 写入
  await page.goto('/write')
  await page.getByRole('main').getByRole('button', { name: 'Line Protocol' }).click()
  await page.getByRole('main').locator('textarea').first().fill('cpu,host=e2e-playwright usage=0.42 1000')
  await page.getByRole('main').getByRole('button', { name: '写入', exact: true }).click()
  await expect(page.getByRole('main').getByText(/写入成功/).first()).toBeVisible({ timeout: 20_000 })
  await expect(page.getByRole('main').getByRole('button', { name: /表单写入|Form write/i })).toBeVisible()
  await expect(page.getByTestId('write-mode-tabs')).toBeVisible()
  await expect(page.getByTestId('write-mode-typed')).toBeVisible()
  await expect(page.getByTestId('write-prefs-hint')).toBeVisible()

  // 4) 查询页可达 + 表单标签 i18n
  await page.goto('/query')
  await expect(page.getByRole('main').getByText(/查询|Query/i).first()).toBeVisible()
  await expect(page.getByRole('main').getByText(/数据库|Database/).first()).toBeVisible()
  await expect(page.getByRole('main').getByText(/开始时间|Start/).first()).toBeVisible()

  // 5) 运维 Flush（确认按钮文案为「执行」）
  await page.goto('/operations')
  await expect(page.getByRole('main').getByRole('heading', { name: /^(运维|Operations)$/ })).toBeVisible()
  await expect(page.getByTestId('ops-flush')).toContainText(/Flush/)
  await expect(page.getByTestId('ops-retention')).toContainText(/保留策略|Retention/i)
  await page.getByRole('main').getByRole('button', { name: /Flush/ }).first().click()
  await page.getByRole('button', { name: '执行', exact: true }).click()
  await expect(page.getByRole('main').getByText('Flush 已完成').first()).toBeVisible({ timeout: 20_000 })

  // 6) 权限矩阵 / 实时授权 / 指标
  await page.goto('/access')
  await expect(page.getByRole('main').getByRole('heading', { name: /权限能力矩阵|Capability matrix/ })).toBeVisible()
  // a11y 树中 th 可能暴露为 cell
  await expect(page.getByRole('main').getByText(/管理员|Admin/).first()).toBeVisible()
  await page.goto('/access/grants')
  await expect(page.getByRole('main').getByRole('heading', { name: /实时授权|Live grants/ })).toBeVisible()
  await page.goto('/observability/metrics')
  await expect(page.getByRole('main').getByRole('heading', { name: /指标浏览|Metrics explorer/ })).toBeVisible()
  await expect(page.getByRole('main').getByText(/指标族|Families|样本|Samples/i).first()).toBeVisible()

  // 7) 存储与 data-snapshot 入口
  await page.goto('/storage')
  await expect(page.getByTestId('storage-drill-source')).toBeVisible()
  await expect(page.getByTestId('storage-drill-source-select')).toBeVisible()
  await expect(page.getByRole('main').getByRole('heading', { name: /^(存储|Storage)$/ })).toBeVisible()
  await expect(page.getByTestId('storage-data-snapshot')).toBeVisible()

  // Config 表头 i18n（表头在空数据时仍可见；a11y 树中 th 可能暴露为 cell）
  await page.goto('/config')
  await expect(page.getByTestId('config-page')).toBeVisible()
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
  await expect(page.getByTestId('readiness-archive')).toBeVisible()
  await expect(page.getByTestId('readiness-acceptance-pack')).toBeVisible()
  await expect(page.getByTestId('readiness-deploy-kit')).toBeVisible()
  await expect(page.getByTestId('deploy-kit-local-hints')).toBeVisible()
  await expect(page.getByTestId('deploy-kit-hint-reviewed')).toBeVisible()
  await expect(page.getByTestId('readiness-deploy-kit-download')).toBeVisible()
  await expect(page.getByTestId('readiness-signoff-notes')).toBeVisible()
  await expect(page.getByTestId('signoff-completeness')).toBeVisible()
  await expect(page.getByTestId('signoff-edge-https')).toBeVisible()
  await expect(page.getByTestId('signoff-backup-offsite')).toBeVisible()
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
  await expect(page.getByTestId('session-badge')).toBeVisible()

  // 11) 命令面板跳转 + 快捷键帮助 + 最近访问
  await expect(page.getByTestId('topbar-shortcuts')).toBeVisible()
  await page.getByTestId('topbar-shortcuts').click()
  await expect(page.getByTestId('shortcuts-help')).toBeVisible()
  await page.getByTestId('shortcuts-help-close').click()
  await expect(page.getByTestId('recent-routes')).toBeVisible()
  await page.keyboard.press('Control+KeyK')
  await expect(page.getByTestId('command-palette')).toBeVisible()
  await page.getByTestId('command-palette-input').fill('audit')
  await page.getByTestId('command-item-audit').click()
  await expect(page).toHaveURL(/\/audit/)
  await expect(page.getByTestId('audit-quick-ranges')).toBeVisible()
  await expect(page.getByTestId('audit-export-json')).toBeVisible()
  await expect(page.getByTestId('audit-export-csv')).toBeVisible()
  await expect(page.getByTestId('audit-limit')).toBeVisible()
  await expect(page.getByTestId('audit-merged-hint')).toBeVisible()

  // 命令面板运维深链：签核备注 / 部署材料
  await page.keyboard.press('Control+KeyK')
  await expect(page.getByTestId('command-palette')).toBeVisible()
  await page.getByTestId('command-palette-input').fill('signoff')
  await page.getByTestId('command-item-readiness-signoff').click()
  await expect(page).toHaveURL(/ops\/readiness/)
  await expect(page.getByTestId('readiness-signoff-notes')).toBeVisible()
  await page.keyboard.press('Control+KeyK')
  await page.getByTestId('command-palette-input').fill('deploy kit')
  await page.getByTestId('command-item-readiness-deploy-kit').click()
  await expect(page).toHaveURL(/ops\/readiness/)
  await expect(page.getByTestId('readiness-deploy-kit')).toBeVisible()

  // 12) 跳过链接 + 运维操作历史导出入口
  await page.goto('/operations')
  await expect(page.getByTestId('ops-action-log')).toBeVisible()
  await expect(page.getByTestId('ops-export-log')).toBeVisible()
  await page.locator('[data-testid="skip-to-main"]').evaluate((el: HTMLElement) => el.focus())
  await expect(page.getByTestId('skip-to-main')).toBeFocused()

  // 13) 用户/数据库筛选入口
  await page.goto('/users')
  await expect(page.getByTestId('users-filter')).toBeVisible()
  await expect(page.getByTestId('users-role-filter')).toBeVisible()
  await expect(page.getByTestId('users-role-filter')).toContainText(/管理员|Admin|普通用户|User/)
  await page.goto('/databases')
  await expect(page.getByTestId('databases-filter')).toBeVisible()
  await expect(page.getByRole('main').getByRole('heading', { name: /数据库|Databases/i })).toBeVisible()

  // 14) 降采样筛选/批量/状态/区间操作入口
  await page.goto('/downsample')
  await expect(page.getByTestId('downsample-filter-bar')).toBeVisible()
  await expect(page.getByTestId('downsample-filter')).toBeVisible()
  await expect(page.getByTestId('downsample-batch-enable')).toBeVisible()
  await expect(page.getByTestId('downsample-status-panel')).toBeVisible()

  // 15) Overview 就绪评分入口
  await page.goto('/')
  await expect(page.getByTestId('overview-readiness-score')).toBeVisible()
  await expect(page.getByTestId('overview-readiness-total')).toBeVisible()
  await expect(page.getByTestId('overview-go-deploy-kit')).toBeVisible()
  await expect(page.getByTestId('overview-signoff-completeness')).toBeVisible()
  await expect(page.getByTestId('overview-go-signoff')).toBeVisible()
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
  await expect(page.getByTestId('account-password-form')).toBeVisible()
  await page.getByTestId('account-password-submit').click()
  await expect(page.getByTestId('account-password-error')).toBeVisible()
  await expect(page.getByTestId('account-old-password')).toHaveAttribute('aria-invalid', 'true')

  // About 字段标签
  await page.goto('/about')
  await expect(page.getByRole('main').getByText(/BASE_URL|API/).first()).toBeVisible()

  // 16) 权限矩阵 / 实时授权 / 指标 / 404（含矩阵行双语）
  await page.goto('/access')
  await expect(page.getByRole('main').getByRole('heading', { name: /权限能力矩阵|Capability matrix/ })).toBeVisible()
  // 使用 table cell，避免命中 select 中隐藏的 option
  await expect(page.getByRole('main').getByRole('cell', { name: '数据面' }).first()).toBeVisible()
  await expect(page.getByRole('main').getByRole('cell', { name: /查询 rows/i }).first()).toBeVisible()
  // 切换语言后 capability 行与区域标签应为英文
  const localeBtn = page.locator('header button').filter({ has: page.locator('.sr-only', { hasText: /^(zh|en)$/ }) })
  await localeBtn.click()
  await expect(page.getByRole('main').getByRole('heading', { name: /Capability matrix/ })).toBeVisible()
  await expect(page.getByRole('main').getByRole('cell', { name: 'Data plane' }).first()).toBeVisible()
  await expect(page.getByRole('main').getByRole('cell', { name: /Query rows\/columns\/stream\/explain/i }).first()).toBeVisible()
  await expect(page.getByRole('main').getByRole('cell', { name: /Non-admin needs read grant/i }).first()).toBeVisible()
  // 切回中文，避免后续步骤依赖中文文案
  await localeBtn.click()
  await expect(page.getByRole('main').getByRole('heading', { name: /权限能力矩阵/ })).toBeVisible()
  await page.goto('/access/grants')
  await expect(page.getByRole('main').getByRole('heading', { name: /实时授权|Live grants/ })).toBeVisible()
  await page.goto('/observability/metrics')
  await expect(page.getByRole('main').getByRole('heading', { name: /指标浏览|Metrics explorer/ })).toBeVisible()
  await page.goto('/this-route-should-404')
  await expect(page.getByText(/页面不存在|Page not found|404/).first()).toBeVisible()
})
