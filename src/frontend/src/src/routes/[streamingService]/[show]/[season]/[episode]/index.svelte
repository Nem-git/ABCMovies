<!-- routify:meta reset -->

<script lang="ts">

    import { url } from "@roxi/routify";
    import EpisodePage from "../../../../../lib/EpisodePage.svelte";

    

</script>

<EpisodePage baseUrl={$url("$leaf")} />