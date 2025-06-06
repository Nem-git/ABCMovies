<?php

declare(strict_types=1);

function format_headers(array $headers): array {
    $formatted = [];
    foreach ($headers as $key => $value) {
        $formatted[] = "$key: $value";
    }
    return $formatted;
}

function format_parameters(array $parameters): string {
    $formatted = "?";
    foreach ($parameters as $key => $value) {
        $formatted .= "$key=$value&";
    }
    return $formatted;
}

function http_request(string $url, array $headers = [], array $options = []): string | null {
    # TODO: Make the requests asynchronously

    $ch = curl_init();
    
    curl_setopt_array($ch, [
        CURLOPT_URL => $url,
        CURLOPT_RETURNTRANSFER => true,
        CURLOPT_HEADER => false,
        CURLOPT_HTTPHEADER => format_headers($headers)
    ]);

    curl_setopt_array($ch, $options);
    
    $response = curl_exec($ch);

    curl_close($ch);
    
    return $response;
}

function get_request(string $url, array $headers = [], array $parameters = [], array $options = []): string | null {
    
    # If no URL was given
    if (!$url) { return null; }

    # If there are parameters
    if (count($parameters) > 0) {
        $url .= format_parameters($parameters);
    }

    return http_request($url, $headers, $options);

}

function post_request(string $url, array $headers = [], array $options = [], array $data = []): string | null {

    # Add data to the request body
    $options = array_merge($options, [
        CURLOPT_POSTFIELDS => $data
    ]);
    
    return http_request($url, $headers, $options);
}