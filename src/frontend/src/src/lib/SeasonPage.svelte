<script lang="ts">

    let { baseUrl }: { baseUrl: string } = $props();

    import type { Season } from "../api/config";
    import { getSeason } from "../api/season";
    import EpisodeCard from "./EpisodeCard.svelte";
    import { seasonId } from "./shared.svelte";

    let s: Promise<Season> = $derived(getSeason(baseUrl + "/" + seasonId.id));

</script>

{#if (s)}
    {#await s then sea}
        <h3>{sea.title}</h3>
        <ol>
            {#each sea.episodes as episode}
                <EpisodeCard {episode} baseUrl={baseUrl + "/" + sea.id + "/" + episode.number} />
            {/each}
        </ol>
    {/await}
{/if}