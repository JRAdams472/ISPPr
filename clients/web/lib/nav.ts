// Primary portal navigation — the (portal) layout renders these in the rail.

export interface NavItem {
  href: string;
  label: string;
}

export const NAV: NavItem[] = [
  { href: "/", label: "Dashboard" },
  { href: "/clients", label: "Clients" },
  { href: "/templates", label: "Templates" },
  { href: "/requests", label: "Requests" },
  { href: "/usage", label: "Usage" },
];

export function navActive(pathname: string, href: string): boolean {
  return href === "/" ? pathname === "/" : pathname.startsWith(href);
}
