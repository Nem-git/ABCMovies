import * as shaka from "shaka-player/dist/shaka-player.compiled.js";

export const init = async (url: string, videoPlayerEl: HTMLVideoElement) => {
    // Install built-in polyfills to patch browser incompatibilities.
    shaka.polyfill.installAll();

    // Check to see if the browser supports the basic APIs Shaka needs.
    if (!shaka.Player.isBrowserSupported()) {
        // This browser does not have the minimum set of APIs we need.
        console.error("Browser not supported!");
        return;
    } else {
    }

    // Create a Player instance.;
    const player = new shaka.Player();
    await player.attach(videoPlayerEl);

    // Attach player to the window to make it easy to access in the JS console.
    // window.player = player;

    // Listen for error events.
    player.addEventListener("error", onErrorEvent);

    // Try to load a manifest.
    // This is an asynchronous process.
    try {
        await player.load(url);
        // This runs if the asynchronous load is successful.
        console.log("The video has now been loaded!");
    } catch (e) {
        // onError is executed if the asynchronous load fails.
        onError(e);
    }

    function onErrorEvent(event) {
        // Extract the shaka.util.Error object from the event.
        onError(event.detail);
    }

    function onError(error) {
        // Log the error.
        console.error("Error code", error.code, "object", error);
    }
};
