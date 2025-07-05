<script lang="ts">
	import { url } from "@roxi/routify";

	import type { Show } from "../api/config";
	let { show }: { show: Show } = $props();
</script>

<a
	href={$url("/[streamingService]/[show]", {
		streamingService: show.provider.toLowerCase(),
		show: show.id,
	})}
	aria-label={show.title}
>
	<div class="card">
		<img src={show.imageCard.replace("_Size_", "480")} alt={show.title} />
		<div class="card-info">
			<span class="description">{show.shortDescription}</span>
		</div>
	</div>
	<span class="title">{show.title}</span>
</a>

<style>
	a {
		display: flex;
		flex-flow: column nowrap;
	}

	span {
		position: inherit;
		bottom: 0;
	}

	img {
		max-width: 100%;
		max-height: 100%;
		object-fit: cover;

		border-radius: var(--showcard-image-border-radius);

		transition-property: transform;
		transition-duration: 0.2s;
		transition-timing-function: ease-in;
	}

	.card {
		position: relative;
		overflow: hidden;
	}

	.card-info {
		opacity: 0;
		position: absolute;
		top: 0;
		left: 0;
		background: var(--showcard-hover-overlay);
		width: 100%;
		height: 100%;

		display: flex;
		flex-flow: row nowrap;

		justify-content: center;
	}

	.title {
		margin-top: 10px;
		line-height: 1.3em;
		font-size: large;
	}

	/* This part is only shown on hover */

	a:hover .card-info {
		opacity: 1;
	}

	a:hover .title {
		text-decoration: underline;
	}

	a:hover img {
		transform: scale(1.1);
	}
</style>
