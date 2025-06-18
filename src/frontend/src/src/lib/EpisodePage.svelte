<script lang="ts">
  import { init } from "./dashjsPlayer.svelte";
  import { manifestUrl } from "./dashjsPlayer.svelte";
  // import { manifestUrl } from "./videojsPlayer.svelte";
  // import { init } from "./videojsPlayer.svelte";
  // import { manifestUrl } from "./shakaPlayer.svelte";
  // import { init } from "./shakaPlayer.svelte";
  import type { Episode } from "../api/config";
  import { getEpisode } from "../api/episode";
  import { onMount } from "svelte";

  let { baseUrl }: { baseUrl: string } = $props();

  let e: Promise<Episode> = getEpisode(baseUrl);

  onMount(async () => {
    manifestUrl.url = (await e).url;
    init();
  });
</script>

<div id="fullscreen">
  <div id="container">
    <a href="./" id="back-button">Back</a>
    <video id="video-player" controls autoplay>
      <track kind="captions" />
      <!-- I am unsure about the need for this, but it gives me errors in svelte when I don't put it -->
    </video>
  </div>
</div>

<style>
  #video-player {
    max-width: 100vw;
    max-height: 100vh;
  }

  #container {
    position: absolute;
  }

  #fullscreen {
    display: flex;
    align-items: center;
    justify-content: center;

    height: 100vh;
    min-width: 100%;
    position: absolute;
    top: 0;
    left: 0;
  }

  #back-button {
    position: absolute;
    top: 0;
    left: 0;
    z-index: 1;
  }
</style>
