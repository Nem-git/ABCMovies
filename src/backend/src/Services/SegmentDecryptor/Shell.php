<?php

declare(strict_types=1);

namespace App\Services\SegmentDecryptor;

use App\Models\SegmentDecryptor;

require_once __DIR__ . "/../../../config/constants.php"; // TODO: Verify if that's actually a good way to do it (prob not)


/**
 * Using a mix of PHP and calling external scripts in shell to decrypt the segment
 */
class Shell extends SegmentDecryptor
{
    private function getMp4decryptCommand(string $encryptedFilePath, string $decryptedFilePath, array $decryptionKeys): string
    {
        $cmd = "/run/current-system/sw/bin/mp4decrypt";

        foreach ($decryptionKeys as $decryptionKey) {
            $cmd .= " --key " . escapeshellarg($decryptionKey);
        }

        $cmd .= " " . escapeshellarg($encryptedFilePath);
        $cmd .= " " . escapeshellarg($decryptedFilePath);

        return $cmd;
    }

    private function getMp4boxCommand(string $initFilePath, string $segmentFilePath, string $mergedFilePath): string
    {
        putenv('GPAC_CACHE_DIR=' . TEMP_DIR);

        $cmd = "/run/current-system/sw/bin/MP4Box";

        $cmd .= " -add $initFilePath -cat $segmentFilePath -new $mergedFilePath";

        return $cmd;
    }

    private function getMp4muxCommand(string $initFilePath, string $segmentFilePath, string $mergedFilePath): string
    {
        $cmd = "/run/current-system/sw/bin/mp4mux";

        $cmd .= " --init " . escapeshellarg($initFilePath);
        $cmd .= " --fragment " . escapeshellarg($segmentFilePath);
        $cmd .= " --output " . escapeshellarg($mergedFilePath);

        return escapeshellcmd($cmd);
    }


    private function getFfmpegCommand(string $encryptedFilePath, string $decryptedFilePath, array $decryptionKeys): string
    {

        $cmd = "/run/current-system/sw/bin/ffmpeg"; // TODO: Find a better way, because it only works on Nix

        $cmd .= " -i $encryptedFilePath -c copy"; // TODO: Add -loglevel error -nostdin

        foreach ($decryptionKeys as $decryptionKey) {
            $key = explode(":", $decryptionKey)[1]; // Test
            $cmd .= " -decryption_key $decryptionKey";
        }

        $cmd .= " $decryptedFilePath";

        return escapeshellcmd($cmd);
    }


    public function getDecryptedSegment($initContent, $segmentContent, $decryptionKeys): string
    {
        $initFilePath = tempnam(TEMP_DIR, "ABC_I_");
        $segmentFilePath = tempnam(TEMP_DIR, "ABC_S_");
        $mergedFilePath = tempnam(TEMP_DIR, "ABC_M_");
        $decryptedFilePath = tempnam(TEMP_DIR, "ABC_D_");

        // file_put_contents($initFilePath, $initContent);
        // file_put_contents($segmentFilePath, $segmentContent);

        // // Concatenate init and segment
        // $cmd = $this->getMp4muxCommand($initFilePath, $segmentFilePath, $mergedFilePath);
        // exec($cmd . " 2>&1", $output, $returnVar);
        // if ($returnVar !== 0) {
        //     throw new \RuntimeException("MP4Box concatenation failed: " . implode("\n", $output));
        // }

        // unlink($initFilePath);
        // unlink($segmentFilePath);

        file_put_contents($mergedFilePath, $initContent . $segmentContent);

        // Decrypt concatenated file
        $cmd = $this->getMp4decryptCommand($mergedFilePath, $decryptedFilePath, $decryptionKeys);
        exec($cmd, $output, $returnVar);
        if ($returnVar !== 0) {
            unlink($mergedFilePath);
            throw new \RuntimeException("mp4decrypt failed: " . implode("\n", $output));
        }

        unlink($mergedFilePath);

        $decryptedContent = file_get_contents($decryptedFilePath);
        unlink($decryptedFilePath);

        return $decryptedContent;
    }

}
