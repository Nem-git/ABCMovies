<script lang="ts">
	import { url } from "@roxi/routify";
	import ShowPage from "../../../lib/components/ShowPage.svelte";
	import { params } from "@roxi/routify";
	import { seasonId } from "../../../lib/shared.svelte";
	import { Path } from "../../../lib/path";

	let { s } = $params;

	// Support giving the season as parameters
	seasonId.id = s;
</script>

<ShowPage path={new Path($url("$leaf"))} />
