import { test, expect, Page } from '@playwright/test'
import path from 'node:path'

const SCREENSHOT_DIR = process.env.SCREENSHOT_DIR || ''

async function maybeScreenshot(page: Page, filename: string) {
  if (!SCREENSHOT_DIR) return
  await page.screenshot({
    path: path.join(SCREENSHOT_DIR, filename),
    fullPage: true,
  })
}

async function assertNoHorizontalOverflow(page: Page, screenName: string) {
  const overflow = await page.evaluate(() => {
    return document.documentElement.scrollWidth > document.documentElement.clientWidth
  })
  expect(overflow, `Expected no horizontal overflow on ${screenName}`).toBe(false)
}

test('desktop (1280x800) and mobile (360x640) user flows walkthrough', async ({ browser }) => {
  const desktopContext = await browser.newContext({ viewport: { width: 1280, height: 800 } })
  const mobileContext = await browser.newContext({ viewport: { width: 360, height: 640 } })

  const desktopPage = await desktopContext.newPage()
  const mobilePage = await mobileContext.newPage()

  try {
    // 1. Home Screen on Desktop & 360px Mobile
    await desktopPage.goto('/')
    await mobilePage.goto('/')

    await expect(desktopPage.getByLabel('Callsign / Display Name')).toBeVisible()
    await expect(mobilePage.getByLabel('Callsign / Display Name')).toBeVisible()
    await assertNoHorizontalOverflow(mobilePage, 'Mobile Home Screen')
    await maybeScreenshot(mobilePage, '01-mobile-home.png')

    // Login on both viewports
    await desktopPage.getByLabel('Callsign / Display Name').fill('DesktopHost')
    await desktopPage.getByRole('button', { name: 'Play Battleship' }).click()

    await mobilePage.getByLabel('Callsign / Display Name').fill('MobileGuest')
    await mobilePage.getByRole('button', { name: 'Play Battleship' }).click()

    await expect(desktopPage.getByText('Public Battle Lobby')).toBeVisible()
    await expect(mobilePage.getByText('Public Battle Lobby')).toBeVisible()
    await assertNoHorizontalOverflow(mobilePage, 'Mobile Lobby Screen')
    await maybeScreenshot(desktopPage, '02-desktop-lobby.png')

    // 2. Refresh browser on Mobile to verify localStorage session restoration
    await mobilePage.reload()
    await expect(mobilePage.getByText('Public Battle Lobby')).toBeVisible()
    await expect(mobilePage.getByText('MobileGuest')).toBeVisible()

    // 3. Host creates room & cancels room
    await desktopPage.getByRole('button', { name: 'Create Game' }).click()
    await expect(desktopPage.getByText('Waiting for an opponent')).toBeVisible()
    await maybeScreenshot(desktopPage, '03-desktop-waiting-room.png')

    await desktopPage.getByRole('button', { name: /Cancel Game & Return to Lobby/i }).click()
    await expect(desktopPage.getByText('Public Battle Lobby')).toBeVisible()

    // 4. Host creates room again; Mobile guest sees it live and joins
    await desktopPage.getByRole('button', { name: 'Create Game' }).click()
    await expect(desktopPage.getByText('Waiting for an opponent')).toBeVisible()

    const mobileJoinBtn = mobilePage.getByRole('button', { name: 'Join' }).first()
    await expect(mobileJoinBtn).toBeVisible()
    await assertNoHorizontalOverflow(mobilePage, 'Mobile Lobby with Room')
    await maybeScreenshot(mobilePage, '04-mobile-lobby-with-room.png')
    await mobileJoinBtn.click()

    // 5. Ready-Up: test Ready/Un-ready toggle and Guest Leave returning Host to Waiting Room
    await expect(desktopPage.getByText('Pre-Game Readiness Check')).toBeVisible()
    await expect(mobilePage.getByText('Pre-Game Readiness Check')).toBeVisible()
    await assertNoHorizontalOverflow(mobilePage, 'Mobile Ready-Up Screen')
    await maybeScreenshot(mobilePage, '05-mobile-ready-up.png')

    await mobilePage.getByRole('button', { name: 'Ready for Battle' }).click()
    await expect(mobilePage.getByRole('button', { name: 'Cancel Ready' })).toBeVisible()
    await mobilePage.getByRole('button', { name: 'Cancel Ready' }).click()
    await expect(mobilePage.getByRole('button', { name: 'Ready for Battle' })).toBeVisible()

    // Guest leaves Ready-Up -> Host returns to Waiting Room, Guest returns to Lobby
    await mobilePage.getByRole('button', { name: 'Leave Room' }).click()
    await expect(desktopPage.getByText('Waiting for an opponent')).toBeVisible()
    await expect(mobilePage.getByText('Public Battle Lobby')).toBeVisible()

    // Guest re-joins and both ready up
    await mobilePage.getByRole('button', { name: 'Join' }).first().click()
    await expect(mobilePage.getByText('Pre-Game Readiness Check')).toBeVisible()
    await desktopPage.getByRole('button', { name: 'Ready for Battle' }).click()
    await mobilePage.getByRole('button', { name: 'Ready for Battle' }).click()

    // 6. Placement Screen on Desktop (keyboard/randomize) & Mobile 360px (tap-to-place)
    await expect(desktopPage.getByText('Tactical Deployment Phase')).toBeVisible()
    await expect(mobilePage.getByText('Tactical Deployment Phase')).toBeVisible()
    await assertNoHorizontalOverflow(mobilePage, 'Mobile Placement Screen')

    // Mobile tap-to-select Carrier, tap Rotate, tap A1, then Randomize all 5
    await mobilePage.getByText('Carrier', { exact: true }).click()
    await mobilePage.getByRole('button', { name: /Rotate/i }).click()
    await mobilePage.getByRole('button', { name: 'Cell A1', exact: true }).click()
    await mobilePage.getByRole('button', { name: 'Randomize' }).click()
    await maybeScreenshot(mobilePage, '06-mobile-placement.png')

    await desktopPage.getByRole('button', { name: 'Randomize' }).click()
    await maybeScreenshot(desktopPage, '07-desktop-placement.png')

    await desktopPage.getByRole('button', { name: 'Confirm Fleet' }).click()
    await mobilePage.getByRole('button', { name: 'Confirm Fleet' }).click()

    // 7. Battle Screen on Desktop (side-by-side) & Mobile 360px (tabs: Target Radar, Own Fleet, Log)
    await expect(desktopPage.getByText('Targeting Radar (Hostile Waters)')).toBeVisible()
    await expect(mobilePage.getByText('Targeting Radar (Hostile Waters)')).toBeVisible()
    await assertNoHorizontalOverflow(mobilePage, 'Mobile Battle Target Tab')
    await maybeScreenshot(desktopPage, '08-desktop-battle.png')
    await maybeScreenshot(mobilePage, '09-mobile-battle-target.png')

    // Switch mobile tabs: Own Fleet and Log
    await mobilePage.getByRole('button', { name: /Own Fleet/i }).click()
    await assertNoHorizontalOverflow(mobilePage, 'Mobile Battle Defense Tab')
    await maybeScreenshot(mobilePage, '10-mobile-battle-defense.png')

    await mobilePage.getByRole('button', { name: /Log/i }).click()
    await assertNoHorizontalOverflow(mobilePage, 'Mobile Battle Log Tab')

    await mobilePage.getByRole('button', { name: /Target Radar/i }).click()

    // 8. Reconnect mid-battle via page reload on Desktop
    await desktopPage.reload()
    await expect(desktopPage.getByText('Targeting Radar (Hostile Waters)')).toBeVisible()

    // 9. Explicit Forfeit via Leave Game confirmation modal on Mobile
    await mobilePage.getByRole('button', { name: 'Leave Game' }).click()
    await expect(mobilePage.getByText(/Leave Game \/ Forfeit Match\?/i)).toBeVisible()
    await mobilePage.getByRole('button', { name: /Confirm Forfeit/i }).click()

    // Mobile returns to Lobby; Desktop sees Victory via Enemy Forfeited on GameOverScreen
    await expect(mobilePage.getByText('Public Battle Lobby')).toBeVisible()
    await expect(desktopPage.getByRole('heading', { name: 'VICTORY ACHIEVED' })).toBeVisible()
    await expect(desktopPage.getByText('Enemy Forfeited')).toBeVisible()
    await maybeScreenshot(desktopPage, '11-desktop-gameover.png')
  } finally {
    await desktopContext.close()
    await mobileContext.close()
  }
})
