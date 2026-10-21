import { z } from 'zod'
import type { Tenant, TenantUpdate } from '@/api/types'
import { hasCuitFormat, isValidCuit } from '@/lib/cuit'

// Form values are strings; an empty optional field means "no value" (null in the PATCH).
export type CompanyFormValues = Record<keyof Required<TenantUpdate>, string>

const optionalText = (max: number) => z.string().trim().max(max, 'too_long')

// T-F602 (DD-16, ui.md §16): the server stays the authority; the codes are FieldError codes.
export const companySchema = z.object({
  name: z.string().trim().min(1, 'required').max(120, 'too_long'),
  legal_name: optionalText(200),
  tax_id: z
    .string()
    .trim()
    .superRefine((value, context) => {
      if (value === '') return
      if (!hasCuitFormat(value))
        context.addIssue({ code: 'custom', message: 'invalid_format' })
      else if (!isValidCuit(value))
        context.addIssue({ code: 'custom', message: 'invalid_tax_id' })
    }),
  address: optionalText(300),
  phone: optionalText(50),
  email: z
    .string()
    .trim()
    .max(254, 'too_long')
    .refine((value) => value === '' || z.email().safeParse(value).success, {
      error: 'invalid_format',
    }),
  timezone: z.string().min(1, 'invalid_timezone'),
}) satisfies z.ZodType<CompanyFormValues>

export const companyFields = Object.keys(
  companySchema.shape,
) as (keyof CompanyFormValues)[]

export function tenantToFormValues(tenant: Tenant): CompanyFormValues {
  return {
    name: tenant.name,
    legal_name: tenant.legal_name ?? '',
    tax_id: tenant.tax_id ?? '',
    address: tenant.address ?? '',
    phone: tenant.phone ?? '',
    email: tenant.email ?? '',
    timezone: tenant.timezone,
  }
}

/**
 * INV-F15 (DD-43): only the fields that changed; a cleared optional field is null;
 * `name` and `timezone` are never null; `base_currency` is not part of the form at all.
 */
export function buildTenantUpdate(
  values: CompanyFormValues,
  dirty: Partial<Record<keyof CompanyFormValues, unknown>>,
): TenantUpdate {
  const update: TenantUpdate = {}
  for (const field of companyFields) {
    if (!dirty[field]) continue
    const value = values[field]
    if (field === 'name' || field === 'timezone') update[field] = value
    else update[field] = value === '' ? null : value
  }
  return update
}
