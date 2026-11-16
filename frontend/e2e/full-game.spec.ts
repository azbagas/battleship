import { test, expect, Page } from '@playwright/test'

const FLEET_SETUP = [
  { name: 'Carrier', cell: 'A1' },
  { name: 'Battleship', cell: 'A2' },
  { name: 'Cruiser', cell: 'A3' },
  { name: 'Submarine', cell: 'A4' },
  { name: 'Destroyer', cell: 'A5' },
]

// All 17 coordinates occupied by the deterministic horizontal fleet on rows 1..5
const SHIP_TARGET_CELLS = [
  'A1', 'B1', 'C1', 'D1', 'E1', // Carrier (5)
  'A2', 'B2', 'C2', 'D2',       // Battleship (4)
  'A3', 'B3', 'C3',             // Cruiser (3)
  'A4', 'B4', 'C4',             // Submarine (3)
  'A5', 'B5',                   // Destroyer (2)
]

// Guaranteed empty water coordinates on rows 8..10 for the losing player's turns
const MISS_TARGET_CELLS = [
  'A8', 'B8', 'C8', 'D8', 'E8', 'F8', 'G8', 'H8', 'I8', 'J8',
  'A9', 'B9', 'C9', 'D9', 'E9', 'F9', 'G9', 'H9', 'I9', 'J9',
]

async function placeAndConfirmStandardFleet(page: Page, expectStayInPlacement: boolean) {
  for (const ship of FLEET_SETUP) {
    // Select ship from the Fleet Dock by its vessel name
    await page.getByText(ship.name, { exact: true }).click()
    // Place horizontally at the designated origin cell
    await page.getByRole('button', { name: `Cell ${ship.cell}`, exact: true }).click()
  }

  const confirmBtn = page.getByRole('button', { name: 'Confirm Fleet' })
  await expect(confirmBtn).toBeEnabled()
  await confirmBtn.click()
  if (expectStayInPlacement) {
    await expect(page.getByRole('button', { name: 'Fleet Confirmed' })).toBeVisible()
  }
}

test('two browsers complete a full Battleship game and trigger a rematch', async ({ browser }) => {
  const hostContext = await browser.newContext({ viewport: { width: 1280, height: 800 } })
  const guestContext = await browser.newContext({ viewport: { width: 1280, height: 800 } })

  const hostPage = await hostContext.newPage()
  const guestPage = await guestContext.newPage()

  try {
    // 1. Both players land on Home screen, enter display names, and connect
    await hostPage.goto('/')
    await guestPage.goto('/')

    await hostPage.getByLabel('Callsign / Display Name').fill('Admiral Nimitz')
    await hostPage.getByRole('button', { name: 'Play Battleship' }).click()

    await guestPage.getByLabel('Callsign / Display Name').fill('Commander Halsey')
    await guestPage.getByRole('button', { name: 'Play Battleship' }).click()

    // Both reach the Public Battle Lobby
    await expect(hostPage.getByText('Public Battle Lobby')).toBeVisible()
    await expect(guestPage.getByText('Public Battle Lobby')).toBeVisible()

    // 2. Host creates a room; Guest sees it live and joins
    await hostPage.getByRole('button', { name: 'Create Game' }).click()
    await expect(hostPage.getByText('Waiting for an opponent')).toBeVisible()

    const joinBtn = guestPage.getByRole('button', { name: 'Join' }).first()
    await expect(joinBtn).toBeVisible()
    await joinBtn.click()

    // 3. Both players enter Pre-Game Ready-Up and click Ready for Battle
    await expect(hostPage.getByText('Pre-Game Readiness Check')).toBeVisible()
    await expect(guestPage.getByText('Pre-Game Readiness Check')).toBeVisible()

    await hostPage.getByRole('button', { name: 'Ready for Battle' }).click()
    await guestPage.getByRole('button', { name: 'Ready for Battle' }).click()

    // 4. Both players enter Fleet Placement and position all 5 ships
    await expect(hostPage.getByText('Tactical Deployment Phase')).toBeVisible()
    await expect(guestPage.getByText('Tactical Deployment Phase')).toBeVisible()

    await placeAndConfirmStandardFleet(hostPage, true)
    await expect(guestPage.getByText('Fleet Confirmed')).toBeVisible()
    await placeAndConfirmStandardFleet(guestPage, false)

    // 5. Both players enter the Battle Phase
    await expect(hostPage.getByText('Targeting Radar (Hostile Waters)')).toBeVisible()
    await expect(guestPage.getByText('Targeting Radar (Hostile Waters)')).toBeVisible()

    let hostHitIndex = 0
    let guestMissIndex = 0

    const hostTurnBanner = hostPage.getByRole('heading', {
      name: /YOUR TURN — SELECT TARGET CELL/i,
    })
    const guestTurnBanner = guestPage.getByRole('heading', {
      name: /YOUR TURN — SELECT TARGET CELL/i,
    })

    // Determine who was randomly selected for the first turn
    let isHostTurn = await hostTurnBanner.isVisible()

    // Strictly alternate turns until Host sinks all 5 of Guest's ships (17 hits)
    while (hostHitIndex < SHIP_TARGET_CELLS.length) {
      if (isHostTurn) {
        await expect(hostTurnBanner).toBeVisible()
        const targetCoord = SHIP_TARGET_CELLS[hostHitIndex]
        hostHitIndex++

        const cellBtn = hostPage.getByRole('button', {
          name: new RegExp(`^Target cell ${targetCoord}, ready to fire`),
        })
        await expect(cellBtn).toBeEnabled()
        await cellBtn.click()
        isHostTurn = false
      } else {
        await expect(guestTurnBanner).toBeVisible()
        const missCoord = MISS_TARGET_CELLS[guestMissIndex]
        guestMissIndex++

        const cellBtn = guestPage.getByRole('button', {
          name: new RegExp(`^Target cell ${missCoord}, ready to fire`),
        })
        await expect(cellBtn).toBeEnabled()
        await cellBtn.click()
        isHostTurn = true
      }
    }

    // 6. Both browsers transition to the Game Over screen
    await expect(hostPage.getByRole('heading', { name: 'VICTORY ACHIEVED' })).toBeVisible()
    await expect(guestPage.getByRole('heading', { name: 'DEFEAT — FLEET LOST' })).toBeVisible()

    await expect(hostPage.getByText('All Ships Sunk')).toBeVisible()
    await expect(guestPage.getByText('All Ships Sunk')).toBeVisible()
    await expect(hostPage.getByText('Total Salvos Fired')).toBeVisible()

    // 7. Mutual Rematch flow starts a fresh Placement phase
    await hostPage.getByRole('button', { name: 'Request Rematch' }).click()
    await expect(hostPage.getByText(/Rematch request sent/i)).toBeVisible()

    const acceptRematchBtn = guestPage.getByRole('button', { name: 'Accept Rematch' })
    await expect(acceptRematchBtn).toBeVisible()
    await acceptRematchBtn.click()

    await expect(hostPage.getByText('Tactical Deployment Phase')).toBeVisible()
    await expect(guestPage.getByText('Tactical Deployment Phase')).toBeVisible()
  } finally {
    await hostContext.close()
    await guestContext.close()
  }
})
