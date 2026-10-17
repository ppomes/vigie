# Importing a Cachet 2.4 database

`api import-cachet` copies a Cachet 2.4 installation into a fresh Vigie
installation. The Cachet database is only read. The full move, with DNS and
redirects, is described in the migration guide.

## Run it

Start Vigie once, so its database exists, then run the command in the API
container. The Vigie database is the one the API is configured with.

```bash
# See what would be imported, without writing anything
docker compose exec api ./api import-cachet \
  --source-driver mysql \
  --source-dsn 'cachet:password@tcp(cachet-db:3306)/cachet' \
  --dry-run

# Import
docker compose exec api ./api import-cachet \
  --source-driver mysql \
  --source-dsn 'cachet:password@tcp(cachet-db:3306)/cachet'
```

| Option | Meaning |
|---|---|
| `--source-driver` | `mysql` or `sqlite` |
| `--source-dsn` | MySQL: `user:password@tcp(host:3306)/database`. SQLite: path to the file |
| `--dry-run` | Run the whole import in a transaction, print the report, roll back |
| `--locale` | `en` or `fr`: the language of Cachet's text. Defaults to Cachet's `app_locale`, else `en` |
| `--source-timezone` | The zone Cachet wrote its dates in, for example `Europe/Paris`: `APP_TIMEZONE` in Cachet's `.env`. Defaults to `UTC`, Cachet's default |

PostgreSQL sources are not supported yet: export the Cachet database to MySQL
or SQLite first.

The import runs in one transaction: if anything fails, nothing is written.
It refuses a Vigie database that already holds incidents, maintenances,
subscribers, groups or real components. The three example components of a
fresh installation are removed.

## What is imported

| Cachet 2.4 | Vigie |
|---|---|
| component groups | groups, same ids and order. `collapsed` and `visible` are mapped |
| components | components, same ids. Description, link, enabled, meta and tags kept |
| incidents, incident updates | incidents with their timeline, same ids. Author, visibility, stickiness and start date kept |
| incident components | affected components, with the status each incident set |
| schedules, schedule components | maintenances with their components, same ids |
| subscribers, subscriptions | subscribers, same ids. Only confirmed ones stay confirmed |
| users | users, matched by username or email. Passwords keep working |
| user API keys | API keys, stored hashed. Integrations keep working |
| `app_name` | the page title |

Ids are kept so that monitoring scripts, which address components by id,
keep working, through the Cachet compatible API.

Cachet's text is single-language. It is stored in the import locale; add the
other language in the admin where needed.

## What the report lists

The report counts what was imported, then every row that was adapted or left
out, with the reason:

- **Adapted**, for example: level 2 users become viewers, since Vigie has no
  role that manages incidents without being admin; a schedule without an end
  gets one an hour after its start; incident status 0, from Cachet 1.x,
  becomes resolved; a subscriber following only deleted components follows
  everything.
- **Not imported**: rows deleted in Cachet, phone and Slack subscribers.
- **Two-factor login**: Cachet users who had it keep it, with the same entry in
  their authenticator app. They have no recovery codes yet; the report reminds
  you to ask them to make some.
- **Not supported by Vigie**: metrics, incident templates (Cachet's use Twig),
  pending invitations.
- **To do by hand**: such as setting `PUBLIC_URL` to the new address.
