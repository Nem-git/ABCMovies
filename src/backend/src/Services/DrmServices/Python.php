<?php

namespace App\Services\DrmServices;

use App\Models\WidevineDrmService;
use App\Helpers\RequestHelper;

/**
 * Class that allows interaction with the Python Backend API
 */
class Python extends WidevineDrmService
{
    private RequestHelper $request;

    public function __construct(RequestHelper $requestHelper)
    {
        $this->request = $requestHelper;
    }

    public function get_pssh(string $mpdUrl, array $mpdHeaders = [], array $segmentsHeaders = []): string
    {
        $data = [
        "mpd_url" => $mpdUrl,
        "mpd_headers" => $mpdHeaders,
        "segments_headers" => $segmentsHeaders
        ];

        $response = json_decode($this->request->post(PYTHON_URL_BACKEND . "pssh", data: $data), true);

        if ($response["error"] !== "0") {
            echo $response; // TODO: Remove, just for debug
        }
        return $response["pssh"];
    }

    public function get_decryption_keys(string $pssh, string $licenseUrl, array $licenseHeaders = []): array
    {

        $data = [
        "pssh" => $pssh,
        "license_url" => $licenseUrl,
        "license_headers" => $licenseHeaders
        ];

        $response = json_decode($this->request->post(PYTHON_URL_BACKEND . "decrypt", data: $data), true);

        if ($response["error"] !== "0") {
            echo $response; // TODO: Remove, just for debug
        }
        return $response["decryption_keys"];
    }

}
