<script lang="ts">
    import type { PageProps } from "./$types";

    import ShowPage from "$lib/components/ShowPage.svelte";
    import SeasonPage from "$lib/components/SeasonPage.svelte";

    let { data }: PageProps = $props();
</script>

<ShowPage show={data.show} />
<ol>
    {#each data.show.seasons as season}
        <a href={season.number.toString()}><li>{season.title}</li></a>
    {/each}
</ol>
<SeasonPage season={data.season} />

<style>
    ol {
        list-style-type: none;
        display: flex;
        flex-direction: row;
        flex-wrap: wrap;
    }
</style>
