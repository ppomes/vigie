'use client';

import { useEffect, useState } from 'react';
import { useTranslations } from 'next-intl';
import { getComponents, Component } from '@/lib/api';

const statusColors = {
  operational: 'bg-green-500',
  degraded_performance: 'bg-yellow-500',
  partial_outage: 'bg-orange-500',
  major_outage: 'bg-red-500',
};

const statusTranslationKeys = {
  operational: 'operational',
  degraded_performance: 'degraded',
  partial_outage: 'partial_outage',
  major_outage: 'major_outage',
};

export default function ComponentStatus() {
  const t = useTranslations('components');
  const tStatus = useTranslations('componentStatus');
  const tCommon = useTranslations('common');
  const tErrors = useTranslations('errors');

  const [components, setComponents] = useState<Component[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const fetchComponents = async () => {
      try {
        setLoading(true);
        setError(null);
        const data = await getComponents();
        setComponents(data);
      } catch (err) {
        setError(err instanceof Error ? err.message : tErrors('loadingFailed'));
      } finally {
        setLoading(false);
      }
    };

    fetchComponents();
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

  if (components.length === 0) {
    return (
      <div className="bg-white rounded-lg shadow-md p-6">
        <h2 className="text-2xl font-bold text-gray-900 mb-4">
          {t('title')}
        </h2>
        <p className="text-gray-600">{t('noComponents')}</p>
      </div>
    );
  }

  return (
    <div className="bg-white rounded-lg shadow-md p-6">
      <h2 className="text-2xl font-bold text-gray-900 mb-4">
        {t('title')}
      </h2>
      <div className="space-y-4">
        {components.map((component) => (
          <div
            key={component.id}
            className="flex items-center justify-between py-3 border-b last:border-b-0"
          >
            <div className="flex items-center space-x-3">
              <div
                className={`h-4 w-4 rounded-full ${statusColors[component.status]}`}
                aria-label={tStatus(statusTranslationKeys[component.status])}
              ></div>
              <div>
                <h3 className="font-semibold text-gray-900">{component.name}</h3>
                {component.description && (
                  <p className="text-sm text-gray-600">{component.description}</p>
                )}
              </div>
            </div>
            <span className="text-sm font-medium text-gray-700">
              {tStatus(statusTranslationKeys[component.status])}
            </span>
          </div>
        ))}
      </div>
    </div>
  );
}
