<?php

declare(strict_types=1);

namespace App\Config;

final class Constants
{
    public const DEFAULT_SEARCH_RESULTS_AMOUNT = 20;
    public const DEFAULT_RECOMMENDATIONS_AMOUNT = 50;
    public const DEFAULT_REDIS_TTL_TYPE = "EX"; // EX seconds, PX milliseconds, https://redis.io/docs/latest/commands/set/
    public const DEFAULT_INIT_CONTENT_TTL = 60 * 60 * 24;

    public const RECOMMENDATION_TYPES = ["movies", "series", "documentaries"];

    public const STREAMING_TECH_TO_FILENAME = [
        "dash" => "manifest.mpd",
        "hls" => "master.m3u8",
        "mp4" => "video.mp4",
    ];

    public const WORD_TO_STREAMING_TECH = [
        "dash" => "dash",
        "manifest.mpd" => "dash",
        "urn:mpeg:dash:profile:isoff-live:2011" => "dash",
        "application/dash+xml" => "dash",
        "hls" => "hls",
        "master.m3u8" => "hls",
        "application/x-mpegurl" => "hls",
        "mp4" => "mp4",
    ];

    public const STREAMING_TECH_RANK = ["dash", "hls", "mp4", "smooth"];

    public const HTTP_DEFAULT_HEADERS = [
        "User-Agent" =>
            "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:109.0) Gecko/20100101 Firefox/109.0",
        "Accept" => "application/json",
        "Accept-Language" => "fr-CA,fr",
        "Sec-GPC" => "1",
        "Sec-Fetch-Dest" => "document",
        "Sec-Fetch-Mode" => "naviguate",
        "Sec-Fetch-Site" => "cross-site",
        "Priority" => "u=0, i",
        "Pragma" => "no-cache",
        "Cache-Control" => "no-cache",
    ];
}
