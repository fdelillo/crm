export interface PreloadErrorDependencies {
  reload: () => void
  showUpdate: (
    title: string,
    action: { label: string; onClick: () => void },
  ) => void
}
export function installPreloadErrorHandler({
  reload,
  showUpdate,
}: PreloadErrorDependencies) {
  const onError = (event: Event) => {
    event.preventDefault()
    if (sessionStorage.getItem('crm.preloadReloaded') !== 'true') {
      sessionStorage.setItem('crm.preloadReloaded', 'true')
      reload()
    } else
      showUpdate('Hay una versión nueva de la app', {
        label: 'Actualizar',
        onClick: reload,
      })
  }
  window.addEventListener('vite:preloadError', onError)
  return () => window.removeEventListener('vite:preloadError', onError)
}
