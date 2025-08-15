<script lang="ts">
    import type { PageProps } from "./$types";

    import StreamingServiceSection from "$lib/components/StreamingServiceSection.svelte";

    let { data }: PageProps = $props();
</script>

<StreamingServiceSection streamingService={data.streamingService} />
