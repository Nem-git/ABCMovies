<script lang="ts">
	let { streamingService, show }: { streamingService: string; show: string } =
		$props();

	import type { Show } from "../types";
	import { onMount } from "svelte";
	import { url } from "@roxi/routify";
	import { getShow } from "../api";
	import { id } from "../shared.svelte";
	import SeasonPage from "./SeasonPage.svelte";

	let s: Promise<Show> | undefined = $state();

	onMount(async () => {
		s = getShow(streamingService, show);

		// Make that seasonId check to avoid race conditions, where it sets the right season
		// then the first available season
		if (!$state.snapshot(id).season) {
			setSeason((await s).seasons[0].id);
		}
	});

	const setSeason = (sId: string) => {
		id.season = sId;
	};
</script>

<div class="hero">
	<div class="hero-info">
		{#if s}
			{#await s}
				<p>Loading...</p>
			{:then sh}
				<span class="title">{sh.title}</span>
				<span class="description">{sh.fullDescription}</span>
				<span class="year">Release year: {sh.year}</span>
			{/await}
		{/if}
	</div>
	{#if s}
		{#await s then sh}
			<div class="img-container">
				<img
					src={sh.imageBackground.replace("_Size_", "1280")}
					alt={sh.title}
				/>
			</div>
		{/await}
	{/if}
</div>

<ol>
	{#if s}
		{#await s then sh}
			{#each sh.seasons as season}
				<a
					onclick={() => {
						setSeason(season.id);
					}}
					href={$url("/[streamingService]/[show]", {
						streamingService: streamingService,
						show: show,
						s: season.id,
					})}
					aria-label={season.title}>{season.title}</a
				>
			{/each}
		{/await}
	{/if}
</ol>

<SeasonPage {streamingService} {show} />

<style>
	img {
		object-fit: cover;
		width: 100%;

		border-style: solid;
		border-width: 5px;
		border-radius: var(--showpage-image-border-radius);
		border-color: var(--showpage-image-border-color);

		box-shadow: var(--showpage-image-box-shadow);
	}

	ol {
		display: flex;
		flex-direction: row;
		flex-wrap: nowrap;
		overflow: scroll;

		gap: 50px;
	}

	.hero {
		display: flex;
		flex-direction: row;
		flex-wrap: nowrap;

		justify-content: center;
	}

	.hero-info {
		display: flex;
		flex-direction: column;

		max-width: 40vw;

		margin-right: 30px;
	}

	.title {
		font-size: 50px;
	}

	.description {
		margin-top: 50px;
		font-size: large;

		line-height: 1.3em;
	}

	.year {
		margin-top: 10px;
		font-size: small;

		color: var(--showpage-year-color);
	}

	.img-container {
		max-width: 30vw;
	}
</style>
