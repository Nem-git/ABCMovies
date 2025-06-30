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

What needs to be done:
Fuzzy search taking into account all the streaming services



# TODOs

## PHP

## Common

- [ ] Validation of HTTP requests
- [ ] Error detection and logging
- [ ] API Access logging
- [ ] More try/catch
- [ ] Figure out if require_once is a good way to import constants
- [ ] Add logical groups in the API endpoints using Slim
- [ ] Add logging for database access

### StreamingService.php

- [ ] Add multiple methods for recommendations, like series, movies, documentaries etc..
- [X] Split those big methods into small methods that only do one thing and one thing only
- [X] Add a helper class for most of those small methods that don't need overriding or direct access from outside the class
- [ ] Add other abstract methods that give the different headers for the requests, as now I am just using HTTP_DEFAULT_HEADERS
- [X] Improve the link Manifest link creation to support other than Dash Manifest and make it more robust
- [ ] Remove/Rename all the functions about Manifest, init, segment and stuff, so this can support in the future HLS and MP4s that are either encrypted or decrypted. Now that I'm thinking about it, I should instead create classes that manage dash, hls, straight MP4, encrypted and decrypted data
- [X] Add a logic that recommends you an episode/show after watching an episode
- [X] For the next recommendation, maybe only recommend episodes, or when recommending shows say it clearly in the JSON, so not to get confused

### ObjectFactory.php

- [ ] Verify if I should actually put it in Models or somewhere else
- [ ] Look up how other people manage this kind of object creation, if that's even a good way to do it

### Toutv.php

- [ ] Add a method that allows for logging in the streaming service
- [ ] Add the login keys to the database, with a TTL of how long it says in the JWT
- [ ] Make the login all in PHP, no interacting with the Python backend pleaaaase

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

## SearchPage.svelte

- [X] Fix inputting ../ and stuff allowing movement through the site, instead use url query params

### NavBar.svelte

- [ ] It seems that trying to put an icon and text side-by-side gives me a result that is not expected, so fix that
- [ ] 


