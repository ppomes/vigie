// Root locale page - redirects to dashboard
// This ensures users land on the proper dashboard route

import { redirect } from 'next/navigation';

export default function LocaleRootPage() {
  // Redirect to the dashboard (which is in the (dashboard) route group)
  redirect('/');
}
