'use client';

// Dashboard home page with overview statistics
// Displays key metrics: active incidents, total components, and subscriber count

import { useEffect, useState } from 'react';
import { useTranslations } from 'next-intl';
import { getIncidents, getComponents, getSubscribers } from '@/lib/api';
import type { Incident, Component, Subscriber } from '@/lib/api';
import Link from 'next/link';

export default function DashboardPage() {
  const t = useTranslations('dashboard');
  const tIncidentStatus = useTranslations('incidentStatus');
  const tComponentStatus = useTranslations('componentStatus');

  const [incidents, setIncidents] = useState<Incident[]>([]);
  const [components, setComponents] = useState<Component[]>([]);
  const [subscribers, setSubscribers] = useState<Subscriber[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    loadDashboardData();
  }, []);

  const loadDashboardData = async () => {
    try {
      setLoading(true);
      setError(null);

      // Fetch all data in parallel
      const [incidentsData, componentsData, subscribersData] = await Promise.all([
        getIncidents(),
        getComponents(),
        getSubscribers(),
      ]);

      setIncidents(incidentsData);
      setComponents(componentsData);
      setSubscribers(subscribersData);
    } catch (err) {
      if (err instanceof Error) {
        setError(err.message);
      } else {
        setError('Failed to load dashboard data');
      }
    } finally {
      setLoading(false);
    }
  };

  // Calculate active incidents (not resolved)
  const activeIncidents = incidents.filter(
    (incident) => incident.status !== 'resolved'
  );

  // Calculate confirmed subscribers
  const confirmedSubscribers = subscribers.filter(
    (subscriber) => subscriber.confirmed
  );

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-96">
        <div className="text-gray-600">Loading...</div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="bg-red-50 border border-red-200 rounded-md p-4">
        <p className="text-red-800">{error}</p>
        <button
          onClick={loadDashboardData}
          className="mt-2 text-red-600 hover:text-red-700 underline"
        >
          Retry
        </button>
      </div>
    );
  }

  return (
    <div>
      {/* Page Header */}
      <div className="mb-8">
        <h1 className="text-3xl font-bold text-gray-900">{t('title')}</h1>
        <p className="mt-2 text-gray-600">{t('welcome')}</p>
      </div>

      {/* Stats Overview Cards */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-6 mb-8">
        {/* Active Incidents Card */}
        <Link href="/incidents">
          <div className="bg-white rounded-lg shadow p-6 hover:shadow-lg transition-shadow cursor-pointer">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm font-medium text-gray-600">
                  {t('activeIncidents')}
                </p>
                <p className="text-3xl font-bold text-gray-900 mt-2">
                  {activeIncidents.length}
                </p>
              </div>
              <div className="bg-orange-100 rounded-full p-3">
                <svg
                  className="w-8 h-8 text-orange-600"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                  />
                </svg>
              </div>
            </div>
          </div>
        </Link>

        {/* Total Components Card */}
        <Link href="/components">
          <div className="bg-white rounded-lg shadow p-6 hover:shadow-lg transition-shadow cursor-pointer">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm font-medium text-gray-600">
                  {t('totalComponents')}
                </p>
                <p className="text-3xl font-bold text-gray-900 mt-2">
                  {components.length}
                </p>
              </div>
              <div className="bg-blue-100 rounded-full p-3">
                <svg
                  className="w-8 h-8 text-blue-600"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10"
                  />
                </svg>
              </div>
            </div>
          </div>
        </Link>

        {/* Total Subscribers Card */}
        <Link href="/subscribers">
          <div className="bg-white rounded-lg shadow p-6 hover:shadow-lg transition-shadow cursor-pointer">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm font-medium text-gray-600">
                  {t('totalSubscribers')}
                </p>
                <p className="text-3xl font-bold text-gray-900 mt-2">
                  {confirmedSubscribers.length}
                </p>
              </div>
              <div className="bg-green-100 rounded-full p-3">
                <svg
                  className="w-8 h-8 text-green-600"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z"
                  />
                </svg>
              </div>
            </div>
          </div>
        </Link>
      </div>

      {/* Recent Incidents Section */}
      <div className="bg-white rounded-lg shadow">
        <div className="px-6 py-4 border-b border-gray-200 flex items-center justify-between">
          <h2 className="text-xl font-semibold text-gray-900">
            {t('recentIncidents')}
          </h2>
          <Link
            href="/incidents"
            className="text-blue-600 hover:text-blue-700 text-sm font-medium"
          >
            {t('viewAll')}
          </Link>
        </div>

        <div className="p-6">
          {incidents.length === 0 ? (
            <p className="text-gray-500 text-center py-8">
              No incidents to display
            </p>
          ) : (
            <div className="space-y-4">
              {incidents.slice(0, 5).map((incident) => (
                <div
                  key={incident.id}
                  className="flex items-center justify-between py-3 border-b border-gray-100 last:border-0"
                >
                  <div className="flex-1">
                    <h3 className="font-medium text-gray-900">
                      {incident.name}
                    </h3>
                    {incident.component && (
                      <p className="text-sm text-gray-500 mt-1">
                        {incident.component.name}
                      </p>
                    )}
                  </div>
                  <div className="flex items-center space-x-4">
                    <span
                      className={`px-3 py-1 rounded-full text-sm font-medium ${
                        incident.status === 'resolved'
                          ? 'bg-green-100 text-green-800'
                          : incident.status === 'monitoring'
                          ? 'bg-blue-100 text-blue-800'
                          : incident.status === 'identified'
                          ? 'bg-yellow-100 text-yellow-800'
                          : 'bg-red-100 text-red-800'
                      }`}
                    >
                      {tIncidentStatus(incident.status)}
                    </span>
                    <span className="text-sm text-gray-500">
                      {new Date(incident.created_at).toLocaleDateString()}
                    </span>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
