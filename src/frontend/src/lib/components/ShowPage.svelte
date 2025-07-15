<script lang="ts">
    import type { Show } from "$lib/types";

    let { show }: { show: Show } = $props();
</script>

<div class="hero">
    <div class="hero-info">
        <span class="title">{show.title}</span>
        <span class="description">{show.fullDescription}</span>
        <span class="year">Release year: {show.year}</span>
    </div>
    <div class="img-container">
        <img
            src={show.imageBackground.replace("_Size_", "1280")}
            alt={show.title}
        />
    </div>
</div>

<style>
    img {
        object-fit: cover;
        width: 100%;

        border-style: solid;
        border-width: 5px;
        border-radius: var(--showpage-image-border-radius);
        border-color: var(--showpage-image-border-color);

        box-shadow: var(--showpage-image-box-shadow);
    }

    .hero {
        display: flex;
        flex-direction: row;
        flex-wrap: nowrap;

        justify-content: center;
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
        max-width: 30vw;
    }
</style>
