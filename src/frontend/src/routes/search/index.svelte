<script lang="ts">
	import { goto } from "@roxi/routify";
	import type { Show } from "../../lib/api/config";
	import { getSearchResults } from "../../lib/api/search";

	import SearchBar from "./SearchBar.svelte";
	import ShowCard from "../../lib/components/ShowCard.svelte";

	import { q } from "../../lib/shared.svelte";
	import { onMount } from "svelte";

	onMount(() => {
		document.getElementById("search-input")?.focus();
	});

	let searchResults: Show[] | undefined = $state();

	$effect(() => {
		if ($state.snapshot(q).query === "") {
			$goto("/search");
		} else {
			$goto("/search", { q: $state.snapshot(q).query });
			getSearchResults($state.snapshot(q).query).then(
				async (sr) => (searchResults = await sr),
			);
		}
	});
</script>

<SearchBar />

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
	ol {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(325px, 1fr));

		padding: 0;
		gap: 1em;
		row-gap: 50px;
	}
</style>
