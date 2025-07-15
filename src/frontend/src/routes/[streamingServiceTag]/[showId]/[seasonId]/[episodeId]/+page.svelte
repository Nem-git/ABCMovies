<script lang="ts">
    import type { PageProps } from "./$types";

    import { onMount } from "svelte";

    let { data }: PageProps = $props();

    let videoPlayerEl: HTMLVideoElement;

    onMount(() => {
        import("dashjs").then((dashjs) => {
            let player = dashjs.MediaPlayer().create();
            player.initialize(videoPlayerEl, data.episode.url, true);
        });
    });
</script>

<div id="fullscreen">
    <div id="container">
        <a href="./" id="back-button">Back</a>
        <video id="video-player" bind:this={videoPlayerEl} controls autoplay>
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
