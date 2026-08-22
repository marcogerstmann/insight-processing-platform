export type IconName = "insights" | "create" | "import" | "knowledge" | "plan" | "profile" | "logout";

// Minimal hand-drawn line icons (24x24, stroke=currentColor) — just enough of
// a visual anchor per nav item without pulling in an icon library dependency.
const PATHS: Record<IconName, string> = {
  insights: "M4 6h16 M4 12h16 M4 18h10",
  create: "M12 5v14 M5 12h14",
  import: "M12 3v10 M8 9l4 4 4-4 M4 19h16",
  knowledge: "M5 4a1 1 0 011-1h12a1 1 0 011 1v16l-7-4-7 4V4z",
  plan: "M4 5h16v16H4z M4 10h16 M8 3v4 M16 3v4",
  profile: "M12 12a4 4 0 100-8 4 4 0 000 8z M4 20c0-4.4 3.6-6 8-6s8 1.6 8 6",
  logout: "M9 21H5a2 2 0 01-2-2V5a2 2 0 012-2h4 M16 17l5-5-5-5 M21 12H9",
};

export function Icon({ name, className }: { name: IconName; className?: string }) {
  return (
    <svg
      viewBox="0 0 24 24"
      width="18"
      height="18"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.8"
      strokeLinecap="round"
      strokeLinejoin="round"
      className={className}
      aria-hidden="true"
    >
      <path d={PATHS[name]} />
    </svg>
  );
}
