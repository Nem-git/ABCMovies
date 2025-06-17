<script lang="ts">
    
    let query = $state("");
    let logged = $state(false);

    let queryChange = (event) => {
        query = event.target.value;
    }

</script>

<nav>
    <a href="/">ABCMovies</a>
    <div class="search-container">
        <form action="/search/{query}" method="get">
            <input type="text" placeholder="Search..." oninput={queryChange}>
            <input type="submit" value="Search">
        </form>
    </div>
    {#if (!logged)}
        <div class="auth-buttons">
            <a href="/login" class="auth-button" id="login-button">Login</a>
            <a href="/register" class="auth-button" id="register-button">Register</a>
        </div>
    {/if}
   
</nav>

<style>
    nav {
        background-color: purple;
        display: flex;
        flex-flow: row nowrap;
        padding: 1em;
        margin-top: 0;
        margin-left: 0;
    }

    .auth-buttons {
        padding: 1em;
        background-color: blueviolet;
    }




</style>