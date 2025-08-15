<script lang="ts">
    import type { PageProps } from "./$types";

    import { goto } from "$app/navigation";
    import { onMount } from "svelte";

    import ShowSection from "$lib/components/ShowSection.svelte";
    import SeasonSelection from "$lib/components/SeasonSelection.svelte";
    import SeasonSection from "$lib/components/SeasonSection.svelte";

    let { data }: PageProps = $props();

    let createUrl = async () => {
        return (
            "/" +
            [data.streamingServiceTag, data.showId, data.season.number].join(
                "/",
            )
        );
    };

    onMount(async () => {
        goto(await createUrl(), { replaceState: true });
    });
</script>

<ShowSection show={data.show} />
<SeasonSelection seasons={data.show.seasons} selectedSeason={data.season} />
<SeasonSection season={data.season} />
