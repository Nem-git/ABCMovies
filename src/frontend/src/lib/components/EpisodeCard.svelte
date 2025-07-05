<script lang="ts">
	let { episode, path }: { episode: Episode; path: Path } = $props();

	import type { Episode } from "../api/config";
	import type { Path } from "../path";
	import { seasonId } from "../shared.svelte";
</script>

<div id="layout">
	<a href={[path.getShow(), seasonId.id, episode.number].join("/")}>
		<img
			src={episode.imageCard.replace("_Size_", "480")}
			alt={episode.title}
		/>
	</a>
	<div id="text">
		<a href={[path.getShow(), seasonId.id, episode.number].join("/")}>
			<h5>{episode.title}</h5>
		</a>
		<p id="description">{episode.fullDescription}</p>
	</div>
</div>

<style>
	#layout {
		display: flex;
		flex-flow: row nowrap;
	}

	#text {
		display: flex;
		flex-flow: column nowrap;
	}

	img {
		object-fit: cover;
	}

	#description {
		font-family: sans-serif;
	}
</style>
