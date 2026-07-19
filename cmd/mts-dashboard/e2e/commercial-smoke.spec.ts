import { expect, test, type Page } from '@playwright/test'

const NEW_PASSWORD = 'AdminChanged!2026'

async function login(page: Page, user: string, password: string) {
  await page.goto('/login')
  await page.locator('#username').fill(user)
  await page.locator('#password').fill(password)
  await page.locator('button[type="submit"]').click()
}

test.describe.configure({ mode: 'serial' })

test('commercial browser smoke path', async ({ page }) => {
  // 1) bootstrap 默认密码 -> 强制改密
  await login(page, 'admin', 'admin')
  await expect(page).toHaveURL(/force-change-password/)
  await page.locator('#old').fill('admin')
  await page.locator('#new').fill(NEW_PASSWORD)
  await page.locator('#confirm').fill(NEW_PASSWORD)
  await page.getByRole('button', { name: '修改密码并重新登录' }).click()
  await expect(page).toHaveURL(/login/)
  await expect(page.getByText(/密码已更新|new password/i)).toBeVisible()

  // 2) 新密码登录
  await login(page, 'admin', NEW_PASSWORD)
  await expect(page).not.toHaveURL(/login|force-change/)
  await expect(page.getByText(/概览|健康|Healthy|Ready/i).first()).toBeVisible()
  await expect(page.getByTestId('overview-summary')).toBeVisible()
  await expect(page).toHaveTitle(/仪表盘|概览|Overview/)

  // 3) Line Protocol 写入
  await page.goto('/write')
  await page.getByRole('main').getByRole('button', { name: 'Line Protocol' }).click()
  await page.getByRole('main').locator('textarea').first().fill('cpu,host=e2e-playwright usage=0.42 1000')
  await page.getByRole('main').getByRole('button', { name: '写入', exact: true }).click()
  await expect(page.getByRole('main').getByText(/写入成功/).first()).toBeVisible({ timeout: 20_000 })

  // 4) 查询页可达
  await page.goto('/query')
  await expect(page.getByRole('main').getByText(/查询|Query/i).first()).toBeVisible()

  // 5) 运维 Flush（确认按钮文案为「执行」）
  await page.goto('/operations')
  await expect(page.getByRole('main').getByRole('heading', { name: /^(运维|Operations)$/ })).toBeVisible()
  await page.getByRole('main').getByRole('button', { name: /Flush/ }).first().click()
  await page.getByRole('button', { name: '执行', exact: true }).click()
  await expect(page.getByRole('main').getByText('Flush 已完成').first()).toBeVisible({ timeout: 20_000 })

  // 6) 权限矩阵 / 实时授权 / 指标
  await page.goto('/access')
  await expect(page.getByRole('main').getByRole('heading', { name: /权限能力矩阵|Capability matrix/ })).toBeVisible()
  await page.goto('/access/grants')
  await expect(page.getByRole('main').getByRole('heading', { name: /实时授权|Live grants/ })).toBeVisible()
  await page.goto('/observability/metrics')
  await expect(page.getByRole('main').getByRole('heading', { name: /指标浏览|Metrics explorer/ })).toBeVisible()

  // 7) 存储与 data-snapshot 入口
  await page.goto('/storage')
  await expect(page.getByRole('main').getByRole('heading', { name: /^(存储|Storage)$/ })).toBeVisible()
  await expect(page.getByTestId('storage-data-snapshot')).toBeVisible()

  // 8) 就绪中心：勾选持久化 + 导出/归档入口
  await page.goto('/ops/readiness')
  await expect(page.getByRole('main').getByRole('heading', { name: /就绪中心|Commercial readiness|可商用就绪/ })).toBeVisible()
  const firstCheckbox = page.locator('[data-testid^="readiness-prod-"]').first()
  await firstCheckbox.check()
  await expect(firstCheckbox).toBeChecked()
  await page.reload()
  await expect(page.locator('[data-testid^="readiness-prod-"]').first()).toBeChecked()
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

  // 12) 跳过链接 + 运维操作历史导出入口
  await page.goto('/operations')
  await expect(page.getByTestId('ops-action-log')).toBeVisible()
  await expect(page.getByTestId('ops-export-log')).toBeVisible()
  await page.locator('[data-testid="skip-to-main"]').evaluate((el: HTMLElement) => el.focus())
  await expect(page.getByTestId('skip-to-main')).toBeFocused()

  // 13) 用户/数据库筛选入口
  await page.goto('/users')
  await expect(page.getByTestId('users-filter')).toBeVisible()
  await page.goto('/databases')
  await expect(page.getByTestId('databases-filter')).toBeVisible()

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
  await page.getByTestId('overview-go-deploy-kit').click()
  await expect(page).toHaveURL(/ops\/readiness/)
  await expect(page.getByTestId('readiness-deploy-kit')).toBeVisible()
  await page.goto('/')
  await page.getByTestId('overview-go-readiness').click()
  await expect(page).toHaveURL(/ops\/readiness/)

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
