<script lang="ts">
    let searchOnClick = () => {
        document.getElementById("search-input")?.focus();
    };
</script>

<header>
    <nav>
        <a href="/" class="home-button">
            <img src="/logoX.svg" alt="ABCMovies' Logo" class="logo" />
            <span class="abc">ABC</span>
        </a>
        <div class="right">
            <a href="/search" onclick={searchOnClick} class="search-container">
                <img
                    src="/searchX.svg"
                    alt="Magnifying glass"
                    class="search-icon"
                />
                <span class="search">Search</span>
            </a>

            <a href="/account">
                <img
                    src="/accountX.svg"
                    alt="User account"
                    class="account-icon"
                />
            </a>
        </div>
    </nav>
</header>

<style>
    header {
        display: flex;
        justify-content: center;
        position: fixed;
        top: 0;
        left: 0;
        width: calc(100vw - 2 * 4em);

        padding: 2em 4em 2em 4em;

        pointer-events: none;
    }

    nav {
        background: var(--navbar-background-color);
        box-shadow: var(--navbar-box-shadow);
        display: flex;
        justify-content: space-between;
        padding: 0.75em 1.5em 0.75em 1.5em;
        flex-direction: row;
        flex-wrap: nowrap;
        flex-grow: 1;

        pointer-events: auto;

        border-radius: var(--navbar-border-radius);
        backdrop-filter: var(--navbar-backdrop-filter);
    }

    .home-button {
        display: flex;
        align-items: center;

        column-gap: 7px;
    }

    .abc {
        font-size: 1.5em;
        font-weight: 500;

        flex-grow: 1;
    }

    .right {
        display: flex;
        align-items: center;

        justify-content: space-between;

        column-gap: 5vw;
    }

    .search-container {
        display: flex;
        align-items: center;

        column-gap: 10px;

        background-color: var(--navbar-background-highlight-color);
        box-shadow: 0px 0px 7px 7px rgba(44, 0, 53, 0.37);

        align-items: center;
        align-content: stretch;
        justify-content: space-between;
        padding: 12px 20px 12px 20px;
        flex-direction: row;
        flex-wrap: nowrap;
        flex-grow: 1;

        border-radius: var(--button-border-radius);
    }

    .account-icon {
        width: 50px;
    }
</style>
