import { BrowserRouter, Route, Routes } from "react-router-dom";

import { Layout } from "./components/Layout";
import { DashboardView } from "./views/DashboardView";
import { StatsView } from "./views/StatsView";
import { RecordsView } from "./views/RecordsView";
import { GearView } from "./views/GearView";
import { WorkoutDetailView } from "./views/WorkoutDetailView";

// The client-side route tree, shared by the app shell and the routing tests
// (which mount it inside a MemoryRouter).
export function AppRoutes() {
  return (
    <Routes>
      <Route element={<Layout />}>
        <Route index element={<DashboardView />} />
        <Route path="stats" element={<StatsView />} />
        <Route path="records" element={<RecordsView />} />
        <Route path="gear" element={<GearView />} />
        <Route path="workouts/:id" element={<WorkoutDetailView />} />
      </Route>
    </Routes>
  );
}

// The SPA is a multi-route app served from `/`. The server's SPA fallback serves
// index.html for any non-API GET (and keeps the JSON 404 under /api/v1), so
// deep-linking and reloads on any of these client-side routes resolve here.
export function App() {
  return (
    <BrowserRouter>
      <AppRoutes />
    </BrowserRouter>
  );
}
