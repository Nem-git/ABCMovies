<?php

declare(strict_types=1);

use Psr\Http\Message\ResponseInterface as Response;
use Psr\Http\Message\ServerRequestInterface as Request;
use Slim\Factory\AppFactory;
use App\Models\ObjectFactory;

require __DIR__ . '/../vendor/autoload.php';

$app = AppFactory::create();

$app->get(
    "/api/",
    function (Request $request, Response $response, array $args) {
        $response->getBody()->write("Hello world!");
        return $response;
    }
);

$app->get(
    "/api/{streamingService}/search/{query}",
    function (Request $request, Response $response, array $args) {
        return ObjectFactory::createStreamingService($args["streamingService"])->getSearchResults($request, $response, $args);
    }
);

$app->get(
    "/api/{streamingService}/{show}/recommendations",
    function (Request $request, Response $response, array $args) {
        // return $srh->response_json($ssh->pick($args["streamingService"])->getShowRecommendations($request, $response, $args), $response);
    }
);

$app->get(
    "/api/{streamingService}/{show}",
    function (Request $request, Response $response, array $args) {
        return ObjectFactory::createStreamingService($args["streamingService"])->getShowInfo($request, $response, $args);
    }
);

$app->get(
    "/api/{streamingService}/{show}/{season}",
    function (Request $request, Response $response, array $args) {
        return ObjectFactory::createStreamingService($args["streamingService"])->getSeasonInfo($request, $response, $args);
    }
);

$app->get(
    "/api/{streamingService}/{show}/{season}/{episode}",
    function (Request $request, Response $response, array $args) {
        return ObjectFactory::createStreamingService($args["streamingService"])->getEpisodeInfo($request, $response, $args);
    }
);

$app->get(
    "/api/{streamingService}/{show}/{season}/{episode}/manifest.mpd",
    function (Request $request, Response $response, array $args) {
        return ObjectFactory::createStreamingService($args["streamingService"])->getEpisodeManifest($request, $response, $args);
    }
);

$app->get(
    "/api/{streamingService}/{show}/{season}/{episode}/init/{encodedBaseUrl}/{segmentPath:.*}",
    function (Request $request, Response $response, array $args) {
        return ObjectFactory::createStreamingService($args["streamingService"])->getEpisodeInitSegment($request, $response, $args);
    }
);

$app->get(
    "/api/{streamingService}/{show}/{season}/{episode}/media/{encodedInitUrl}/{encodedBaseUrl}/{segmentPath:.*}",
    function (Request $request, Response $response, array $args) {
        return ObjectFactory::createStreamingService($args["streamingService"])->getEpisodeMediaSegment($request, $response, $args);
    }
);

try {
    $app->run();
} catch (Exception $e) {
    die(json_encode(array("status" => "failed", "message" => "Error: $e"))); // For DEBUG purpoises ;)
}
