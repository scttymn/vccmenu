# Valley Coffee Co Menu Manager

A Gantry + SQLite app deployed with Houston. `https://vccmenu.svnmns.com/` is the live menu; `/admin` is the phone-friendly editor.

## Editing

Sign in at `/admin` to see every menu and which one is live now. **+ New menu** starts a draft; **Duplicate menu** copies one (also as a draft, so it can't clash with the original until you reschedule and publish it).

Each menu has a **Menu name** (only editors see it) and an optional **Display title**, the heading on the live menu; blank uses the name. Under **When it shows**, a menu either **repeats weekly** (days and hours, with different hours for other days, optionally only during a season) or is a **one-off event** (a start and end, possibly over several days). Uncheck **Publish this menu** to keep it as a draft.

Which menu is live: of the published menus scheduled at that moment, the one that started latest; between menus, the last one to run stays up until the next one starts; before anything has run, the oldest menu. The editor warns when a schedule overlaps another published menu, says which one would show, and for events offers to move it clear (drag or use the arrow keys on the timeline to adjust by hand). Past events move to an **Ended** list. The last remaining menu can't be deleted.

**Preview ↗** in a menu's editor opens that menu in a new browser window, live or not, exactly as it will look. It shows the editor's unsaved changes as you type and goes back to the saved version once you save or leave the editor. Previews are only for signed-in editors and never change the live menu.

Schedules use the shop's time zone, `TIME_ZONE` (an IANA name such as `America/Chicago`, the default). Set it through Houston if the shop is elsewhere.

Within a menu, **WHAT’S ON IT** lists its one to three sections (Drinks, Food, …), each a column on the live menu. **Edit** opens a section (one at a time; **Done** closes it) to rename it, change names or prices, add, reorder or remove items, and turn **Show on menu** off to hide an item without deleting it; **Notes & toppings** holds its notes. **Add section** adds one, up to three, and the bin icon deletes one with its items after you confirm. A section with changes not yet saved says so. **Save menu** saves the menu and its schedule; Open menu pages check every 30 seconds. Conflicting saves are rejected so one editor cannot overwrite another's newer changes.

Prices are display text, so `$4`, `$4 / $4.50 / $5`, and `$.75 each` all work. The menu board scales to fit, including in full-screen mode (click the menu or press Enter/F). Adding many items reduces the type size to keep everything visible, and a long display title shrinks to stay on one line.

After the first successful load, the menu page keeps its last menu locally if the network or server becomes unavailable. Reconnecting picks up the latest saved menu.

## Run locally

Docker is required. The pinned Gantry and Houston tools are downloaded into the ignored `.gantry/` folder.

```sh
bin/gantry dev
bin/gantry test
```

The app is at `http://vccmenu.localhost`. SQLite lives in the `data` volume; migrations and initial menu seeding run at startup. Seeding inserts the Regular Menu only into an empty database and never replaces saved edits.

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

The public menu endpoint excludes hidden items. Editor writes require a signed-in session on the app's own origin. The old GitHub Pages site is retired; Screens use the Houston address above.

## Menu source

`app/menu/initial.json` contains the original menu and is used only to seed a new database. After setup, the editor/database is the source of truth.
