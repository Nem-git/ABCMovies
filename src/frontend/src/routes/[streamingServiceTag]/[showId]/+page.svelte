<script lang="ts">
    import type { PageProps } from "./$types";

    import { goto } from "$app/navigation";
    import { onMount } from "svelte";

    import ShowSection from "$lib/components/ShowSection.svelte";
    import SeasonSelection from "$lib/components/SeasonSelection.svelte";
    import SeasonSection from "$lib/components/SeasonSection.svelte";

    let { data }: PageProps = $props();

    onMount(async () => {
        goto(
            "/" +
                [
                    data.streamingServiceTag,
                    data.showId,
                    data.season.number,
                ].join("/"),
            { replaceState: true },
        );
    });
</script>

<ShowSection show={data.show} />
<SeasonSelection seasons={data.show.seasons} selectedSeason={data.season} />
<SeasonSection season={data.season} />
