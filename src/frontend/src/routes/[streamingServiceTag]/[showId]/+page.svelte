<script lang="ts">
    import type { PageProps } from "./$types";

    import { goto } from "$app/navigation";
    import { onMount } from "svelte";

    import ShowPage from "$lib/components/ShowPage.svelte";

    let { data }: PageProps = $props();

    onMount(() => {
        goto(
            "/" +
                [data.streamingServiceTag, data.showId, data.seasonId].join(
                    "/",
                ),
            { replaceState: true },
        );
    });
</script>

<ShowPage show={data.show} />
<ol>
    {#each data.show.seasons as season}
        <a href={season.number.toString()}><li>{season.title}</li></a>
    {/each}
</ol>

<style>
    ol {
        list-style-type: none;
        display: flex;
        flex-direction: row;
        flex-wrap: wrap;
    }
</style>
