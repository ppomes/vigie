-- A small Cachet 2.4 installation covering the cases the importer handles.
-- Portable between SQLite and MySQL.

INSERT INTO settings (name, value, created_at, updated_at) VALUES
  ('app_name', 'Acme Status', '2024-01-01 10:00:00', '2024-01-01 10:00:00'),
  ('app_domain', 'https://status.acme.test', '2024-01-01 10:00:00', '2024-01-01 10:00:00'),
  ('app_locale', 'fr', '2024-01-01 10:00:00', '2024-01-01 10:00:00'),
  ('app_about', 'About Acme', '2024-01-01 10:00:00', '2024-01-01 10:00:00');

INSERT INTO component_groups (id, name, created_at, updated_at, `order`, collapsed, visible) VALUES
  (1, 'Web', '2024-01-01 10:00:00', '2024-01-01 10:00:00', 2, 2, 1),
  (2, 'Internal', '2024-01-01 10:00:00', '2024-01-01 10:00:00', 1, 0, 0);

INSERT INTO components (id, name, description, status, `order`, group_id, created_at, updated_at, deleted_at, enabled, meta, link) VALUES
  (1, 'API', 'Public API', 2, 1, 1, '2024-01-01 10:00:00', '2024-01-02 10:00:00', NULL, 1, '{"zabbix_host":"api-01"}', 'https://api.acme.test'),
  (2, 'Website', '', 1, 2, 1, '2024-01-01 10:00:00', '2024-01-01 10:00:00', NULL, 1, NULL, NULL),
  (3, 'Billing DB', 'Internal database', 4, 1, 2, '2024-01-01 10:00:00', '2024-01-01 10:00:00', NULL, 0, '[]', NULL),
  (4, 'Old service', 'Gone', 1, 9, 0, '2024-01-01 10:00:00', '2024-01-01 10:00:00', '2024-02-01 10:00:00', 1, NULL, NULL),
  (5, 'Mystery', '', 0, 3, 0, '2024-01-01 10:00:00', '2024-01-01 10:00:00', NULL, 1, NULL, NULL);

INSERT INTO tags (id, name, slug, created_at, updated_at) VALUES
  (1, 'Production', 'production', NULL, NULL),
  (2, 'EU West', 'eu-west', NULL, NULL);
INSERT INTO taggables (tag_id, taggable_type, taggable_id, created_at, updated_at) VALUES
  (1, 'components', 1, NULL, NULL),
  (2, 'components', 1, NULL, NULL),
  (1, 'components', 4, NULL, NULL);

-- Passwords are PHP bcrypt hashes of "secret".
INSERT INTO users (id, username, password, remember_token, email, api_key, active, level, created_at, updated_at, google_2fa_secret, welcomed) VALUES
  (1, 'pierre', '$2y$10$BqFQR94/w2QY/r1Y6recn.xqBVuzzNRhbjklg0.7tFjngtZPUv2vi', NULL, 'Pierre@Acme.test', 'Ab3dEf6hIj9kLm2nOp5q', 1, 1, '2024-01-01 10:00:00', '2024-01-01 10:00:00', 'JBSWY3DPEHPK3PXP', 1),
  (2, 'bob', '$2y$10$BqFQR94/w2QY/r1Y6recn.xqBVuzzNRhbjklg0.7tFjngtZPUv2vi', NULL, 'bob@acme.test', 'Zz9yXw8vUt7sRq6pOn5m', 1, 2, '2024-01-01 10:00:00', '2024-01-01 10:00:00', NULL, 1),
  (3, 'admin', '$2y$10$BqFQR94/w2QY/r1Y6recn.xqBVuzzNRhbjklg0.7tFjngtZPUv2vi', NULL, 'root@acme.test', '', 1, 1, '2024-01-01 10:00:00', '2024-01-01 10:00:00', NULL, 1);

