/** External documentation link that opens in a new tab. */
export function DocsLink({ href, label }: { href: string; label: string }) {
  return (
    <a
      href={href}
      target="_blank"
      rel="noopener noreferrer"
      className="inline-block text-sm text-blue-400 hover:text-blue-300 underline"
    >
      {label}
    </a>
  )
}
