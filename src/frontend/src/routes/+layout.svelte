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
        flex-wrap: nowrap;
        min-height: 100vh;
        box-sizing: border-box;
    }

    main {
        display: flex;
        flex-direction: column;
        flex-grow: 1;
        max-width: 100vw;

        z-index: 0;
    }
</style>
