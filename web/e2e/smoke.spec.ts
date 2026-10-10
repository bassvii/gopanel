import { test, expect } from '@playwright/test'

const BASE_PATH = '/c7f75b36c0d98fcf0fda64f6532215dd'  // ← замените на ваш
const ADMIN_USER = 'admin'
const ADMIN_PASS = 'rusins1l0a0v1i1k995yandex.ru'

test.describe('smoke', () => {
  test('открывается форма логина', async ({ page }) => {
    await page.goto(`${BASE_PATH}/`)
    await expect(page.getByText('gopanel')).toBeVisible()
    await expect(page.getByText('Вход в панель')).toBeVisible()
  })

  test('логин и переход на dashboard', async ({ page }) => {
    await page.goto(`${BASE_PATH}/`)
    await page.fill('input[type="text"]', ADMIN_USER)
    await page.fill('input[type="password"]', ADMIN_PASS)
    await page.click('button[type="submit"]')

    // После входа открывается Dashboard — заголовок «Панель».
    await expect(page.getByRole('heading', { name: 'Панель', level: 2 })).toBeVisible({ timeout: 5000 })
  })

  test('переход в Пользователи и обратно', async ({ page }) => {
    await page.goto(`${BASE_PATH}/`)
    await page.fill('input[type="text"]', ADMIN_USER)
    await page.fill('input[type="password"]', ADMIN_PASS)
    await page.click('button[type="submit"]')

    await page.getByRole('button', { name: 'Пользователи' }).click()
    await expect(page.getByRole('heading', { name: 'Пользователи', level: 2 })).toBeVisible()

    await page.getByRole('button', { name: 'Инбаунды' }).click()
    await expect(page.getByRole('heading', { name: 'Инбаунды', level: 2 })).toBeVisible()
  })
})
