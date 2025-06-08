<?php

declare(strict_types=1);

use Psr\Http\Message\ResponseInterface as Response;

function format_headers(array $headers): array
{
    $formatted = [];
    foreach ($headers as $key => $value) {
        $formatted[] = "$key: $value";
    }
    return $formatted;
}

function format_parameters(array $parameters): string
{
    $formatted = "?";
    foreach ($parameters as $key => $value) {
        $formatted .= "$key=$value&";
    }
    return $formatted;
}

function http_request(string $url, array $headers = [], array $options = []): string | null
{
    // TODO: Make the requests asynchronously

    $ch = curl_init();

    curl_setopt_array(
        $ch,
        [
        CURLOPT_URL => $url,
        CURLOPT_RETURNTRANSFER => true,
        CURLOPT_HEADER => false,
        CURLOPT_HTTPHEADER => format_headers($headers)
        ]
    );

    curl_setopt_array($ch, $options);

    $response = curl_exec($ch);

    curl_close($ch);

    return $response;
}

function get_request(string $url, array $headers = [], array $parameters = [], array $options = []): string | null
{

    // If no URL was given
    if (!$url) {
        return null;
    }

    // If there are parameters
    if (count($parameters) > 0) {
        $url .= format_parameters($parameters);
    }

    return http_request($url, $headers, $options);
}

function post_request(string $url, array $headers = [], array $options = [], array $data = []): string | null
{

    // Add data to the request body
    if (empty($options)) {
        $options = [
            CURLOPT_POSTFIELDS => json_encode($data, JSON_FORCE_OBJECT)
        ];
    }

    $headers = array_merge(
        [
        "Content-Type" => "application/json"
        ],
        $headers
    );

    return http_request($url, $headers, $options);
}

function response_json($data, Response $response)
{
    $response = $response->withHeader("Content-Type", "application/json");
    $response->getBody()->write(json_encode($data, JSON_PRETTY_PRINT)); // TODO: Remove the pretty print, just for debug
    return $response;
}


function get_pssh(string $mpdUrl, array $mpdHeaders = [], array $segmentsHeaders = [])
{
    $data = [
        "mpd_url" => $mpdUrl,
        "mpd_headers" => $mpdHeaders,
        "segments_headers" => $segmentsHeaders
    ];

    $response = json_decode(post_request(PYTHON_URL_BACKEND . "pssh", data: $data), true);

    if ($response["error"] !== "0") {
        echo $response; // TODO: Remove, just for debug
    }
    return $response["pssh"];
}

function get_decryption_keys(string $pssh, string $licenseUrl, array $licenseHeaders = [])
{

    $data = [
        "pssh" => $pssh,
        "license_url" => $licenseUrl,
        "license_headers" => $licenseHeaders
    ];

    $response = json_decode(post_request(PYTHON_URL_BACKEND . "decrypt", data: $data), true);

    if ($response["error"] !== "0") {
        echo $response; // TODO: Remove, just for debug
    }
    return $response["decryption_keys"];
}
