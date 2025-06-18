<script lang="ts">
  let { episode, baseUrl }: { episode: Episode; baseUrl: string } = $props();

  import type { Episode } from "../api/config";
</script>

<div id="layout">
  <a href={baseUrl}>
    <img src={episode.imageCard.replace("_Size_", "400")} alt={episode.title} />
  </a>
  <div id="text">
    <a href={baseUrl}>
      <h5>{episode.title}</h5>
    </a>
    <p id="description">{episode.fullDescription}</p>
  </div>
</div>

<style>
  #layout {
    display: flex;
    flex-flow: row nowrap;
  }

  #text {
    display: flex;
    flex-flow: column nowrap;
  }

  img {
    object-fit: cover;
  }

  #description {
    font-family: sans-serif;
  }
</style>
