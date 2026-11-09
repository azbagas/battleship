import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { HomeScreen } from './HomeScreen'
import { SessionProvider } from '../context/SessionProvider'
import { WebSocketClient } from '../services/wsClient'
import { sessionStore } from '../services/storage'

describe('HomeScreen', () => {
  let mockClient: WebSocketClient

  beforeEach(() => {
    localStorage.clear()
    mockClient = new WebSocketClient()
  })

  it('renders callsign input, character counter, and play button', () => {
    render(
      <SessionProvider client={mockClient}>
        <HomeScreen />
      </SessionProvider>,
    )

    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Battleship')
    expect(screen.getByLabelText(/Callsign \/ Display Name/i)).toBeInTheDocument()
    expect(screen.getByText('0 / 20')).toBeInTheDocument()
    const playButton = screen.getByRole('button', { name: /Play Battleship/i })
    expect(playButton).toBeDisabled()
  })

  it('does not reset input to saved name when user clears the input field', () => {
    sessionStore.saveDisplayName('Captain Hook')

    render(
      <SessionProvider client={mockClient}>
        <HomeScreen />
      </SessionProvider>,
    )

    const input = screen.getByLabelText<HTMLInputElement>(/Callsign \/ Display Name/i)
    expect(input.value).toBe('Captain Hook')

    // User deletes the text until empty
    fireEvent.change(input, { target: { value: '' } })

    // Input must remain empty and not snap back to 'Captain Hook'
    expect(input.value).toBe('')
    expect(screen.getByText('0 / 20')).toBeInTheDocument()
  })

  it('enables play button when valid name is entered and updates character count', () => {
    render(
      <SessionProvider client={mockClient}>
        <HomeScreen />
      </SessionProvider>,
    )

    const input = screen.getByLabelText(/Callsign \/ Display Name/i)
    fireEvent.change(input, { target: { value: 'Captain Jack' } })

    expect(screen.getByText('12 / 20')).toBeInTheDocument()
    const playButton = screen.getByRole('button', { name: /Play Battleship/i })
    expect(playButton).not.toBeDisabled()
  })

  it('disables play button and alerts if name exceeds 20 characters', () => {
    render(
      <SessionProvider client={mockClient}>
        <HomeScreen />
      </SessionProvider>,
    )

    const input = screen.getByLabelText(/Callsign \/ Display Name/i)
    fireEvent.change(input, { target: { value: '123456789012345678901' } }) // 21 chars

    expect(screen.getByText('21 / 20')).toBeInTheDocument()
    const playButton = screen.getByRole('button', { name: /Play Battleship/i })
    expect(playButton).toBeDisabled()
  })

  it('submits name on form submit and connects client', () => {
    const connectSpy = vi.spyOn(mockClient, 'connect')
    render(
      <SessionProvider client={mockClient}>
        <HomeScreen />
      </SessionProvider>,
    )

    const input = screen.getByLabelText(/Callsign \/ Display Name/i)
    fireEvent.change(input, { target: { value: 'Admiral Nelson' } })
    const playButton = screen.getByRole('button', { name: /Play Battleship/i })
    fireEvent.click(playButton)

    expect(connectSpy).toHaveBeenCalledWith(
      expect.objectContaining({
        displayName: 'Admiral Nelson',
      }),
    )
  })
})
