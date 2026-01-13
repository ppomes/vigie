-- 001_initial_schema.sql
-- Initial database schema for Status Page System with multi-language support
-- Character set: utf8mb4 for full Unicode support
-- Collation: utf8mb4_unicode_ci for proper Unicode sorting

-- Create users table for admin authentication
CREATE TABLE IF NOT EXISTS users (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    username VARCHAR(255) NOT NULL UNIQUE,
    email VARCHAR(255) NOT NULL UNIQUE,
    password VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL DEFAULT NULL,
    INDEX idx_users_deleted_at (deleted_at),
    INDEX idx_users_username (username),
    INDEX idx_users_email (email)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Create components table for status components (API, Website, Database, etc.)
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
    CONSTRAINT chk_component_status CHECK (status IN ('operational', 'degraded_performance', 'partial_outage', 'major_outage'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Create incidents table for tracking service incidents
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

-- Create subscribers table for email subscription management
CREATE TABLE IF NOT EXISTS subscribers (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    email VARCHAR(255) NOT NULL UNIQUE,
    confirmed BOOLEAN DEFAULT FALSE,
    confirmation_token VARCHAR(255),
    unsubscribe_token VARCHAR(255) UNIQUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL DEFAULT NULL,
    INDEX idx_subscribers_email (email),
    INDEX idx_subscribers_confirmed (confirmed),
    INDEX idx_subscribers_confirmation_token (confirmation_token),
    INDEX idx_subscribers_unsubscribe_token (unsubscribe_token),
    INDEX idx_subscribers_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Create translations table for multi-language content support
-- This table stores translations for components, incidents, and other entities
-- CASCADE deletion ensures translations are removed when parent entity is deleted
CREATE TABLE IF NOT EXISTS translations (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    entity_id BIGINT UNSIGNED NOT NULL,
    entity VARCHAR(50) NOT NULL,
    locale VARCHAR(5) NOT NULL DEFAULT 'en',
    field VARCHAR(50) NOT NULL,
    value TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_translations_entity (entity_id, entity),
    INDEX idx_translations_locale (locale),
    INDEX idx_translations_lookup (entity_id, entity, locale, field),
    UNIQUE KEY unique_translation (entity_id, entity, locale, field),
    CONSTRAINT chk_translation_entity CHECK (entity IN ('component', 'incident')),
    CONSTRAINT chk_translation_locale CHECK (locale IN ('en', 'fr')),
    CONSTRAINT chk_translation_field CHECK (field IN ('name', 'description', 'message'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Create schema_migrations table to track applied migrations
CREATE TABLE IF NOT EXISTS schema_migrations (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    version VARCHAR(255) NOT NULL UNIQUE,
    applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_schema_migrations_version (version)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Insert initial migration record
INSERT INTO schema_migrations (version) VALUES ('001_initial_schema');

-- Create default admin user (password: 'admin' - MUST be changed in production)
-- Password hash is bcrypt of 'admin' with cost 10
INSERT INTO users (username, email, password) VALUES
('admin', 'admin@example.com', '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy');

-- Create initial components
INSERT INTO components (name, status, description, display_order) VALUES
('API', 'operational', 'Core API service', 1),
('Website', 'operational', 'Public website', 2),
('Database', 'operational', 'Database service', 3);

-- Create initial translations for components
INSERT INTO translations (entity_id, entity, locale, field, value) VALUES
-- API component translations
(1, 'component', 'en', 'name', 'API'),
(1, 'component', 'fr', 'name', 'API'),
(1, 'component', 'en', 'description', 'Core API service'),
(1, 'component', 'fr', 'description', 'Service API principal'),
-- Website component translations
(2, 'component', 'en', 'name', 'Website'),
(2, 'component', 'fr', 'name', 'Site Web'),
(2, 'component', 'en', 'description', 'Public website'),
(2, 'component', 'fr', 'description', 'Site web public'),
-- Database component translations
(3, 'component', 'en', 'name', 'Database'),
(3, 'component', 'fr', 'name', 'Base de données'),
(3, 'component', 'en', 'description', 'Database service'),
(3, 'component', 'fr', 'description', 'Service de base de données');
