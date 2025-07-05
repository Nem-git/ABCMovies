<script lang="ts">
	import { q } from "../../lib/shared.svelte";

	let searchInputEl: HTMLInputElement;

	let oninput = (event: any) => {
		q.query = event.target.value;
	};
</script>

<div class="search-container">
	<img src="/searchX.svg" alt="Magnifying glass" class="search-icon" />
	<input
		type="text"
		id="search-input"
		bind:this={searchInputEl}
		value={$state.snapshot(q.query)}
		{oninput}
	/>
</div>

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

	#search-input {
		background-color: transparent;

		width: 100%;
		height: 100%;

		font-size: 1em;
		font-weight: 500;
	}

	.search-container:focus-within {
		border-color: var(--searchbar-border-color);
	}
</style>
