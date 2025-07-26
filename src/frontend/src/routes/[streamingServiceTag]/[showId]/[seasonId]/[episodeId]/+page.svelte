<script lang="ts">
    import type { PageProps } from "./$types";

    import VideoPlayer from "./VideoPlayer.svelte";

    let { data }: PageProps = $props();
</script>

<VideoPlayer url={data.episode.url} />
