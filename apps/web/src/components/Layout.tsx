import { NavLink, Outlet } from "react-router-dom";

// The app shell: a slim nav bar (brand + top-level route links) above the routed
// view. The rich training header (phase/season/status) stays inside the
// dashboard view; this bar is the always-present navigation.
const NAV = [
  { to: "/", label: "Dashboard", end: true },
  { to: "/stats", label: "Stats", end: false },
  { to: "/records", label: "Records", end: false },
  { to: "/gear", label: "Gear", end: false },
];

export function Layout() {
  return (
    <div className="mx-auto flex min-h-full max-w-screen-2xl flex-col gap-4 p-4 lg:p-6">
      <nav className="flex items-center gap-4 border-b border-ink-600/60 pb-3">
        <span className="text-xs font-semibold uppercase tracking-widest text-accent">
          Kazper · Coach
        </span>
        <div className="flex gap-1">
          {NAV.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              end={item.end}
              className={({ isActive }) =>
                `rounded-md px-3 py-1 text-sm font-medium transition-colors ${
                  isActive
                    ? "bg-ink-700/70 text-slate-100"
                    : "text-slate-400 hover:text-slate-200"
                }`
              }
            >
              {item.label}
            </NavLink>
          ))}
        </div>
      </nav>
      <Outlet />
    </div>
  );
}
