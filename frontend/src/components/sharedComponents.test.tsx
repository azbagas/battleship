import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { ReconnectingBanner } from './ReconnectingBanner'
import { ConfirmDialog } from './ConfirmDialog'

describe('ReconnectingBanner', () => {
  it('renders only while connecting or disconnected', () => {
    const { rerender } = render(<ReconnectingBanner status="connected" />)
    expect(screen.queryByRole('status')).toBeNull()

    rerender(<ReconnectingBanner status="connecting" />)
    expect(screen.getByRole('status')).toHaveTextContent(/Connection lost/)

    rerender(<ReconnectingBanner status="disconnected" message="Custom text" />)
    expect(screen.getByRole('status')).toHaveTextContent('Custom text')
  })
})

describe('ConfirmDialog', () => {
  const setup = (pending = false) => {
    const onCancel = vi.fn()
    const onConfirm = vi.fn()
    render(
      <ConfirmDialog
        titleId="t"
        title="Leave?"
        confirmLabel="Confirm Forfeit"
        pending={pending}
        pendingLabel="Forfeiting..."
        onCancel={onCancel}
        onConfirm={onConfirm}
      >
        Body text
      </ConfirmDialog>,
    )
    return { onCancel, onConfirm }
  }

  it('is a labelled modal dialog with working buttons', () => {
    const { onCancel, onConfirm } = setup()
    expect(screen.getByRole('dialog', { name: 'Leave?' })).toBeInTheDocument()
    fireEvent.click(screen.getByText('Cancel'))
    fireEvent.click(screen.getByText('Confirm Forfeit'))
    expect(onCancel).toHaveBeenCalledTimes(1)
    expect(onConfirm).toHaveBeenCalledTimes(1)
  })

  it('disables confirm and shows pending label while pending', () => {
    setup(true)
    expect(screen.getByText('Forfeiting...')).toBeDisabled()
  })
})
