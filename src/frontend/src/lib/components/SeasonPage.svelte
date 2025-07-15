<script lang="ts">
    import type { Season } from "../types";

    import EpisodeCard from "./EpisodeCard.svelte";

    let { season }: { season: Season } = $props();
</script>

<h3>{season.title}</h3>
<ol>
    {#each season.episodes as episode}
        <li>
            <EpisodeCard seasonId={season.number.toString()} {episode} />
        </li>
    {/each}
</ol>

<style>
    ol {
        list-style-type: none;
    }
</style>
