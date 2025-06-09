<?php

declare(strict_types=1);

namespace App\Helpers;

use App\Models\DownloadInfo;

class RequestHelper
{
    private function format_headers(array $headers): array
    {
        $formatted = [];
        foreach ($headers as $key => $value) {
            $formatted[] = "$key: $value";
        }
        return $formatted;
    }

    private function format_parameters(array $parameters): string
    {
        $formatted = "?";
        foreach ($parameters as $key => $value) {
            $formatted .= "$key=$value&";
        }
        return $formatted;
    }

    private function http(string $url, array $headers = [], array $options = []): string | null
    {
        // TODO: Make the requests asynchronously

        $ch = curl_init();

        curl_setopt_array(
            $ch,
            [
            CURLOPT_URL => $url,
            CURLOPT_RETURNTRANSFER => true,
            CURLOPT_HEADER => false,
            CURLOPT_HTTPHEADER => $this->format_headers($headers)
            ]
        );

        curl_setopt_array($ch, $options);

        $response = curl_exec($ch);

        curl_close($ch);

        return $response;
    }

    public function get(string $url, array $headers = [], array $parameters = [], array $options = []): string | null
    {

        // If no URL was given
        if (!$url) {
            return null;
        }

        // If there are parameters
        if (count($parameters) > 0) {
            $url .= $this->format_parameters($parameters);
        }

        return $this->http($url, $headers, $options);
    }

    public function post(string $url, array $headers = [], array $options = [], $data = []): string | null
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

        return $this->http($url, $headers, $options);
    }

    public function pythonBackend(string $endpoint, DownloadInfo $downloadInfo)
    {
        $response = json_decode($this->post(PYTHON_URL_BACKEND . $endpoint, data: $downloadInfo), true);

        if ($response["error"] !== "0") {
            echo $response; // TODO: Remove, just for debug
        }

        return $response["value"];
    }

}
