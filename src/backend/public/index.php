<?php

declare(strict_types=1);

use Psr\Http\Message\ResponseInterface as Response;
use Psr\Http\Message\ServerRequestInterface as Request;
use Slim\Factory\AppFactory;
use Dotenv\Dotenv;
use App\Helpers\SlimResponseHelper;
use App\Factory\ObjectFactory;

require __DIR__ . "/../vendor/autoload.php";

// Loads the environment variables from .env to $_ENV
$dotenv = Dotenv::createImmutable(__DIR__ . "/../src/Config/");
$dotenv->load();

$app = AppFactory::create();


$app->get(
    "/api/search/{query}",
    function (Request $request, Response $response, array $args) {
        $streamingServiceManager = ObjectFactory::createStreamingServiceManager();
        $searchResults = $streamingServiceManager->getSearchResults($request, $args);
        $response = SlimResponseHelper::response_json($searchResults, $response);
        return $response;
    }
);

$app->get(
    "/api/recommendations/{type}",
    function (Request $request, Response $response, array $args) {
        $streamingServiceManager = ObjectFactory::createStreamingServiceManager();
        $searchResults = $streamingServiceManager->getMediaRecommendations($request, $args);
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
    "/api/{streamingService}/recommendations/{type}",
    function (Request $request, Response $response, array $args) {
        $streamingService = ObjectFactory::createStreamingService(strtoupper($args["streamingService"]));
        $recommendations = $streamingService->getMediaRecommendations($request, $args);
        $response = SlimResponseHelper::response_json($recommendations, $response);
        return $response;
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
    "/api/{streamingService}/{show}/recommendations",
    function (Request $request, Response $response, array $args) {
        $streamingService = ObjectFactory::createStreamingService(strtoupper($args["streamingService"]));
        $recommendations = $streamingService->getShowRecommendations($request, $args);
        $response = SlimResponseHelper::response_json($recommendations, $response);
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
    "/api/{streamingService}/{show}/{season}/{episode}/next",
    function (Request $request, Response $response, array $args) {
        $streamingService = ObjectFactory::createStreamingService(strtoupper($args["streamingService"]));
        $show = $streamingService->getNextRecommendation($request, $args);
        $response = SlimResponseHelper::response_json($show, $response);
        return $response;
    }
);

$app->get(
    "/api/{streamingService}/{show}/{season}/{episode}/{filename}",
    function (Request $request, Response $response, array $args) {
        $streamingService = ObjectFactory::createStreamingService(strtoupper($args["streamingService"]));
        $modifiedManifestContent = $streamingService->getEpisodeVideo($request, $args);
        $response = SlimResponseHelper::response_dash($modifiedManifestContent, $response);
        return $response;
    }
);

// The goal of extraArgs is to avoid creating a new endpoint for each streaming tech
// Instead, this code will look at the streamingTechnology and decide from there what
// method to call
$app->get(
    "/api/{streamingService}/{show}/{season}/{episode}/{streamingTechnology}/{extraArgs:.*}",
    function (Request $request, Response $response, array $args) {
        $streamingService = ObjectFactory::createStreamingService(strtoupper($args["streamingService"]));
        $initContent = $streamingService->getEpisodeVideo($request, $args);
        $response = SlimResponseHelper::response_segment($initContent, $response);
        return $response;
    }
);

//endregion

try {
    $app->run();
} catch (Exception $e) {
    die(json_encode(array("status" => "failed", "message" => "Error: $e"))); // For DEBUG purpoises ;)
}
