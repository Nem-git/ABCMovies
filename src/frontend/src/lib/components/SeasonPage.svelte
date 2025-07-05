<script lang="ts">
	let { path }: { path: Path } = $props();

	import { Path } from "../path";
	import type { Season } from "../api/config";
	import { getSeason } from "../api/season";
	import EpisodeCard from "./EpisodeCard.svelte";
	import { seasonId } from "../shared.svelte";

	let s: Promise<Season> = $derived(
		getSeason([path.getShow(), seasonId.id].join("/")),
	);
</script>

{#if s}
	{#await s then sea}
		<h3>{sea.title}</h3>
		<ol>
			{#each sea.episodes as episode}
				<EpisodeCard {episode} {path} />
			{/each}
		</ol>
	{/await}
{/if}
