<script lang="ts">
    import { onMount } from "svelte";

    import {
        playerInfo,
        handleShortcuts,
    } from "$lib/helpers/videojsPlayerHelper.svelte";

    let { url }: { url: string } = $props();

    let videoPlayerEl: HTMLVideoElement;

    let onkeydown = (event: KeyboardEvent): void => {
        handleShortcuts(event.key.toUpperCase());
    };

    onMount(async () => {
        let videojs = await import("$lib/videoPlayer/videojsPlayer.svelte");
        videojs.init(url, videoPlayerEl);
    });
</script>

<svelte:window {onkeydown} />

<div class="container">
    <video
        class="video-player video-js"
        bind:this={videoPlayerEl}
        controls
        autoplay
        bind:paused={playerInfo.paused}
        bind:currentTime={playerInfo.currentTime}
        bind:volume={playerInfo.volume}
        bind:muted={playerInfo.muted}
    >
        <track kind="captions" />
        <!-- I am unsure about the need for this, but it gives me errors in svelte when I don't put it -->
    </video>
</div>

<style>
    .container {
        display: flex;

        height: 100%;
        width: 100%;

        justify-content: center;
    }

    .video-player {
        max-width: 70vw;
        max-height: 70vh;
    }
</style>
