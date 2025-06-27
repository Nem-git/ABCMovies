<script lang="ts">
  import { goto } from "@roxi/routify";
  import { onMount } from "svelte";

  import SearchPage from "../../lib/components/SearchPage.svelte";

  onMount(() => {
    document.getElementById("search-input")?.focus();
  });

  let invalidCharacters = ["."];

  let q: string = $state("");

  const verifyQueryChange = $derived.by(() => {
    $goto("/search/[query]", { query: encodeURI(q) });
  });

  const updateQuery = async (event) => {
    let input: string = await event.target.value;

    // TODO: Make this verification actually work, because it allows weird movement through the site
    invalidCharacters.forEach(async (char) => {
      console.log(input);
      input = input.replaceAll(await char, "");
      console.log(input);
    });

    q = input;
    verifyQueryChange;
  };
</script>

<div id="search-container">
  <input
    type="text"
    id="search-input"
    value={$state.snapshot(q)}
    oninput={updateQuery}
  />
</div>

<SearchPage query={q} />

<style>
  input {
    
  }
</style>