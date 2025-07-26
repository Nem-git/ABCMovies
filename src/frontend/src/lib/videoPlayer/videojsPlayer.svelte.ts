import type Player from "video.js/dist/types/player";

import videojs from "video.js";

import "video.js/dist/video-js.min.css";

export const init = async (url: string, videoPlayerEl: HTMLVideoElement) => {
    let player: Player = videojs(videoPlayerEl, {
        controls: true,
        autoplay: true,
        preload: "auto",
        fluid: true,
    });

    player.src({
        src: url,
        type: "application/dash+xml",
    });

    videojs.log.level("debug"); // TODO: Remove, just for debug
};
