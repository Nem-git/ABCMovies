# Operations (Build 1)

Simple notes for people running an ABCMovies instance. This document will grow over time.

## Running ABCMovies locally (Build 1)

**What you need:** Go (see `go.mod`) and nothing else — storage is the embedded SQLite database and there are no external services to install.

1. Create a `.env` from the example: `cp .env.example .env`. The example contains **variable names only** — you set the values yourself. The names: `CORE_BIND_ADDRESS`, `CORE_GRPC_PORT`, `ABCMOVIES_DB` (default `data/abcmovies.db`), `API_BIND_ADDRESS`, `API_HTTP_PORT`, `CORE_GRPC_ADDR`, `WEB_BIND_ADDRESS`, `WEB_HTTP_PORT`, `API_HTTP_ADDR`.
2. Build the three services: `make build` — produces `bin/abcmovies-core`, `bin/abcmovies-api`, `bin/abcmovies-web`.
3. Start them: `make run-all` (loads `.env`, starts all three; Ctrl-C stops them) or individually with `make run-core`, `make run-api`, `make run-web`.
4. Open the web UI at the configured address (default `http://127.0.0.1:8097`). On first run you are asked to create the admin account — this account can administer the instance.
5. Your data lives in the file named by `ABCMOVIES_DB` (default `data/abcmovies.db`); the audit/event log (`abcmovies-audit.log`, written by operator recovery and source removal) sits next to it. Deleting the database file deletes the instance.

**Build 1 has no backup, restore, or upgrade support.** There is no backup button, no migrate-on-upgrade, and no uninstaller. Back the data files up yourself before changing anything about the instance. Install/upgrade/backup/restore and broader admin controls are deferred to Build 2, and this paragraph is the record of that risk: a data mistake in Build 1 is on the operator.

## If you lose admin access (operator recovery)

ABCMovies can reset the local admin password from the machine that runs the instance. This is an operator/host action, not a normal login feature.

**Important:** Anyone who can access the host (OS account, container, or the instance files/database) can reset admin access. Protect the machine, backups, and instance data. ABCMovies cannot prove a remote password reset came from the real admin.

**How to reset the local admin password**

1. Open a shell on the host where ABCMovies runs (or where its data is stored).
2. Run the admin tool against the same database the instance uses:

   ```sh
   abcmovies-admin reset-admin
   ```

   Set `ABCMOVIES_DB` if your database is not in the default location (name only; do not put secrets in docs or commits).

3. The tool asks you to confirm by typing the exact confirmation it shows for that admin account.
4. Enter a new password when asked. The tool changes only that local admin password and ends that user’s active sessions.

**Notes**

- The reset is recorded in an audit/event log entry (no passwords or secrets).
- This does not reveal the old password.
- If the instance is not initialized yet, there is no admin account to reset—finish first-run setup instead.

## Watching media on your instance (Build 1)

- **Playing means the original file, always.** ABCMovies never converts or shrinks what you watch; it passes the source's own file to your browser. If your browser can't decode that file (some video and audio types just aren't supported), playback fails and you get a clear message telling you so — not a broken player and no silence. A different browser or device is usually the answer, not anything to fix in ABCMovies.
- **Protected titles can never play.** If a title is DRM-protected or in a format that can't play here, ABCMovies says exactly that. This is a limit of the feature, not a bug, and it will not change without a product decision.
- **Series can't play yet.** Series appear in the library, but pressing Play on one is refused: "series playback is not supported in Build 1 yet." That's a Build 1 limit, not a bug — episode-level playback is not built.
- **You only enter your source credentials once in a while.** After you've entered them, ABCMovies remembers them while the instance keeps running — so syncing again or hitting Play on another title doesn't ask again. After a restart it forgets everything and you'll be asked once. Logging out does not forget them either, and they never leave the machine's memory.
- **Changed the password or revoked the key on the source?** If you revoked an API key, Play verifies it right then and you'll see a clear credentials error instead of a dead player. A password change can keep working from memory until a restart or 12 unused hours pass — log out doesn't help; restarting does.
- **Play links are temporary and single-title.** The Watch button gets a link valid for that one title, refreshed while you're actually watching, that dies 30 minutes after you stop. Bookmarking the link or sharing it later won't open the video; other people's watches still obey their own source access.
- **Links can appear in web logs.** Playing sends the link in the web request; anything between you and ABCMovies that logs addresses can see it. The link is short-lived and opens one title, but if that matters to you, configure whatever logs your web traffic to not record anything after the `?` in an address.
- **Originals are big files.** Because nothing is converted, every watch streams the full original file — rebuffering means your connection is slower than the file, not that anything is broken. There is no limit yet on how many watches can run at once; ABCMovies logs the count instead.
- **When it doesn't work.** The three things that fix almost everything: reload the page, try a different browser, and re-enter your source credentials. If it still fails, send the service owner the title and the exact message you see — and never share the watch link itself anywhere.

## Removing a source (Build 1)

- **Disconnecting a source hides its titles; it does not delete them.** The source and its library entries are kept, so watch history and future features stay meaningful. Titles from a disconnected source stop appearing in your library and cannot be played or synced.
- **Removal takes effect for new use immediately.** Playing or syncing against a disconnected source fails with a clear message, and any play links you had for it stop working on the next request (a stream already mid-download finishes, but seeking or reloading will not).
- **Re-adding the same server brings the source back.** Reconnect the same Jellyfin server; after a sync, its titles return as they were.
- **Removing a source is recorded in the instance's audit log**, with no credentials.

Build 1 release hygiene (install/upgrade/backup/restore, broader admin controls) is deferred (see REQ-OPS-01). Follow `decisions/product/0001-first-run-admin-and-recovery.md` for the recovery decision and trust boundary.
