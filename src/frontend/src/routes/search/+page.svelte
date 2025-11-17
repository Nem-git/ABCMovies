<script lang="ts">
    import SearchBar from "$lib/components/search/SearchBar.svelte";
    import type { PageProps } from "./$types";

    let { data }: PageProps = $props();

    console.log(data);
</script>

<SearchBar search={data.search} />

<div
    class="grid grid-cols-[repeat(auto-fill,minmax(325px,1fr))] items-center gap-x-4 gap-y-10 p-[5%]"
></div>
