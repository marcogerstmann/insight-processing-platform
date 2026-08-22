import { useState } from "react";
import { useAuth } from "./auth/AuthContext.tsx";
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

const SECTIONS: { id: SectionId; label: string }[] = [
  { id: "insights", label: "Insights" },
  { id: "create", label: "Create Insight" },
  { id: "import-readwise", label: "Import Readwise Highlights" },
  { id: "import-raindrop", label: "Import Raindrop Highlights" },
  { id: "knowledge", label: "Knowledge" },
  { id: "weekly-plan", label: "Weekly Plan" },
  { id: "profile", label: "Profile" },
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
      <header className="app-header">
        <h1>Insight Processing Platform</h1>
        <div className="app-header-actions">
          <nav className="app-nav">
            {SECTIONS.map((section) => (
              <button
                key={section.id}
                type="button"
                className={section.id === active ? "active" : ""}
                onClick={() => setActive(section.id)}
              >
                {section.label}
              </button>
            ))}
          </nav>
          <button type="button" className="logout-btn" onClick={logout}>
            Log out
          </button>
        </div>
      </header>

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
