/** A promise that stays pending until `open()` is called: holds a mocked response or step. */
export function gate() {
  let open!: () => void
  const opened = new Promise<void>((resolve) => {
    open = resolve
  })
  return { opened, open }
}
