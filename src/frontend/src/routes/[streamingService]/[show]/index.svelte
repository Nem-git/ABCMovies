<script lang="ts">
  import { url } from "@roxi/routify";
  import ShowPage from "../../../lib/components/ShowPage.svelte";
</script>

<ShowPage baseUrl={$url("$leaf")} />
