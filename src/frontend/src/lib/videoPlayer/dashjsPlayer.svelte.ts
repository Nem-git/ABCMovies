import { MediaPlayer } from "dashjs";

export const init = async (url: string, videoPlayerEl: HTMLVideoElement) => {
    let player = MediaPlayer().create();
    player.initialize(videoPlayerEl, url, true);

    player.updateSettings({
        streaming: {
            // https://cdn.dashjs.org/latest/jsdoc/module-Settings.html#~Protection
            protection: {
                ignoreKeyStatuses: true,
            },
        },
    });
};
