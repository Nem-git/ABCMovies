# 🎬 ABCMovies
A web app for exploring movies, built using **Svelte**, **Slim (PHP)**, and **Python**. This is my first full-stack project combining these technologies.

---

## 🧰 Requirements

Make sure you have the following installed:

- [Docker and Docker compose](https://docs.docker.com/engine/install)
- [WVD File](#prepare-the-wvd-file-required-for-specific-drm-related-tasks)

---

## 📦 Cloning the Project

```bash
git clone https://github.com/Nem-git/ABCMovies.git
cd ABCMovies/
```


## ⚙️ Environment Configuration

Some files contain environment-specific paths or credentials that you'll need to modify.

**Edit these files:**

- **Docker**
  - `.env`
```TOUTV_EMAIL=
TOUTV_PASSWORD=
DB_PW=
```
- **PHP**
  - `src/backend/src/Config/Constants.php`
- **Python**
  - `src/python-backend/config/constants.py`
- **Frontend**
  - `src/frontend/src/lib/constants.ts`


## Prepare the `.wvd` file (required for specific DRM-related tasks):

```bash
mkdir -p config/devices
cp {path to the wvd you retrieved} config/devices/device.wvd
```

ℹ️ **How to get your own `.wvd` file**:  
[VideoHelp Guide](https://forum.videohelp.com/threads/408031-Dumping-Your-own-L3-CDM-with-Android-Studio)  
[Widevine Guesser Guide](https://github.com/FoxRefire/wvg/wiki/How-to-dump-CDM-key-pair)


---

## 🚀 All Set!

Your app is now configured and ready to go. Launch the frontend, ensure the backends are running, and enjoy exploring ABCMovies! 🎉

Feel free to contribute, open issues, or suggest features!
