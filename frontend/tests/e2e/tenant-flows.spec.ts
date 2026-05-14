import { expect, test } from '@playwright/test'

test('public user is redirected away from tenant route', async ({ page }) => {
  await page.goto('/tenant/dashboard')
  await expect(page).toHaveURL('/')
})

test('tenant admin can open tenant modules', async ({ page }) => {
  await page.addInitScript(() => {
    window.localStorage.setItem('gateway_console_role', 'tenant_admin')
  })
  await page.goto('/tenant')
  await expect(page.getByRole('heading', { name: 'Tenant Admin' })).toBeVisible()

  await page.getByRole('link', { name: 'Dashboard' }).click()
  await expect(page).toHaveURL('/tenant/dashboard')
  await expect(page.getByRole('heading', { name: 'Tenant Dashboard' })).toBeVisible()

  await page.goto('/tenant/routes')
  await expect(page.getByRole('heading', { name: 'Tenant Routes' })).toBeVisible()
})
