# ABCMovies

## What it is

ABCMovies is a media hub for people who use streaming services. It gathers many
streaming services into one place: a single catalog you can search and browse,
and a single place to watch, download, and manage access to that content.

Instead of opening Tubi, then Disney+, then a folder of files on your computer,
you open ABCMovies and everything is there. Each service contributes what it has,
and ABCMovies presents it as one library.

## The problem it solves

People today use many streaming services at once. Each has its own app, its own
account, its own search, and its own way of playing video. Some content is also
just files on a hard drive, or available through command-line tools that are
powerful but hard to use. There is no single place to look at all of it, play all
of it, or download any of it the way you want.

## The idea

A single, self-hosted service that:

- **Aggregates** — brings together content from any number of sources: streaming
  websites, command-line downloaders, local video files, databases — anything that
  has video and can be talked to.
- **Enriches** — adds rich metadata (posters, descriptions, ratings) from external
  catalogues to whatever each source provides.
- **Streams** — plays video from any source, adapting it on the fly so it plays in
  a browser or on a device. This includes converting formats, and, where the user
  has the means, removing DRM encryption.
- **Manages accounts** — each user links their own streaming accounts, can share
  access to them with others, and can use accounts the host of the instance has
  provided.
- **Downloads** — lets users download any title in the format, resolution, and
  container they choose.
- **Extends** — everything about it is designed to be added to: new streaming
  services, new capabilities, and new ways of interacting with it.

## How it's meant to feel

One library. One search. Play or download anything, from any of your services,
from one place — with your own accounts, your own identity, and your own instance
that you control.

## Design philosophy

- **It is a service, not an app.** The core is a quiet capability that does the
  work. Interfaces on top of it — a website, an API, a command line — are just
  ways to reach it, and more can be added freely.
- **Everything is optional and swappable.** No part of the system is the "one
  true way." Every feature is a slot that can be filled differently: a streaming
  source, a way to get DRM keys, a way to download, a way to talk to the service.
- **Sources and clients can speak any language.** ABCMovies translates. Streaming
  services, existing tools, and user interfaces all keep their own nature; the
  service meets them wherever they are.

## What it does not do

It does not invent its own content, and it does not bundle secrets. It orchestrates the user's
own legitimate access to content, across many services, in one place.
