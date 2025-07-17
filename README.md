# 🎬 ABCMovies
A web app for exploring movies, built using **Svelte**, **Slim (PHP)**, and **Python**. This is my first full-stack project combining these technologies.

---

## 🧰 Requirements

Make sure you have the following installed:

- [Nginx](https://nginx.org/en/download.html)
- [PHP](https://www.php.net/downloads.php/)
- [Composer](https://getcomposer.org/download/)
- [Python 3.12+](https://www.python.org/downloads/)
- [Node.js and npm](https://nodejs.org/)
- [Redis](https://redis.io/docs/getting-started/installation/)
- [Bento4 Binaries](https://www.bento4.com/downloads/)

---

## 📦 Cloning the Project

```bash
git clone https://github.com/Nem-git/ABCMovies.git
cd ABCMovies/
```

---

## Setting up NGINX

Configure the nginx service

**Edit this file:**

```bash
/etc/nginx/nginx.conf
```

**Example Configuration**
```nginx
server {
    listen 127.0.0.1:80;
    server_name localhost;

    location / {
        root /srv/www/ABCMovies/src/frontend/public;
        try_files $uri $uri/ /index.php$is_args$args;
    }

    location /api {
        root /srv/www/ABCMovies/src/backend/public;

        # FastCGI configuration
        include fastcgi_params;
        fastcgi_pass 127.0.0.1:9000

        # Handle requests that do not map to a file
        if (!-e $request_filename) {
            rewrite ^(.*)$ /index.php break;
        }
    }
}
```


---

## ⚙️ Environment Configuration

Some files contain environment-specific paths or API keys that you'll need to modify.

**Edit these files:**

- **PHP**
  - `src/backend/src/Config/Constants.php`
  - `src/backend/src/Config/.env`
- **Python**
  - `src/python-backend/config/constants.py`
- **Frontend**
  - `src/frontend/src/lib/constants.ts`

---

## 🖥️ PHP Backend Setup

1. Navigate to the backend:

```bash
cd src/backend
```

2. Install dependencies:

```bash
composer update
```

---

## 🐍 Python Backend Setup

1. Navigate to the Python backend:

```bash
cd src/python-backend
```

2. Set up a virtual environment and install dependencies:

```bash
python -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt
```

3. Prepare the `.wvd` file (required for specific DRM-related tasks):

```bash
mkdir -p config/devices
cp device.wvd config/devices/device.wvd
```

ℹ️ **How to get your own `.wvd` file**:  
[VideoHelp Guide](https://forum.videohelp.com/threads/408031-Dumping-Your-own-L3-CDM-with-Android-Studio)  
[Widevine Guesser Guide](https://github.com/FoxRefire/wvg/wiki/How-to-dump-CDM-key-pair)

4. (Optional) Run the Python API:

```bash
uvicorn main:app
```

---

## 🌐 Frontend Setup

1. Navigate to the frontend directory:

```bash
cd src/frontend/src
```

2. Install dependencies:

```bash
npm install
```

3. (Optional) Start the development server:

```bash
npm run dev
```

4. (Optional) Build the production-ready frontend:

```bash
npm run build
```

---

## 🚀 All Set!

Your app is now configured and ready to go. Launch the frontend, ensure the backends are running, and enjoy exploring ABCMovies! 🎉

Feel free to contribute, open issues, or suggest features!
