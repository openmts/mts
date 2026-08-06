import { expect, test, type Page } from '@playwright/test'

const INITIAL_ADMIN_PASSWORD = 'BootstrapAdmin!2026'
const CHANGED_ADMIN_PASSWORD = 'AdminChanged!2026'
const DASHBOARD_BASE = process.env.MTS_E2E_DASHBOARD_BASE || '/'

type AdminLoginState = 'authenticated' | 'force-change' | 'invalid'

function appPath(path: string): string {
  const base = DASHBOARD_BASE === '/' ? '' : `/${DASHBOARD_BASE.replace(/^\/+|\/+$/g, '')}`
  return `${base}/${path.replace(/^\/+/, '')}`
}

async function submitAdminPassword(page: Page, password: string): Promise<AdminLoginState> {
  const loginError = page.getByTestId('login-error')
  if (await loginError.isVisible()) {
    await page.getByTestId('login-error-dismiss').click()
  }
  await page.getByTestId('login-password').fill(password)
  await page.getByTestId('login-submit').click()

  let state: AdminLoginState | 'pending' = 'pending'
  await expect.poll(async () => {
    const pathname = new URL(page.url()).pathname
    if (pathname.includes('force-change-password')) state = 'force-change'
    else if (!pathname.includes('login')) state = 'authenticated'
    else if (await loginError.isVisible()) state = 'invalid'
    return state
  }).not.toBe('pending')
  return state
}

async function loginAdmin(page: Page) {
  await page.goto(appPath('/login'))
  await page.getByTestId('login-username').fill('admin')
  let state = await submitAdminPassword(page, CHANGED_ADMIN_PASSWORD)
  if (state === 'invalid') {
    state = await submitAdminPassword(page, INITIAL_ADMIN_PASSWORD)
  }
  if (state === 'force-change') {
    await page.getByTestId('force-old').fill(INITIAL_ADMIN_PASSWORD)
    await page.getByTestId('force-new').fill(CHANGED_ADMIN_PASSWORD)
    await page.getByTestId('force-confirm').fill(CHANGED_ADMIN_PASSWORD)
    await page.getByTestId('force-password-submit').click()
    await expect(page).toHaveURL(/login/)
    await page.getByTestId('login-username').fill('admin')
    state = await submitAdminPassword(page, CHANGED_ADMIN_PASSWORD)
  }
  expect(state).toBe('authenticated')
  await expect(page).not.toHaveURL(/login|force-change-password/)
}

test('分享链接保留当前 Dashboard 部署前缀', async ({ page }) => {
  await page.addInitScript(() => {
    Object.defineProperty(navigator, 'clipboard', {
      configurable: true,
      value: {
        writeText(value: string) {
          ;(window as Window & { __MTS_COPIED_TEXT?: string }).__MTS_COPIED_TEXT = value
          return Promise.resolve()
        },
      },
    })
  })

  await loginAdmin(page)

  await page.goto(appPath('/query?database=default'))
  await expect(page.getByTestId('query-page')).toBeVisible()
  await page.getByTestId('query-share-link').click()

  const copied = await page.evaluate(() => {
    return (window as Window & { __MTS_COPIED_TEXT?: string }).__MTS_COPIED_TEXT || ''
  })
  const copiedURL = new URL(copied)
  expect(copiedURL.origin).toBe(new URL(page.url()).origin)
  expect(copiedURL.pathname).toBe(appPath('/query'))
  expect(copiedURL.searchParams.get('database')).toBe('default')
  expect(copiedURL.hash).toBe('#query-form')
})

test('危险操作的可访问名称包含目标用户', async ({ page }) => {
  await loginAdmin(page)

  await page.goto(appPath('/users'))
  await expect(page.getByRole('button', { name: /删除用户 admin|Delete user admin/i })).toBeVisible()
})
