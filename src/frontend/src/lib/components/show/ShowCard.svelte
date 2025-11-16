<script lang="ts">
    import { resolve } from "$app/paths";
    import type { SearchResult } from "$lib/types/search";

    let { result }: { result: SearchResult } = $props();
</script>

<div class="flex flex-col bg-blue-500">
    <a
        href={resolve("/service/[service]/[show]", {
            service: result.serviceTag ?? "",
            show: result.id,
        })}
        aria-label={result.name}
        class="rounded-lg"
    >
        <img src={result.posterURL} alt={result.name} class="" />
    </a>
    <a
        href={resolve("/service/[service]/[show]", {
            service: result.serviceTag ?? "",
            show: result.id,
        })}
        aria-label={result.name}
    >
        <span class="text-xl">{result.name}</span>
    </a>
</div>
