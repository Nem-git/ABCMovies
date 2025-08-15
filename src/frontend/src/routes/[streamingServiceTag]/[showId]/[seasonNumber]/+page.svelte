<script lang="ts">
    import type { PageProps } from "./$types";

    import SeasonSelection from "$lib/components/SeasonSelection.svelte";
    import ShowSection from "$lib/components/ShowSection.svelte";
    import SeasonSection from "$lib/components/SeasonSection.svelte";

    let { data }: PageProps = $props();
</script>

<ShowSection show={data.show} />
<SeasonSelection seasons={data.show.seasons} selectedSeason={data.season} />
<SeasonSection season={data.season} />
