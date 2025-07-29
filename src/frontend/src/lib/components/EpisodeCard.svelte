<script lang="ts">
    import type { Episode } from "$lib/types";

    let { seasonId, episode }: { seasonId: string; episode: Episode } =
        $props();

    let href = [seasonId, episode.number].join("/");
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
</style>
