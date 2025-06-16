<script lang="ts">
    export let query;

    import { onMount } from "svelte";
    import type { Show } from "../../api/config";
    import { getSearchResults } from "../../api/search";

    import ShowCard from "../../lib/ShowCard.svelte";

    let searchResults: Show[];

    onMount(async () => {
        getSearchResults(query).then((sr) => searchResults = sr);
    });
</script>

{query}

{#each searchResults as show}
    <ShowCard {show} />
{/each}