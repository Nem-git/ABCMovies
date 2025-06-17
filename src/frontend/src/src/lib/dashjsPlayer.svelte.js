import * as dashjs from "dashjs";
import "../../node_modules/dashjs/dist/modern/esm/dash.mss.debug";


export const manifestUrl = $state({
  url: ""
})

export const init = async () => {
    let player = dashjs.MediaPlayer().create();
    player.initialize(document.getElementById("video-player"), manifestUrl.url, true);
};