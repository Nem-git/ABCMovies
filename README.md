# ABCMovies
My first Svelte and Slim app ( If it ever even gets to that point )


## 🛠️ Setting Up

```
git clone https://github.com/Nem-git/ABCMovies.git
cd ABCMovies/
```

### PHP Backend Setup 🖥️

1. Navigate to the **backend** folder:
```
cd src/backend
```

2. Install Slim Framework (we're going with version 4.x here!):
```
composer require slim/slim:"4.*"
```

3. You’ll also need the Slim PSR-7 implementation:
```
composer require slim/psr7
```

Once you're done, your PHP backend will be ready, now move on to the **Python** backend

### Python Backend Setup 🖥️

1. Navigate to the **python-backend** folder:
```
cd src/backend-python
```

2. Create vitural envrionment and install requirements:
```
python -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt
```

3. Place the wvd file in the app/wvds folder:
```
cp device.wvd app/wvds/device.wvd
```
More informations on acquiring your own wvd: [[1]](https://forum.videohelp.com/threads/408031-Dumping-Your-own-L3-CDM-with-Android-Studio) [[2]](https://github.com/FoxRefire/wvg/wiki/How-to-dump-CDM-key-pair)


4. Run the API ( Optional )
```
uvicorn main:app
```


Once you’re done here, your backend will be ready to serve up some awesome responses!

### Frontend Setup 🌐

1. Now, let's move on to the **frontend** folder:
```
cd src/frontend/
```

2. Install the necessary npm dependencies:
```
npm create vite@latest src -- --template svelte-ts
```

3. Install npm dependencies:
```
cd src
```

4. Run the web server dynamically ( Optional )
```
npm run dev
```

5. Build the project ( Optional )
```
npm run build
```

Voilà! Now your frontend is ready to make some cool things happen on the web!

## 🚀 You're All Set!

You should now be all set up and ready to run ABCMovies! 🌟