<script lang="ts">
    import { resolve } from "$app/paths";
    import type { SearchResult } from "$lib/types/search";

    let { result }: { result: SearchResult } = $props();
</script>

<div>
    <a
        href={resolve("/service/[service]/[show]", {
            service: result.serviceTag ?? "",
            show: result.id,
        })}
        aria-label={result.name}
        class="group flex flex-col"
    >
        <div class="relative">
            <img
                src={result.posterURL}
                alt={result.name}
                class="rounded-sm shadow-2xl shadow-purple-950/30 group-hover:shadow-purple-950/70"
            />
            <div
                class="absolute inset-0 flex h-full flex-col justify-end overflow-hidden from-neutral-950/80 to-neutral-950 group-hover:bg-linear-to-b"
            >
                <span
                    class="mb-2 px-4 text-neutral-200 opacity-0 group-hover:opacity-100"
                    >{result.overview}</span
                >
            </div>
        </div>

        <span class="text-lg text-neutral-200 group-hover:underline"
            >{result.name}</span
        >
    </a>
</div>
