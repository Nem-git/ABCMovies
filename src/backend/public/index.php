<?php

declare(strict_types=1);

use Psr\Http\Message\ResponseInterface as Response;
use Psr\Http\Message\ServerRequestInterface as Request;
use Slim\Factory\AppFactory;
use App\Helpers\StreamingServiceHelper as Ssh; // LMAO
use App\Helpers\SlimResponseHelper as Srh;

require __DIR__ . '/../vendor/autoload.php';

$app = AppFactory::create();
$ssh = new Ssh();
$srh = new Srh();

$app->get(
    "/api/",
    function (Request $request, Response $response) use ($ssh, $srh) {
        $response->getBody()->write("Hello world!");
        return $response;
    }
);

$app->get(
    "/api/{streamingService}/search/{query}",
    function (Request $request, Response $response, array $args) use ($ssh, $srh) {
        return $srh->response_json($ssh->pick($args["streamingService"])->getSearchResults($request, $response, $args), $response);
    }
);

$app->get(
    "/api/{streamingService}/{show}/recommendations",
    function (Request $request, Response $response, array $args) use ($ssh, $srh) {
        // return $srh->response_json($ssh->pick($args["streamingService"])->getShowRecommendations($request, $response, $args), $response);
    }
);

$app->get(
    "/api/{streamingService}/{show}",
    function (Request $request, Response $response, array $args) use ($ssh, $srh) {
        return $srh->response_json($ssh->pick($args["streamingService"])->getShowInfo($request, $response, $args), $response);
    }
);

$app->get(
    "/api/{streamingService}/{show}/{season}",
    function (Request $request, Response $response, array $args) use ($ssh, $srh) {
        return $srh->response_json($ssh->pick($args["streamingService"])->getSeasonInfo($request, $response, $args), $response);
    }
);

$app->get(
    "/api/{streamingService}/{show}/{season}/{episode}",
    function (Request $request, Response $response, array $args) use ($ssh, $srh) {
        return $srh->response_json($ssh->pick($args["streamingService"])->getEpisodeInfo($request, $response, $args), $response);
    }
);

$app->get(
    "/api/{streamingService}/{show}/{season}/{episode}/manifest.mpd",
    function (Request $request, Response $response, array $args) use ($ssh, $srh) {
        return $srh->response_dash($ssh->pick($args["streamingService"])->getEpisodeManifest($request, $response, $args), $response);
    }
);

$app->get(
    "/api/{streamingService}/{show}/{season}/{episode}/init/{encodedBaseUrl}/{segmentPath:.*}",
    function (Request $request, Response $response, array $args) use ($ssh, $srh) {
        return $srh->response_segment($ssh->pick($args["streamingService"])->getEpisodeInitSegment($request, $response, $args), $response);
    }
);

$app->get(
    "/api/{streamingService}/{show}/{season}/{episode}/media/{encodedInitUrl}/{encodedBaseUrl}/{segmentPath:.*}",
    function (Request $request, Response $response, array $args) use ($ssh, $srh) {
        return $srh->response_segment($ssh->pick($args["streamingService"])->getEpisodeMediaSegment($request, $response, $args), $response);
    }
);

try {
    $app->run();
} catch (Exception $e) {
    die(json_encode(array("status" => "failed", "message" => "Error: $e"))); // For DEBUG purpoises ;)
}
