<script lang="ts">
    import { params } from "@roxi/routify";
    import ShowPage from "../../../../lib/components/ShowPage.svelte";
    import { id } from "../../../../lib/shared.svelte";

    let { streamingService, show, season } = $params;

    id.season = season;
</script>

<ShowPage {streamingService} {show} />
