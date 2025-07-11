<?php

declare(strict_types=1);

namespace App\Streaming\DRMTechnology\Widevine\SegmentDecryptor;

use App\Streaming\DRMTechnology\Widevine\Classes\SegmentDecryptor;

/**
 * Using a mix of PHP and calling external scripts in shell to decrypt the segment
 */
class Shell extends SegmentDecryptor
{
    /**
     * Decrypt ONLY the segment using the init as fragment info, so no binary merging
     */
    private function getMp4decryptCommand(string $encryptedFilePath, string $initFilePath, string $decryptedFilePath, array $decryptionKeys): string
    {
        $cmd = $_ENV["MP4DECRYPT_PATH"];

        foreach ($decryptionKeys as $decryptionKey) {
            $cmd .= " --key " . escapeshellarg($decryptionKey);
        }

        $cmd .= " --fragments-info " . escapeshellarg($initFilePath);
        $cmd .= " " . escapeshellarg($encryptedFilePath);
        $cmd .= " " . escapeshellarg($decryptedFilePath);

        return $cmd;
    }

    /**
     * Decrypting the segment made from init and segment merging
     */
    private function getMp4DecryptFullSegmentCommand(string $encryptedFilePath, string $decryptedFilePath, array $decryptionKeys): string
    {
        $cmd = $_ENV["MP4DECRYPT_PATH"];

        foreach ($decryptionKeys as $decryptionKey) {
            $cmd .= " --key " . escapeshellarg($decryptionKey);
        }

        $cmd .= " " . escapeshellarg($encryptedFilePath);
        $cmd .= " " . escapeshellarg($decryptedFilePath);

        return $cmd;
    }


    public function getDecryptedSegment($initContent, $segmentContent, $decryptionKeys): string
    {
        $initFilePath = tempnam($_ENV["TEMP_DIR"], "ABC_I_");
        $segmentFilePath = tempnam($_ENV["TEMP_DIR"], "ABC_S_");
        $decryptedFilePath = tempnam($_ENV["TEMP_DIR"], "ABC_D_");

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
