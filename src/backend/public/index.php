<?php

declare(strict_types=1);

use Psr\Http\Message\ResponseInterface as Response;
use Psr\Http\Message\ServerRequestInterface as Request;
use Slim\Factory\AppFactory;
use App\Models\ObjectFactory;
use App\Helpers\SlimResponseHelper;

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
    "/api/search/{query}",
    function (Request $request, Response $response, array $args) {
        $streamingServiceManager = ObjectFactory::createStreamingServiceManager();
        $searchResults = $streamingServiceManager->getSearchResults($request, $args);
        $response = SlimResponseHelper::response_json($searchResults, $response);
        return $response;
    }
);

//region Streaming service specific

$app->get(
    "/api/{streamingService}/search/{query}",
    function (Request $request, Response $response, array $args) {
        $streamingService = ObjectFactory::createStreamingService(strtoupper($args["streamingService"]));
        $searchResults = $streamingService->getSearchResults($request, $args);
        $response = SlimResponseHelper::response_json($searchResults, $response);
        return $response;
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
        $streamingService = ObjectFactory::createStreamingService(strtoupper($args["streamingService"]));
        $show = $streamingService->getShowInfo($request, $args);
        $response = SlimResponseHelper::response_json($show, $response);
        return $response;
    }
);

$app->get(
    "/api/{streamingService}/{show}/{season}",
    function (Request $request, Response $response, array $args) {
        $streamingService = ObjectFactory::createStreamingService(strtoupper($args["streamingService"]));
        $season = $streamingService->getSeasonInfo($request, $args);
        $response = SlimResponseHelper::response_json($season, $response);
        return $response;
    }
);

$app->get(
    "/api/{streamingService}/{show}/{season}/{episode}",
    function (Request $request, Response $response, array $args) {
        $streamingService = ObjectFactory::createStreamingService(strtoupper($args["streamingService"]));
        $episode = $streamingService->getEpisodeInfo($request, $args);
        $response = SlimResponseHelper::response_json($episode, $response);
        return $response;
    }
);

$app->get(
    "/api/{streamingService}/{show}/{season}/{episode}/manifest.mpd",
    function (Request $request, Response $response, array $args) {
        $streamingService = ObjectFactory::createStreamingService(strtoupper($args["streamingService"]));
        $modifiedManifestContent = $streamingService->getEpisodeStream($request, $args);
        $response = SlimResponseHelper::response_dash($modifiedManifestContent, $response);
        return $response;
    }
);

$app->get(
    "/api/{streamingService}/{show}/{season}/{episode}/init/{encodedBaseUrl}/{segmentPath:.*}",
    function (Request $request, Response $response, array $args) {
        $streamingService = ObjectFactory::createStreamingService(strtoupper($args["streamingService"]));
        $initContent = $streamingService->getEpisodeInitSegment($request, $args);
        $response = SlimResponseHelper::response_segment($initContent, $response);
        return $response;
    }
);

$app->get(
    "/api/{streamingService}/{show}/{season}/{episode}/media/{encodedInitUrl}/{encodedBaseUrl}/{segmentPath:.*}",
    function (Request $request, Response $response, array $args) {
        $streamingService = ObjectFactory::createStreamingService(strtoupper($args["streamingService"]));
        $initContent = $streamingService->getEpisodeMediaSegment($request, $args);
        $response = SlimResponseHelper::response_segment($initContent, $response);
        return $response;
    }
);

try {
    $app->run();
} catch (Exception $e) {
    die(json_encode(array("status" => "failed", "message" => "Error: $e"))); // For DEBUG purpoises ;)
}
