<script lang="ts">
    import Footer from "$lib/components/common/Footer.svelte";
    import Header from "$lib/components/common/Header.svelte";

    let { children } = $props();
</script>

<Header />

<div class="flex flex-col items-center bg-neutral-950">
    {@render children()}
</div>

<Footer />
