import * as dashjs from "dashjs";

export const init = async (url: string, videoPlayerEl: HTMLVideoElement) => {
    let player = dashjs.MediaPlayer().create();
    player.initialize(videoPlayerEl, url, true);

    player.updateSettings({
        streaming: {
            protection: {
                ignoreEmeEncryptedEvent: true,
            },
        },
    });
};
