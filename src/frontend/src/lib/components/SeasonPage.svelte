<script lang="ts">
    import type { Season } from "$lib/types";

    import EpisodeCard from "./EpisodeCard.svelte";

    let { season }: { season: Season } = $props();
</script>

<ol>
    {#each season.episodes as episode}
        <li>
            <EpisodeCard seasonId={season.id} {episode} />
        </li>
    {/each}
</ol>

<style>
    ol {
        display: flex;
        flex-direction: column;
        flex-wrap: nowrap;

        margin: 0;
        padding: 0;
        margin-inline: 5vw;
        margin-top: 3em;

        list-style-type: none;
    }
</style>
