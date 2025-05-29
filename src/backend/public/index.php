<?php
use Psr\Http\Message\ResponseInterface as Response;
use Psr\Http\Message\ServerRequestInterface as Request;
use Slim\Factory\AppFactory;

require __DIR__ . '/../vendor/autoload.php';

$app = AppFactory::create();

$app->get('/api/', function (Request $request, Response $response, $args) {
    $response->getBody()->write("Hello world!");
    return $response;
});

$app->get("/api/getrich", function (Request $request, Response $response, $args) {
	$response->getBody()->write("YTOUBE");
	return $response;
});

try {
	$app->run();
}
catch(Exception $e) {
	die(json_encode(array("status" => "failed", "message" => "Error: $e"))); # For DEBUG purpoises ;)
}
