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
        list-style-type: none;
    }
</style>
