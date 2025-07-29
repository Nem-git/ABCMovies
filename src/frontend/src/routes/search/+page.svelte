<script lang="ts">
    import { afterNavigate } from "$app/navigation";
    import { goto } from "$app/navigation";
    import type { PageProps } from "./$types";

    import ShowCard from "../../lib/components/ShowCard.svelte";

    let { data }: PageProps = $props();

    let searchInputEl: HTMLInputElement;

    afterNavigate(async () => {
        focusOnSearchBar();
        if (data.query !== query) {
            getSearchResults();
        }
    });

    let focusOnSearchBar = (): void => {
        let length: number = searchInputEl.value.length;
        searchInputEl.focus();
        searchInputEl.setSelectionRange(length, length);
    };

    let query: string = $state(data.query);

    let onkeypress = (event: KeyboardEvent): void => {
        if (document.activeElement !== searchInputEl) {
            query += event.key;
            focusOnSearchBar();
        }
    };

    let getSearchResults = () => {
        if (query === "") {
            goto("/search", { keepFocus: true });
        } else {
            goto("?" + new URLSearchParams({ q: query }), { keepFocus: true });
        }
    };

    $effect(() => {
        getSearchResults();
    });
</script>

<svelte:window {onkeypress} />

<div class="flex-center">
    <button
        class="search-container"
        onclick={() => focusOnSearchBar()}
        aria-label="Search Bar"
    >
        <svg
            class="search-icon"
            xmlns="http://www.w3.org/2000/svg"
            viewBox="0 0 24 24"
        >
            <path
                class="search-icon-line"
                d="m20 20l-4.05-4.05m0 0a7 7 0 1 0-9.9-9.9a7 7 0 0 0 9.9 9.9"
            />
        </svg>
        <input type="text" bind:this={searchInputEl} bind:value={query} />
    </button>
</div>

<div class="flex-center">
    <ul>
        {#each data.searchResults as show}
            <li>
                <ShowCard {show} />
            </li>
        {/each}
    </ul>
</div>

<style>
    :root {
        --searchbar-background-color: var(--color-darker);
        --searchbar-border-radius: 0.5em;
        --searchbar-border-color: var(--color-light);
        --searchbar-border-inactive-color: var(--color-dark);

        --search-icon-color: var(--color-whiteish);
    }

    .flex-center {
        display: flex;
        justify-content: center;

        width: 100%;
    }

    .search-container {
        margin-top: 3em;

        background-color: var(--searchbar-background-color);
        border-radius: var(--searchbar-border-radius);

        border-width: 3px;
        border-style: solid;
        border-color: var(--searchbar-border-inactive-color);

        display: flex;
        align-items: center;
        justify-content: space-evenly;

        gap: 1em;
        height: 4em;

        width: 80%;
        max-width: 1200px;

        padding-inline: 1em;
    }

    .search-container:hover {
        cursor: text;
    }

    .search-container:focus-within {
        border-color: var(--searchbar-border-color);
    }

    input {
        background-color: transparent;

        font-size: 1em;
        font-weight: 500;

        flex-grow: 1;
    }

    ul {
        display: grid;
        grid-template-columns: repeat(auto-fill, minmax(325px, 1fr));

        justify-items: center;
        list-style-type: none;
        padding: 0;
        gap: 1em;
        row-gap: 50px;

        width: 90%;
    }

    .search-icon {
        width: 2em;
        fill: none;
    }

    .search-icon-line {
        stroke: var(--search-icon-color);
        stroke-linecap: round;
        stroke-linejoin: round;
        stroke-width: 0.12em;
    }
</style>
