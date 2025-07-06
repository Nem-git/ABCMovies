<script lang="ts">
	let { streamingService, show }: { streamingService: string; show: string } =
		$props();

	import type { Season } from "../api/config";
	import { getSeason } from "../api/season";
	import EpisodeCard from "./EpisodeCard.svelte";
	import { id } from "../shared.svelte";

	let s: Promise<Season> | undefined = $state();

	$effect(() => {
		s = getSeason(streamingService, show, id.season);
	});
</script>

{#if s}
	{#await s then sea}
		<h3>{sea.title}</h3>
		<ol>
			{#each sea.episodes as episode}
				<EpisodeCard {streamingService} {show} {episode} />
			{/each}
		</ol>
	{/await}
{/if}
