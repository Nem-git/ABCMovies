<script lang="ts">
    import type { Season } from "$lib/types";

    import EpisodeCard from "./EpisodeCard.svelte";

    let { season }: { season: Season } = $props();
</script>

<ol>
    {#each season.episodes as episode}
        <li
            class={season.episodes.indexOf(episode) ===
            season.episodes.length - 1
                ? "last-li"
                : undefined}
        >
            <EpisodeCard seasonId={season.id} {episode} />
        </li>
    {/each}
</ol>

<style>
    :root {
        --seasonpage-bottom-border-color: var(--color-grey-normal);
    }

    ol {
        display: flex;
        flex-direction: column;
        flex-wrap: nowrap;

        margin: 0;
        padding: 0;
        margin-inline: 5vw;
        margin-top: 3em;

        list-style-type: none;

        gap: 2em;
    }

    li {
        padding-bottom: 2em;

        border-bottom-style: solid;
        border-bottom-width: 2px;
        border-bottom-color: var(--seasonpage-bottom-border-color);
    }

    .last-li {
        border-bottom: none;
    }
</style>
