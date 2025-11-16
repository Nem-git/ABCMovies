<script lang="ts">
    import VideoPlayer from "$lib/components/stream/VideoPlayer.svelte";
    import type { PageProps } from "./$types";

    let { data }: PageProps = $props();

    console.log(data);
</script>

<div class="h-full">
    <VideoPlayer />
</div>
