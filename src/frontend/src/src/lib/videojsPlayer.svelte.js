import videojs from "video.js";
import "../../node_modules/video.js/dist/video";


export const manifestUrl = $state({
  url: ""
})

export const init = async () => {
    let player = videojs("videoPlayer", {
        controls: true,
        autoplay: true,
        preload: "auto",
    });

    player.src({
      src: await manifestUrl.url,
      type: "application/dash+xml",
    })
};