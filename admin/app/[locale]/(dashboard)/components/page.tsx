'use client';

// Components management page - displays list of all components with CRUD operations
// Allows admin to create, edit, and delete components with status management

import { useEffect, useState } from 'react';
import { useTranslations } from 'next-intl';
import { getComponents, createComponent, updateComponent, deleteComponent } from '@/lib/api';
import type { Component, CreateComponentRequest, UpdateComponentRequest } from '@/lib/api';

export default function ComponentsPage() {
  const t = useTranslations('components');
  const tCommon = useTranslations('common');
  const tForms = useTranslations('forms');
  const tComponentStatus = useTranslations('componentStatus');

  const [components, setComponents] = useState<Component[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [showEditModal, setShowEditModal] = useState(false);
  const [showDeleteModal, setShowDeleteModal] = useState(false);
  const [selectedComponent, setSelectedComponent] = useState<Component | null>(null);
  const [submitting, setSubmitting] = useState(false);

  // Form state
  const [formData, setFormData] = useState({
    name: '',
    status: 'operational' as 'operational' | 'degraded_performance' | 'partial_outage' | 'major_outage',
    order: 0,
    name_en: '',
    name_fr: '',
    description_en: '',
    description_fr: '',
  });

  useEffect(() => {
    loadData();
  }, []);

  const loadData = async () => {
    try {
      setLoading(true);
      setError(null);

      const componentsData = await getComponents();
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
      status: 'operational',
      order: components.length,
      name_en: '',
      name_fr: '',
      description_en: '',
      description_fr: '',
    });
    setShowCreateModal(true);
  };

  const handleEditClick = (component: Component) => {
    setSelectedComponent(component);

    // Extract translations
    const nameEn = component.translations?.find(
      (t) => t.locale === 'en' && t.field === 'name'
    )?.value || '';
    const nameFr = component.translations?.find(
      (t) => t.locale === 'fr' && t.field === 'name'
    )?.value || '';
    const descriptionEn = component.translations?.find(
      (t) => t.locale === 'en' && t.field === 'description'
    )?.value || '';
    const descriptionFr = component.translations?.find(
      (t) => t.locale === 'fr' && t.field === 'description'
    )?.value || '';

    setFormData({
      name: component.name,
      status: component.status,
      order: component.order,
      name_en: nameEn,
      name_fr: nameFr,
      description_en: descriptionEn,
      description_fr: descriptionFr,
    });
    setShowEditModal(true);
  };

  const handleDeleteClick = (component: Component) => {
    setSelectedComponent(component);
    setShowDeleteModal(true);
  };

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    setSubmitting(true);

    try {
      const createData: CreateComponentRequest = {
        name: formData.name,
        status: formData.status,
        order: formData.order,
        translations: [],
      };

      // Add translations if provided
      if (formData.name_en) {
        createData.translations?.push({
          locale: 'en',
          field: 'name',
          value: formData.name_en,
        });
      }
      if (formData.name_fr) {
        createData.translations?.push({
          locale: 'fr',
          field: 'name',
          value: formData.name_fr,
        });
      }
      if (formData.description_en) {
        createData.translations?.push({
          locale: 'en',
          field: 'description',
          value: formData.description_en,
        });
      }
      if (formData.description_fr) {
        createData.translations?.push({
          locale: 'fr',
          field: 'description',
          value: formData.description_fr,
        });
      }

      await createComponent(createData);
      setShowCreateModal(false);
      await loadData();
    } catch (err) {
      if (err instanceof Error) {
        setError(err.message);
      } else {
        setError('Failed to create component');
      }
    } finally {
      setSubmitting(false);
    }
  };

  const handleUpdate = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedComponent) return;

    setSubmitting(true);

    try {
      const updateData: UpdateComponentRequest = {
        name: formData.name,
        status: formData.status,
        order: formData.order,
        translations: [],
      };

      // Add translations if provided
      if (formData.name_en) {
        updateData.translations?.push({
          locale: 'en',
          field: 'name',
          value: formData.name_en,
        });
      }
      if (formData.name_fr) {
        updateData.translations?.push({
          locale: 'fr',
          field: 'name',
          value: formData.name_fr,
        });
      }
      if (formData.description_en) {
        updateData.translations?.push({
          locale: 'en',
          field: 'description',
          value: formData.description_en,
        });
      }
      if (formData.description_fr) {
        updateData.translations?.push({
          locale: 'fr',
          field: 'description',
          value: formData.description_fr,
        });
      }

      await updateComponent(selectedComponent.id, updateData);
      setShowEditModal(false);
      await loadData();
    } catch (err) {
      if (err instanceof Error) {
        setError(err.message);
      } else {
        setError('Failed to update component');
      }
    } finally {
      setSubmitting(false);
    }
  };

  const handleDelete = async () => {
    if (!selectedComponent) return;

    setSubmitting(true);

    try {
      await deleteComponent(selectedComponent.id);
      setShowDeleteModal(false);
      await loadData();
    } catch (err) {
      if (err instanceof Error) {
        setError(err.message);
      } else {
        setError('Failed to delete component');
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
          <p className="mt-2 text-gray-600">Manage system components</p>
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

      {/* Components Table */}
      <div className="bg-white rounded-lg shadow overflow-hidden">
        <table className="min-w-full divide-y divide-gray-200">
          <thead className="bg-gray-50">
            <tr>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                {t('name')}
              </th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                {t('status')}
              </th>
              <th className="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase tracking-wider">
                {tCommon('actions')}
              </th>
            </tr>
          </thead>
          <tbody className="bg-white divide-y divide-gray-200">
            {components.length === 0 ? (
              <tr>
                <td colSpan={3} className="px-6 py-8 text-center text-gray-500">
                  {t('noComponents')}
                </td>
              </tr>
            ) : (
              components.map((component) => (
                <tr key={component.id} className="hover:bg-gray-50">
                  <td className="px-6 py-4 whitespace-nowrap">
                    <div className="text-sm font-medium text-gray-900">
                      {component.name}
                    </div>
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap">
                    <span
                      className={`px-2 py-1 inline-flex text-xs leading-5 font-semibold rounded-full ${
                        component.status === 'operational'
                          ? 'bg-green-100 text-green-800'
                          : component.status === 'degraded_performance'
                          ? 'bg-yellow-100 text-yellow-800'
                          : component.status === 'partial_outage'
                          ? 'bg-orange-100 text-orange-800'
                          : 'bg-red-100 text-red-800'
                      }`}
                    >
                      {tComponentStatus(component.status)}
                    </span>
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-right text-sm font-medium space-x-2">
                    <button
                      onClick={() => handleEditClick(component)}
                      className="text-blue-600 hover:text-blue-900"
                    >
                      {tForms('edit')}
                    </button>
                    <button
                      onClick={() => handleDeleteClick(component)}
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
                  <option value="operational">{tComponentStatus('operational')}</option>
                  <option value="degraded_performance">{tComponentStatus('degraded')}</option>
                  <option value="partial_outage">{tComponentStatus('partial_outage')}</option>
                  <option value="major_outage">{tComponentStatus('major_outage')}</option>
                </select>
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  {t('nameEn')}
                </label>
                <input
                  type="text"
                  value={formData.name_en}
                  onChange={(e) =>
                    setFormData({ ...formData, name_en: e.target.value })
                  }
                  className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
                />
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  {t('nameFr')}
                </label>
                <input
                  type="text"
                  value={formData.name_fr}
                  onChange={(e) =>
                    setFormData({ ...formData, name_fr: e.target.value })
                  }
                  className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
                />
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  {t('descriptionEn')}
                </label>
                <textarea
                  value={formData.description_en}
                  onChange={(e) =>
                    setFormData({ ...formData, description_en: e.target.value })
                  }
                  rows={3}
                  className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
                />
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  {t('descriptionFr')}
                </label>
                <textarea
                  value={formData.description_fr}
                  onChange={(e) =>
                    setFormData({ ...formData, description_fr: e.target.value })
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
      {showEditModal && selectedComponent && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
          <div className="bg-white rounded-lg p-6 w-full max-w-2xl max-h-[90vh] overflow-y-auto">
            <h2 className="text-2xl font-bold mb-4">{t('editComponent')}</h2>
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
                  <option value="operational">{tComponentStatus('operational')}</option>
                  <option value="degraded_performance">{tComponentStatus('degraded')}</option>
                  <option value="partial_outage">{tComponentStatus('partial_outage')}</option>
                  <option value="major_outage">{tComponentStatus('major_outage')}</option>
                </select>
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  {t('nameEn')}
                </label>
                <input
                  type="text"
                  value={formData.name_en}
                  onChange={(e) =>
                    setFormData({ ...formData, name_en: e.target.value })
                  }
                  className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
                />
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  {t('nameFr')}
                </label>
                <input
                  type="text"
                  value={formData.name_fr}
                  onChange={(e) =>
                    setFormData({ ...formData, name_fr: e.target.value })
                  }
                  className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
                />
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  {t('descriptionEn')}
                </label>
                <textarea
                  value={formData.description_en}
                  onChange={(e) =>
                    setFormData({ ...formData, description_en: e.target.value })
                  }
                  rows={3}
                  className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
                />
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  {t('descriptionFr')}
                </label>
                <textarea
                  value={formData.description_fr}
                  onChange={(e) =>
                    setFormData({ ...formData, description_fr: e.target.value })
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
      {showDeleteModal && selectedComponent && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
          <div className="bg-white rounded-lg p-6 w-full max-w-md">
            <h2 className="text-xl font-bold mb-4">{t('deleteComponent')}</h2>
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
