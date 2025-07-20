<script lang="ts">
    import { onMount } from "svelte";

    import { goto } from "$app/navigation";
    import type { PageProps } from "./$types";

    import search from "$lib/images/search.svg";
    import ShowCard from "../../lib/components/ShowCard.svelte";

    let { data }: PageProps = $props();

    let searchInputEl: HTMLInputElement;

    onMount(() => {
        let length = searchInputEl.value.length;
        searchInputEl.focus();
        searchInputEl.setSelectionRange(length, length);
    });

    let query = $state(data.query);

    $effect(() => {
        if (query === "") {
            goto("/search", { keepFocus: true });
        } else {
            goto("?" + new URLSearchParams({ q: query }), { keepFocus: true });
        }
    });
</script>

<div class="search-container">
    <img src="x" alt="Magnifying glass" class="search-icon" />
    <input type="text" bind:this={searchInputEl} bind:value={query} />
</div>

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

        gap: 5px;
        height: 60px;
    }

    input {
        background-color: transparent;

        width: 100%;
        height: 100%;

        font-size: 1em;
        font-weight: 500;
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
</style>
