<script lang="ts">
  import { url } from "@roxi/routify";
  import { params } from "@roxi/routify";
  import ShowPage from "../../../../lib/ShowPage.svelte";
  import { seasonId } from "../../../../lib/shared.svelte";

  let { season } = $params;

  let splitUrl = $url("$leaf").split("/");
  splitUrl.pop();
  let showUrl = splitUrl.join("/");

  seasonId.id = season;
</script>

<ShowPage baseUrl={showUrl} />
