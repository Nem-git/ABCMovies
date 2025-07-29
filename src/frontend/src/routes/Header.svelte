<header>
    <nav>
        <ol>
            <li>
                <a href="/" class="home-button">
                    <svg
                        class="abc-logo"
                        xmlns="http://www.w3.org/2000/svg"
                        viewBox="0 0 64 64"
                    >
                        <path
                            class="abc-logo-background"
                            d="M35 27.9a8 8 0 0 1-7 4.1h14a8 8 0 0 1-7-4.1"
                        />
                        <circle class="abc-logo-circle" />
                        <circle class="abc-logo-circle" />
                        <path
                            class="abc-logo-background"
                            d="M32 2a30 30 0 1 0 0 60 30 30 0 0 0 0-60m14 30a2 2 0 0 1 2 2v12a2 2 0 0 1-2 2H23a2 2 0 0 1-2-2v-.5L16 48V32l5 2.5V34c0-1.1.9-2 2-2h5a8 8 0 1 1 7-11.9A8 8 0 1 1 42 32h4"
                        />
                    </svg>
                    <span class="abc">ABC</span>
                </a>
            </li>
            <div class="right">
                <li>
                    <a href="/search" class="search-container">
                        <svg
                            class="search-icon"
                            xmlns="http://www.w3.org/2000/svg"
                            viewBox="0 0 24 24"
                        >
                            <path
                                class="search-icon-line"
                                d="m20 20l-4.05-4.05m0 0a7 7 0 1 0-9.9-9.9a7 7 0 0 0 9.9 9.9"
                            />
                        </svg>
                        <span class="search">Search</span>
                    </a>
                </li>

                <li>
                    <a href="/account" aria-label="Account">
                        <svg
                            class="account-icon"
                            xmlns="http://www.w3.org/2000/svg"
                            viewBox="0 0 24 24"
                        >
                            <path
                                class="account-icon-line"
                                d="M12 2a10 10 0 1 0 0 20 10 10 0 0 0 0-20zm0 3a3 3 0 1 1 0 6 3 3 0 0 1 0-6zm0 14.2a7.2 7.2 0 0 1-6-3.22c.03-1.99 4-3.08 6-3.08 1.99 0 5.97 1.09 6 3.08a7.2 7.2 0 0 1-6 3.22z"
                            />
                        </svg>
                    </a>
                </li>
            </div>
        </ol>
    </nav>
</header>

<style>
    :root {
        --search-container-border-radius: 0.5em;
        --search-container-background-color: var(--color-dark);

        --navbar-background-color: var(--color-black);
        --logo-icon-color: var(--color-light);
        --search-icon-color: var(--color-whiteish);
        --account-icon-color: var(--color-whiteish);
    }

    header {
        position: sticky;
        top: 0;
        left: 0;
        width: 100%;

        z-index: 5;
    }

    nav {
        background: var(--navbar-background-color);
        padding-inline: 1.5em;
        height: 64px;
    }

    ol {
        display: flex;
        justify-content: space-between;
        flex-direction: row;
        flex-wrap: nowrap;
        list-style: none;
        align-items: center;
        margin: 0;
        padding: 0;
        height: 100%;
        width: 100%;
    }

    .home-button {
        display: flex;
        flex-direction: row;
        flex-wrap: nowrap;
        align-items: center;

        column-gap: 7px;
    }

    .abc {
        font-size: 1.5em;
        font-weight: 500;

        flex-grow: 1;
    }

    .abc-logo-background {
        fill: var(--logo-icon-color);
    }

    .abc-logo-circle {
        fill: none;
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

        background-color: var(--search-container-background-color);

        align-items: center;
        align-content: stretch;
        justify-content: space-between;
        padding: 12px 24px 12px 24px;
        flex-direction: row;
        flex-wrap: nowrap;
        flex-grow: 1;

        border-radius: var(--search-container-border-radius);
    }

    .abc-logo {
        width: 3em;
    }

    .search {
        color: var(--search-icon-color);
    }

    .search-icon {
        width: 1.6em;
        fill: none;
        stroke-width: 5em;
    }

    .search-icon-line {
        stroke: var(--search-icon-color);
        stroke-linecap: round;
        stroke-linejoin: round;
        stroke-width: 0.15em;
    }

    .account-icon {
        width: 3em;
        fill: var(--account-icon-color);
    }
</style>
