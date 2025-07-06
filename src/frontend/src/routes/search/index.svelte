<script lang="ts">
	import { goto } from "@roxi/routify";
	import { params } from "@roxi/routify";

	import type { Show } from "../../lib/types";
	import { getSearchResults } from "../../lib/api";

	import ShowCard from "../../lib/components/ShowCard.svelte";

	import { onMount } from "svelte";

	let { q } = $params;

	let searchResults: Show[] | undefined = $state();
	let query: string = $state(q ?? "");

	let searchInputEl: HTMLInputElement;

	onMount(() => {
		searchInputEl.focus();
	});

	let oninput = (event: any) => {
		query = event.target.value;
	};

	$effect(() => {
		if (query === "") {
			$goto("/search");
		} else {
			$goto("/search", { q: query });
			getSearchResults(query).then(
				async (sr) => (searchResults = await sr),
			);
		}
	});
</script>

<div class="search-container">
	<img src="/searchX.svg" alt="Magnifying glass" class="search-icon" />
	<input type="text" bind:this={searchInputEl} value={query} {oninput} />
</div>

<ol>
	{#if searchResults}
		{#await searchResults then}
			{#each searchResults as show}
				<ShowCard {show} />
			{/each}
		{/await}
	{/if}
</ol>

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

	ol {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(325px, 1fr));

		padding: 0;
		gap: 1em;
		row-gap: 50px;
	}
</style>
