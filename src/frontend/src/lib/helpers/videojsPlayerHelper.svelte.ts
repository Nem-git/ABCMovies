import type Player from "video.js/dist/types/player";

import { SHORTCUTS, VIDEO_PLAYER_VALUES } from "$lib/constants";

import {
    toggleFullscreen,
    toggleSubtitles,
} from "$lib/videoPlayer/videojsPlayer.svelte";

export const playerInfo = $state({
    // Interacts with the video element directly
    paused: false,
    currentTime: 0,
    volume: 1,
    muted: false,

    // Depends on the video player
    theaterEnabled: false,
    subtitlesEnabled: false,
    fullscreen: false,

    // READ-ONLY
    duration: 0,
});

export function handleShortcuts(key: string, player: Player | null = null) {
    switch (true) {
        case SHORTCUTS.togglepause.includes(key):
            playerInfo.paused = !playerInfo.paused;
            break;

        case SHORTCUTS.toggletheater.includes(key):
            playerInfo.theaterEnabled = !playerInfo.theaterEnabled;
            break;

        case SHORTCUTS.togglesubtitles.includes(key):
            if (player) {
                playerInfo.subtitlesEnabled = !playerInfo.subtitlesEnabled;
                toggleSubtitles(player);
            }

            break;

        case SHORTCUTS.togglemute.includes(key):
            playerInfo.muted = !playerInfo.muted;
            break;

        case SHORTCUTS.volumeup.includes(key):
            if (playerInfo.volume > 1 + VIDEO_PLAYER_VALUES.volumeJump) {
                playerInfo.volume = 1;
            } else {
                playerInfo.volume += VIDEO_PLAYER_VALUES.volumeJump;
            }

            break;

        case SHORTCUTS.volumedown.includes(key):
            if (playerInfo.volume < VIDEO_PLAYER_VALUES.volumeJump) {
                playerInfo.volume = 0;
            } else {
                playerInfo.volume -= VIDEO_PLAYER_VALUES.volumeJump;
            }

            break;

        case SHORTCUTS.seekforward.includes(key):
            if (
                playerInfo.currentTime + VIDEO_PLAYER_VALUES.seekJump >
                playerInfo.duration
            ) {
                playerInfo.currentTime = playerInfo.duration;
            } else {
                playerInfo.currentTime += VIDEO_PLAYER_VALUES.seekJump;
            }
            break;

        case SHORTCUTS.seekbackward.includes(key):
            if (playerInfo.currentTime < VIDEO_PLAYER_VALUES.seekJump) {
                playerInfo.currentTime = 0;
            } else {
                playerInfo.currentTime -= VIDEO_PLAYER_VALUES.seekJump;
            }

            break;

        case SHORTCUTS.gotostart.includes(key):
            playerInfo.currentTime = 0;
            break;

        case SHORTCUTS.gotoend.includes(key):
            playerInfo.currentTime = playerInfo.duration;
            break;

        case SHORTCUTS.exitfullscreen.includes(key) &&
            SHORTCUTS.enterfullscreen.includes(key):
            if (player && playerInfo.fullscreen) {
                playerInfo.fullscreen = false;
                toggleFullscreen(player, false);
            } else if (player && !playerInfo.fullscreen) {
                playerInfo.fullscreen = true;
                toggleFullscreen(player);
            }

            break;

        case SHORTCUTS.exitfullscreen.includes(key):
            if (player) {
                playerInfo.fullscreen = false;
                toggleFullscreen(player, false);
            }

            break;

        case SHORTCUTS.enterfullscreen.includes(key):
            if (player) {
                playerInfo.fullscreen = true;
                toggleFullscreen(player);
            }

            break;

        default:
            break;
    }
}
