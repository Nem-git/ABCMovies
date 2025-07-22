<script lang="ts">
    import { onMount } from "svelte";

    import { goto } from "$app/navigation";
    import type { PageProps } from "./$types";

    import ShowCard from "../../lib/components/ShowCard.svelte";

    let { data }: PageProps = $props();

    let searchInputEl: HTMLInputElement;

    onMount(() => {
        focusOnSearchBar();
    });

    let focusOnSearchBar = () => {
        let length = searchInputEl.value.length;
        searchInputEl.focus();
        searchInputEl.setSelectionRange(length, length);
    };

    let query = $state(data.query);

    $effect(() => {
        if (query === "") {
            goto("/search", { keepFocus: true });
        } else {
            goto("?" + new URLSearchParams({ q: query }), { keepFocus: true });
        }
    });
</script>

<button
    class="search-container"
    onclick={() => focusOnSearchBar()}
    aria-label="Search Bar"
>
    <svg
        class="search-icon"
        xmlns="http://www.w3.org/2000/svg"
        fill="none"
        viewBox="0 0 24 24"
    >
        <path
            stroke="#000"
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M15.8 15.8 21 21m-3-10.5a7.5 7.5 0 1 1-15 0 7.5 7.5 0 0 1 15 0Z"
        />
    </svg>
    <input type="text" bind:this={searchInputEl} bind:value={query} />
</button>

<ul>
    {#each data.searchResults as show}
        <li><ShowCard {show} /></li>
    {/each}
</ul>

<style>
    .search-container {
        margin-inline: 10vw 10vw;

        background-color: var(--searchbar-background-color);
        box-shadow: var(--searchbar-box-shadow);
        border-radius: var(--searchbar-border-radius);

        border-width: 3px;
        border-style: solid;
        border-color: var(--searchbar-border-inactive-color);

        display: flex;
        align-items: center;
        justify-content: space-evenly;

        gap: 5px;
        height: 60px;
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

        list-style-type: none;
        padding: 0;
        gap: 1em;
        row-gap: 50px;
    }

    .search-icon {
        width: 40px;
    }
</style>
