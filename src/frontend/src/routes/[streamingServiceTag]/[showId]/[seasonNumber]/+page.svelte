<script lang="ts">
    import type { PageProps } from "./$types";

    import ShowPage from "$lib/components/ShowPage.svelte";
    import SeasonSelection from "$lib/components/SeasonSelection.svelte";
    import SeasonPage from "$lib/components/SeasonPage.svelte";

    let { data }: PageProps = $props();
</script>

<ShowPage show={data.show} />
<SeasonSelection seasons={data.show.seasons} selectedSeason={data.season} />
<SeasonPage season={data.season} />
