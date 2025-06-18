<script lang="ts">
  import Navbar from "../lib/Navbar.svelte";
</script>

<Navbar />

<main>
  <slot />
</main>

<style>
  :root {
    --background-color: #313131;
  }
  main {
    width: 100vw;
    left: 0;
    top: 0;
    position: absolute;
    background-color: var(--background-color);
    padding-top: 25vh;

    display: flex;
    justify-content: center;
    flex-flow: column nowrap;
  }
</style>
