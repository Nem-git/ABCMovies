<script lang="ts">
  let searchOnClick = () => {
    document.getElementById("search-input")?.focus();
  };
</script>

<header>
  <nav>
    <a href="/">Home</a>
    <a href="/search" onclick={searchOnClick}>Search</a>
    <a href="/account">Account</a>
  </nav>
</header>

<style>
  :root {
    /** Nav */
    --nav-background-blur: blur(1.5rem);
    --nav-border-radius: 1.5em;

    /** Buttons */
    --button-background-color: white;
    --button-border-radius: 3em;
  }

  header {
    display: flex;
    justify-content: center;
    position: fixed;
    top: 0;
    left: 0;
    width: 100vw;
    padding-top: 5vh;
    z-index: 5;
  }

  nav {
    display: flex;
    flex-flow: row nowrap;
    padding: 1em;
    justify-content: space-evenly;
    width: 15vw;

    backdrop-filter: var(--nav-background-blur);
    border-radius: var(--nav-border-radius);
  }

  a {
    margin: 0.3em;
    background-color: var(--button-background-color);
    border-radius: var(--button-border-radius);
    padding-left: 1em;
    padding-right: 1em;
    padding-top: 0.3em;
    padding-bottom: 0.3em;
  }
</style>
