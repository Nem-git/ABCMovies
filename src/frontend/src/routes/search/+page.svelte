<script lang="ts">
    import { afterNavigate } from "$app/navigation";
    import { goto } from "$app/navigation";
    import type { PageProps } from "./$types";

    import ShowCard from "../../lib/components/ShowCard.svelte";

    let { data }: PageProps = $props();

    let searchInputEl: HTMLInputElement;

    afterNavigate((): void => {
        focusOnSearchBar();
    });

    let focusOnSearchBar = (): void => {
        let length: number = searchInputEl.value.length;
        searchInputEl.focus();
        searchInputEl.setSelectionRange(length, length);
    };

    let query: string = $state(data.query);

    let keydown = (event: KeyboardEvent): void => {
        if (document.activeElement !== searchInputEl) {
            focusOnSearchBar();
        }
    };

    $effect(() => {
        if (query === "") {
            goto("/search", { keepFocus: true });
        } else {
            goto("?" + new URLSearchParams({ q: query }), { keepFocus: true });
        }
    });
</script>

<svelte:window onkeydown={keydown} />

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
                d="M15.8 15.8 21 21m-3-10.5a7.5 7.5 0 1 1-15 0 7.5 7.5 0 0 1 15 0Z"
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
        --searchbar-background-color: transparent;
        --searchbar-border-radius: 0.5em;
        --searchbar-border-color: rgb(95, 60, 95);
        --searchbar-border-inactive-color: rgba(120, 120, 120, 0.3);
    }

    .flex-center {
        display: flex;
        justify-content: center;

        width: 100%;
    }

    .search-container {
        margin-top: 3em;

        background-color: var(--searchbar-background-color);
        box-shadow: var(--searchbar-box-shadow);
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

    input {
        background-color: transparent;

        font-size: 1em;
        font-weight: 500;

        flex-grow: 1;
    }

    .search-container:focus-within {
        border-color: var(--searchbar-border-color);
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
