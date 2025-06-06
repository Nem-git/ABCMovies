<?php

declare(strict_types=1);

use Psr\Http\Message\ResponseInterface as Response;
use Psr\Http\Message\ServerRequestInterface as Request;
use Slim\Factory\AppFactory;

require __DIR__ . '/../vendor/autoload.php';


// TEMPORARY
require_once __DIR__ . "/../src/classes/services/Toutv.php";

$app = AppFactory::create();

$app->get("/api/", function (Request $request, Response $response) {
    $response->getBody()->write("Hello world!");
    return $response;
});

$app->get("/api/{streamingService}/search/{query}", function (Request $request, Response $response, array $args) {
	$response->getBody()->write(new Toutv()->getSearchResults($request, $response, $args));
	return $response;
});

$app->get("/api/{streamingService}/{show}/recommendations", function (Request $request, Response $response, array $args) {
	$response->getBody()->write(new Toutv()->getShowRecommendations($request, $response, $args));
	return $response;
});

$app->get("/api/{streamingService}/{show}", function (Request $request, Response $response, array $args) {
	$response->getBody()->write(new Toutv()->getShowInfo($request, $response, $args));
	return $response;
});

$app->get("/api/{streamingService}/{show}/{season}", function (Request $request, Response $response, array $args) {
	$response->getBody()->write(new Toutv()->getSeasonInfo($request, $response, $args));
	return $response;
});

$app->get("/api/{streamingService}/{show}/{season}/{episode}", function (Request $request, Response $response, array $args) {
	$response->getBody()->write(new Toutv()->getEpisodeInfo($request, $response, $args));
	return $response;
});

try {
	$app->run();
}
catch(Exception $e) {
	die(json_encode(array("status" => "failed", "message" => "Error: $e"))); # For DEBUG purpoises ;)
}
