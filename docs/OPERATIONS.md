# Operations (Build 1)

Simple notes for people running an ABCMovies instance. This document will grow over time.

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

Build 1 release hygiene (install/upgrade/backup/restore, broader admin controls) is deferred (see REQ-OPS-01). Follow `decisions/product/0001-first-run-admin-and-recovery.md` for the recovery decision and trust boundary.
