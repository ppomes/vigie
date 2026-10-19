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
| `--locale` | `en` or `fr`: the language of Cachet's text. Defaults to the language most of the text is written in, else Cachet's `app_locale`, else `en` |
| `--source-timezone` | The zone Cachet wrote its dates in, for example `Europe/Paris`: `APP_TIMEZONE` in Cachet's `.env`. Defaults to `UTC`, Cachet's default |
| `--translate` | Translate Cachet's text into the other language of the page with a local model, see below |
| `--translator-url` | Ollama's address. Defaults to `$OLLAMA_URL`, else `http://ollama:11434` |
| `--translator-model` | The model. Defaults to `qwen2.5:7b` |
| `--translate-since` | `YYYY-MM-DD`: leave incidents and maintenances older than this date untranslated |
| `--translate-cache` | The file that keeps translations between runs. Defaults to `vigie-translation-cache.json` in the temporary directory |

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
| `app_banner` | the logo, if it is under 500 KB |
| `stylesheet`, `style_*` colours | the custom CSS: colours become the theme's CSS variables, the stylesheet follows |
| `header`, `footer` | the custom header and footer, sanitized (no scripts, event handlers, forms or frames) |

Cachet's stylesheet was written for Cachet's page, whose HTML is not Vigie's:
its selectors may match nothing. Check it under Settings, Customization. The
`style_greys`, `style_metrics`, banner padding and full-width header settings
have no equivalent.

Ids are kept so that monitoring scripts, which address components by id,
keep working, through the Cachet compatible API.

Cachet's text is single-language. It is stored in the import locale; add the
other language in the admin where needed, or let the import translate it (see
below). The import also sets the public page's default language to the import
locale, in "always the default language" mode, unless Vigie already has a
language setting.

The import locale is the language most of the text is written in, when the
text makes it clear: a Cachet left on its default `app_locale`, `en`, often
holds French text. The report says when it does not follow `app_locale`.
`--locale` forces it.

## Translating Cachet's text

With `--translate`, the import translates Cachet's text into the other
language of the page with a language model that runs on the machine, through
[Ollama](https://ollama.com). Nothing is sent outside, and no account or API
key is needed. A graphics card is not needed either.

The simplest way is the script, which starts Ollama, downloads the model the
first time, runs a dry run, asks before importing, and stops Ollama at the
end:

```bash
scripts/import-cachet.sh   --source-dsn 'cachet:password@tcp(cachet-db:3306)/cachet'   --translate
```

It takes the options of the command. Without the script, start the `ollama`
service of the `translate` profile, pull the model, and run the command with
`--translate`:

```bash
docker compose --profile translate up -d ollama
docker compose exec ollama ollama pull qwen2.5:7b
docker compose exec api ./api import-cachet --source-dsn '...' --translate --dry-run
```

What is translated: the page title, groups, components (name, description),
incidents with their updates, and maintenances. URLs, host names, numbers and
abbreviations such as `API` or `VPN` are left as they are, and so is a text
the model returns unchanged, such as a product name.

Each text is translated from the language it is written in: an English
component name on a French page gets a French translation. Short texts, where
the language cannot be told reliably, are taken to be in the import locale.

The translations are marked "machine translation" in the admin, under the
field. Edit one, or tick the box once it reads right, and the mark goes away.
Saving a form without touching a translation keeps its mark. Incident updates
show the mark in the timeline.

Since the text then exists in both languages, the public page is set to
follow the visitor's browser, with the import locale for other languages.

**Time and memory.** On a recent processor without a graphics card,
`qwen2.5:7b` needs about 6 GB of free memory, `qwen2.5:3b` about 3 GB and
translates a text in about 2 seconds. A history of a few thousand texts takes
an hour or two: `--translate-since` leaves older incidents in their language
only. Translating happens before the import transaction opens, and every
translation is kept in the cache file as it is made, so a dry run followed by
the import, or a run interrupted halfway, does not translate twice. The cache
lives in the API container: it is lost if the container is recreated.

**Quality.** Small models translate French into English well; English into
French less so, with anglicisms. Prefer `qwen2.5:7b`, or a larger model if
the machine allows it, and review the translations of what visitors read
most: components, groups and recent incidents.

A text the model fails on stays in its language only; the report counts
them.

On Kubernetes, run Ollama next to the import job, and pass its address with
`--translator-url`.

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
