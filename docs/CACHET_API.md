# Cachet compatible API

Vigie answers the Cachet 2.x API under `/cachet/api/v1`, so tools written
for Cachet keep working: monitoring scripts, the Python `cachet-client`,
`curl` examples from the Cachet documentation. Point them at the new base URL:

```
https://status.example.com/cachet/api/v1
```

The native Vigie API, under `/api/v1`, is unchanged.

## Authentication

Create an API key in the admin, under **API Keys**, and send it as Cachet
clients do:

```bash
curl -X PUT https://status.example.com/cachet/api/v1/components/1 \
  -H "X-Cachet-Token: <key>" -d "status=3"
```

`Authorization: Bearer <key>` works too. Keys imported from Cachet keep
working. A key acts with the role of its user: writes need an admin.

Reads need no key, as in Cachet. Anonymous readers see what the public page
shows: enabled components, public groups, visible incidents. With a key, they
see everything. A wrong key is refused rather than ignored.

## Endpoints

| Endpoint | Methods |
|---|---|
| `/ping`, `/version`, `/status` | GET |
| `/components`, `/components/{id}` | GET, POST, PUT, DELETE |
| `/components/groups`, `/components/groups/{id}` | GET, POST, PUT, DELETE |
| `/incidents`, `/incidents/{id}` | GET, POST, PUT, DELETE |
| `/incidents/{id}/updates`, `/incidents/{id}/updates/{update}` | GET, POST, PUT, DELETE |
| `/schedules`, `/schedules/{id}` | GET, POST, PUT, DELETE |

Bodies can be JSON, a form, or query parameters, as in Cachet. Lists take
`per_page` (20 by default), `page`, `sort` and `order=desc`, filters on the
main fields, and `tags[]` for components. Responses use Cachet's envelope
(`data`, `meta.pagination`) and error format (`errors`). `/version` reports
Cachet 2.4.1.

## Status values

| Cachet | Vigie |
|---|---|
| component 1, 2, 3, 4 | operational, degraded_performance, partial_outage, major_outage |
| incident 1, 2, 3, 4 | investigating, identified, monitoring, resolved |
| schedule 0, 1, 2 | scheduled, in_progress, completed |
| group `collapsed` 0, 1, 2 | never, always, while operational |
| group `visible` 0, 1 | logged-in users only, public |

## Differences from Cachet

- **Components under maintenance** read as status `0` (Unknown), with
  `status_name` "Under Maintenance". Cachet 2.4 has no maintenance status.
  Writing status `0` is refused.
- **Incident status 0** (Cachet's old "scheduled") is refused: use schedules.
- **Incident templates** (`template`, `vars`) are refused. Send `name` and
  `message`.
- **Schedules need `completed_at`**: Vigie maintenances have an end.
  Cachet's per-component schedule statuses are not kept: covered components
  are put under maintenance while the schedule runs, and restored after.
  Schedules do not notify subscribers unless `notify` is true, as in Cachet.
- **Text is single-language in Cachet.** Descriptions and messages written
  through this API are stored in English, or in the locale set by
  `CACHET_API_LOCALE` (`en` or `fr`). Other translations, added in the admin,
  are kept.
- **Groups created through this API** are visible to logged-in users only
  unless `visible=1` is sent, as in Cachet.
- Metrics, subscribers, subscriptions and incident templates have no endpoint
  yet.
