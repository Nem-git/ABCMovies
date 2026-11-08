package web

const commonStyle = `
body { font-family: system-ui, sans-serif; max-width: 420px; margin: 3rem auto; padding: 0 1rem; color: #1f2933; }
h1 { font-size: 1.4rem; }
label { display: block; margin: 0.75rem 0; }
input { width: 100%; padding: 0.5rem; box-sizing: border-box; }
button { padding: 0.5rem 1rem; }
.error { color: #b91c1c; margin-top: 0.75rem; }
`

const setupPageHTML = `<!doctype html>
<html lang="en">
<head><meta charset="utf-8"><title>Set up ABCMovies</title><style>` + commonStyle + `</style></head>
<body>
<h1>Welcome to ABCMovies</h1>
<p>Create the initial admin account to start this instance.</p>
<form id="setup" novalidate>
<label>Username<br><input name="username" required minlength="2" autocomplete="username"></label>
<label>Password<br><input name="password" type="password" required minlength="8" autocomplete="new-password"></label>
<button type="submit">Create admin</button>
</form>
<p class="error" id="err" role="alert"></p>
<script>
const form = document.getElementById('setup');
const err = document.getElementById('err');
form.addEventListener('submit', async (e) => {
  e.preventDefault();
  err.textContent = '';
  const data = new FormData(form);
  const res = await fetch('/api/setup', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username: data.get('username'), password: data.get('password') }),
  });
  if (res.ok) { location.href = '/login'; return; }
  const body = await res.json().catch(() => ({}));
  err.textContent = body.error || 'Could not create admin.';
});
</script>
</body>
</html>`

const loginPageHTML = `<!doctype html>
<html lang="en">
<head><meta charset="utf-8"><title>Sign in — ABCMovies</title><style>` + commonStyle + `</style></head>
<body>
<h1>Sign in</h1>
<form id="login">
<label>Username<br><input name="username" required autocomplete="username"></label>
<label>Password<br><input name="password" type="password" required autocomplete="current-password"></label>
<button type="submit">Sign in</button>
</form>
<p class="error" id="err" role="alert"></p>
<script>
const form = document.getElementById('login');
const err = document.getElementById('err');
form.addEventListener('submit', async (e) => {
  e.preventDefault();
  err.textContent = '';
  const data = new FormData(form);
  const res = await fetch('/api/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username: data.get('username'), password: data.get('password') }),
  });
  if (res.ok) { location.href = '/'; return; }
  const body = await res.json().catch(() => ({}));
  err.textContent = body.error || 'Invalid username or password.';
});
</script>
</body>
</html>`

const homeHTML = `<!doctype html>
<html lang="en">
<head><meta charset="utf-8"><title>ABCMovies</title><style>` + commonStyle + `</style></head>
<body>
<h1>Hello, {{username}}</h1>
<p>You are signed in to ABCMovies.</p>
<form id="logout"><button type="submit">Sign out</button></form>
<script>
document.getElementById('logout').addEventListener('submit', async (e) => {
  e.preventDefault();
  await fetch('/api/logout', { method: 'POST' });
  location.href = '/login';
});
</script>
</body>
</html>`