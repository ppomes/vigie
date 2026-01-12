'use client';

import { useEffect, useState } from 'react';
import { useTranslations } from 'next-intl';
import { getIncidents, Incident } from '@/lib/api';

const statusColors = {
  investigating: 'bg-red-500',
  identified: 'bg-orange-500',
  monitoring: 'bg-yellow-500',
  resolved: 'bg-green-500',
};

const statusTranslationKeys = {
  investigating: 'investigating',
  identified: 'identified',
  monitoring: 'monitoring',
  resolved: 'resolved',
};

// Format timestamp to relative time
function formatRelativeTime(timestamp: string, t: any): string {
  const date = new Date(timestamp);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffMinutes = Math.floor(diffMs / 60000);
  const diffHours = Math.floor(diffMs / 3600000);
  const diffDays = Math.floor(diffMs / 86400000);
  const diffWeeks = Math.floor(diffMs / 604800000);

  if (diffMinutes < 1) {
    return t('justNow');
  } else if (diffMinutes < 60) {
    return t('minutesAgo', { count: diffMinutes });
  } else if (diffHours < 24) {
    return t('hoursAgo', { count: diffHours });
  } else if (diffDays < 7) {
    return t('daysAgo', { count: diffDays });
  } else {
    return t('weeksAgo', { count: diffWeeks });
  }
}

// Format absolute timestamp
function formatAbsoluteTime(timestamp: string): string {
  const date = new Date(timestamp);
  return date.toLocaleString();
}

export default function IncidentTimeline() {
  const t = useTranslations('incidents');
  const tStatus = useTranslations('incidentStatus');
  const tTime = useTranslations('time');
  const tCommon = useTranslations('common');
  const tErrors = useTranslations('errors');

  const [incidents, setIncidents] = useState<Incident[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const fetchIncidents = async () => {
      try {
        setLoading(true);
        setError(null);
        const data = await getIncidents();
        setIncidents(data);
      } catch (err) {
        setError(err instanceof Error ? err.message : tErrors('loadingFailed'));
      } finally {
        setLoading(false);
      }
    };

    fetchIncidents();
  }, [tErrors]);

  if (loading) {
    return (
      <div className="bg-white rounded-lg shadow-md p-6">
        <h2 className="text-2xl font-bold text-gray-900 mb-4">
          {t('title')}
        </h2>
        <p className="text-gray-600">{tCommon('loading')}</p>
      </div>
    );
  }

  if (error) {
    return (
      <div className="bg-white rounded-lg shadow-md p-6">
        <h2 className="text-2xl font-bold text-gray-900 mb-4">
          {t('title')}
        </h2>
        <div className="bg-red-50 border border-red-200 rounded-md p-4">
          <p className="text-red-800">{error}</p>
        </div>
      </div>
    );
  }

  if (incidents.length === 0) {
    return (
      <div className="bg-white rounded-lg shadow-md p-6">
        <h2 className="text-2xl font-bold text-gray-900 mb-4">
          {t('title')}
        </h2>
        <p className="text-gray-600">{t('noIncidents')}</p>
      </div>
    );
  }

  // Separate active and resolved incidents
  const activeIncidents = incidents.filter(incident => incident.status !== 'resolved');
  const resolvedIncidents = incidents.filter(incident => incident.status === 'resolved');

  return (
    <div className="bg-white rounded-lg shadow-md p-6">
      <h2 className="text-2xl font-bold text-gray-900 mb-6">
        {t('title')}
      </h2>

      {/* Active Incidents */}
      {activeIncidents.length > 0 && (
        <div className="mb-8">
          <h3 className="text-xl font-semibold text-gray-900 mb-4">
            {t('active')}
          </h3>
          <div className="space-y-6">
            {activeIncidents.map((incident) => (
              <div
                key={incident.id}
                className="border-l-4 pl-4"
                style={{ borderColor: statusColors[incident.status].replace('bg-', '#') }}
              >
                <div className="flex items-start justify-between mb-2">
                  <div className="flex items-center space-x-3">
                    <div
                      className={`h-3 w-3 rounded-full ${statusColors[incident.status]}`}
                      aria-label={tStatus(statusTranslationKeys[incident.status])}
                    ></div>
                    <h4 className="font-semibold text-gray-900">{incident.name}</h4>
                  </div>
                  <span className={`text-xs font-medium px-2 py-1 rounded ${statusColors[incident.status]} text-white`}>
                    {tStatus(statusTranslationKeys[incident.status])}
                  </span>
                </div>

                {incident.component && (
                  <p className="text-sm text-gray-600 mb-1">
                    Component: {incident.component.name}
                  </p>
                )}

                {incident.message && (
                  <p className="text-gray-700 mb-2">{incident.message}</p>
                )}

                <div className="flex items-center space-x-4 text-sm text-gray-500">
                  <span title={formatAbsoluteTime(incident.created_at)}>
                    {t('created')}: {formatRelativeTime(incident.created_at, tTime)}
                  </span>
                  {incident.updated_at !== incident.created_at && (
                    <span title={formatAbsoluteTime(incident.updated_at)}>
                      {t('updated')}: {formatRelativeTime(incident.updated_at, tTime)}
                    </span>
                  )}
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Past Incidents */}
      {resolvedIncidents.length > 0 && (
        <div>
          <h3 className="text-xl font-semibold text-gray-900 mb-4">
            {t('past')}
          </h3>
          <div className="space-y-6">
            {resolvedIncidents.map((incident) => (
              <div
                key={incident.id}
                className="border-l-4 border-gray-300 pl-4 opacity-75"
              >
                <div className="flex items-start justify-between mb-2">
                  <div className="flex items-center space-x-3">
                    <div
                      className={`h-3 w-3 rounded-full ${statusColors[incident.status]}`}
                      aria-label={tStatus(statusTranslationKeys[incident.status])}
                    ></div>
                    <h4 className="font-semibold text-gray-900">{incident.name}</h4>
                  </div>
                  <span className={`text-xs font-medium px-2 py-1 rounded ${statusColors[incident.status]} text-white`}>
                    {tStatus(statusTranslationKeys[incident.status])}
                  </span>
                </div>

                {incident.component && (
                  <p className="text-sm text-gray-600 mb-1">
                    Component: {incident.component.name}
                  </p>
                )}

                {incident.message && (
                  <p className="text-gray-700 mb-2">{incident.message}</p>
                )}

                <div className="flex items-center space-x-4 text-sm text-gray-500">
                  <span title={formatAbsoluteTime(incident.created_at)}>
                    {t('created')}: {formatRelativeTime(incident.created_at, tTime)}
                  </span>
                  <span title={formatAbsoluteTime(incident.updated_at)}>
                    {t('resolved')}: {formatRelativeTime(incident.updated_at, tTime)}
                  </span>
                </div>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
