export const queryKeys = {
  session: ['session'] as const,
  tenant: ['tenant'] as const,
  users: ['users'] as const,
  industryTemplates: ['industry-templates'] as const,
  invitationPreview: (token: string) => ['invitation-preview', token] as const,
}
