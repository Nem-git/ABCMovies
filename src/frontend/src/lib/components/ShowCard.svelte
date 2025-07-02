<script lang="ts">
  import { url } from "@roxi/routify";

  import type { Show } from "../../api/config";
  let { show }: { show: Show } = $props();
</script>

<a
  href={$url("/[streamingService]/[show]", {
    streamingService: show.provider.toLowerCase(),
    show: show.id,
  })}
  aria-label={show.title}
>
  <ol>
    <div id="card">
      <img src={show.imageCard.replace("_Size_", "400")} alt={show.title} />
      <div id="card-info">
        <span id="description">{show.shortDescription}</span>
      </div>
    </div>
    <h3>{show.title}</h3>
  </ol>
</a>

<style>
  :root {
    --hover-overlay: linear-gradient(0deg, #0d0d0d, transparent);
  }

  a {
    display: flex;
    flex-flow: column nowrap;

    margin: 1em;
  }

  span {
    position: inherit;
    bottom: 0;
  }

  img {
    object-fit: cover;
  }

  /* ol {
    max-width: 400px;
  } */

  #card {
    position: relative;
  }

  #card-info {
    opacity: 0;
    position: absolute;
    top: 0;
    left: 0;
    background: var(--hover-overlay);
    width: 100%;
    height: 100%;

    display: flex;
    flex-flow: row nowrap;

    align-items: center;
    justify-content: center;
  }

  #description {
    font-family: sans-serif;
  }

  /* This part is only shown on hover */

  a:hover #card-info {
    opacity: 1;
  }
</style>
