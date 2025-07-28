<script lang="ts">
    import type Player from "video.js/dist/types/player";

    import { onMount } from "svelte";

    import {
        playerInfo,
        handleShortcuts,
    } from "$lib/helpers/videojsPlayerHelper.svelte";

    let { url }: { url: string } = $props();

    let videoPlayerEl: HTMLVideoElement;
    let videoPlayer: Player;

    let onkeydown = (event: KeyboardEvent): void => {
        handleShortcuts(event.key.toUpperCase(), videoPlayer);
    };

    onMount(async () => {
        let videojs = import("$lib/videoPlayer/videojsPlayer.svelte");
        videoPlayer = await (await videojs).init(url, videoPlayerEl);
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
        bind:duration={playerInfo.duration}
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
