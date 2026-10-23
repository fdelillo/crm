import { z } from 'zod'

// NFR-F10 (ADR-019): with `script-src 'self'` Zod's probe for `new Function` is reported as a
// `securitypolicyviolation` even though Zod catches the throw. `jitless` skips the probe and the
// compiled parsers, which the CSP would block anyway. Imported first by main.tsx so it runs before
// any schema module is evaluated.
z.config({ jitless: true })
