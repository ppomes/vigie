'use client';

// Create new incident page
// Uses the IncidentForm component for data entry

import { useRouter } from 'next/navigation';
import { useTranslations } from 'next-intl';
import { useState } from 'react';
import IncidentForm from '@/components/IncidentForm';
import { createIncident } from '@/lib/api';
import type { CreateIncidentRequest, UpdateIncidentRequest } from '@/lib/api';

export default function NewIncidentPage() {
  const router = useRouter();
  const t = useTranslations('incidents');
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = async (data: CreateIncidentRequest | UpdateIncidentRequest) => {
    try {
      setSubmitting(true);
      setError(null);
      // In create mode, the form always provides all required fields
      await createIncident(data as CreateIncidentRequest);
      router.push('/incidents');
    } catch (err) {
      if (err instanceof Error) {
        setError(err.message);
      } else {
        setError('Failed to create incident');
      }
      setSubmitting(false);
    }
  };

  const handleCancel = () => {
    router.push('/incidents');
  };

  return (
    <div className="max-w-3xl">
      {/* Page Header */}
      <div className="mb-8">
        <h1 className="text-3xl font-bold text-gray-900">{t('createNew')}</h1>
        <p className="mt-2 text-gray-600">Create a new incident to notify subscribers</p>
      </div>

      {/* Error Message */}
      {error && (
        <div className="mb-6 bg-red-50 border border-red-200 rounded-md p-4">
          <p className="text-red-800">{error}</p>
        </div>
      )}

      {/* Form */}
      <div className="bg-white rounded-lg shadow-md p-6">
        <IncidentForm
          onSubmit={handleSubmit}
          onCancel={handleCancel}
          isSubmitting={submitting}
        />
      </div>
    </div>
  );
}
