<script lang="ts">
    import { afterNavigate, goto } from "$app/navigation";
    import { resolve } from "$app/paths";
    import type { Search } from "$lib/types/search";
    import { onMount } from "svelte";

    let { search }: { search: Search } = $props();

    let input: HTMLInputElement;

    let query = $state(search.query);

    onMount(() => {
        focus();
    });

    let focus = () => {
        if (document.activeElement !== input) {
            input.focus();
            input.setSelectionRange(input.value.length, input.value.length);
        }
    };

    afterNavigate(() => {
        if (query !== search.query) {
            query = search.query;
        }
    });

    $effect(() => {
        if (query.length == 0) {
            goto(resolve("/search"), {
                keepFocus: true,
                replaceState: true,
            });
        } else {
            goto(
                resolve("/search/[query]", {
                    query: encodeURI(query),
                }),
                {
                    keepFocus: true,
                    replaceState: true,
                },
            );
        }
    });
</script>

<button
    class="align-center flex w-full max-w-200 grow cursor-text content-center rounded-xl border-2 border-neutral-800 bg-neutral-900 p-3 focus-within:border-neutral-600"
    aria-label="searchbar"
    onclick={focus}
>
    <div class="mr-3 w-8">
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
            <path
                stroke="#000"
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M15.8 15.8 21 21m-3-10.5a7.5 7.5 0 1 1-15 0 7.5 7.5 0 0 1 15 0Z"
            />
        </svg>
    </div>
    <input
        type="search"
        placeholder="Search for a movie, tv show or documentary..."
        class="grow bg-none text-xl text-neutral-200 outline-none placeholder:font-light"
        bind:this={input}
        bind:value={query}
    />
</button>
