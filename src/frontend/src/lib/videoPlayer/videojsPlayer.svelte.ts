import type Player from "video.js/dist/types/player";

import videojs from "video.js";

import "video.js/dist/video-js.min.css";
import { playerInfo } from "$lib/helpers/videojsPlayerHelper.svelte";

export const init = async (url: string, videoPlayerEl: HTMLVideoElement) => {
    let player: Player = videojs(videoPlayerEl, {
        controls: true,
        autoplay: true,
        fluid: true,
        responsive: true,
        preload: "auto",
    });

    player.src({
        src: url,
        type: "application/dash+xml",
    });

    return player;
};

export const toggleFullscreen = async (
    player: Player,
    wantsFullscreen = true,
) => {
    if (wantsFullscreen) {
        try {
            player.requestFullscreen();
        } catch {
            player.enterFullWindow();
        }
    } else {
        if (document.fullscreenElement) {
            player.exitFullscreen();
        } else if (player.isFullWindow) {
            player.exitFullWindow();
        }
    }
};

export const toggleSubtitles = async (
    player: Player,
    wantsSubtitles = true,
) => {
    if (wantsSubtitles && !playerInfo.subtitlesEnabled) {
        console.log("Subtitles enabled");
    } else if (playerInfo.subtitlesEnabled) {
        console.log("Subtitles disabled");
    }
};
