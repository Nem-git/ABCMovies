<script lang="ts">
    import SearchBar from "$lib/components/search/SearchBar.svelte";
    import ShowCard from "$lib/components/show/ShowCard.svelte";
    import type { PageProps } from "./$types";

    let { data }: PageProps = $props();

    console.log(data);
</script>

<div class="flex flex-col items-center">
    <SearchBar search={data.search} />

    <div
        class="grid grid-cols-[repeat(auto-fill,minmax(325px,1fr))] items-center gap-x-4 gap-y-10 p-[5%]"
    >
        {#each data.search.shows as result (result)}
            <ShowCard {result} />
        {/each}
    </div>
</div>
