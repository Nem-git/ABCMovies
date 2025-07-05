export class Path {
	streamingService: string | undefined;
	show: string | undefined;
	season: string | undefined;
	episode: string | undefined;

	constructor(baseUrl: string) {
		const parametersOrder: Array<
			"streamingService" | "show" | "season" | "episode"
		> = ["streamingService", "show", "season", "episode"];

		let splitUrl = baseUrl.split("/");
		splitUrl.shift(); // remove the first empty string

		splitUrl.forEach((urlSegment, i) => {
			if (i < parametersOrder.length) {
				this[parametersOrder[i]] = urlSegment;
			}
		});
	}

	public getStreamingService(): string {
		return "/" + this.streamingService;
	}

	public getShow(): string {
		return [this.getStreamingService(), this.show].join("/");
	}

	public getSeason(): string {
		return [this.getShow(), this.season].join("/");
	}

	public getEpisode(): string {
		return [this.getSeason(), this.episode].join("/");
	}
}
