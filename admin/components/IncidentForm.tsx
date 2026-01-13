'use client';

// Reusable incident form component for create and edit operations
// Supports multi-language input fields (EN/FR) for incident messages

import { useState, useEffect } from 'react';
import { useTranslations } from 'next-intl';
import { getComponents } from '@/lib/api';
import type { Component, Incident, CreateIncidentRequest, UpdateIncidentRequest } from '@/lib/api';

interface IncidentFormProps {
  incident?: Incident | null;
  onSubmit: (data: CreateIncidentRequest | UpdateIncidentRequest) => Promise<void>;
  onCancel: () => void;
  isSubmitting?: boolean;
}

export default function IncidentForm({ incident, onSubmit, onCancel, isSubmitting = false }: IncidentFormProps) {
  const t = useTranslations('incidents');
  const tForms = useTranslations('forms');
  const tIncidentStatus = useTranslations('incidentStatus');
  const tCommon = useTranslations('common');

  const [components, setComponents] = useState<Component[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Form state
  const [formData, setFormData] = useState({
    name: incident?.name || '',
    status: (incident?.status || 'investigating') as 'investigating' | 'identified' | 'monitoring' | 'resolved',
    component_id: incident?.component_id || 0,
    message_en: '',
    message_fr: '',
  });

  // Load components on mount
  useEffect(() => {
    const loadComponents = async () => {
      try {
        setLoading(true);
        setError(null);
        const componentsData = await getComponents();
        setComponents(componentsData);

        // If no incident (create mode) and components loaded, set first component as default
        if (!incident && componentsData.length > 0 && formData.component_id === 0) {
          setFormData((prev) => ({ ...prev, component_id: componentsData[0].id }));
        }
      } catch (err) {
        if (err instanceof Error) {
          setError(err.message);
        } else {
          setError('Failed to load components');
        }
      } finally {
        setLoading(false);
      }
    };

    loadComponents();
  }, []); // Only run on mount

  // Extract translations from incident (edit mode)
  useEffect(() => {
    if (incident) {
      const messageEn = incident.translations?.find(
        (t) => t.locale === 'en' && t.field === 'message'
      )?.value || '';
      const messageFr = incident.translations?.find(
        (t) => t.locale === 'fr' && t.field === 'message'
      )?.value || '';

      setFormData({
        name: incident.name,
        status: incident.status,
        component_id: incident.component_id,
        message_en: messageEn,
        message_fr: messageFr,
      });
    }
  }, [incident]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    // Build request data
    const requestData: CreateIncidentRequest | UpdateIncidentRequest = {
      name: formData.name,
      status: formData.status,
      component_id: formData.component_id,
      translations: [],
    };

    // Add translations if provided
    if (formData.message_en) {
      requestData.translations?.push({
        locale: 'en',
        field: 'message',
        value: formData.message_en,
      });
    }
    if (formData.message_fr) {
      requestData.translations?.push({
        locale: 'fr',
        field: 'message',
        value: formData.message_fr,
      });
    }

    await onSubmit(requestData);
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center p-8">
        <div className="text-gray-600">{tCommon('loading')}</div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="p-4">
        <div className="bg-red-50 border border-red-200 rounded-md p-4">
          <p className="text-red-800">{error}</p>
        </div>
      </div>
    );
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-4">
      {/* Name Field */}
      <div>
        <label htmlFor="incident-name" className="block text-sm font-medium text-gray-700 mb-1">
          {t('name')} <span className="text-red-500">*</span>
        </label>
        <input
          id="incident-name"
          type="text"
          value={formData.name}
          onChange={(e) => setFormData({ ...formData, name: e.target.value })}
          required
          disabled={isSubmitting}
          className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 disabled:opacity-50 disabled:bg-gray-100"
          placeholder="e.g., Database Connection Issue"
        />
      </div>

      {/* Status Field */}
      <div>
        <label htmlFor="incident-status" className="block text-sm font-medium text-gray-700 mb-1">
          {t('status')} <span className="text-red-500">*</span>
        </label>
        <select
          id="incident-status"
          value={formData.status}
          onChange={(e) =>
            setFormData({
              ...formData,
              status: e.target.value as typeof formData.status,
            })
          }
          disabled={isSubmitting}
          className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 disabled:opacity-50 disabled:bg-gray-100"
        >
          <option value="investigating">{tIncidentStatus('investigating')}</option>
          <option value="identified">{tIncidentStatus('identified')}</option>
          <option value="monitoring">{tIncidentStatus('monitoring')}</option>
          <option value="resolved">{tIncidentStatus('resolved')}</option>
        </select>
      </div>

      {/* Component Selection */}
      <div>
        <label htmlFor="incident-component" className="block text-sm font-medium text-gray-700 mb-1">
          {t('component')} <span className="text-red-500">*</span>
        </label>
        <select
          id="incident-component"
          value={formData.component_id}
          onChange={(e) =>
            setFormData({ ...formData, component_id: Number(e.target.value) })
          }
          required
          disabled={isSubmitting}
          className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 disabled:opacity-50 disabled:bg-gray-100"
        >
          {!incident && <option value={0}>{t('selectComponent')}</option>}
          {components.map((component) => (
            <option key={component.id} value={component.id}>
              {component.name}
            </option>
          ))}
        </select>
      </div>

      {/* Multi-language Message Fields */}
      <div className="border-t border-gray-200 pt-4">
        <h3 className="text-sm font-medium text-gray-900 mb-3">{t('message')}</h3>

        {/* English Message */}
        <div className="mb-4">
          <label htmlFor="message-en" className="block text-sm font-medium text-gray-700 mb-1">
            {t('messageEn')}
          </label>
          <textarea
            id="message-en"
            value={formData.message_en}
            onChange={(e) =>
              setFormData({ ...formData, message_en: e.target.value })
            }
            rows={3}
            disabled={isSubmitting}
            className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 disabled:opacity-50 disabled:bg-gray-100"
            placeholder="Detailed description of the incident in English..."
          />
        </div>

        {/* French Message */}
        <div>
          <label htmlFor="message-fr" className="block text-sm font-medium text-gray-700 mb-1">
            {t('messageFr')}
          </label>
          <textarea
            id="message-fr"
            value={formData.message_fr}
            onChange={(e) =>
              setFormData({ ...formData, message_fr: e.target.value })
            }
            rows={3}
            disabled={isSubmitting}
            className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 disabled:opacity-50 disabled:bg-gray-100"
            placeholder="Description détaillée de l'incident en français..."
          />
        </div>
      </div>

      {/* Form Actions */}
      <div className="flex justify-end space-x-2 pt-4 border-t border-gray-200">
        <button
          type="button"
          onClick={onCancel}
          disabled={isSubmitting}
          className="px-4 py-2 text-gray-700 bg-gray-100 rounded-md hover:bg-gray-200 disabled:opacity-50 transition-colors"
        >
          {tForms('cancel')}
        </button>
        <button
          type="submit"
          disabled={isSubmitting}
          className="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 disabled:opacity-50 transition-colors"
        >
          {isSubmitting
            ? (incident ? tForms('saving') : tForms('creating'))
            : (incident ? tForms('save') : tForms('create'))
          }
        </button>
      </div>
    </form>
  );
}
