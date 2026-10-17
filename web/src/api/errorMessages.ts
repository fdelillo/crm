import type {
  FieldError as FormFieldError,
  FieldValues,
  Path,
  UseFormSetError,
} from 'react-hook-form'
import type { ErrorCode, FieldError, FieldErrorCode } from './types'
import { isApiError, type ApiError } from './errors'
import { formatRetryAt } from '@/lib/format'
export interface UserMessage {
  title: string
  description?: string
  retryable: boolean
  requestId?: string
}
export type ErrorContext =
  | 'signup'
  | 'login'
  | 'passwordReset'
  | 'resetConfirm'
  | 'verifyEmail'
  | 'invitationPreview'
  | 'invitationAccept'
  | 'users'
  | 'userAction'
  | 'invite'
  | 'tenant'
  | 'logo'
  | 'resendVerification'
  | 'generic'
export type LogoRejectReason =
  | 'svg'
  | 'unsupported_format'
  | 'unreadable'
  | 'too_large_to_process'
  | 'cannot_shrink'
function waitText(seconds: number | null) {
  return seconds === null
    ? 'unos minutos'
    : seconds < 60
      ? 'unos segundos'
      : seconds === 60
        ? '1 minuto'
        : `${Math.ceil(seconds / 60)} minutos`
}
export type ErrorMessageResolver = (
  error: ApiError | null,
  context: ErrorContext,
) => UserMessage
const malformed: ErrorMessageResolver = (error, context) => ({
  title:
    context === 'logo'
      ? 'No pudimos subir el logo.'
      : 'No pudimos procesar el pedido.',
  description: 'Recargá la página y probá de nuevo.',
  retryable: false,
  ...(error?.requestId ? { requestId: error.requestId } : {}),
})
// T-F102 (INV-F05): one resolver per contract code, so a new ErrorCode fails to compile.
export const errorMessages: Record<ErrorCode, ErrorMessageResolver> = {
  malformed_request: malformed,
  method_not_allowed: (error) => malformed(error, 'generic'),
  unsupported_media_type: (error, context) =>
    context === 'logo'
      ? {
          title: 'El logo tiene que ser una imagen PNG o JPG.',
          retryable: false,
        }
      : malformed(error, 'generic'),
  payload_too_large: (error, context) =>
    context === 'logo'
      ? {
          title:
            'La imagen sigue pesando más de lo permitido. Probá con otra imagen del logo.',
          retryable: false,
        }
      : malformed(error, 'generic'),
  validation_failed: () => ({
    title: 'Revisá los datos marcados.',
    retryable: false,
  }),
  unauthenticated: () => ({
    title: 'Tu sesión se cerró. Ingresá de nuevo para seguir.',
    description:
      'Las sesiones se cierran solas después de 24 horas sin uso o a los 7 días.',
    retryable: false,
  }),
  invalid_credentials: () => ({
    title: 'El email o la contraseña no son correctos.',
    retryable: false,
  }),
  account_disabled: () => ({
    title: 'Tu usuario está desactivado.',
    description: 'Pedile a un administrador de tu empresa que lo reactive.',
    retryable: false,
  }),
  forbidden: () => ({
    title: 'No tenés permiso para hacer esto.',
    description: 'Si lo necesitás, pedíselo a un administrador.',
    retryable: false,
  }),
  not_found: (_error, context) => ({
    title: ['users', 'userAction', 'invite'].includes(context)
      ? 'Ese usuario ya no está en tu empresa. Actualizamos la lista.'
      : 'No encontramos lo que buscás.',
    retryable: false,
  }),
  email_already_registered: () => ({
    title: 'Ya existe un usuario con ese email.',
    description: '¿Querés recuperar la contraseña?',
    retryable: false,
  }),
  email_taken: () => ({
    title: 'Ese email ya tiene un usuario en el sistema.',
    description: 'Usá otro email.',
    retryable: false,
  }),
  email_not_verified: () => ({
    title: 'Confirmá tu email para invitar a tu equipo.',
    description:
      'Te mandamos un enlace cuando te registraste. Si no lo encontrás, pedí uno nuevo.',
    retryable: false,
  }),
  last_admin: () => ({
    title: 'Tiene que quedar al menos un administrador activo.',
    description:
      'Nombrá a otra persona como administrador antes de hacer este cambio.',
    retryable: false,
  }),
  invalid_state: () => ({
    title:
      'El estado de este usuario cambió mientras tanto. Actualizamos la lista.',
    retryable: false,
  }),
  token_invalid: (_error, context) => {
    if (context === 'resetConfirm')
      return {
        title: 'Este enlace ya no sirve.',
        description: 'Vence a la hora o ya se usó. Pedí uno nuevo.',
        retryable: false,
      }
    if (context === 'verifyEmail')
      return {
        title: 'Este enlace de confirmación ya no sirve.',
        description: 'Vence a las 48 horas o ya se usó.',
        retryable: false,
      }
    if (context === 'invitationPreview' || context === 'invitationAccept')
      return {
        title: 'Esta invitación ya no es válida.',
        description: 'Venció, ya se usó o la reemplazó una más nueva.',
        retryable: false,
      }
    return {
      title: 'Este enlace ya no sirve.',
      description: 'Pedí uno nuevo.',
      retryable: false,
    }
  },
  login_locked: (error) => {
    const title = 'Por seguridad, pausamos el ingreso con este email.'
    const seconds = error?.retryAfterSeconds ?? null
    let when = 'en unos minutos'
    if (error && seconds !== null) {
      const { time, minutes } = formatRetryAt(
        seconds,
        new Date(error.receivedAt),
      )
      when = `a las ${time} (en ${minutes === 1 ? '1 minuto' : `${minutes} minutos`})`
    }
    return {
      title,
      description: `Hubo varios intentos fallidos. Podés volver a intentar ${when} o restablecer tu contraseña ahora.`,
      retryable: false,
    }
  },
  rate_limited: (error, context) => {
    const wait = `Esperá ${waitText(error?.retryAfterSeconds ?? null)} y probá de nuevo.`
    if (context === 'signup')
      return {
        title: 'Hubo muchos intentos de registro seguidos.',
        description:
          wait + ' No se creó nada: podés reintentar con los mismos datos.',
        retryable: false,
      }
    if (context === 'invite' || context === 'userAction')
      return {
        title: 'Por ahora no podemos enviar más invitaciones.',
        description:
          'Hay un límite de invitaciones y reactivaciones seguidas para cuidar el envío de emails. ' +
          wait,
        retryable: false,
      }
    return {
      title:
        context === 'resendVerification'
          ? 'Ya te lo mandamos hace poco.'
          : 'Hiciste muchos intentos seguidos.',
      description: wait,
      retryable: false,
    }
  },
  service_unavailable: (error, context) => ({
    title: 'El servicio no está disponible en este momento.',
    description:
      `Probá de nuevo en ${waitText(error?.retryAfterSeconds ?? null)}.` +
      (context === 'signup'
        ? ' No se creó nada: podés reintentar con los mismos datos.'
        : ''),
    retryable: true,
  }),
  internal: (error) => ({
    title: 'Algo salió mal de nuestro lado.',
    description: error?.requestId
      ? `Probá de nuevo. Si sigue pasando, avisanos con este código: ${error.requestId}.`
      : 'Probá de nuevo.',
    retryable: true,
    ...(error?.requestId ? { requestId: error.requestId } : {}),
  }),
}
export function messageForError(
  error: unknown,
  context: ErrorContext,
): UserMessage {
  if (isApiError(error) && error.kind === 'network')
    return {
      title: 'No hay conexión.',
      description: 'Revisá tu internet y probá de nuevo.',
      retryable: true,
    }
  const normalized = isApiError(error) ? error : null
  const code =
    normalized?.code ??
    (normalized?.status !== null &&
    normalized?.status !== undefined &&
    normalized.status >= 500
      ? 'service_unavailable'
      : 'internal')
  return errorMessages[code](normalized, context)
}
const maximums: Record<string, number> = {
  name: 120,
  company_name: 120,
  legal_name: 200,
  address: 300,
  phone: 50,
  email: 254,
  password: 128,
}
const knownFieldErrorCodes = {
  required: true,
  invalid_format: true,
  too_short: true,
  too_long: true,
  invalid_value: true,
  invalid_tax_id: true,
  same_as_email: true,
  unknown_template: true,
  invalid_timezone: true,
} satisfies Record<FieldErrorCode, true>
function isFieldErrorCode(value: string): value is FieldErrorCode {
  return Object.hasOwn(knownFieldErrorCodes, value)
}

