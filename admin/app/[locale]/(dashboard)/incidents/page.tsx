'use client';

// Incidents management page - displays list of all incidents with CRUD operations
// Allows admin to create, edit, and delete incidents

import { useEffect, useState } from 'react';
import { useTranslations } from 'next-intl';
import { getIncidents, getComponents, createIncident, updateIncident, deleteIncident } from '@/lib/api';
import type { Incident, Component, CreateIncidentRequest, UpdateIncidentRequest } from '@/lib/api';

export default function IncidentsPage() {
  const t = useTranslations('incidents');
  const tCommon = useTranslations('common');
  const tForms = useTranslations('forms');
  const tIncidentStatus = useTranslations('incidentStatus');
  const tComponentStatus = useTranslations('componentStatus');

  const [incidents, setIncidents] = useState<Incident[]>([]);
  const [components, setComponents] = useState<Component[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [showEditModal, setShowEditModal] = useState(false);
  const [showDeleteModal, setShowDeleteModal] = useState(false);
  const [selectedIncident, setSelectedIncident] = useState<Incident | null>(null);
  const [submitting, setSubmitting] = useState(false);

  // Form state
  const [formData, setFormData] = useState({
    name: '',
    status: 'investigating' as 'investigating' | 'identified' | 'monitoring' | 'resolved',
    component_id: 0,
    message_en: '',
    message_fr: '',
  });

  useEffect(() => {
    loadData();
  }, []);

  const loadData = async () => {
    try {
      setLoading(true);
      setError(null);

      const [incidentsData, componentsData] = await Promise.all([
        getIncidents(),
        getComponents(),
      ]);

      setIncidents(incidentsData);
      setComponents(componentsData);
    } catch (err) {
      if (err instanceof Error) {
        setError(err.message);
      } else {
        setError('Failed to load data');
      }
    } finally {
      setLoading(false);
    }
  };

  const handleCreateClick = () => {
    setFormData({
      name: '',
      status: 'investigating',
      component_id: components.length > 0 ? components[0].id : 0,
      message_en: '',
      message_fr: '',
    });
    setShowCreateModal(true);
  };

  const handleEditClick = (incident: Incident) => {
    setSelectedIncident(incident);

    // Extract translations
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
    setShowEditModal(true);
  };

  const handleDeleteClick = (incident: Incident) => {
    setSelectedIncident(incident);
    setShowDeleteModal(true);
  };

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    setSubmitting(true);

    try {
      const createData: CreateIncidentRequest = {
        name: formData.name,
        status: formData.status,
        component_id: formData.component_id,
        translations: [],
      };

      // Add translations if provided
      if (formData.message_en) {
        createData.translations?.push({
          locale: 'en',
          field: 'message',
          value: formData.message_en,
        });
      }
      if (formData.message_fr) {
        createData.translations?.push({
          locale: 'fr',
          field: 'message',
          value: formData.message_fr,
        });
      }

      await createIncident(createData);
      setShowCreateModal(false);
      await loadData();
    } catch (err) {
      if (err instanceof Error) {
        setError(err.message);
      } else {
        setError('Failed to create incident');
      }
    } finally {
      setSubmitting(false);
    }
  };

  const handleUpdate = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedIncident) return;

    setSubmitting(true);

    try {
      const updateData: UpdateIncidentRequest = {
        name: formData.name,
        status: formData.status,
        component_id: formData.component_id,
        translations: [],
      };

      // Add translations if provided
      if (formData.message_en) {
        updateData.translations?.push({
          locale: 'en',
          field: 'message',
          value: formData.message_en,
        });
      }
      if (formData.message_fr) {
        updateData.translations?.push({
          locale: 'fr',
          field: 'message',
          value: formData.message_fr,
        });
      }

      await updateIncident(selectedIncident.id, updateData);
      setShowEditModal(false);
      await loadData();
    } catch (err) {
      if (err instanceof Error) {
        setError(err.message);
      } else {
        setError('Failed to update incident');
      }
    } finally {
      setSubmitting(false);
    }
  };

  const handleDelete = async () => {
    if (!selectedIncident) return;

    setSubmitting(true);

    try {
      await deleteIncident(selectedIncident.id);
      setShowDeleteModal(false);
      await loadData();
    } catch (err) {
      if (err instanceof Error) {
        setError(err.message);
      } else {
        setError('Failed to delete incident');
      }
    } finally {
      setSubmitting(false);
    }
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-96">
        <div className="text-gray-600">{tCommon('loading')}</div>
      </div>
    );
  }

  return (
    <div>
      {/* Page Header */}
      <div className="mb-8 flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold text-gray-900">{t('title')}</h1>
          <p className="mt-2 text-gray-600">Manage system incidents</p>
        </div>
        <button
          onClick={handleCreateClick}
          className="bg-blue-600 text-white px-4 py-2 rounded-md hover:bg-blue-700 transition-colors"
        >
          {t('createNew')}
        </button>
      </div>

      {/* Error Message */}
      {error && (
        <div className="mb-6 bg-red-50 border border-red-200 rounded-md p-4">
          <p className="text-red-800">{error}</p>
          <button
            onClick={loadData}
            className="mt-2 text-red-600 hover:text-red-700 underline"
          >
            {tCommon('retry')}
          </button>
        </div>
      )}

      {/* Incidents Table */}
      <div className="bg-white rounded-lg shadow overflow-hidden">
        <table className="min-w-full divide-y divide-gray-200">
          <thead className="bg-gray-50">
            <tr>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                {t('name')}
              </th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                {t('component')}
              </th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                {t('status')}
              </th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                {t('createdAt')}
              </th>
              <th className="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase tracking-wider">
                {tCommon('actions')}
              </th>
            </tr>
          </thead>
          <tbody className="bg-white divide-y divide-gray-200">
            {incidents.length === 0 ? (
              <tr>
                <td colSpan={5} className="px-6 py-8 text-center text-gray-500">
                  {t('noIncidents')}
                </td>
              </tr>
            ) : (
              incidents.map((incident) => (
                <tr key={incident.id} className="hover:bg-gray-50">
                  <td className="px-6 py-4 whitespace-nowrap">
                    <div className="text-sm font-medium text-gray-900">
                      {incident.name}
                    </div>
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap">
                    <div className="text-sm text-gray-900">
                      {incident.component?.name || '-'}
                    </div>
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap">
                    <span
                      className={`px-2 py-1 inline-flex text-xs leading-5 font-semibold rounded-full ${
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
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                    {new Date(incident.created_at).toLocaleDateString()}
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-right text-sm font-medium space-x-2">
                    <button
                      onClick={() => handleEditClick(incident)}
                      className="text-blue-600 hover:text-blue-900"
                    >
                      {tForms('edit')}
                    </button>
                    <button
                      onClick={() => handleDeleteClick(incident)}
                      className="text-red-600 hover:text-red-900"
                    >
                      {tForms('delete')}
                    </button>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      {/* Create Modal */}
      {showCreateModal && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
          <div className="bg-white rounded-lg p-6 w-full max-w-2xl max-h-[90vh] overflow-y-auto">
            <h2 className="text-2xl font-bold mb-4">{t('createNew')}</h2>
            <form onSubmit={handleCreate} className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  {t('name')}
                </label>
                <input
                  type="text"
                  value={formData.name}
                  onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                  required
                  className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
                />
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  {t('status')}
                </label>
                <select
                  value={formData.status}
                  onChange={(e) =>
                    setFormData({
                      ...formData,
                      status: e.target.value as typeof formData.status,
                    })
                  }
                  className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
                >
                  <option value="investigating">{tIncidentStatus('investigating')}</option>
                  <option value="identified">{tIncidentStatus('identified')}</option>
                  <option value="monitoring">{tIncidentStatus('monitoring')}</option>
                  <option value="resolved">{tIncidentStatus('resolved')}</option>
                </select>
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  {t('component')}
                </label>
                <select
                  value={formData.component_id}
                  onChange={(e) =>
                    setFormData({ ...formData, component_id: Number(e.target.value) })
                  }
                  required
                  className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
                >
                  <option value={0}>{t('selectComponent')}</option>
                  {components.map((component) => (
                    <option key={component.id} value={component.id}>
                      {component.name}
                    </option>
                  ))}
                </select>
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  {t('messageEn')}
                </label>
                <textarea
                  value={formData.message_en}
                  onChange={(e) =>
                    setFormData({ ...formData, message_en: e.target.value })
                  }
                  rows={3}
                  className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
                />
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  {t('messageFr')}
                </label>
                <textarea
                  value={formData.message_fr}
                  onChange={(e) =>
                    setFormData({ ...formData, message_fr: e.target.value })
                  }
                  rows={3}
                  className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
                />
              </div>

              <div className="flex justify-end space-x-2 pt-4">
                <button
                  type="button"
                  onClick={() => setShowCreateModal(false)}
                  disabled={submitting}
                  className="px-4 py-2 text-gray-700 bg-gray-100 rounded-md hover:bg-gray-200 disabled:opacity-50"
                >
                  {tForms('cancel')}
                </button>
                <button
                  type="submit"
                  disabled={submitting}
                  className="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 disabled:opacity-50"
                >
                  {submitting ? tForms('creating') : tForms('create')}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Edit Modal */}
      {showEditModal && selectedIncident && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
          <div className="bg-white rounded-lg p-6 w-full max-w-2xl max-h-[90vh] overflow-y-auto">
            <h2 className="text-2xl font-bold mb-4">{t('editIncident')}</h2>
            <form onSubmit={handleUpdate} className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  {t('name')}
                </label>
                <input
                  type="text"
                  value={formData.name}
                  onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                  required
                  className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
                />
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  {t('status')}
                </label>
                <select
                  value={formData.status}
                  onChange={(e) =>
                    setFormData({
                      ...formData,
                      status: e.target.value as typeof formData.status,
                    })
                  }
                  className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
                >
                  <option value="investigating">{tIncidentStatus('investigating')}</option>
                  <option value="identified">{tIncidentStatus('identified')}</option>
                  <option value="monitoring">{tIncidentStatus('monitoring')}</option>
                  <option value="resolved">{tIncidentStatus('resolved')}</option>
                </select>
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  {t('component')}
                </label>
                <select
                  value={formData.component_id}
                  onChange={(e) =>
                    setFormData({ ...formData, component_id: Number(e.target.value) })
                  }
                  required
                  className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
                >
                  {components.map((component) => (
                    <option key={component.id} value={component.id}>
                      {component.name}
                    </option>
                  ))}
                </select>
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  {t('messageEn')}
                </label>
                <textarea
                  value={formData.message_en}
                  onChange={(e) =>
                    setFormData({ ...formData, message_en: e.target.value })
                  }
                  rows={3}
                  className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
                />
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  {t('messageFr')}
                </label>
                <textarea
                  value={formData.message_fr}
                  onChange={(e) =>
                    setFormData({ ...formData, message_fr: e.target.value })
                  }
                  rows={3}
                  className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
                />
              </div>

              <div className="flex justify-end space-x-2 pt-4">
                <button
                  type="button"
                  onClick={() => setShowEditModal(false)}
                  disabled={submitting}
                  className="px-4 py-2 text-gray-700 bg-gray-100 rounded-md hover:bg-gray-200 disabled:opacity-50"
                >
                  {tForms('cancel')}
                </button>
                <button
                  type="submit"
                  disabled={submitting}
                  className="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 disabled:opacity-50"
                >
                  {submitting ? tForms('saving') : tForms('save')}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Delete Confirmation Modal */}
      {showDeleteModal && selectedIncident && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
          <div className="bg-white rounded-lg p-6 w-full max-w-md">
            <h2 className="text-xl font-bold mb-4">{t('deleteIncident')}</h2>
            <p className="text-gray-600 mb-6">{t('deleteConfirm')}</p>
            <div className="flex justify-end space-x-2">
              <button
                onClick={() => setShowDeleteModal(false)}
                disabled={submitting}
                className="px-4 py-2 text-gray-700 bg-gray-100 rounded-md hover:bg-gray-200 disabled:opacity-50"
              >
                {tForms('cancel')}
              </button>
              <button
                onClick={handleDelete}
                disabled={submitting}
                className="px-4 py-2 bg-red-600 text-white rounded-md hover:bg-red-700 disabled:opacity-50"
              >
                {submitting ? tForms('deleting') : tForms('delete')}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
