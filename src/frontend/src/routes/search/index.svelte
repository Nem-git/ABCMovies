<script lang="ts">
  import { goto } from "@roxi/routify";
  import { onMount } from "svelte";

  import SearchPage from "../../lib/components/SearchPage.svelte";

  onMount(() => {
    document.getElementById("search-input")?.focus();
  });

  let query: string = $state("");

  $effect(() => {
    if (encodeURI(query) === "") {
      $goto("/search");
    } else {
      $goto("/search", { q: encodeURI(query) });
    }
  });

  const oninput = async (event: any) => {
    query = await event.target.value;
  };
</script>

<div id="search-container">
  <input
    type="text"
    id="search-input"
    value={$state.snapshot(query)}
    {oninput}
  />
</div>

<SearchPage {query} />
