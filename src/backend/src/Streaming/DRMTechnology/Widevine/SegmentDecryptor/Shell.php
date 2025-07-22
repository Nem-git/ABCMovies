<?php

declare(strict_types=1);

namespace App\Streaming\DRMTechnology\Widevine\SegmentDecryptor;

use App\Streaming\DRMTechnology\Widevine\Classes\SegmentDecryptor;
use App\Streaming\Helpers\BentoHelper;

/**
 * Using a mix of PHP and calling external scripts in shell to decrypt the segment
 */
final class Shell extends SegmentDecryptor
{
    #[\Override]
    public function getDecryptedSegment(
        $segmentContent,
        $initContent = "",
        $decryptionKeys = [],
        $shouldBeMerged = false,
    ): string {
        // Merged init and media
        if ($shouldBeMerged && !empty($initContent)) {
            // return $this->removeDrmMediaContent(
            //     $this->decryptMergedSegment(
            //         $initContent,
            //         $this->removeDrmMediaContent($segmentContent),
            //         $decryptionKeys,
            //     ),
            // );
            return $this->decryptMergedSegment(
                $initContent,
                $segmentContent,
                $decryptionKeys,
            );
        } elseif ($segmentContent === $initContent) {
            return $this->removeDrmInitContent($initContent);
        } else {
            // return $this->removeDrmMediaContent(
            //     $this->decryptMediaSegment(
            //         $initContent,
            //         $this->removeDrmMediaContent($segmentContent),
            //         $decryptionKeys,
            //     ),
            // );
            return $this->decryptMediaSegment(
                $initContent,
                $segmentContent,
                $decryptionKeys,
            );
        }
    }

    /**
     * Decrypt ONLY the segment using the init as fragment info, so no binary merging
     */
    private function decryptMediaSegment(
        string $initContent,
        string $segmentContent,
        array $decryptionKeys,
    ): string|false {
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
    ): string|false {
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

    /**
     * Remove unnecessary/DRM atoms from the media segment
     */
    private function removeDrmMediaContent(string $content): string|false
    {
        $mp4Info = BentoHelper::getMp4Info($content);

        foreach ($mp4Info as $atom) {
            // Loop through atoms in moov
            if ($atom["name"] === "moov") {
                BentoHelper::removeAtom($content, [
                    $atom["name"]
                ]);
                // foreach ($atom["children"] as $moovChildren) {
                //     // Retrieve all the PSSH's
                //     if ($moovChildren["name"] === "pssh") {
                //         $content = BentoHelper::removeAtom(
                //             $content,
                //             [
                //             $atom["name"],
                //             $moovChildren["name"],
                //             ]
                //         );
                //     }

                //     // Remove all the minfs (where most DRM data is stored)
                //     // $stsd = BentoHelper::traverseToPath($atom, ["trak", "mdia", "minf", "stbl", "stsd"]);

                //     // TODO: Fix the mess that is merging the cleaned init with the segment
                // }
            }
        }

        return $content;
    }

    /**
     * Remove unnecessary/DRM atoms from the init segment
     */
    private function removeDrmInitContent(string $initContent): string|false
    {
        $mp4Info = BentoHelper::getMp4Info($initContent);

        foreach ($mp4Info as $atom) {
            if ($atom["name"] === "moov") {
                foreach ($atom["children"] as $moovChildren) {
                    // Retrieve all the PSSH's
                    if ($moovChildren["name"] === "pssh") {
                        $initContent = BentoHelper::removeAtom(
                            $initContent,
                            [
                                $atom["name"],
                                $moovChildren["name"],
                            ]
                        );
                    }
                }

                // Remove all the minfs (where most DRM data is stored)
                // $stsd = BentoHelper::traverseToPath($atom, ["trak", "mdia", "minf", "stbl", "stsd"]);

                // foreach ($stsd["children"] as $stsdChildren) {

                //     // TODO: Add a mp4edit command that extracts the name from sinf/original_format

                //     foreach ($stsdChildren["children"] as $encChildren) {
                //         if ($encChildren["name"] === "sinf") {
                //             $initContent = BentoHelper::removeAtom($initContent, [
                //                 "moov",
                //                 "trak",
                //                 "mdia",
                //                 "minf",
                //                 "stbl",
                //                 "stsd",
                //                 $stsdChildren["name"],
                //                 $encChildren["name"],
                //             ]);
                //         }
                //     }
                // }
            }

            if ($atom["name"] === "moof") {
                foreach ($atom["children"] as $moofChildren) {
                    // Retrieve all the PSSH's
                    if ($moofChildren["name"] === "pssh") {
                        $initContent = BentoHelper::removeAtom(
                            $initContent,
                            [
                                $atom["name"],
                                $moofChildren["name"],
                            ]
                        );
                    }
                }
            }
        }

        return $initContent;
    }
}
