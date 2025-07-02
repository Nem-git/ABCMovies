<script lang="ts">
  import type { Show } from "../../api/config";
  import { getSearchResults } from "../../api/search";

  import { q } from "../shared.svelte";
  import SearchBar from "./SearchBar.svelte";
  import ShowCard from "./ShowCard.svelte";

  let searchResults: Show[] | undefined = $state();

  $effect(() => {
    getSearchResults(q.query).then(async (sr) => (searchResults = await sr));
  });
</script>

<SearchBar />

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
    justify-content: center;
  }
</style>
