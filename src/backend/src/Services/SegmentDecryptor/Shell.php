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
    private function getMp4decryptCommand(string $encryptedFilePath, string $initFilePath, string $decryptedFilePath, array $decryptionKeys): string
    {
        $cmd = "/run/current-system/sw/bin/mp4decrypt";

        foreach ($decryptionKeys as $decryptionKey) {
            $cmd .= " --key " . escapeshellarg($decryptionKey);
        }

        $cmd .= " --fragments-info " . escapeshellarg($initFilePath);
        $cmd .= " " . escapeshellarg($encryptedFilePath);
        $cmd .= " " . escapeshellarg($decryptedFilePath);

        return $cmd;
    }

    private function getMp4DecryptFullSegmentCommand(string $encryptedFilePath, string $decryptedFilePath, array $decryptionKeys): string
    {
        $cmd = "/run/current-system/sw/bin/mp4decrypt";

        foreach ($decryptionKeys as $decryptionKey) {
            $cmd .= " --key " . escapeshellarg($decryptionKey);
        }

        $cmd .= " " . escapeshellarg($encryptedFilePath);
        $cmd .= " " . escapeshellarg($decryptedFilePath);

        return $cmd;
    }


    public function getDecryptedSegment($initContent, $segmentContent, $decryptionKeys): string
    {
        $initFilePath = tempnam(TEMP_DIR, "ABC_I_");
        $segmentFilePath = tempnam(TEMP_DIR, "ABC_S_");
        $decryptedFilePath = tempnam(TEMP_DIR, "ABC_D_");

        // Separate init and media
        // file_put_contents($initFilePath, $initContent);
        // file_put_contents($segmentFilePath, $segmentContent);

        // $cmd = $this->getMp4decryptCommand($segmentFilePath, $initFilePath, $decryptedFilePath, $decryptionKeys);


        // Merged init and media
        file_put_contents($segmentFilePath, $initContent . $segmentContent);

        $cmd = $this->getMp4DecryptFullSegmentCommand($segmentFilePath, $decryptedFilePath, $decryptionKeys);

        exec($cmd . " 2>&1", $output, $err);
        unlink($segmentFilePath);
        //
        // unlink($initFilePath);

        if ($err !== 0) {
            throw new \RuntimeException("mp4decrypt failed: " . implode("\n", $output));
        }

        $decryptedContent = file_get_contents($decryptedFilePath);
        unlink($decryptedFilePath);

        return $decryptedContent;
    }

}
