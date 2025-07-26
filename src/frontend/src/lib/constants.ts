export const INTERNAL_API_URL = "http://nginx/api/";
export const API_URL = "http://localhost:8080/api/";

export const SHORTCUTS = {
    togglepause: [" ", "K"],
    toggletheater: ["T"],
    togglesubtitles: ["C"],
    togglemute: ["M"],
    volumeup: ["ARROWUP"],
    volumedown: ["ARROWDOWN"],
    seekforward: ["ARROWRIGHT", "L"],
    seekbackward: ["ARROWLEFT", "J"],
    gotostart: ["0"],
    gotoend: ["9"],
    enterfullscreen: ["F"],
    exitfullscreen: ["F", "ESCAPE"],
};

export const VIDEO_PLAYER_VALUES = {
    volumeJump: 0.05, // Volume between 0 and 1
    seekJump: 5, // In seconds
};
