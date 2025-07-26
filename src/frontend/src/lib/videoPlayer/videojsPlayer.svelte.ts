import type Player from "video.js/dist/types/player";

import videojs from "video.js";
import "videojs-contrib-quality-levels";

import "video.js/dist/video-js.min.css";

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

    // player.qualityLevels();

    videojs.log.level("debug"); // TODO: Remove, just for debug
};
