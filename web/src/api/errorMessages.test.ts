import { afterEach, describe, expect, it, vi } from 'vitest'
import { toApiError } from './errors'
import type { ErrorCode, FieldErrorCode } from './types'
import { problem } from '@/test/fixtures'
import {
  messageForError,
  fieldErrorMessage,
  logoRejectMessage,
  applyServerFieldErrors,
} from './errorMessages'

function error(code: ErrorCode, seconds?: number, status = 400) {
  return toApiError({
    response: new Response(null, {
      status,
      headers: {
        'Content-Type': 'application/problem+json',
        ...(seconds === undefined ? {} : { 'Retry-After': String(seconds) }),
      },
    }),
    body: problem(code, status, { instance: 'support-reference' }),
  })
}
// T-F101 (INV-F05): exact contract messages, contexts and field variants.
describe('user error messages', () => {
  it.each([
    ['malformed_request', 'No pudimos procesar el pedido.'],
    ['validation_failed', 'Revisá los datos marcados.'],
    ['unsupported_media_type', 'No pudimos procesar el pedido.'],
    ['payload_too_large', 'No pudimos procesar el pedido.'],
    ['unauthenticated', 'Tu sesión se cerró. Ingresá de nuevo para seguir.'],
    ['invalid_credentials', 'El email o la contraseña no son correctos.'],
    ['account_disabled', 'Tu usuario está desactivado.'],
    ['forbidden', 'No tenés permiso para hacer esto.'],
    ['not_found', 'No encontramos lo que buscás.'],
    ['method_not_allowed', 'No pudimos procesar el pedido.'],
    ['email_already_registered', 'Ya existe un usuario con ese email.'],
    ['email_taken', 'Ese email ya tiene un usuario en el sistema.'],
    ['email_not_verified', 'Confirmá tu email para invitar a tu equipo.'],
    ['last_admin', 'Tiene que quedar al menos un administrador activo.'],
    [
      'invalid_state',
      'El estado de este usuario cambió mientras tanto. Actualizamos la lista.',
    ],
    ['rate_limited', 'Hiciste muchos intentos seguidos.'],
    ['service_unavailable', 'El servicio no está disponible en este momento.'],
    ['internal', 'Algo salió mal de nuestro lado.'],
  ] satisfies [ErrorCode, string][])(
    '%s has its approved generic title',
    (code, title) => {
      expect(messageForError(error(code), 'generic').title).toBe(title)
    },
  )
  it.each([
    ['malformed_request', 'Recargá la página y probá de nuevo.'],
    ['unsupported_media_type', 'Recargá la página y probá de nuevo.'],
    ['payload_too_large', 'Recargá la página y probá de nuevo.'],
    [
      'unauthenticated',
      'Las sesiones se cierran solas después de 24 horas sin uso o a los 7 días.',
    ],
    [
      'account_disabled',
      'Pedile a un administrador de tu empresa que lo reactive.',
    ],
    ['forbidden', 'Si lo necesitás, pedíselo a un administrador.'],
    ['email_already_registered', '¿Querés recuperar la contraseña?'],
    ['email_taken', 'Usá otro email.'],
    [
      'email_not_verified',
      'Te mandamos un enlace cuando te registraste. Si no lo encontrás, pedí uno nuevo.',
    ],
    [
      'last_admin',
      'Nombrá a otra persona como administrador antes de hacer este cambio.',
    ],
  ] satisfies [ErrorCode, string][])(
    'keeps the approved explanation for %s',
    (code, description) =>
      expect(messageForError(error(code), 'generic').description).toBe(
        description,
      ),
  )
  it.each([
    [3, 'unos segundos'],
    [60, '1 minuto'],
    [61, '2 minutos'],
    [1800, '30 minutos'],
    [undefined, 'unos minutos'],
  ] as const)('rounds the wait %s', (seconds, wait) => {
    expect(
      messageForError(error('rate_limited', seconds, 429), 'generic')
        .description,
    ).toBe(`Esperá ${wait} y probá de nuevo.`)
  })
  it.each(['invite', 'userAction', 'generic'] as const)(
    'explains email verification in %s',
    (context) => {
      expect(
        messageForError(error('email_not_verified'), context),
      ).toMatchObject({
        title: 'Confirmá tu email para invitar a tu equipo.',
        description:
          'Te mandamos un enlace cuando te registraste. Si no lo encontrás, pedí uno nuevo.',
      })
    },
  )
  it.each([
    [45, 'unos segundos'],
    [720, '12 minutos'],
  ] as const)('explains registration throttling %s', (seconds, wait) => {
    expect(
      messageForError(error('rate_limited', seconds, 429), 'signup'),
    ).toMatchObject({
      title: 'Hubo muchos intentos de registro seguidos.',
      description: `Esperá ${wait} y probá de nuevo. No se creó nada: podés reintentar con los mismos datos.`,
    })
  })
  it.each(['invite', 'userAction'] as const)(
    'explains invitation throttling %s',
    (context) => {
      expect(
        messageForError(error('rate_limited', 360, 429), context).description,
      ).toBe(
        'Hay un límite de invitaciones y reactivaciones seguidas para cuidar el envío de emails. Esperá 6 minutos y probá de nuevo.',
      )
    },
  )
  it.each([300, 1200])('explains resend throttling %s', (seconds) => {
    expect(
      messageForError(
        error('rate_limited', seconds, 429),
        'resendVerification',
      ),
    ).toMatchObject({
      title: 'Ya te lo mandamos hace poco.',
      description: `Esperá ${seconds / 60} minutos y probá de nuevo.`,
    })
  })
  it.each([
    [
      'resetConfirm',
      'Este enlace ya no sirve.',
      'Vence a la hora o ya se usó. Pedí uno nuevo.',
    ],
    [
      'verifyEmail',
      'Este enlace de confirmación ya no sirve.',
      'Vence a las 48 horas o ya se usó.',
    ],
    [
      'invitationPreview',
      'Esta invitación ya no es válida.',
      'Venció, ya se usó o la reemplazó una más nueva.',
    ],
  ] as const)('describes invalid links in %s', (context, title, description) =>
    expect(messageForError(error('token_invalid'), context)).toMatchObject({
      title,
      description,
    }),
  )
  // T-F101 (P-F16): fallback text for any context without its own token_invalid variant.
  it.each(['generic', 'login'] as const)(
    'uses the generic invalid link text in %s',
    (context) =>
      expect(messageForError(error('token_invalid'), context)).toMatchObject({
        title: 'Este enlace ya no sirve.',
        description: 'Pedí uno nuevo.',
        retryable: false,
      }),
  )
  describe('login_locked', () => {
    const title = 'Por seguridad, pausamos el ingreso con este email.'
    const lockedAt = (seconds: number | undefined, now: Date) => {
      vi.useFakeTimers()
      vi.setSystemTime(now)
      return messageForError(error('login_locked', seconds, 429), 'login')
    }
    afterEach(() => vi.useRealTimers())
    it.each([
      [900, new Date(2026, 9, 16, 14, 20, 0), 'a las 14:35 (en 15 minutos)'],
      [900, new Date(2026, 9, 16, 14, 20, 30), 'a las 14:36 (en 16 minutos)'],
      [60, new Date(2026, 9, 16, 14, 20, 0), 'a las 14:21 (en 1 minuto)'],
    ])('with Retry-After %i s: %#', (seconds, now, when) =>
      expect(lockedAt(seconds, now)).toMatchObject({
        title,
        description: `Hubo varios intentos fallidos. Podés volver a intentar ${when} o restablecer tu contraseña ahora.`,
        retryable: false,
      }),
    )
    // T-F101 (P-F18): the time comes from receivedAt, not from the render clock.
    it('shows the same time when the clock moves after the error arrived', () => {
      vi.useFakeTimers()
      vi.setSystemTime(new Date(2026, 9, 16, 14, 20, 0))
      const received = error('login_locked', 900, 429)
      const texts = [
        new Date(2026, 9, 16, 14, 20, 0),
        new Date(2026, 9, 16, 14, 27, 0),
      ].map((clock) => {
        vi.setSystemTime(clock)
        return messageForError(received, 'login').description
      })
      expect(texts[0]).toBe(
        'Hubo varios intentos fallidos. Podés volver a intentar a las 14:35 (en 15 minutos) o restablecer tu contraseña ahora.',
      )
      expect(texts[1]).toBe(texts[0])
    })
    it('omits the time without Retry-After', () =>
      expect(
        lockedAt(undefined, new Date(2026, 9, 16, 14, 20, 0)),
      ).toMatchObject({
        title,
        description:
          'Hubo varios intentos fallidos. Podés volver a intentar en unos minutos o restablecer tu contraseña ahora.',
      }))
  })
  it.each([
    ['unsupported_media_type', 'El logo tiene que ser una imagen PNG o JPG.'],
    [
      'payload_too_large',
      'La imagen sigue pesando más de lo permitido. Probá con otra imagen del logo.',
    ],
    ['malformed_request', 'No pudimos subir el logo.'],
  ] satisfies [ErrorCode, string][])('describes logo %s', (code, title) =>
    expect(messageForError(error(code), 'logo').title).toBe(title),
  )
  it('keeps reference codes and explains temporary outages and network failures', () => {
    expect(
      messageForError(error('method_not_allowed'), 'generic'),
    ).toMatchObject({
      description: 'Recargá la página y probá de nuevo.',
      requestId: 'support-reference',
    })
    expect(messageForError(error('malformed_request'), 'logo').requestId).toBe(
      'support-reference',
    )
    expect(messageForError(error('internal'), 'generic').description).toContain(
      'support-reference',
    )
    expect(
      messageForError(error('service_unavailable', 2, 503), 'signup')
        .description,
    ).toBe(
      'Probá de nuevo en unos segundos. No se creó nada: podés reintentar con los mismos datos.',
    )
    expect(
      messageForError(error('service_unavailable', undefined, 503), 'generic')
        .description,
    ).toBe('Probá de nuevo en unos minutos.')
    expect(
      messageForError(toApiError({ cause: new TypeError() }), 'generic'),
    ).toMatchObject({ title: 'No hay conexión.', retryable: true })
    for (const [status, code] of [
      [502, 'service_unavailable'],
      [418, 'internal'],
    ] as const)
      expect(
        messageForError(
          toApiError({ response: new Response(null, { status }) }),
          'generic',
        ).title,
      ).toBe(messageForError(error(code), 'generic').title)
  })
  it.each([
    ['x', 'required', 'Completá este campo.'],
    ['industry_template_code', 'required', 'Elegí el rubro de tu empresa.'],
    ['base_currency', 'required', 'Elegí la moneda base.'],
    ['file', 'required', 'Elegí una imagen para el logo.'],
    ['x', 'invalid_format', 'El formato no es válido.'],
    [
      'email',
      'invalid_format',
      'Ingresá un email válido, por ejemplo nombre@empresa.com',
    ],
    [
      'tax_id',
      'invalid_format',
      'El CUIT tiene 11 números (podés escribirlo con o sin guiones).',
    ],
    ['x', 'too_short', 'Es demasiado corto.'],
    [
      'password',
      'too_short',
      'La contraseña tiene que tener al menos 10 caracteres.',
    ],
    ['name', 'invalid_value', 'Tiene que tener entre 1 y 120 caracteres.'],
    [
      'company_name',
      'invalid_value',
      'Tiene que tener entre 1 y 120 caracteres.',
    ],
    [
      'legal_name',
      'invalid_value',
      'Es demasiado largo (máximo 200 caracteres).',
    ],
    ['base_currency', 'invalid_value', 'Elegí la moneda base.'],
    ['role', 'invalid_value', 'Elegí un rol.'],
    [
      'password',
      'invalid_value',
      'Ingresá tu contraseña (hasta 128 caracteres).',
    ],
    ['x', 'invalid_value', 'Revisá este dato.'],
    [
      'file',
      'invalid_value',
      'No pudimos leer la imagen o mide más de 2000 × 2000 px. Probá con otra imagen del logo.',
    ],
    ['password', 'too_long', 'La contraseña puede tener hasta 128 caracteres.'],
    ['x', 'invalid_tax_id', 'El CUIT no es válido. Revisá los números.'],
    ['x', 'same_as_email', 'La contraseña no puede ser igual a tu email.'],
    ['x', 'unknown_template', 'Elegí un rubro de la lista.'],
    ['x', 'invalid_timezone', 'Elegí una zona horaria de la lista.'],
  ] satisfies [string, FieldErrorCode, string][])(
    'describes %s %s',
    (field, code, text) => expect(fieldErrorMessage(field, code)).toBe(text),
  )
  it.each([
    ['name', 120],
    ['company_name', 120],
    ['legal_name', 200],
    ['address', 300],
    ['phone', 50],
    ['email', 254],
  ] as const)('uses the contract maximum for %s', (field, max) =>
    expect(fieldErrorMessage(field, 'too_long')).toBe(
      `Es demasiado largo (máximo ${max} caracteres).`,
    ),
  )
  it.each([
    ['address', 300],
    ['phone', 50],
  ] as const)('explains an invalid optional text %s', (field, max) =>
    expect(fieldErrorMessage(field, 'invalid_value')).toBe(
      `Es demasiado largo (máximo ${max} caracteres).`,
    ),
  )
  it.each([
    [
      'svg',
      'No podemos usar logos en SVG. Exportalo como PNG desde el programa donde lo tenés y subilo de nuevo.',
    ],
    ['unsupported_format', 'El logo tiene que ser una imagen PNG o JPG.'],
    [
      'unreadable',
      'No pudimos leer la imagen. Probá con otro archivo PNG o JPG.',
    ],
    [
      'too_large_to_process',
      'La imagen es demasiado grande para prepararla en este dispositivo. Probá con una captura de pantalla del logo o con una imagen más chica.',
    ],
    [
      'cannot_shrink',
      'No pudimos achicar la imagen lo suficiente. Probá con otra imagen del logo.',
    ],
  ] as const)('explains rejected logo %s', (reason, text) =>
    expect(logoRejectMessage(reason)).toBe(text),
  )
  it('sets known field errors and returns unknown fields', () => {
    const setError = vi.fn()
    const unknown = { field: 'other', code: 'required' } as const
    expect(
      applyServerFieldErrors<{ email: string }>(
        setError,
        [{ field: 'email', code: 'invalid_format' }, unknown],
        ['email'],
      ),
    ).toEqual([unknown])
    expect(setError).toHaveBeenCalledWith('email', {
      type: 'server',
      message: fieldErrorMessage('email', 'invalid_format'),
    })
  })
})
