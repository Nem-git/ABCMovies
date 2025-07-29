<script lang="ts">
    import type { Season } from "$lib/types";

    let {
        seasons,
        selectedSeason,
    }: { seasons: Season[]; selectedSeason: Season } = $props();
</script>

<ol>
    {#each seasons as season}
        <li>
            <a
                href={season.number.toString()}
                class={season.id === selectedSeason.id
                    ? "selected-season"
                    : "unselected-season"}
            >
                <span>
                    {season.title}
                </span>
            </a>
        </li>
    {/each}
</ol>

<style>
    :root {
        --season-selection-background-hover-color: var(--color-grey-dark);
        --season-selection-background-color: var(--color-grey-normal);

        --season-selection-border-radius: 0.3em;

        --season-selection-selected-underline-color: var(--color-light);
    }

    ol {
        display: grid;
        grid-template-columns: repeat(auto-fill, minmax(150px, 1fr));

        justify-items: center;
        list-style-type: none;
        padding: 0;

        gap: 1em;
    }

    a {
        padding: 1em 2em;

        border-radius: var(--season-selection-border-radius);
        border-color: var(--season-selection-border-color);

        transition-property: background-color;
        transition-duration: 0.2s;
        transition-timing-function: ease-in;

        transition-property: border-bottom-width;
        transition-duration: 0.05s;
        transition-timing-function: ease-in;
    }

    a:hover {
        text-decoration: none;
    }

    span {
        font-weight: bold;
    }

    .selected-season {
        border-bottom-width: 3px;
        border-bottom-style: solid;
        border-bottom-color: var(--season-selection-selected-underline-color);

        background-color: var(--season-selection-background-color);
    }

    .unselected-season:hover {
        background-color: var(--season-selection-background-hover-color);
    }
</style>
