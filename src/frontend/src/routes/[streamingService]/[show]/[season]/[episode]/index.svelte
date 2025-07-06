<script lang="ts">
	import { init } from "../../../../../lib/videoPlayer/dashjsPlayer.svelte";
	import { manifestUrl } from "../../../../../lib/videoPlayer/dashjsPlayer.svelte";

	import { url } from "@roxi/routify";
	import type { Episode } from "../../../../../lib/api/config";
	import { getEpisode } from "../../../../../lib/api/episode";
	import { onMount } from "svelte";
	import { Path } from "../../../../../lib/path";
	import { params } from "@roxi/routify";

	let { streamingService, show, season, episode } = $params;

	let e: Promise<Episode> = getEpisode(
		streamingService,
		show,
		season,
		episode,
	);

	onMount(async () => {
		manifestUrl.url = (await e).url;
		init();
	});
</script>

<div id="fullscreen">
	<div id="container">
		<a href="./" id="back-button">Back</a>
		<video id="video-player" controls autoplay>
			<track kind="captions" />
			<!-- I am unsure about the need for this, but it gives me errors in svelte when I don't put it -->
		</video>
	</div>
</div>

<style>
	#video-player {
		max-width: 100vw;
		max-height: 100vh;
	}

	#container {
		position: absolute;
	}

	#fullscreen {
		display: flex;
		align-items: center;
		justify-content: center;

		height: 100vh;
		min-width: 100%;
		position: absolute;
		top: 0;
		left: 0;
	}

	#back-button {
		position: absolute;
		top: 0;
		left: 0;
		z-index: 1;
	}
</style>
