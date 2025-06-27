<script lang="ts">
  import Navbar from "../lib/components/Navbar.svelte";
</script>

<Navbar />

<main>
  <div id="slot">
    <slot />
  </div>
</main>
<div id="background"></div>

<style>
  :root {
    --background-color: #313131;
  }
  main {
    position: relative;

    display: flex;
    justify-content: center;
    flex-flow: column nowrap;

    z-index: 1;
  }

  #slot {
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
