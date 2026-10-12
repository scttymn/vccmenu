# Valley Coffee Co menu

A Gantry + SQLite app deployed with Houston. `/` is the TV menu; `/admin` is the phone-friendly editor. The original GitHub Pages URL can remain the TV's entry point and read the public `/api/menu` endpoint.

## Editing

Sign in at `/admin`. Change names or prices, add or reorder items, and turn **On the menu** off to hide an item without deleting it. Edit **Section name** at the top of each section to rename its TV heading. Expand **Notes & toppings** for the notes. **Save menu** publishes the complete draft. TVs check every 30 seconds. Conflicting saves are rejected so one editor cannot overwrite another's newer changes.

Prices are display text, so `$4`, `$4 / $4.50 / $5`, and `$.75 each` all work. The three-column TV board scales to fit, including in full-screen mode (click the menu or press Enter/F). Adding many items reduces the type size to keep everything visible.

After the first successful load, the TV retains its last menu locally if the network or server becomes unavailable. Reconnecting picks up the latest saved menu. The GitHub Pages entry point retains the original image as a first-load fallback.

## Run locally

Docker is required. The pinned Gantry and Houston tools are downloaded into the ignored `.gantry/` folder.

```sh
bin/gantry dev
bin/gantry test
```

The app is at `http://vccmenu.localhost`. SQLite lives in the `data` volume; migrations and initial menu seeding run at startup. Seeding inserts only into an empty database and never replaces saved edits.

## Editor accounts

There is no public registration and no default password. Generate a one-use setup code:

```sh
bin/gantry task invite-editor
```

On a deployed app, run `/app task invite-editor` in its Houston server console. Give the code privately to an editor, who opens `/setup` and chooses their own email and password. Each code expires in 24 hours and works once. Generate a separate code for each editor. No email is sent automatically.

Gantry manages hashed passwords, server-side sessions, sign-in rate limits and cross-origin form protection. An unused sign-in expires after 30 days; every sign-in expires after 90 days. Email-based password recovery is not configured.

## Houston deployment

`compose.yml` declares the persistent `data` volume and `/up` health check. Link this GitHub repository in Houston, add its read-only deploy key, then deploy. Houston supplies HTTPS and persistent volume storage. Do not remove the data volume when updating the app.

```sh
bin/gantry deploy --server --follow
```

`SECRET_KEY` can be provided through Houston; when unset, Gantry creates and keeps the key in the database. Never put credentials in this repository.

The public menu endpoint permits reads from `https://scttymn.github.io`. It excludes hidden items. Editor writes require a signed-in session on the app's own origin.

## Menu source

`app/menu/initial.json` contains the original menu and is used only to seed a new database. After setup, the editor/database is the source of truth. The original `menu.png` remains available for fallback and comparison.
