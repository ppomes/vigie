// Dashboard home page with overview statistics
// Displays key metrics: active incidents, total components, and subscriber count
// Server component that renders the client component to avoid Next.js 15.5.9 route group bug

import DashboardClient from './DashboardClient';

export default function DashboardPage() {
  return <DashboardClient />;
}