INSERT INTO incidents (id, component_id, name, status, message, created_at, updated_at, deleted_at, visible, stickied, occurred_at, user_id, notifications) VALUES
  (1, 1, 'API slow', 2, 'Looking into it', '2024-03-01 09:05:00', '2024-03-01 10:00:00', NULL, 1, 1, '2024-03-01 09:00:00', 1, 1),
  (2, 0, 'DNS change', 4, 'Planned DNS change', '2024-03-02 09:00:00', '2024-03-02 09:00:00', NULL, 1, 0, NULL, 2, 0),
  (3, 3, 'Billing DB down', 1, 'Internal only', '2024-03-03 09:00:00', '2024-03-03 09:00:00', NULL, 0, 0, '2024-03-03 08:55:00', NULL, 0),
  (4, 1, 'Deleted incident', 1, 'x', '2024-03-04 09:00:00', '2024-03-04 09:00:00', '2024-03-05 09:00:00', 1, 0, NULL, 1, 1),
  (5, 0, 'Legacy scheduled', 0, 'Cachet 1.x maintenance', '2015-01-01 09:00:00', '2015-01-01 09:00:00', NULL, 1, 0, NULL, NULL, 0),
  (6, 0, 'Both down', 1, 'API and website', '2024-03-06 09:00:00', '2024-03-06 09:00:00', NULL, 1, 0, NULL, 1, 1);

INSERT INTO incident_components (incident_id, component_id, status_id, created_at, updated_at) VALUES
  (1, 1, 2, NULL, NULL),
  (6, 1, 4, NULL, NULL),
  (6, 2, 3, NULL, NULL),
  (6, 4, 4, NULL, NULL);

INSERT INTO incident_updates (id, incident_id, status, message, user_id, created_at, updated_at) VALUES
  (1, 1, 3, 'Fix deployed', 1, '2024-03-01 10:00:00', '2024-03-01 10:00:00'),
  (2, 1, 4, 'Resolved', 2, '2024-03-01 11:00:00', '2024-03-01 11:00:00'),
  (3, 4, 4, 'Update of a deleted incident', 1, '2024-03-04 10:00:00', '2024-03-04 10:00:00');

INSERT INTO schedules (id, name, message, status, scheduled_at, completed_at, created_at, updated_at, deleted_at) VALUES
  (1, 'DB upgrade', 'Upgrading MySQL', 2, '2024-04-01 02:00:00', '2024-04-01 03:00:00', '2024-03-20 10:00:00', '2024-04-01 03:00:00', NULL),
  (2, 'Office move', NULL, 0, '2030-05-01 08:00:00', NULL, '2024-03-20 10:00:00', '2024-03-20 10:00:00', NULL),
  (3, 'Deleted schedule', 'x', 0, '2030-06-01 08:00:00', '2030-06-01 09:00:00', '2024-03-20 10:00:00', '2024-03-20 10:00:00', '2024-03-21 10:00:00');

INSERT INTO schedule_components (schedule_id, component_id, component_status, created_at, updated_at) VALUES
  (1, 1, 2, NULL, NULL),
  (1, 3, 2, NULL, NULL);

INSERT INTO subscribers (id, verify_code, verified_at, created_at, updated_at, global, email, phone_number, slack_webhook_url) VALUES
  (1, 'code1', '2024-01-05 10:00:00', '2024-01-05 09:00:00', '2024-01-05 10:00:00', 1, 'All@Example.test', NULL, NULL),
  (2, 'code2', '2024-01-06 10:00:00', '2024-01-06 09:00:00', '2024-01-06 10:00:00', 0, 'api-fan@example.test', NULL, NULL),
  (3, 'code3', NULL, '2024-01-07 09:00:00', '2024-01-07 09:00:00', 1, 'unconfirmed@example.test', NULL, NULL),
  (4, 'code4', '2024-01-08 10:00:00', '2024-01-08 09:00:00', '2024-01-08 10:00:00', 1, NULL, '+33600000000', NULL),
  (5, 'code5', '2024-01-09 10:00:00', '2024-01-09 09:00:00', '2024-01-09 10:00:00', 0, 'gone-fan@example.test', NULL, NULL);

INSERT INTO subscriptions (subscriber_id, component_id, created_at, updated_at) VALUES
  (2, 1, NULL, NULL),
  (5, 4, NULL, NULL);

INSERT INTO metrics (id, name, suffix, description, default_value, calc_type, display_chart, created_at, updated_at, places, default_view, threshold, `order`, visible) VALUES
  (1, 'Response time', 'ms', 'API latency', 0, 0, 1, NULL, NULL, 2, 1, 5, 0, 1);
INSERT INTO metric_points (metric_id, created_at, updated_at, value, counter) VALUES
  (1, '2024-03-01 10:00:00', '2024-03-01 10:00:00', 120, 1),
  (1, '2024-03-01 10:01:00', '2024-03-01 10:01:00', 130, 1);
INSERT INTO incident_templates (name, slug, template, created_at, updated_at, deleted_at) VALUES
  ('Outage', 'outage', '{{ name }} is down', NULL, NULL, NULL);