export function fieldErrorMessage(field: string, code: FieldErrorCode): string {
  const long = `Es demasiado largo (máximo ${maximums[field]} caracteres).`
  const defaults: Record<FieldErrorCode, string> = {
    required: 'Completá este campo.',
    invalid_format: 'El formato no es válido.',
    too_short: 'Es demasiado corto.',
    too_long: long,
    invalid_value: 'Revisá este dato.',
    invalid_tax_id: 'El CUIT no es válido. Revisá los números.',
    same_as_email: 'La contraseña no puede ser igual a tu email.',
    unknown_template: 'Elegí un rubro de la lista.',
    invalid_timezone: 'Elegí una zona horaria de la lista.',
  }
  const variants: Partial<Record<FieldErrorCode, Record<string, string>>> = {
    required: {
      industry_template_code: 'Elegí el rubro de tu empresa.',
      base_currency: 'Elegí la moneda base.',
      file: 'Elegí una imagen para el logo.',
    },
    invalid_format: {
      email: 'Ingresá un email válido, por ejemplo nombre@empresa.com',
      tax_id: 'El CUIT tiene 11 números (podés escribirlo con o sin guiones).',
    },
    too_short: {
      password: 'La contraseña tiene que tener al menos 10 caracteres.',
    },
    too_long: { password: 'La contraseña puede tener hasta 128 caracteres.' },
    invalid_value: {
      name: 'Tiene que tener entre 1 y 120 caracteres.',
      company_name: 'Tiene que tener entre 1 y 120 caracteres.',
      legal_name: long,
      address: long,
      phone: long,
      base_currency: 'Elegí la moneda base.',
      role: 'Elegí un rol.',
      password: 'Ingresá tu contraseña (hasta 128 caracteres).',
      file: 'No pudimos leer la imagen o mide más de 2000 × 2000 px. Probá con otra imagen del logo.',
    },
  }
  return variants[code]?.[field] ?? defaults[code]
}
export function fieldErrorText(
  field: string,
  error: FormFieldError | undefined,
): string | undefined {
  if (!error) return undefined
  if (error.type === 'server') return error.message
  const code =
    error.message && isFieldErrorCode(error.message)
      ? error.message
      : 'required'
  return fieldErrorMessage(field, code)
}
export function logoRejectMessage(reason: LogoRejectReason): string {
  return {
    svg: 'No podemos usar logos en SVG. Exportalo como PNG desde el programa donde lo tenés y subilo de nuevo.',
    unsupported_format: 'El logo tiene que ser una imagen PNG o JPG.',
    unreadable: 'No pudimos leer la imagen. Probá con otro archivo PNG o JPG.',
    too_large_to_process:
      'La imagen es demasiado grande para prepararla en este dispositivo. Probá con una captura de pantalla del logo o con una imagen más chica.',
    cannot_shrink:
      'No pudimos achicar la imagen lo suficiente. Probá con otra imagen del logo.',
  }[reason]
}
export function applyServerFieldErrors<T extends FieldValues>(
  setError: UseFormSetError<T>,
  fieldErrors: FieldError[],
  knownFields: ReadonlyArray<Path<T>>,
): FieldError[] {
  return fieldErrors.filter((error) => {
    const field = knownFields.find((field) => field === error.field)
    if (!field) return true
    setError(field, {
      type: 'server',
      message: fieldErrorMessage(error.field, error.code),
    })
    return false
  })
}
