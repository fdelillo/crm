import { Link } from 'react-router'

export interface SettingsLinkListProps {
  items: ReadonlyArray<{ to: string; label: string }>
}

// T-002-F103 (002/DD-F12, P-F3): every link is at least 44 px tall and neighbours are 8 px apart.
export function SettingsLinkList({ items }: SettingsLinkListProps) {
  return (
    <ul className="flex flex-col gap-2">
      {items.map(({ to, label }) => (
        <li key={to}>
          <Link
            to={to}
            className="flex min-h-14 items-center justify-between rounded-lg border bg-card px-4 py-3 text-primary"
          >
            {label}
            <span aria-hidden="true">›</span>
          </Link>
        </li>
      ))}
    </ul>
  )
}
