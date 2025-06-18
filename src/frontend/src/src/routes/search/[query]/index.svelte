<script lang="ts">
  import { goto } from "@roxi/routify";
  import { params } from "@roxi/routify";

  import SearchPage from "../../../lib/SearchPage.svelte";

  let invalidCharacters = ["."];

  let { query } = $params;

  let q: string = $state("");
  q = decodeURI(query);

  const verifyQueryChange = $derived.by(() => {
    $goto("/search/[query]", { query: encodeURI(q) });
  });

  const updateQuery = async (event) => {
    let query: string = await event.target.value;
    if (query.length === 0) {
      return;
    }
    invalidCharacters.forEach(async (char) => {
      query.replaceAll(char, "");
    });
    q = decodeURI(await event.target.value);
    verifyQueryChange;
  };
</script>

<div id="search-container">
  <input type="text" value={q} oninput={updateQuery} autofocus />
</div>

<SearchPage query={q} />
