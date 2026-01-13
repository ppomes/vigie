// Dashboard layout with authentication check and navigation
// Wraps all dashboard pages with sidebar navigation and auth protection

'use client';

import { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { useTranslations } from 'next-intl';
import Link from 'next/link';
import { isAuthenticated, logout, getUserInfo, UserInfo } from '@/lib/auth';

export default function DashboardLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const router = useRouter();
  const t = useTranslations('navigation');
  const [user, setUser] = useState<UserInfo | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    // Check authentication on mount
    if (!isAuthenticated()) {
      router.push('/login');
      return;
    }

    // Get user info
    const userInfo = getUserInfo();
    setUser(userInfo);
    setLoading(false);
  }, [router]);

  const handleLogout = () => {
    logout();
  };

  // Show loading while checking authentication
  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <div className="text-gray-600">Loading...</div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-gray-100 flex">
      {/* Sidebar Navigation */}
      <aside className="w-64 bg-white shadow-md flex flex-col">
        {/* Logo/Header */}
        <div className="p-6 border-b border-gray-200">
          <h1 className="text-xl font-bold text-gray-800">Status Page</h1>
          <p className="text-sm text-gray-600 mt-1">Admin Panel</p>
        </div>

        {/* User Info */}
        {user && (
          <div className="px-6 py-4 border-b border-gray-200">
            <p className="text-sm text-gray-600">Logged in as</p>
            <p className="text-sm font-medium text-gray-800 truncate">
              {user.username}
            </p>
          </div>
        )}

        {/* Navigation Links */}
        <nav className="flex-1 px-4 py-6 space-y-2">
          <Link
            href="/"
            className="block px-4 py-2 text-gray-700 hover:bg-blue-50 hover:text-blue-600 rounded-md transition-colors"
          >
            {t('dashboard')}
          </Link>
          <Link
            href="/incidents"
            className="block px-4 py-2 text-gray-700 hover:bg-blue-50 hover:text-blue-600 rounded-md transition-colors"
          >
            {t('incidents')}
          </Link>
          <Link
            href="/components"
            className="block px-4 py-2 text-gray-700 hover:bg-blue-50 hover:text-blue-600 rounded-md transition-colors"
          >
            {t('components')}
          </Link>
          <Link
            href="/subscribers"
            className="block px-4 py-2 text-gray-700 hover:bg-blue-50 hover:text-blue-600 rounded-md transition-colors"
          >
            {t('subscribers')}
          </Link>
        </nav>

        {/* Logout Button */}
        <div className="p-4 border-t border-gray-200">
          <button
            onClick={handleLogout}
            className="w-full px-4 py-2 text-gray-700 hover:bg-red-50 hover:text-red-600 rounded-md transition-colors text-left"
          >
            {t('logout')}
          </button>
        </div>
      </aside>

      {/* Main Content Area */}
      <main className="flex-1 overflow-auto">
        <div className="max-w-7xl mx-auto p-8">
          {children}
        </div>
      </main>
    </div>
  );
}
