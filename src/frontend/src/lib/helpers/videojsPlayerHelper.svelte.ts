import { SHORTCUTS, VIDEO_PLAYER_VALUES } from "$lib/constants";

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
});

export function handleShortcuts(key: string) {
    switch (true) {
        case SHORTCUTS.togglepause.includes(key):
            playerInfo.paused = !playerInfo.paused;
            break;

        case SHORTCUTS.toggletheater.includes(key):
            playerInfo.theaterEnabled = !playerInfo.theaterEnabled;
            break;

        case SHORTCUTS.togglesubtitles.includes(key):
            playerInfo.subtitlesEnabled = !playerInfo.subtitlesEnabled;
            break;

        case SHORTCUTS.togglemute.includes(key):
            playerInfo.muted = !playerInfo.muted;
            break;

        case SHORTCUTS.volumeup.includes(key):
            playerInfo.volume += VIDEO_PLAYER_VALUES.volumeJump;
            break;

        case SHORTCUTS.volumedown.includes(key):
            playerInfo.volume -= VIDEO_PLAYER_VALUES.volumeJump;
            break;

        case SHORTCUTS.seekforward.includes(key):
            playerInfo.currentTime += VIDEO_PLAYER_VALUES.seekJump;
            break;

        case SHORTCUTS.seekbackward.includes(key):
            playerInfo.currentTime -= VIDEO_PLAYER_VALUES.seekJump;
            break;

        case SHORTCUTS.gotostart.includes(key):
            playerInfo.currentTime = 0;
            break;

        case SHORTCUTS.gotoend.includes(key):
            playerInfo.currentTime = -1;
            break;

        case SHORTCUTS.enterfullscreen.includes(key):
            playerInfo.fullscreen = true;
            break;

        case SHORTCUTS.exitfullscreen.includes(key):
            playerInfo.fullscreen = false;
            break;

        default:
            break;
    }
}
