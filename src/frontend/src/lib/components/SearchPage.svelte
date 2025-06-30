<script lang="ts">
  import { onMount } from "svelte";

  import type { Show } from "../../api/config";
  import { getSearchResults } from "../../api/search";

  import ShowCard from "./ShowCard.svelte";

  let searchResults: Show[] | undefined = $state();

  let { query }: { query: string } = $props();

  onMount(async () => {
    search;
  });

  $effect(() => {
    search;
  });

  const search = $derived.by(async () => {
    getSearchResults(query).then(async (sr) => (searchResults = await sr));
    console.log(query);
  });
</script>

<ol>
  {#if searchResults}
    {#await searchResults then}
      {#each searchResults as show}
        <ShowCard {show} />
      {/each}
    {/await}
  {/if}
</ol>

<style>
  ol {
    display: flex;
    flex-wrap: wrap;
  }
</style>
