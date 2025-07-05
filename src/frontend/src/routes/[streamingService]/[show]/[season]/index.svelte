<script lang="ts">
	import { url } from "@roxi/routify";
	import { params } from "@roxi/routify";
	import ShowPage from "../../../../lib/components/ShowPage.svelte";
	import { seasonId } from "../../../../lib/shared.svelte";
	import { Path } from "../../../../lib/path";

	let { season } = $params;

	seasonId.id = season;
</script>

<ShowPage path={new Path($url("$leaf"))} />
