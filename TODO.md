NO INIT MERGING:
1. Utiliser le init comme metadata pour decrypter le segment et garder le init, mais le clean de la pssh box/decryption informations (U-MPV U-WEB)
2. Utiliser le init comme metadata pour decrypter le segment et garder le init original (Y-MPV N-WEB)

INIT MERGING:
1. Merge le init avec le segment pour decrypter, et garder le init mais le clean de de la pssh box/decryption informations (U-MPV U-WEB)
2. Merge le init avec le segment pour decrypter, et garder le init comme original (E-MPV E-WEB)

U - Unknown
Y - Works perfectly
N - Does not play at all
E - Plays with errors / warnings


# TODOs

## PHP

## Common

- [ ] Add .env to gitignore when .env is feature complete
- [ ] Validation of HTTP requests
- [ ] Error detection and logging
- [ ] Throw HTTP errors, don't just return half-broken JSON
- [ ] API Access logging
- [ ] More try/catch
- [ ] Figure out if require_once is a good way to import constants
- [ ] Add logical groups in the API endpoints using Slim
- [ ] Add logging for database access

### StreamingService.php

- [ ] Add other abstract methods that give the different headers for the requests, as now I am just using HTTP_DEFAULT_HEADERS
- [X] Abstract the logic with Dash Manifest and create a parent class that can englobe more streaming techs
- [ ] Make the decryption of segments optional
- [ ] Abstract the decryption so it can support other forms of DRM, like Fairplay or PlayReady

### ObjectFactory.php

- [ ] Verify if I should actually put it in Models or somewhere else
- [ ] Look up how other people manage this kind of object creation, if that's even a good way to do it

### Toutv.php

- [ ] Add a method that allows for logging in the streaming service
- [ ] Add the login keys to the database, with a TTL of how long it says in the JWT
- [ ] Make the login all in PHP, no interacting with the Python backend pleaaaase

### SegmentDecryptor\

- [ ] Find out if/how to clean the PSSH box to remove the remaining DRM informations

### SegmentDecryptor\PHP.php

- [ ] Figure out what FFI is and how it might be useful to call directly functions, to avoid as much as possible to open shell

### SlimResponseHelper.php

- [ ] Remove the pretty printing from the JSON response
- [ ] Find a way to actually return the right MIME and not just a generic video/mp4 while returning segments

### RequestHelper.php

- [ ] Make the http requests asynchronously

### ManifestController.php

- [ ] Look online for how I should actually name this
- [ ] Create a parent class that will make all interactions with a repository the same, no matter the database

### RedisRepository.php

- [ ] Add a nullable TTL parameter to every data entry


## Svelte

## Common

- [ ] Make the switch from Routify to SvelteKit (Big project, takes time to do right, maybe new branch)

## SearchPage.svelte

- [ ] Find out if the autofocus on hover is actually a fun feature or if it should be removed

### NavBar.svelte

- [ ] It seems that trying to put an icon and text side-by-side gives me a result that is not expected, so fix that

### ShowPage.svelte

- [ ] Find a nice and intuitive way to position the show image, title, description etc.. at the top of page


