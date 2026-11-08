# Security and privacy baseline

Product-level baseline. Technical controls are recorded as implementation/tech decisions.

- **No bundled secrets.** The repo and distribution ship no credentials, tokens, or private keys. The instance may store user-supplied source credentials/session material; that storage and exposure are security-sensitive.
- **Least privilege.** Components and users get only the access/data needed. Admins manage the instance; they do not see other users’ private history/credentials by default.
- **Minimize sensitive data.** Collect/retain only what is needed. Do not log secrets, credentials, or private content identifiers beyond what is necessary and safe.
- **Honest boundaries.** State what the host/instance can observe. “Privacy” is not assumed where components must process data.
- **Connector isolation mindset.** Source connectors and extensions are not inherently trusted; failures must be contained and fail closed where practical.
- **User separation.** Personal state and access are scoped correctly; sharing is explicit.
- **Protected content.** DRM/protected content is out of scope until an explicit decision records the stance and limits.

Security issues should be reported privately (see root `SECURITY.md` once contact details are set).
