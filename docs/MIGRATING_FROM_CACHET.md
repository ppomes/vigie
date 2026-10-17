# Migrating from Cachet 2.4

This guide moves a Cachet 2.4 status page to Vigie, with its components,
incidents, schedules, subscribers, users and API keys. Readers, subscribers
and integrations should not notice the switch.

It takes about an hour, most of it checking. Cachet keeps running until the
DNS switch, so you can go back at any point before it.

## 1. Before you start

- Cachet 2.4, on MySQL or SQLite. For PostgreSQL, export the database to
  MySQL first.
- Vigie installed with Docker Compose or the Helm chart, on a fresh
  database: the import refuses a database that already holds data.
- A network path from the Vigie API container to the Cachet database.

Back up the Cachet database and files, and check that you can restore them.
The import only reads Cachet, but the backup is what lets you go back.

## 2. Configure Vigie

Set these variables for the API (in `.env` for Docker Compose, under
`config` for Helm, which derives them from the ingress hosts by default):

| Variable | Value |
|---|---|
| `PUBLIC_URL` | Address of the public status page, for example the Cachet one: `https://status.example.com` |
| `API_URL` | Address readers use to reach the API, in email links |
| `ADMIN_URL` | Address of the admin, where old `/dashboard` links go |
| `CACHET_API_TIMEZONE` | Cachet's `APP_TIMEZONE`, so that integrations see the same times as before. `UTC` by default |
| `CACHET_API_LOCALE` | The language of Cachet's text, as `--locale` below. `en` by default |
| `EMAIL_*` | Your email provider, as in [the README](../README.md) |

## 3. Dry run

Run the import with `--dry-run` first. It does everything in a transaction,
prints the report, and rolls back:

```bash
docker compose exec api ./api import-cachet \
  --source-driver mysql \
  --source-dsn 'cachet:password@tcp(cachet-db:3306)/cachet' \
  --dry-run
```

Read the report. [IMPORT_CACHET.md](IMPORT_CACHET.md) explains each line.
Pay attention to:

- **Users imported as viewers**: Cachet level 2 users. Make the ones who post
  incidents admins after the import.
- **Rows not imported**: deleted rows, phone and Slack subscribers, metrics
  and incident templates.
- **Two-factor login** carries over. Users who had it make recovery codes from
  their profile.
- **Text language**: Cachet's text is stored in one language. Use `--locale`
  if `app_locale` is not the language of your incidents. The public page is
  set to show that language to every visitor, so pages do not mix languages;
  visitors can still switch. Change it under Settings, Public Page Language
  once incidents are written in both languages.
- **Timezone**: Cachet stores its dates in the zone of `APP_TIMEZONE`, in its
  `.env` file, not in its database. If it is not `UTC`, add
  `--source-timezone` with that value, for example
  `--source-timezone Europe/Paris`, or every date is shifted. The report
  warns when Cachet showed its times in another zone.

## 4. Import

Run the same command without `--dry-run`. It writes everything in one
transaction: if it fails, nothing is written, and you can run it again once
the problem is fixed.

## 5. Check

Before switching, check in Vigie, at its temporary address:

- the public page shows the same components, in the same groups and order;
- recent incidents show their timeline, and internal ones stay hidden;
- upcoming maintenances are listed;
- in the admin, **Subscribers** has the same count as Cachet's confirmed
  subscribers, and **Users** has your team;
- a Cachet user can log in with their Cachet password;
- an integration works with its existing key. For example:

```bash
curl https://vigie.example.com/cachet/api/v1/components \
  -H "X-Cachet-Token: <an existing Cachet key>"
```

## 6. Update integrations

Tools that call the Cachet API keep their keys and component ids. Change
their base URL from `https://status.example.com/api/v1` to:

```
https://status.example.com/cachet/api/v1
```

[CACHET_API.md](CACHET_API.md) lists what the compatible API supports.

## 7. Switch

Point the status page hostname at the Vigie public site. Old addresses keep
working there:

| Cachet URL | Now |
|---|---|
| `/incidents/{id}` | the incident on the public page |
| `/subscribe` | the subscription form |
| `/subscribe/manage/{code}`, `/unsubscribe/{code}` | the same actions, from old subscription emails |
| `/dashboard` | the Vigie admin |
| `/rss`, `/atom` | the incident feeds |

Then delete the default `admin` account of the fresh installation, or change
its password, and remove the Cachet example settings you do not need.

## Going back

Until the DNS switch, Cachet is untouched: drop the Vigie database and start
again. After the switch, point the hostname back at Cachet. Incidents posted
in Vigie in the meantime are not copied back to Cachet.

## What does not carry over

- Metrics and their points.
- Incident templates: Cachet's use Twig. Recreate them in Vigie.
- Phone and Slack subscribers: Vigie sends email only.
- A role between admin and viewer: Cachet level 2 users become viewers.
- Cachet's "about" text.
- Custom styles carry over, but Cachet's stylesheet targets Cachet's HTML:
  check it under Settings, Customization.
