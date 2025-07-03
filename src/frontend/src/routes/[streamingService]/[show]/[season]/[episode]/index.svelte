<!-- routify:meta reset -->

<script lang="ts">
  import { url } from "@roxi/routify";
  import EpisodePage from "../../../../../lib/components/EpisodePage.svelte";
  import { Path } from "../../../../../lib/path";
</script>

<EpisodePage path={new Path($url("$leaf"))} />
