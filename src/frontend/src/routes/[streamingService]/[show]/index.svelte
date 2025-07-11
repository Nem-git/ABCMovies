<script lang="ts">
    import ShowPage from "../../../lib/components/ShowPage.svelte";
    import { params } from "@roxi/routify";
    import { id } from "../../../lib/shared.svelte";

    let { streamingService, show, s } = $params;

    // Support giving the season as parameters
    id.season = s;
</script>

<ShowPage {streamingService} {show} />
