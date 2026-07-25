## Metadata improvements

- Check if there would be a standard for content ratings, and see if a library exists to parse them. Also check if ISO for it exists

### CBC Gem

- Parse country name to country code
- Check if it would be interesting to add navigationFilters as Genres
- Check if it would be possible to link the trailer (prob not)
- Make all shows/seasons and episodes say that their language is english (if that's true for all Gem content, verify first)
- Most dates seem to be hard to understand. Find what's the meaning of startDate, airDate, availabilityDate, datePublished, etc
- Episode: What's the difference between metadata and metdata[media]?
- Check if there would be a way to differenciate show types (ex: movies vs documentary)
- Maybe add a backup for search (ex: 1 character search, search with old v1 search)
- Have a better way of determining if content is movie/series when searching
- Maybe creating the image url would give better results, as the API doesn't return every image type, but they all exist (background, logo, program, network)
- Parse the time to show it in a good way in the web ui (ex: https://github.com/sosodev/duration)

