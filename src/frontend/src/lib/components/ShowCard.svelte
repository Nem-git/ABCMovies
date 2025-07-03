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
  <div class="card">
    <img src={show.imageCard.replace("_Size_", "480")} alt={show.title} />
    <div class="card-info">
      <span class="description">{show.shortDescription}</span>
    </div>
  </div>
  <h3>{show.title}</h3>
</a>

<style>
  :root {
    --hover-overlay: linear-gradient(0deg, #0d0d0d, transparent);
  }

  a {
    display: flex;
    flex-flow: column nowrap;
  }

  span {
    position: inherit;
    bottom: 0;
  }

  img {
    max-width: 100%;
    max-height: 100%;
    object-fit: cover;

    transition-property: transform;
    transition-duration: 0.2s;
    transition-timing-function: ease-in;
  }

  .card {
    position: relative;
    overflow: hidden;
  }

  .card-info {
    opacity: 0;
    position: absolute;
    top: 0;
    left: 0;
    background: var(--hover-overlay);
    width: 100%;
    height: 100%;

    display: flex;
    flex-flow: row nowrap;

    justify-content: center;
  }

  /* This part is only shown on hover */

  a:hover .card-info {
    opacity: 1;
  }

  a:hover img {
    transform: scale(1.05);
  }
</style>
