<script lang="ts">
  import Navbar from "../lib/components/Navbar.svelte";
</script>

<Navbar />

<main>
  <slot />
</main>
<div id="background"></div>

<style>
  :root {
    --background-color: #313131;
  }
  main {
    width: 100vw;
    left: 0;
    top: 0;
    position: absolute;

    display: flex;
    justify-content: center;
    flex-flow: column nowrap;

    z-index: 1;
  }

  #background {
    position: fixed;
    width: 100vw;
    height: 100vh;
    top: 0;
    left: 0;

    background-color: var(--background-color);
  }
</style>
