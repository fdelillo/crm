import { render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { VerifyToInviteNotice } from './VerifyToInviteNotice'

describe('VerifyToInviteNotice', () => {
  // T-F502 (INV-F19, DD-F29, P-F20): W keeps its single empty live region mounted.
  it('shows both rate-limit title and wait description in its live region', () => {
    render(<VerifyToInviteNotice id="notice" email="ana@example.test" resendStatus="error" errorMessage={{ title: 'Ya te lo mandamos hace poco.', description: 'Esperá 2 minutos y probá de nuevo.', retryable: false }} onResend={vi.fn()} />)
    expect(screen.getByText('Ya te lo mandamos hace poco.')).toBeVisible()
    expect(screen.getByText('Esperá 2 minutos y probá de nuevo.')).toBeVisible()
    expect(screen.getByRole('status')).toHaveTextContent(/Ya te lo mandamos hace poco\.Esperá 2 minutos y probá de nuevo\./)
    expect(screen.getAllByRole('status')).toHaveLength(1)
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('shows both service-unavailable lines and a retry action', () => {
    render(<VerifyToInviteNotice id="notice" email="ana@example.test" resendStatus="error" errorMessage={{ title: 'El servicio no está disponible en este momento.', description: 'Probá de nuevo en 1 minuto.', retryable: true }} onResend={() => {}} />)
    expect(screen.getByText('El servicio no está disponible en este momento.')).toBeVisible()
    expect(screen.getByText('Probá de nuevo en 1 minuto.')).toBeVisible()
    expect(screen.getByRole('status')).toHaveTextContent(/El servicio no está disponible en este momento\.Probá de nuevo en 1 minuto\./)
    expect(screen.getByRole('button', { name: 'Reintentar' })).toBeVisible()
  })
})
