<script lang="ts">
  let { path }: { path: Path } = $props();

  import { Path } from "../path";
  import type { Show } from "../../api/config";
  import { onMount } from "svelte";
  import { url } from "@roxi/routify";
  import { getShow } from "../../api/show";
  import { seasonId } from "../shared.svelte";
  import SeasonPage from "./SeasonPage.svelte";

  let s: Promise<Show> | undefined = $state();

  onMount(async () => {
    s = getShow(path.getShow());

    // Make that seasonId check to avoid race conditions, where it sets the right season
    // then the first available season
    if (!$state.snapshot(seasonId.id)) {
      setSeason((await s).seasons[0].id);
    }
  });

  const setSeason = (id: string) => {
    seasonId.id = id;
  };
</script>

{#if s}
  {#await s}
    <div id="hero">
      <div id="hero-info">
        <p>Loading...</p>
      </div>
    </div>
  {:then sh}
    <div id="hero">
      <div id="hero-info">
        <h2>{sh.title}</h2>
        <h4>{sh.fullDescription}</h4>
        <p>Release year: {sh.year}</p>
      </div>
      <img src={sh.imageBackground.replace("_Size_", "720")} alt={sh.title} />
    </div>
  {/await}
{/if}

<ol>
  {#if s}
    {#await s then sh}
      {#each sh.seasons as season}
        <a
          onclick={() => {
            setSeason(season.id);
          }}
          href={$url("../", { s: season.id })}
          aria-label={season.number.toString()}>{season.title}</a
        >
      {/each}
    {/await}
  {/if}
</ol>

<SeasonPage {path} />
