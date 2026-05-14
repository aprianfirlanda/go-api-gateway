import { expect, test } from '@playwright/test'

test('public home shows operational sections', async ({ page }) => {
  await page.goto('/')
  await expect(page.getByRole('heading', { name: 'Gateway Operations Hub' })).toBeVisible()
  await expect(page.getByRole('heading', { name: 'System Status' })).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Operator Onboarding Checklist' })).toBeVisible()
})

test('status page loads', async ({ page }) => {
  await page.goto('/status')
  await expect(page.getByRole('heading', { name: 'Service Status' })).toBeVisible()
  await expect(page.getByText('Run health and readiness checks for gateway and control plane.')).toBeVisible()
})
