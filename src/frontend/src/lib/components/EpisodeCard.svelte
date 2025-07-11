<script lang="ts">
    let {
        streamingService,
        show,
        episode,
    }: { streamingService: string; show: string; episode: Episode } = $props();

    import type { Episode } from "../types";
    import { id } from "../shared.svelte";

    let href = `/${[streamingService, show, id.season, episode.number].join("/")}`;
</script>

<div id="layout">
    <a {href}>
        <img
            src={episode.imageCard.replace("_Size_", "480")}
            alt={episode.title}
        />
    </a>
    <div id="text">
        <a {href}>
            <h5>{episode.title}</h5>
        </a>
        <p id="description">{episode.fullDescription}</p>
    </div>
</div>

<style>
    #layout {
        display: flex;
        flex-flow: row nowrap;
    }

    #text {
        display: flex;
        flex-flow: column nowrap;
    }

    img {
        object-fit: cover;
    }

    #description {
        font-family: sans-serif;
    }
</style>
