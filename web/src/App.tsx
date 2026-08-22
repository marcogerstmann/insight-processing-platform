import { useState } from "react";
import { useAuth } from "./auth/AuthContext.tsx";
import { Icon, type IconName } from "./icons.tsx";
import { LoginSection } from "./sections/LoginSection.tsx";
import { InsightsSection } from "./sections/InsightsSection.tsx";
import { CreateInsightSection } from "./sections/CreateInsightSection.tsx";
import { ImportReadwiseSection } from "./sections/ImportReadwiseSection.tsx";
import { ImportRaindropSection } from "./sections/ImportRaindropSection.tsx";
import { KnowledgeSection } from "./sections/KnowledgeSection.tsx";
import { WeeklyPlanSection } from "./sections/WeeklyPlanSection.tsx";
import { ProfileSection } from "./sections/ProfileSection.tsx";

// The authenticated sections of the demo client. We switch between them with
// plain local state instead of a router — there are only a few "tabs" and no
// URLs worth deep-linking to, so react-router would be dead weight here.
// Login isn't a tab: it's what renders instead of this whole app when signed
// out (see the `!token` branch below), so there's nothing to gate per-section.
type SectionId =
  | "insights"
  | "create"
  | "import-readwise"
  | "import-raindrop"
  | "knowledge"
  | "weekly-plan"
  | "profile";

interface NavItem {
  id: SectionId;
  label: string;
  icon: IconName;
}

// Grouped sidebar nav (IPP-68 follow-up): the two imports and the
// browse/create insight actions were reading as seven equal-weight tabs in a
// flat row. Grouping them under headings gives the same set of destinations
// visible hierarchy without adding a router or any per-section URLs.
const NAV_GROUPS: { label: string; items: NavItem[] }[] = [
  {
    label: "Insights",
    items: [
      { id: "insights", label: "All Insights", icon: "insights" },
      { id: "create", label: "Create Insight", icon: "create" },
    ],
  },
  {
    label: "Import",
    items: [
      { id: "import-readwise", label: "Readwise", icon: "import" },
      { id: "import-raindrop", label: "Raindrop", icon: "import" },
    ],
  },
  {
    label: "Workspace",
    items: [
      { id: "knowledge", label: "Knowledge", icon: "knowledge" },
      { id: "weekly-plan", label: "Weekly Plan", icon: "plan" },
    ],
  },
];

function App() {
  const { token, logout } = useAuth();
  const [active, setActive] = useState<SectionId>("insights");

  if (!token) {
    return (
      <div className="app app--centered">
        <LoginSection />
      </div>
    );
  }

  return (
    <div className="app">
      <aside className="sidebar">
        <div className="sidebar-brand">
          <span className="sidebar-brand-mark">IPP</span>
          <span className="sidebar-brand-name">Insight Processing Platform</span>
        </div>

        <nav className="sidebar-nav">
          {NAV_GROUPS.map((group) => (
            <div className="nav-group" key={group.label}>
              <div className="nav-group-label">{group.label}</div>
              <div className="nav-group-items">
                {group.items.map((item) => (
                  <button
                    key={item.id}
                    type="button"
                    className={item.id === active ? "nav-item active" : "nav-item"}
                    onClick={() => setActive(item.id)}
                  >
                    <Icon name={item.icon} />
                    {item.label}
                  </button>
                ))}
              </div>
            </div>
          ))}
        </nav>

        <div className="sidebar-footer">
          <button
            type="button"
            className={active === "profile" ? "nav-item active" : "nav-item"}
            onClick={() => setActive("profile")}
          >
            <Icon name="profile" />
            Profile
          </button>
          <button type="button" className="nav-item nav-item--logout" onClick={logout}>
            <Icon name="logout" />
            Log out
          </button>
        </div>
      </aside>

      <main className="app-main">
        {active === "insights" && <InsightsSection />}
        {active === "create" && <CreateInsightSection />}
        {active === "import-readwise" && <ImportReadwiseSection />}
        {active === "import-raindrop" && <ImportRaindropSection />}
        {active === "knowledge" && <KnowledgeSection />}
        {active === "weekly-plan" && <WeeklyPlanSection />}
        {active === "profile" && <ProfileSection />}
      </main>
    </div>
  );
}

export default App;
