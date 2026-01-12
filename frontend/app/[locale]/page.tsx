'use client';

import { useTranslations } from 'next-intl';

export default function StatusPage() {
  const t = useTranslations('status');

  return (
    <div className="min-h-screen bg-gray-50">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-12">
        {/* Header */}
        <div className="text-center mb-12">
          <h1 className="text-4xl font-bold text-gray-900 mb-4">
            {t('title')}
          </h1>
          <p className="text-lg text-gray-600">
            {t('description')}
          </p>
        </div>

        {/* Overall Status */}
        <div className="bg-white rounded-lg shadow-md p-6 mb-8">
          <div className="flex items-center justify-between">
            <div className="flex items-center">
              <div className="h-4 w-4 bg-green-500 rounded-full mr-3"></div>
              <span className="text-xl font-semibold text-gray-900">
                {t('allSystemsOperational')}
              </span>
            </div>
          </div>
        </div>

        {/* Components Section Placeholder */}
        <div className="bg-white rounded-lg shadow-md p-6 mb-8">
          <h2 className="text-2xl font-bold text-gray-900 mb-4">
            Components
          </h2>
          <p className="text-gray-600">Component list will be displayed here.</p>
        </div>

        {/* Incidents Section Placeholder */}
        <div className="bg-white rounded-lg shadow-md p-6">
          <h2 className="text-2xl font-bold text-gray-900 mb-4">
            Incidents
          </h2>
          <p className="text-gray-600">Recent incidents will be displayed here.</p>
        </div>
      </div>
    </div>
  );
}
