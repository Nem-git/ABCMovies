<?php

declare(strict_types=1);

namespace App\Services\DrmServices;

use App\Models\WidevineDrmService;
use App\Helpers\RequestHelper;
use App\Models\DownloadInfo;

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

    public function get_pssh(DownloadInfo $downloadInfo): void
    {
        $response = json_decode($this->request->post(PYTHON_URL_BACKEND . "pssh", data: $downloadInfo), true);

        $downloadInfo->pssh = $response["pssh"];

        if ($response["error"] !== "0") {
            echo $response; // TODO: Remove, just for debug
        }
    }

    public function get_decryption_keys(DownloadInfo $downloadInfo): void
    {
        $response = json_decode($this->request->post(PYTHON_URL_BACKEND . "decrypt", data: $downloadInfo), true);

        $downloadInfo->decryptionKeys = $response["decryptionKeys"];

        if ($response["error"] !== "0") {
            echo $response; // TODO: Remove, just for debug
        }
    }

}
