<script lang="ts">
  import { goto } from "@roxi/routify";

  import SearchPage from "../../lib/SearchPage.svelte";

  let invalidCharacters = ["."];

  let q: string = $state("");

  const verifyQueryChange = $derived.by(() => {
    $goto("/search/[query]", { query: encodeURI(q) });
  });

  const updateQuery = async (event) => {
    let query: string = await event.target.value;
    if (query.length === 0) {
      return;
    }

    // TODO: Make this verification actually work, because it allows weird movement through the site
    invalidCharacters.forEach(async (char) => {
      console.log(query);
      query = query.replaceAll(await char, "");
      console.log(query);
    });
    q = decodeURI(query);
    verifyQueryChange;
  };
</script>

<div id="search-container">
  <input
    type="text"
    value={$state.snapshot(q)}
    oninput={updateQuery}
    autofocus
  />
</div>

<SearchPage query={q} />
