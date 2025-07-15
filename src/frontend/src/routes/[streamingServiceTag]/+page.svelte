<script lang="ts">
    import type { PageProps } from "./$types";

    let { data }: PageProps = $props();
</script>

<span>
    This is the {data.streamingServiceTag} streaming service
</span>
