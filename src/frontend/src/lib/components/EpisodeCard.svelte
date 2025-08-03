<script lang="ts">
    import type { Episode } from "$lib/types";

    let { seasonNumber, episode }: { seasonNumber: number; episode: Episode } =
        $props();

    let href = [seasonNumber, episode.number].join("/");
</script>

<article>
    <a {href} tabindex="-1">
        <img
            src={episode.imageCard.replace("_Size_", "480")}
            alt={episode.title}
        />
    </a>
    <div class="text">
        <a {href} class="title">
            <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20">
                <path
                    fill="currentColor"
                    fill-rule="evenodd"
                    d="M10 18a8 8 0 1 0 0-16 8 8 0 0 0 0 16M9.6 7.2A1 1 0 0 0 8 8v4a1 1 0 0 0 1.6.8l3-2a1 1 0 0 0 0-1.6z"
                />
            </svg>
            <h3>{episode.title}</h3>
        </a>
        <p class="description">{episode.fullDescription}</p>
    </div>
</article>

<style>
    :root {
        --play-icon-color: var(--color-light);

        --container-border-radius: 2em;
    }

    article {
        display: flex;
        flex-direction: row;
        flex-wrap: nowrap;

        gap: 1.5em;

        border-radius: var(--container-border-radius);
    }

    .text {
        display: flex;
        flex-flow: column nowrap;
    }

    .title {
        display: flex;
        flex-direction: row;
        flex-wrap: nowrap;

        gap: 0.5em;
    }

    .description {
        margin-left: 2em;
    }

    img {
        object-fit: cover;
    }

    svg {
        width: 3em;
    }

    path {
        fill: var(--play-icon-color);
    }
</style>
