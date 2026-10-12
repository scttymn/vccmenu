# Valley Coffee Co Menu Manager

A Gantry + SQLite app deployed with Houston. `https://vccmenu.svnmns.com/` is the TV menu; `/admin` is the phone-friendly editor.

## Editing

Sign in at `/admin`. Change names or prices, add or reorder items, and turn **On the menu** off to hide an item without deleting it. Edit **Section name** at the top of each section to rename its TV heading. Expand **Notes & toppings** for the notes. **Save menu** publishes the complete draft. TVs check every 30 seconds. Conflicting saves are rejected so one editor cannot overwrite another's newer changes.

Prices are display text, so `$4`, `$4 / $4.50 / $5`, and `$.75 each` all work. The three-column TV board scales to fit, including in full-screen mode (click the menu or press Enter/F). Adding many items reduces the type size to keep everything visible.

After the first successful load, the TV retains its last menu locally if the network or server becomes unavailable. Reconnecting picks up the latest saved menu.

## Run locally

Docker is required. The pinned Gantry and Houston tools are downloaded into the ignored `.gantry/` folder.

```sh
bin/gantry dev
bin/gantry test
```

The app is at `http://vccmenu.localhost`. SQLite lives in the `data` volume; migrations and initial menu seeding run at startup. Seeding inserts only into an empty database and never replaces saved edits.

In development, startup and `bin/gantry db seed` also create the local editor account **admin@example.com** with password **Password1!**. This account is created only when `GANTRY_ENV=development`; seeding never resets its password or replaces existing users. Production and test environments do not create this account.

Gantry Strict runs source checks when `bin/gantry dev` starts. Its router automatically injects a bottom warning panel into development pages when findings exist; no page-specific integration is needed. Run `bin/gantry check --strict` for the full check result. To hide the panel, set `GANTRY_DEV_WARNINGS=false` and restart development. Production never shows it.

Every page uses the shared templ layout in `app/shared/layout`. Scripts and styles live in `assets/js` and `assets/css` and use Gantry's asset pipeline: small stylesheets are inlined, scripts are fingerprinted and cached, and production builds minify scripts. Page responses are not cached.

## Editor accounts

There is no public registration or default production password. For the first production admin account, set `ADMIN_EMAIL` and `ADMIN_PASSWORD` through Houston secrets before deploying. Startup creates the account only when no users exist; it never resets an existing password. These bootstrap secrets can be removed after account creation.

Once signed in, open **Users** in Menu Manager to add an account with an email and starting password. Every account has full access to edit the menu and add users. Share the starting password privately; the new user can change it using **Password**. No invitation email is sent.

For server-side account setup, you can also generate a one-use setup code:

```sh
bin/gantry task invite-editor
```

For a deployed app, run this on the Docker host using its running app container:

```sh
docker ps --filter label=service=vccmenu --filter label=role=web
docker exec <container-id> /app task invite-editor
```

Give the code privately to an editor, who opens `/setup` and chooses their own email and password. Each code expires in 24 hours and works once. Generate a separate code for each editor. No email is sent automatically.

Gantry manages hashed passwords, server-side sessions, sign-in rate limits and cross-origin form protection. An unused sign-in expires after 30 days; every sign-in expires after 90 days. Use **Password** in the editor to change your password; this signs out existing sessions. Email-based password recovery is not configured.

## Houston deployment

`compose.yml` declares the persistent `data` volume and `/up` health check. The `vccmenu` project has been linked to `github.com/scttymn/vccmenu`, with a read-only deploy key and a push webhook. Its default address is `https://vccmenu.svnmns.com`. Houston supplies HTTPS and persistent volume storage. Do not remove the data volume when updating the app.

```sh
bin/gantry deploy --server --follow
```

`SECRET_KEY` can be provided through Houston; when unset, Gantry creates and keeps the key in the database. Never put credentials in this repository.

The public menu endpoint excludes hidden items. Editor writes require a signed-in session on the app's own origin. The old GitHub Pages site is retired; TVs use the Houston address above.

## Menu source

`app/menu/initial.json` contains the original menu and is used only to seed a new database. After setup, the editor/database is the source of truth.
