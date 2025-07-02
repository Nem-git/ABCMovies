<script lang="ts">
  import { q } from "../shared.svelte";

  let searchInputEl: HTMLInputElement;
  // Represents if the focus state is the one from the mounting of the component
  let shouldBlur = false;

  let onmouseenter = (event: any) => {
    if (searchInputEl !== document.activeElement) {
      shouldBlur = true;
    }

    searchInputEl.focus();
  };

  let onmouseleave = (event: any) => {
    if (shouldBlur) {
      searchInputEl.blur();
    }
  };

  let onclick = (event: any) => {
    shouldBlur = false;
  };

  let oninput = (event: any) => {
    q.query = event.target.value;

    // Because if the user started typing, we don't remove focus from field
    shouldBlur = false;
  };
</script>

<div class="search-container">
  <img src="/searchX.svg" alt="Magnifying glass" class="search-icon" />
  <input
    type="text"
    id="search-input"
    bind:this={searchInputEl}
    value={$state.snapshot(q.query)}
    {oninput}
    {onmouseenter}
    {onmouseleave}
    {onclick}
  />
</div>

<style>
  .search-container {
    margin-inline: 10vw 10vw;

    background-color: var(--searchbar-background-color);
    box-shadow: var(--searchbar-box-shadow);
    border-radius: var(--searchbar-border-radius);

    border-width: 3px;
    border-style: solid;
    border-color: var(--searchbar-border-inactive-color);

    display: flex;
    align-items: center;

    gap: 5px;
    height: 60px;
  }

  #search-input {
    background-color: transparent;

    width: 100%;
    height: 100%;

    font-size: 1em;
    font-weight: 500;
  }

  .search-container:focus-within {
    border-color: var(--searchbar-border-color);
  }
</style>
