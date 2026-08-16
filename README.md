# 🎬 ABCMovies
One streaming service to rule them all.

---
## What is it?
A bunch of things!

Here are the things I want to do in this project:

An application that:
  - Retrieves metadata about video streaming services' content:
    - Through the streaming service directly
    - Through a metadata provider (ex: tmdb)
    - A streaming service can also be local. Video files on a computer could be used as a "streaming service"
    - Allows for easy addition of streaming services through plugins:
      - Uses HTTP to fetch content from web streaming services
      - Uses CLI to interact with projects that allow for metadata and video downloading (unshackle, devine)
      - Reads databases and local files to get video informations and stream them
  - Proxyies videos:
    - Directly from the streaming service
    - Transmuxes them (hls -> mp4, mp4 -> srt, rtmp -> dash, etc..)
    - Encodes them (h264 4k -> vp9 720p, etc..)
    - Decrypts them (widevine -> clearkey, playready -> no encryption)
  - Manages accounts:
    - User accounts can add their own streaming services to their profiles linking their accounts
    - Allows users to share streaming service accounts with each others
    - Streaming services accounts can also be registered in the project's configuration directly, not necessarily through user accounts, so users on the site can use that shared account
  - Makes downloading videos possible:
    - Users select their video format, the resolution and file format
  - Is easy to extend:
    - A REST API allows for interacting with the service
    - A web interface that interacts with that previously mentioned REST API
    - A cli tool that can be ran to directly fetch content using this service
    - Many more.

