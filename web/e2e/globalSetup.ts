import { execFileSync } from 'node:child_process'
import { existsSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

const repositoryRoot = fileURLToPath(new URL('../../', import.meta.url))
const setupHelp =
  'Prepará el entorno siguiendo README.md y ui.md §21.3: make dev-certs, mkcert -install, docker compose up -d --wait y NODE_EXTRA_CA_CERTS="$(mkcert -CAROOT)/rootCA.pem".'

export default async function globalSetup(): Promise<void> {
  const caPath = process.env.NODE_EXTRA_CA_CERTS
  if (!caPath || !existsSync(caPath)) {
    throw new Error(
      `Falta NODE_EXTRA_CA_CERTS o el archivo de CA no existe. ${setupHelp}`,
    )
  }

  let services: string[]
  try {
    const output = execFileSync(
      'docker',
      ['compose', 'ps', '--format', 'json'],
      {
        cwd: repositoryRoot,
        encoding: 'utf8',
        stdio: ['ignore', 'pipe', 'pipe'],
      },
    )
    services = output
      .split('\n')
      .filter(Boolean)
      .map(
        (line) =>
          JSON.parse(line) as {
            Service: string
            State: string
            Health?: string
          },
      )
      .filter((service) => service.State === 'running')
      .map((service) => service.Service)
  } catch (error) {
    throw new Error(`No se pudo consultar docker compose. ${setupHelp}`, {
      cause: error,
    })
  }

  const requiredServices = ['postgres', 'mailpit', 'minio']
  const missingServices = requiredServices.filter(
    (service) => !services.includes(service),
  )
  if (missingServices.length > 0) {
    throw new Error(
      `Faltan servicios de Docker Compose en ejecución: ${missingServices.join(', ')}. ${setupHelp}`,
    )
  }

  let response: Response
  try {
    response = await fetch('https://localhost:8443/healthz', {
      signal: AbortSignal.timeout(5000),
    })
  } catch (error) {
    throw new Error(
      `No se pudo validar https://localhost:8443/healthz con la CA configurada. Revisá TLS y S-12. ${setupHelp}`,
      { cause: error },
    )
  }

  if (response.status !== 200) {
    throw new Error(
      `https://localhost:8443/healthz respondió ${response.status}; se esperaba 200. ${setupHelp}`,
    )
  }
}
