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

  // 3) Line Protocol 写入
  await page.goto('/write')
  await page.getByRole('main').getByRole('button', { name: 'Line Protocol' }).click()
  await page.getByRole('main').locator('textarea').first().fill('cpu,host=e2e-playwright usage=0.42 1000')
  await page.getByRole('main').getByRole('button', { name: '写入', exact: true }).click()
  await expect(page.getByRole('main').getByText(/写入成功/)).toBeVisible({ timeout: 20_000 })

  // 4) 查询页可达
  await page.goto('/query')
  await expect(page.getByRole('main').getByText(/查询|Query/i).first()).toBeVisible()

  // 5) 运维 Flush（确认按钮文案为「执行」）
  await page.goto('/operations')
  await expect(page.getByRole('main').getByRole('heading', { name: '运维' })).toBeVisible()
  await page.getByRole('main').getByRole('button', { name: /Flush/ }).first().click()
  await page.getByRole('button', { name: '执行', exact: true }).click()
  await expect(page.getByRole('main').getByText('Flush 已完成').first()).toBeVisible({ timeout: 20_000 })

  // 6) 权限矩阵 / 实时授权 / 指标
  await page.goto('/access')
  await expect(page.getByRole('main').getByText('权限能力矩阵')).toBeVisible()
  await page.goto('/access/grants')
  await expect(page.getByRole('main').getByText('实时授权总览')).toBeVisible()
  await page.goto('/observability/metrics')
  await expect(page.getByRole('main').getByText('指标浏览')).toBeVisible()

  // 7) 存储与 data-snapshot 入口
  await page.goto('/storage')
  await expect(page.getByRole('main').getByText(/存储|旁路恢复|备份演练/)).toBeVisible()
  await expect(page.getByTestId('storage-data-snapshot')).toBeVisible()

  // 8) 就绪中心：勾选持久化 + 导出/归档入口
  await page.goto('/ops/readiness')
  await expect(page.getByRole('main').getByText(/就绪中心|Commercial readiness|可商用就绪/)).toBeVisible()
  const firstCheckbox = page.locator('[data-testid^="readiness-prod-"]').first()
  await firstCheckbox.check()
  await expect(firstCheckbox).toBeChecked()
  await page.reload()
  await expect(page.locator('[data-testid^="readiness-prod-"]').first()).toBeChecked()
  await expect(page.getByTestId('readiness-export')).toBeVisible()
  await expect(page.getByTestId('readiness-archive')).toBeVisible()

  // 9) About 页
  await page.goto('/about')
  await expect(page.getByRole('main').getByText(/关于|About/i).first()).toBeVisible()
  await expect(page.getByRole('main').getByText(/mts-dashboard/i).first()).toBeVisible()

  // 10) 账户页改密入口 + 会话徽章
  await page.goto('/account')
  await expect(page.getByTestId('account-password-form')).toBeVisible()
  await expect(page.getByTestId('session-badge')).toBeVisible()

  // 11) 命令面板跳转
  await page.keyboard.press('Control+KeyK')
  await expect(page.getByTestId('command-palette')).toBeVisible()
  await page.getByTestId('command-palette-input').fill('audit')
  await page.getByTestId('command-item-audit').click()
  await expect(page).toHaveURL(/\/audit/)
  await expect(page.getByTestId('audit-quick-ranges')).toBeVisible()
})
