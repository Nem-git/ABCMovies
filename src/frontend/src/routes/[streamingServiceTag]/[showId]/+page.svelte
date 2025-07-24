<script lang="ts">
    import type { PageProps } from "./$types";

    import { goto } from "$app/navigation";
    import { onMount } from "svelte";

    import ShowPage from "$lib/components/ShowPage.svelte";
    import SeasonSelection from "$lib/components/SeasonSelection.svelte";
    import SeasonPage from "$lib/components/SeasonPage.svelte";

    let { data }: PageProps = $props();

    onMount(() => {
        goto(
            "/" +
                [data.streamingServiceTag, data.showId, data.season.id].join(
                    "/",
                ),
            { replaceState: true },
        );
    });
</script>

<ShowPage show={data.show} />
<SeasonSelection seasons={data.show.seasons} selectedSeason={data.season} />
<SeasonPage season={data.season} />
