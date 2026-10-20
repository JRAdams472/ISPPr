// Minimal authored icon set — one stroke weight (1.5), 16px box, same
// conventions as the portal palette. Add glyphs here rather than pulling
// a whole icon library for an ops tool.

function Icon({ children, label }: { children: React.ReactNode; label?: string }) {
  return (
    <svg
      viewBox="0 0 16 16"
      width="14"
      height="14"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden={label ? undefined : true}
      aria-label={label}
      role={label ? "img" : undefined}
    >
      {children}
    </svg>
  );
}

export const PlusIcon = () => (
  <Icon>
    <path d="M8 3.5v9M3.5 8h9" />
  </Icon>
);

export const CopyIcon = () => (
  <Icon>
    <rect x="5.5" y="5.5" width="7" height="7" rx="1.5" />
    <path d="M10.5 5.5v-1a1.5 1.5 0 0 0-1.5-1.5H4.5a1.5 1.5 0 0 0-1.5 1.5V9a1.5 1.5 0 0 0 1.5 1.5h1" />
  </Icon>
);

export const CheckIcon = () => (
  <Icon>
    <path d="M3.5 8.5l3 3 6-7" />
  </Icon>
);

export const RotateIcon = () => (
  <Icon>
    <path d="M13.5 8a5.5 5.5 0 1 1-1.61-3.89M13.5 2v2.5H11" />
  </Icon>
);

export const BanIcon = () => (
  <Icon>
    <circle cx="8" cy="8" r="5.5" />
    <path d="M4.2 11.8l7.6-7.6" />
  </Icon>
);

export const DownloadIcon = () => (
  <Icon>
    <path d="M8 2.5v7M5 6.5l3 3 3-3M3.5 12.5v1a1.5 1.5 0 0 0 1.5 1.5h6a1.5 1.5 0 0 0 1.5-1.5v-1" />
  </Icon>
);

export const BackIcon = () => (
  <Icon>
    <path d="M9.5 3.5L5 8l4.5 4.5M5.5 8h7" />
  </Icon>
);
