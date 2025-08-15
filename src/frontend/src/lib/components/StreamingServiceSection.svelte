<script lang="ts">
    import type { StreamingService } from "$lib/types";

    let { streamingService }: { streamingService: StreamingService } = $props();
</script>

<div class="hero">
    <div class="hero-info-container">
        <div class="hero-info">
            <span class="title">{streamingService.name}</span>
            <span class="description">{streamingService.fullDescription}</span>
            <span class="tag">Tag: {streamingService.tag}</span>
        </div>
        <div></div>
    </div>
</div>

<div class="img-background-container">
    <div></div>
    <div class="img-container">
        <img
            src={streamingService.imageBackground.replace("_Size_", "1280")}
            alt={streamingService.name}
        />
    </div>
</div>

<style>
    :root {
        --showpage-year-color: var(--color-whiteish);
    }

    img {
        object-fit: cover;
    }

    .hero {
        height: 500px;
        max-height: 80%;
    }

    .hero-info-container {
        position: relative;

        display: flex;
        flex-direction: row;
        flex-wrap: nowrap;

        overflow: hidden;

        justify-content: space-between;
    }

    .img-background-container {
        position: absolute;

        z-index: -1;

        width: 100%;

        max-height: 80%;

        display: flex;
        flex-direction: row;
        flex-wrap: nowrap;

        justify-content: space-between;
    }

    .hero-info {
        display: flex;
        flex-direction: column;

        max-width: 40vw;

        margin-right: 30px;
    }

    .title {
        font-size: 50px;
    }

    .description {
        margin-top: 50px;
        font-size: large;

        line-height: 1.3em;
    }

    .year {
        margin-top: 10px;
        font-size: small;

        color: var(--showpage-year-color);
    }

    .img-container {
        max-height: 50em;
    }
</style>
