export const queryKeys = {
  session: ['session'] as const,
  tenant: ['tenant'] as const,
  users: ['users'] as const,
  industryTemplates: ['industry-templates'] as const,
  settings: {
    all: ['settings'] as const,
    stages: ['settings', 'stages'] as const,
    categories: ['settings', 'categories'] as const,
    catalogItems: ['settings', 'catalog-items'] as const,
    catalogItem: (id: string) => ['settings', 'catalog-items', id] as const,
    labels: ['settings', 'labels'] as const,
    quoteDefaults: ['settings', 'quote-defaults'] as const,
  },
  invitationPreview: (token: string) => ['invitation-preview', token] as const,
}
