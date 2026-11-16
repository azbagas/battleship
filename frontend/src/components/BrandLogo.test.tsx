import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { BrandLogo } from './BrandLogo'

describe('BrandLogo', () => {
  it('renders the decorative brand logo image backed by /logo.svg by default', () => {
    render(<BrandLogo />)

    const img = screen.getByTestId('brand-logo')
    expect(img).toBeInTheDocument()
    expect(img).toHaveAttribute('src', '/logo.svg')
    expect(img).toHaveAttribute('aria-hidden', 'true')
    expect(img).toHaveAttribute('alt', '')
    expect(img).toHaveClass('w-9', 'h-9')
  })

  it('supports md and lg size variants and accessible alt text when not aria-hidden', () => {
    const { rerender } = render(<BrandLogo size="md" ariaHidden={false} />)
    const imgMd = screen.getByTestId('brand-logo')
    expect(imgMd).toHaveClass('w-10', 'h-10')
    expect(imgMd).toHaveAttribute('alt', 'Battleship logo')

    rerender(<BrandLogo size="lg" />)
    expect(screen.getByTestId('brand-logo')).toHaveClass('w-14', 'h-14')
  })

  it('optionally renders wordmark and subtitle', () => {
    render(<BrandLogo showWordmark subtitle="Naval Combat Command" />)

    expect(screen.getByText('Battleship')).toBeInTheDocument()
    expect(screen.getByText('Naval Combat Command')).toBeInTheDocument()
  })
})
