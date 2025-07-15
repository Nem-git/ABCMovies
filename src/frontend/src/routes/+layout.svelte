<script lang="ts">
    import Header from "./Header.svelte";
    import Footer from "./Footer.svelte";
    import "../app.css";

    let { children } = $props();
</script>

<div class="app">
    <Header />

    <main>
        {@render children()}
    </main>

    <Footer />
</div>

<style>
    .app {
        display: flex;
        flex-direction: column;
        min-height: 100vh;
        box-sizing: border-box;
    }

    main {
        display: flex;
        flex-direction: column;
        padding-top: 150px;
        width: calc(100% - 2 * 5vw);
        max-width: 100rem;
        padding-inline: 5vw;
        margin: 0 auto;
    }
</style>
