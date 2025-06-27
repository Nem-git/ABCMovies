<script lang="ts">

  let searchOnClick = () => {
    document.getElementById("search-input")?.focus();
  };
</script>

<header>
  <nav>
    <div class="left">
      <a href="/" class="home-button">
        <img src="/logo.svg" alt="">
        <span>ABC</span>
      </a>
    </div>
    <div class="right">
      <a href="/search" onclick={searchOnClick}>Search</a>
      <a href="/account">Account</a>
    </div>
  </nav>
</header>

<style>
  :root {

    /** Colors */
    --background-color: #311d2d;
    --foreground-color: #39313a;
    --text-color: #f9f3fc;

    /** Bar */
    --nav-border-radius: 0.5em;

    /** Buttons */
    --button-active-background-color: white;
    --button-hover-background-color: rgba(255, 255, 255, 0.185);
    --button-inactive-background-color: rgba(0, 0, 0, 0);
    --button-border-radius: 99vw;
  }

  header {
    display: flex;
    justify-content: center;
    position: fixed;
    top: 0;
    left: 0;
    width: 100vw;
    
    padding: 10px 20px 10px 20px;
    
    pointer-events: none;
    z-index: 5;
  }

  nav {
    background: var(--background-color);
    border-radius: 15px 15px 15px 15px;
    box-shadow: 0px 0px 10px 10px #3b004788;
    display: flex;
    align-items: center;
    align-content: stretch;
    justify-content: space-between;
    padding: 10px 20px 10px 20px;
    flex-direction: row;
    flex-wrap: nowrap;
    flex-grow: 1;

    pointer-events: auto;

    border-radius: var(--nav-border-radius);
  }

  a {
    margin: 0.3em;
    background-color: var(--button-inactive-background-color);
    border-radius: var(--button-border-radius);

    padding-inline: 1em;
    padding-top: 0.3em;
    padding-bottom: 0.3em;
  }

  a:active {
    background-color: var(--button-active-background-color);
  }

  a:hover, a:focus {
    background-color: var(--button-hover-background-color);
  }
</style>
