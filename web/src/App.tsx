import { useState } from "react";
import { LoginSection } from "./sections/LoginSection.tsx";
import { InsightsSection } from "./sections/InsightsSection.tsx";
import { CreateInsightSection } from "./sections/CreateInsightSection.tsx";
import { ProfileSection } from "./sections/ProfileSection.tsx";

// The four sections of the demo client. We switch between them with plain
// local state instead of a router — there are only four "tabs" and no URLs
// worth deep-linking to, so react-router would be dead weight here.
type SectionId = "login" | "insights" | "create" | "profile";

const SECTIONS: { id: SectionId; label: string }[] = [
  { id: "login", label: "Login" },
  { id: "insights", label: "Insights" },
  { id: "create", label: "Create Insight" },
  { id: "profile", label: "Profile" },
];

function App() {
  const [active, setActive] = useState<SectionId>("login");

  return (
    <div className="app">
      <header className="app-header">
        <h1>Insight Processing Platform</h1>
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
      </header>

      <main className="app-main">
        {active === "login" && <LoginSection />}
        {active === "insights" && <InsightsSection />}
        {active === "create" && <CreateInsightSection />}
        {active === "profile" && <ProfileSection />}
      </main>
    </div>
  );
}

export default App;
