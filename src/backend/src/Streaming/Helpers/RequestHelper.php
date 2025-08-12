<?php

declare(strict_types=1);

namespace App\Streaming\Helpers;

final class RequestHelper
{
    public static function format_headers(array $headers): array
    {
        $formatted = [];
        foreach ($headers as $key => $value) {
            $formatted[] = "$key: $value";
        }
        return $formatted;
    }

    public static function format_cookies(array $cookies): string
    {
        $formattedHeaders = "";
        $formattedCookies = [];

        foreach ($cookies as $cookieName => $cookieValue) {
            $formattedCookies[] = $cookieName . "=" . $cookieValue;
        }

        $formattedHeaders .= join("; ", $formattedCookies) . ";";

        return $formattedHeaders;
    }

    public static function format_parameters(
        array $parameters,
        bool $fromStart = true,
    ): string {
        $formatted = "";
        $lastKey = array_key_last($parameters);

        foreach ($parameters as $key => $value) {
            $formatted .= "$key=$value";
            if ($key !== $lastKey) {
                $formatted .= "&";
            }
        }

        if (empty($formatted)) {
            return "";
        }
        if ($fromStart) {
            return "?" . $formatted;
        } else {
            return $formatted;
        }
    }

    public static function parse_response_headers(
        string $response,
        int $headerSize,
    ): array {
        $header_text = substr($response, 0, strpos($response, "\r\n\r\n")); //$headerSize instead of \r\n\r\n

        foreach (explode("\r\n", $header_text) as $i => $line) {
            if ($i === 0) {
                $headers["http_code"] = $line;
            } else {
                [$key, $value] = explode(": ", $line);

                if (strtolower($key) === "set-cookie") {
                    // Split at the first equal sign (=)
                    [$cookieName, $cookieValue] = explode("=", $value, 2);

                    // Don't care about httpOnly, domain and stuff, only the pure value
                    $cookieValue = explode(";", $cookieValue, 2)[0];

                    if (!isset($headers[$key])) {
                        $headers[$key] = [];
                    }

                    $headers[$key][$cookieName] = $cookieValue;
                }
            }
        }

        $body = substr($response, $headerSize);

        return [
            "headers" => $headers,
            "body" => $body,
        ];
    }

    public static function http(
        string $url,
        array $parameters = [],
        array $headers = [],
        array $options = [],
    ): string|array|null {
        // If no URL was given
        if (!$url) {
            return null;
        }

        // If there are parameters
        if (count($parameters) > 0) {
            $url .= self::format_parameters($parameters);
        }

        $ch = curl_init();

        $defaultOptions = [
            CURLOPT_URL => $url,
            CURLOPT_RETURNTRANSFER => true,
            CURLOPT_HEADER => false,
            CURLOPT_HTTPHEADER => self::format_headers($headers),
        ];

        $options += $defaultOptions;

        curl_setopt_array($ch, $options);

        $response = curl_exec($ch);

        if (curl_errno($ch) || !$response) {
            // TODO: Throw error
            return null;
        }

        curl_close($ch);

        // If you want headers in your response
        if ($options[CURLOPT_HEADER]) {
            $headerSize = curl_getinfo($ch, CURLINFO_HEADER_SIZE);
            $response = self::parse_response_headers($response, $headerSize);
        }

        // If you want to get the redirect URL in your response
        if (isset($options[CURLOPT_FOLLOWLOCATION])) {
            if ($options[CURLOPT_FOLLOWLOCATION]) {
                if (!is_array($response)) {
                    $responseBody = $response;
                    $response["body"] = $responseBody;
                }

                $response["redirectUrl"] = curl_getinfo(
                    $ch,
                    CURLINFO_EFFECTIVE_URL,
                );
            }
        }

        // TOOD: Throw error on bad http code

        return $response;
    }

    public static function get(
        string $url,
        array $parameters = [],
        array $headers = [],
        array $options = [],
    ): string|array|null {
        return self::http($url, $parameters, $headers, $options);
    }

    public static function post(
        string $url,
        array $parameters = [],
        array $headers = [],
        array $options = [],
        array $data = [],
        string $contentType = "application/json",
    ): string|array|null {
        // Add data to the request body
        match ($contentType) {
            "application/json" => ($options += [
                CURLOPT_POSTFIELDS => json_encode($data, JSON_FORCE_OBJECT),
            ]),
            "application/x-www-form-urlencoded" => ($options += [
                CURLOPT_POSTFIELDS => self::format_parameters($data, false),
            ]),
        };

        $headers = array_merge(
            [
                "Content-Type" => $contentType,
            ],
            $headers,
        );

        return self::http($url, $parameters, $headers, $options);
    }
}
