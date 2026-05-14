import { expect, test } from '@playwright/test'

test('public user is redirected away from admin route', async ({ page }) => {
  await page.goto('/admin/tenants')
  await expect(page).toHaveURL('/')
})

test('platform admin can open admin modules', async ({ page }) => {
  await page.addInitScript(() => {
    window.localStorage.setItem('gateway_console_role', 'platform_admin')
  })
  await page.goto('/admin')
  await expect(page.getByRole('heading', { name: 'Platform Admin' })).toBeVisible()

  await page.getByRole('link', { name: 'Tenants' }).click()
  await expect(page).toHaveURL('/admin/tenants')
  await expect(page.getByRole('heading', { name: 'Tenants', exact: true })).toBeVisible()

  await page.goto('/admin/audit-logs')
  await expect(page.getByRole('heading', { name: 'Audit Logs' })).toBeVisible()
})
