-- Vigie Database Schema
-- Single consolidated schema for initial setup
-- Character set: utf8mb4 for full Unicode support

-- =============================================================================
-- USERS
-- =============================================================================
CREATE TABLE IF NOT EXISTS users (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    username VARCHAR(255) NOT NULL UNIQUE,
    email VARCHAR(255) NOT NULL UNIQUE,
    password VARCHAR(255) NOT NULL,
    role VARCHAR(50) NOT NULL DEFAULT 'admin',
    active BOOLEAN NOT NULL DEFAULT TRUE,
    last_login TIMESTAMP NULL,
    password_reset_token VARCHAR(255) NULL,
    password_reset_expiry TIMESTAMP NULL,
    force_password_change BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL DEFAULT NULL,
    INDEX idx_users_deleted_at (deleted_at),
    INDEX idx_users_username (username),
    INDEX idx_users_email (email),
    INDEX idx_users_reset_token (password_reset_token)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- =============================================================================
-- COMPONENTS
-- =============================================================================
CREATE TABLE IF NOT EXISTS components (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'operational',
    description TEXT,
    display_order INT DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL DEFAULT NULL,
    INDEX idx_components_status (status),
    INDEX idx_components_deleted_at (deleted_at),
    INDEX idx_components_display_order (display_order),
    CONSTRAINT chk_component_status CHECK (status IN ('operational', 'degraded_performance', 'partial_outage', 'major_outage', 'under_maintenance'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- =============================================================================
-- INCIDENTS
-- =============================================================================
CREATE TABLE IF NOT EXISTS incidents (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'investigating',
    component_id BIGINT UNSIGNED,
    message TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL DEFAULT NULL,
    INDEX idx_incidents_status (status),
    INDEX idx_incidents_component_id (component_id),
    INDEX idx_incidents_deleted_at (deleted_at),
    INDEX idx_incidents_created_at (created_at),
    CONSTRAINT fk_incidents_component FOREIGN KEY (component_id) REFERENCES components(id) ON DELETE CASCADE ON UPDATE CASCADE,
    CONSTRAINT chk_incident_status CHECK (status IN ('investigating', 'identified', 'monitoring', 'resolved'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- =============================================================================
-- MAINTENANCES
-- =============================================================================
CREATE TABLE IF NOT EXISTS maintenances (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'scheduled',
    component_id BIGINT UNSIGNED NOT NULL,
    start_at TIMESTAMP NOT NULL,
    end_at TIMESTAMP NOT NULL,
    previous_component_status VARCHAR(50) NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL DEFAULT NULL,
    INDEX idx_maintenances_status (status),
    INDEX idx_maintenances_component_id (component_id),
    INDEX idx_maintenances_deleted_at (deleted_at),
    INDEX idx_maintenances_start_at (start_at),
    INDEX idx_maintenances_end_at (end_at),
    INDEX idx_maintenances_active (status, start_at, end_at),
    CONSTRAINT fk_maintenances_component FOREIGN KEY (component_id) REFERENCES components(id) ON DELETE CASCADE ON UPDATE CASCADE,
    CONSTRAINT chk_maintenance_status CHECK (status IN ('scheduled', 'in_progress', 'completed'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- =============================================================================
-- SUBSCRIBERS
-- =============================================================================
CREATE TABLE IF NOT EXISTS subscribers (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    email VARCHAR(255) NOT NULL UNIQUE,
    confirmed BOOLEAN DEFAULT FALSE,
    confirmation_token VARCHAR(255),
    unsubscribe_token VARCHAR(255) UNIQUE,
    locale VARCHAR(5) NOT NULL DEFAULT 'en',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL DEFAULT NULL,
    INDEX idx_subscribers_email (email),
    INDEX idx_subscribers_confirmed (confirmed),
    INDEX idx_subscribers_confirmation_token (confirmation_token),
    INDEX idx_subscribers_unsubscribe_token (unsubscribe_token),
    INDEX idx_subscribers_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- =============================================================================
-- TRANSLATIONS
-- =============================================================================
CREATE TABLE IF NOT EXISTS translations (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    entity_id BIGINT UNSIGNED NOT NULL,
    entity_type VARCHAR(50) NOT NULL,
    locale VARCHAR(5) NOT NULL DEFAULT 'en',
    field VARCHAR(50) NOT NULL,
    value TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_translations_entity (entity_id, entity_type),
    INDEX idx_translations_locale (locale),
    INDEX idx_translations_lookup (entity_id, entity_type, locale, field),
    UNIQUE KEY unique_translation (entity_id, entity_type, locale, field),
    CONSTRAINT chk_translation_entity CHECK (entity_type IN ('component', 'incident', 'maintenance', 'incident_update')),
    CONSTRAINT chk_translation_locale CHECK (locale IN ('en', 'fr')),
    CONSTRAINT chk_translation_field CHECK (field IN ('name', 'description', 'message'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- =============================================================================
-- SETTINGS
-- =============================================================================
CREATE TABLE IF NOT EXISTS settings (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    `key` VARCHAR(50) NOT NULL UNIQUE,
    value TEXT,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_settings_key (`key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- =============================================================================
-- INCIDENT TEMPLATES
-- =============================================================================
CREATE TABLE IF NOT EXISTS incident_templates (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    default_status VARCHAR(50) DEFAULT 'investigating',
    default_component_id BIGINT UNSIGNED NULL,
    name_en VARCHAR(500),
    name_fr VARCHAR(500),
    message_en TEXT,
    message_fr TEXT,
    variables TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL,
    INDEX idx_incident_templates_deleted_at (deleted_at),
    CONSTRAINT fk_incident_templates_component FOREIGN KEY (default_component_id) REFERENCES components(id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- =============================================================================
-- MAINTENANCE TEMPLATES
-- =============================================================================
CREATE TABLE IF NOT EXISTS maintenance_templates (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    default_component_id BIGINT UNSIGNED NULL,
    name_en TEXT,
    name_fr TEXT,
    message_en TEXT,
    message_fr TEXT,
    variables TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL DEFAULT NULL,
    INDEX idx_maintenance_templates_deleted_at (deleted_at),
    CONSTRAINT fk_maintenance_templates_component FOREIGN KEY (default_component_id) REFERENCES components(id) ON DELETE SET NULL ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- =============================================================================
-- NOTIFICATION QUEUE
-- =============================================================================
CREATE TABLE IF NOT EXISTS notification_queue (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    incident_id BIGINT UNSIGNED NOT NULL,
    subscriber_id BIGINT UNSIGNED NOT NULL,
    email VARCHAR(255) NOT NULL,
    locale VARCHAR(5) NOT NULL DEFAULT 'en',
    subject VARCHAR(500),
    body TEXT,
    status ENUM('pending', 'processing', 'sent', 'failed') NOT NULL DEFAULT 'pending',
    attempts INT NOT NULL DEFAULT 0,
    max_attempts INT NOT NULL DEFAULT 3,
    last_error TEXT,
    next_retry_at TIMESTAMP NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    processed_at TIMESTAMP NULL,
    sent_at TIMESTAMP NULL,
    INDEX idx_notification_status (status),
    INDEX idx_notification_next_retry (next_retry_at),
    INDEX idx_notification_incident (incident_id),
    INDEX idx_notification_subscriber (subscriber_id),
    INDEX idx_notification_status_retry (status, next_retry_at),
    CONSTRAINT fk_notification_incident FOREIGN KEY (incident_id) REFERENCES incidents(id) ON DELETE CASCADE,
    CONSTRAINT fk_notification_subscriber FOREIGN KEY (subscriber_id) REFERENCES subscribers(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- =============================================================================
-- AUDIT LOGS
-- =============================================================================
CREATE TABLE IF NOT EXISTS audit_logs (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT UNSIGNED NULL,
    username VARCHAR(255) NOT NULL,
    action VARCHAR(50) NOT NULL,
    resource VARCHAR(100),
    resource_id BIGINT UNSIGNED NULL,
    details TEXT,
    ip_address VARCHAR(45),
    user_agent VARCHAR(500),
    success BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_audit_user_id (user_id),
    INDEX idx_audit_action (action),
    INDEX idx_audit_resource (resource, resource_id),
    INDEX idx_audit_created_at (created_at),
    CONSTRAINT fk_audit_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- =============================================================================
-- API KEYS
-- =============================================================================
-- Permanent credentials for machine access. Only a SHA-256 hash is stored.
CREATE TABLE IF NOT EXISTS api_keys (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    key_hash VARCHAR(64) NOT NULL,
    prefix VARCHAR(16),
    user_id BIGINT UNSIGNED NOT NULL,
    last_used_at TIMESTAMP NULL,
    revoked_at TIMESTAMP NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY idx_api_keys_key_hash (key_hash),
    INDEX idx_api_keys_user_id (user_id),
    INDEX idx_api_keys_revoked_at (revoked_at),
    CONSTRAINT fk_api_keys_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- =============================================================================
-- DEFAULT DATA
-- =============================================================================

-- Default admin user (password: 'admin' - CHANGE IN PRODUCTION)
INSERT INTO users (username, email, password, role) VALUES
('admin', 'admin@example.com', '$2b$10$YDniJ/WQw4e6o.MfeMKSDeQ1xkCURkk5sTYfTffeBLPoGZ7UaYit6', 'admin');

-- Default components
INSERT INTO components (name, status, description, display_order) VALUES
('API', 'operational', 'Core API service', 1),
('Website', 'operational', 'Public website', 2),
('Database', 'operational', 'Database service', 3);

-- Component translations
INSERT INTO translations (entity_id, entity_type, locale, field, value) VALUES
(1, 'component', 'en', 'name', 'API'),
(1, 'component', 'fr', 'name', 'API'),
(1, 'component', 'en', 'description', 'Core API service'),
(1, 'component', 'fr', 'description', 'Service API principal'),
(2, 'component', 'en', 'name', 'Website'),
(2, 'component', 'fr', 'name', 'Site Web'),
(2, 'component', 'en', 'description', 'Public website'),
(2, 'component', 'fr', 'description', 'Site web public'),
(3, 'component', 'en', 'name', 'Database'),
(3, 'component', 'fr', 'name', 'Base de données'),
(3, 'component', 'en', 'description', 'Database service'),
(3, 'component', 'fr', 'description', 'Service de base de données');

-- Default settings
INSERT INTO settings (`key`, value) VALUES ('theme', 'default');

-- Default incident templates
INSERT INTO incident_templates (name, default_status, name_en, name_fr, message_en, message_fr, variables) VALUES
('Service Outage', 'investigating',
 '{{service_name}} is currently unavailable',
 '{{service_name}} est actuellement indisponible',
 'We are aware that {{service_name}} is currently experiencing an outage. Our team is actively investigating the issue and will provide updates as soon as possible.',
 'Nous sommes conscients que {{service_name}} subit actuellement une panne. Notre équipe enquête activement sur le problème et fournira des mises à jour dès que possible.',
 '[{"name":"service_name","label_en":"Service Name","label_fr":"Nom du service","type":"text","required":true}]'),
('Performance Degradation', 'investigating',
 '{{service_name}} is experiencing performance issues',
 '{{service_name}} rencontre des problèmes de performance',
 'We are investigating reports of slower than normal response times for {{service_name}}. Some users may experience delays.',
 'Nous enquêtons sur des signalements de temps de réponse plus lents que la normale pour {{service_name}}. Certains utilisateurs peuvent rencontrer des délais.',
 '[{"name":"service_name","label_en":"Service Name","label_fr":"Nom du service","type":"text","required":true}]'),
('Partial Outage', 'investigating',
 'Some features of {{service_name}} are unavailable',
 'Certaines fonctionnalités de {{service_name}} sont indisponibles',
 'We are experiencing a partial outage affecting {{affected_features}} in {{service_name}}. Other features continue to work normally.',
 'Nous subissons une panne partielle affectant {{affected_features}} dans {{service_name}}. Les autres fonctionnalités continuent de fonctionner normalement.',
 '[{"name":"service_name","label_en":"Service Name","label_fr":"Nom du service","type":"text","required":true},{"name":"affected_features","label_en":"Affected Features","label_fr":"Fonctionnalités affectées","type":"text","required":true}]'),
('Third-party Provider Issue', 'identified',
 '{{provider_name}} issues affecting {{service_name}}',
 'Problèmes {{provider_name}} affectant {{service_name}}',
 'Our upstream provider {{provider_name}} is currently experiencing issues that are impacting {{service_name}}. We are monitoring the situation and will update once the provider resolves the issue.',
 'Notre fournisseur {{provider_name}} rencontre actuellement des problèmes qui impactent {{service_name}}. Nous surveillons la situation et mettrons à jour dès que le fournisseur aura résolu le problème.',
 '[{"name":"provider_name","label_en":"Provider Name","label_fr":"Nom du fournisseur","type":"text","required":true},{"name":"service_name","label_en":"Service Name","label_fr":"Nom du service","type":"text","required":true}]');

-- Default maintenance templates
INSERT INTO maintenance_templates (name, name_en, name_fr, message_en, message_fr, variables) VALUES
('Scheduled Maintenance',
 'Scheduled Maintenance - {{service_name}}',
 'Maintenance planifiée - {{service_name}}',
 'We will be performing scheduled maintenance on {{service_name}}. {{reason}}',
 'Nous effectuerons une maintenance planifiée sur {{service_name}}. {{reason}}',
 '[{"name":"service_name","label_en":"Service Name","label_fr":"Nom du service","type":"text","required":true},{"name":"reason","label_en":"Reason","label_fr":"Raison","type":"text","required":false}]'),
('Database Maintenance',
 'Database Maintenance',
 'Maintenance de la base de données',
 'We will be performing database maintenance. Expected duration: {{duration}}. During this time, some services may be temporarily unavailable.',
 'Nous effectuerons une maintenance de la base de données. Durée prévue : {{duration}}. Pendant ce temps, certains services peuvent être temporairement indisponibles.',
 '[{"name":"duration","label_en":"Duration","label_fr":"Durée","type":"text","required":true,"default":"2 hours"}]'),
('Infrastructure Update',
 'Infrastructure Update - {{description}}',
 'Mise à jour de l''infrastructure - {{description}}',
 'We will be updating our infrastructure: {{description}}. This maintenance is necessary to improve service reliability and performance.',
 'Nous mettrons à jour notre infrastructure : {{description}}. Cette maintenance est nécessaire pour améliorer la fiabilité et les performances du service.',
 '[{"name":"description","label_en":"Description","label_fr":"Description","type":"text","required":true}]'),
('Security Update',
 'Security Update',
 'Mise à jour de sécurité',
 'We will be applying important security updates to {{service_name}}. This is a routine maintenance to ensure the security of our systems.',
 'Nous appliquerons des mises à jour de sécurité importantes sur {{service_name}}. Il s''agit d''une maintenance de routine pour assurer la sécurité de nos systèmes.',
 '[{"name":"service_name","label_en":"Service Name","label_fr":"Nom du service","type":"text","required":true}]');
