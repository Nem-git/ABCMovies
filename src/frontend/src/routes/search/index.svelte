<script lang="ts">
  import { goto } from "@roxi/routify";
  import { onMount } from "svelte";

  import { q } from "../../lib/shared.svelte";
  import SearchPage from "../../lib/components/SearchPage.svelte";

  onMount(() => {
    document.getElementById("search-input")?.focus();
  });

  $effect(() => {
    if (encodeURI(q.query) === "") {
      $goto("/search");
    } else {
      $goto("/search", { q: encodeURI(q.query) });
    }
  });
</script>

<SearchPage />
