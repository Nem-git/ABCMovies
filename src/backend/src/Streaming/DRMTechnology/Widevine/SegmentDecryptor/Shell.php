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
    private function decryptMediaSegment(
        string $initContent,
        string $segmentContent,
        array $decryptionKeys,
    ): string {
        $encryptedSegmentFilePath = tempnam($_ENV["TEMP_DIR"], "ABC_E_");
        $initSegmentFilePath = tempnam($_ENV["TEMP_DIR"], "ABC_I_");
        $decryptedSegmentFilePath = tempnam($_ENV["TEMP_DIR"], "ABC_D_");

        file_put_contents($initSegmentFilePath, $initContent);
        file_put_contents($encryptedSegmentFilePath, $segmentContent);

        $cmd = "mp4decrypt";

        foreach ($decryptionKeys as $decryptionKey) {
            $cmd .= " --key " . escapeshellarg($decryptionKey);
        }

        $cmd .= " --fragments-info " . escapeshellarg($initSegmentFilePath);
        $cmd .= " " . escapeshellarg($encryptedSegmentFilePath);
        $cmd .= " " . escapeshellarg($decryptedSegmentFilePath);

        exec($cmd . " 2>&1");

        $decryptedContent = file_get_contents($decryptedSegmentFilePath);

        unlink($encryptedSegmentFilePath);
        unlink($decryptedSegmentFilePath);

        return $decryptedContent;
    }

    /**
     * Decrypting the segment made from init and segment merging
     */
    private function decryptMergedSegment(
        string $initContent,
        string $segmentContent,
        array $decryptionKeys,
    ): string {
        $encryptedSegmentFilePath = tempnam($_ENV["TEMP_DIR"], "ABC_E_");
        $decryptedSegmentFilePath = tempnam($_ENV["TEMP_DIR"], "ABC_D_");

        file_put_contents(
            $encryptedSegmentFilePath,
            $initContent . $segmentContent,
        );

        $cmd = "mp4decrypt";

        foreach ($decryptionKeys as $decryptionKey) {
            $cmd .= " --key " . escapeshellarg($decryptionKey);
        }

        $cmd .= " " . escapeshellarg($encryptedSegmentFilePath);
        $cmd .= " " . escapeshellarg($decryptedSegmentFilePath);

        exec($cmd . " 2>&1");

        $decryptedContent = file_get_contents($decryptedSegmentFilePath);

        unlink($encryptedSegmentFilePath);
        unlink($decryptedSegmentFilePath);

        return $decryptedContent;
    }

    private function removeDrmInitContent(string $initContent): string
    {
        $cmd = "mp4dump";
        return "";
    }

    public function getDecryptedSegment(
        $initContent,
        $segmentContent,
        $decryptionKeys,
        $shouldBeMerged = false,
    ): string {
        // Merged init and media
        if ($shouldBeMerged) {
            return $this->decryptMergedSegment(
                $initContent,
                $segmentContent,
                $decryptionKeys,
            );
        } else {
            return $this->decryptMediaSegment(
                $initContent,
                $segmentContent,
                $decryptionKeys,
            );
        }
    }
}
