<?php

declare(strict_types=1);

namespace App\Helpers;

use App\Models\DownloadInfo;

class RequestHelper
{
    public static function format_headers(array $headers): array
    {
        $formatted = [];
        foreach ($headers as $key => $value) {
            $formatted[] = "$key: $value";
        }
        return $formatted;
    }

    public static function format_parameters(array $parameters): string
    {
        $formatted = "";
        $lastKey = array_key_last($parameters);
        foreach ($parameters as $key => $value) {
            $formatted .= "$key=$value";
            if ($key !== $lastKey) {
                $formatted .= "&";
            }
        }
        return $formatted ? "?" . $formatted : "";
    }

    public static function http(string $url, array $headers = [], array $options = []): string | null
    {
        $ch = curl_init();

        curl_setopt_array(
            $ch,
            [
            CURLOPT_URL => $url,
            CURLOPT_RETURNTRANSFER => true,
            CURLOPT_HEADER => false,
            CURLOPT_HTTPHEADER => self::format_headers($headers)
            ]
        );

        curl_setopt_array($ch, $options);

        $response = curl_exec($ch);

        curl_close($ch);

        return $response;
    }

    public static function get(string $url, array $headers = [], array $parameters = [], array $options = []): string | null
    {

        // If no URL was given
        if (!$url) {
            return null;
        }

        // If there are parameters
        if (count($parameters) > 0) {
            $url .= self::format_parameters($parameters);
        }

        return self::http($url, $headers, $options);
    }

    public static function post(string $url, array $headers = [], array $options = [], $data = []): string | null
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

        return self::http($url, $headers, $options);
    }

    public static function pythonBackend(string $endpoint, DownloadInfo $downloadInfo)
    {
        $response = json_decode(self::post($_ENV["PYTHON_BACKEND_URL"] . $endpoint, data: $downloadInfo), true);

        return $response["value"];
    }

}
